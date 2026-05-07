package wizard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// SiteData holds information extracted from a website via Playwright.
type SiteData struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	FaviconURL  string   `json:"favicon_url"`
	Colors      []string `json:"colors"`
	FooterLinks []string `json:"footer_links"`
	SocialLinks []string `json:"social_links"`
	SitemapURLs []string `json:"sitemap_urls"`
	LogoURL     string   `json:"logo_url"`
}

// FetchSite uses Playwright to load a website and extract brand-relevant data.
func FetchSite(url string) (*SiteData, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("start playwright: %w", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("launch browser: %w", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		return nil, fmt.Errorf("new page: %w", err)
	}

	if _, err := page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return nil, fmt.Errorf("navigate to %s: %w", url, err)
	}

	// Extract page data using JavaScript
	result, err := page.Evaluate(extractScript)
	if err != nil {
		return nil, fmt.Errorf("extract page data: %w", err)
	}

	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}

	var site SiteData
	if err := json.Unmarshal(data, &site); err != nil {
		return nil, fmt.Errorf("unmarshal site data: %w", err)
	}

	// Fetch sitemap
	sitemapURLs := fetchSitemap(page, url)
	site.SitemapURLs = sitemapURLs

	return &site, nil
}

func fetchSitemap(page playwright.Page, baseURL string) []string {
	sitemapURL := strings.TrimRight(baseURL, "/") + "/sitemap.xml"
	resp, err := page.Goto(sitemapURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil || resp.Status() != 200 {
		return nil
	}

	result, err := page.Evaluate(`() => {
		const locs = document.querySelectorAll('loc');
		return Array.from(locs).map(l => l.textContent).filter(Boolean).slice(0, 50);
	}`)
	if err != nil {
		return nil
	}

	urls, ok := result.([]interface{})
	if !ok {
		return nil
	}

	var out []string
	for _, u := range urls {
		if s, ok := u.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

const extractScript = `() => {
	// Title
	const title = document.title || '';

	// Meta description
	const metaDesc = document.querySelector('meta[name="description"]');
	const description = metaDesc ? metaDesc.getAttribute('content') || '' : '';

	// Favicon
	const favicon = document.querySelector('link[rel*="icon"]');
	const faviconURL = favicon ? favicon.getAttribute('href') || '' : '';

	// Logo - look for common logo patterns
	const logoSelectors = [
		'img[class*="logo"]', 'img[id*="logo"]', 'img[alt*="logo"]',
		'.logo img', '#logo img', '[class*="brand"] img',
		'header img:first-of-type'
	];
	let logoURL = '';
	for (const sel of logoSelectors) {
		const el = document.querySelector(sel);
		if (el && el.src) { logoURL = el.src; break; }
	}

	// Colors - extract from CSS custom properties and computed styles
	const colors = new Set();
	const root = getComputedStyle(document.documentElement);
	const props = ['--primary-color', '--brand-color', '--main-color', '--accent-color',
		'--color-primary', '--color-brand', '--theme-color'];
	for (const p of props) {
		const v = root.getPropertyValue(p).trim();
		if (v && v.startsWith('#')) colors.add(v);
	}
	// Theme color meta tag
	const themeColor = document.querySelector('meta[name="theme-color"]');
	if (themeColor) {
		const c = themeColor.getAttribute('content');
		if (c) colors.add(c);
	}
	// MS tile color
	const msColor = document.querySelector('meta[name="msapplication-TileColor"]');
	if (msColor) {
		const c = msColor.getAttribute('content');
		if (c) colors.add(c);
	}

	// Footer links
	const footer = document.querySelector('footer') || document.querySelector('[class*="footer"]');
	const footerLinks = [];
	if (footer) {
		const links = footer.querySelectorAll('a[href]');
		links.forEach(a => {
			const href = a.href;
			if (href && !href.startsWith('javascript:') && !href.startsWith('#')) {
				footerLinks.push(href);
			}
		});
	}

	// Social media links
	const socialPatterns = ['twitter.com', 'x.com', 'facebook.com', 'instagram.com',
		'linkedin.com', 'youtube.com', 'github.com', 'tiktok.com'];
	const socialLinks = [];
	document.querySelectorAll('a[href]').forEach(a => {
		const href = a.href;
		if (socialPatterns.some(p => href.includes(p))) {
			socialLinks.push(href);
		}
	});

	return {
		title,
		description,
		favicon_url: faviconURL,
		logo_url: logoURL,
		colors: Array.from(colors),
		footer_links: [...new Set(footerLinks)].slice(0, 100),
		social_links: [...new Set(socialLinks)].slice(0, 20),
	};
}`
