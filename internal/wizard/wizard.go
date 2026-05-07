package wizard

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ImageDownload holds a downloaded image ready to be written to the project.
type ImageDownload struct {
	Name string // filename without extension (e.g., "favicon")
	Data []byte // PNG-encoded image data
}

// Result holds the final assembled brand configuration from the wizard.
type Result struct {
	Name        string
	Website     string
	Description string
	Industry    string
	Keywords    []string
	Colors      []string
	Domains     []string
	SocialMedia []string
	LogoURL     string
	FaviconURL  string
	Images      []ImageDownload // downloaded brand images to write into project
}

// Run executes the interactive setup wizard, prompting the user for their
// website URL and then automatically extracting brand information.
func Run() (*Result, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter your brand's website URL: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	websiteURL := strings.TrimSpace(input)

	// Normalize URL
	if !strings.HasPrefix(websiteURL, "http://") && !strings.HasPrefix(websiteURL, "https://") {
		websiteURL = "https://" + websiteURL
	}

	parsed, err := url.Parse(websiteURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	fmt.Printf("\nFetching %s...\n", websiteURL)

	// Step 1: Fetch website with Playwright
	fmt.Print("  Extracting site data with browser... ")
	siteData, err := FetchSite(websiteURL)
	if err != nil {
		fmt.Printf("warning: %v\n", err)
		siteData = &SiteData{}
	} else {
		fmt.Println("done")
	}

	// Step 2: Look up brand info
	fmt.Print("  Looking up brand information... ")
	brandData, err := FetchBrand(websiteURL)
	if err != nil {
		fmt.Printf("warning: %v\n", err)
		brandData = &BrandData{}
	} else {
		fmt.Println("done")
	}

	// Step 3: Use LLM to synthesize missing fields
	fmt.Print("  Synthesizing brand profile with AI... ")
	llmResult, err := SynthesizeWithLLM(siteData, brandData, websiteURL)
	if err != nil {
		fmt.Printf("warning: %v\n", err)
		llmResult = &LLMResult{}
	} else {
		fmt.Println("done")
	}

	// Assemble the result, preferring more specific sources
	result := &Result{
		Website: websiteURL,
	}

	// Name: prefer brandfetch > LLM > site title
	result.Name = coalesce(brandData.Name, llmResult.Name, siteData.Title)

	// Description: prefer brandfetch > LLM > site meta
	result.Description = coalesce(brandData.Description, llmResult.Description, siteData.Description)

	// Industry: prefer brandfetch > LLM
	result.Industry = coalesce(brandData.Industry, llmResult.Industry)

	// Keywords: prefer LLM (it generates phishing-relevant keywords)
	if len(llmResult.Keywords) > 0 {
		result.Keywords = llmResult.Keywords
	} else {
		// Fallback: use domain name as keyword
		result.Keywords = []string{strings.TrimPrefix(parsed.Hostname(), "www.")}
	}

	// Colors: merge brandfetch + site + LLM, deduplicate
	result.Colors = dedup(append(append(brandData.Colors, siteData.Colors...), llmResult.Colors...))

	// Domains: the main domain + any related domains from footer
	result.Domains = extractDomains(parsed.Hostname(), siteData.FooterLinks, siteData.SitemapURLs)

	// Social media
	result.SocialMedia = dedup(siteData.SocialLinks)

	// Logo/Favicon
	result.LogoURL = coalesce(brandData.LogoURL, siteData.LogoURL)
	result.FaviconURL = siteData.FaviconURL

	// Download favicon image via Google's favicon API
	fmt.Print("  Downloading favicon image... ")
	pngData, err := fetchGoogleFavicon(parsed.Hostname())
	if err != nil {
		fmt.Printf("warning: %v\n", err)
	} else {
		fmt.Println("done")
		result.Images = append(result.Images, ImageDownload{Name: "favicon", Data: pngData})
	}

	// Print summary
	fmt.Println("\n--- Brand Profile ---")
	fmt.Printf("  Name:        %s\n", result.Name)
	fmt.Printf("  Website:     %s\n", result.Website)
	fmt.Printf("  Description: %s\n", result.Description)
	fmt.Printf("  Industry:    %s\n", result.Industry)
	fmt.Printf("  Keywords:    %s\n", strings.Join(result.Keywords, ", "))
	if len(result.Colors) > 0 {
		fmt.Printf("  Colors:      %s\n", strings.Join(result.Colors, ", "))
	}
	if len(result.Domains) > 0 {
		fmt.Printf("  Domains:     %s\n", strings.Join(result.Domains, ", "))
	}
	if len(result.SocialMedia) > 0 {
		fmt.Printf("  Social:      %s\n", strings.Join(result.SocialMedia, ", "))
	}
	fmt.Println()

	return result, nil
}

// GenerateConfig produces a config.yaml string from wizard results.
func GenerateConfig(r *Result) string {
	var sb strings.Builder
	sb.WriteString("# OpenBPL Configuration\n\n")
	sb.WriteString("# Brand details for phishing detection.\n")
	sb.WriteString("brand:\n")
	sb.WriteString(fmt.Sprintf("  name: %q\n", r.Name))
	sb.WriteString(fmt.Sprintf("  website: %q\n", r.Website))
	sb.WriteString(fmt.Sprintf("  description: %q\n", r.Description))
	sb.WriteString(fmt.Sprintf("  industry: %q\n", r.Industry))
	sb.WriteString("  twitter: \"\"\n")
	sb.WriteString("  github: \"\"\n")
	sb.WriteString("\n")
	sb.WriteString("  # Keywords used for detection.\n")
	sb.WriteString("  keywords:\n")
	sb.WriteString("    included:\n")
	for _, kw := range r.Keywords {
		sb.WriteString(fmt.Sprintf("      - %s\n", kw))
	}
	sb.WriteString("    excluded: []\n")
	sb.WriteString("\n")
	sb.WriteString("  # Brand images (paths on disk for favicon/logo matching).\n")
	if len(r.Images) > 0 {
		sb.WriteString("  images:\n")
		for _, img := range r.Images {
			sb.WriteString(fmt.Sprintf("    - images/%s.png\n", img.Name))
		}
	} else {
		sb.WriteString("  images: []\n")
	}
	sb.WriteString("\n")
	sb.WriteString("  # Brand colors (hex values).\n")
	sb.WriteString("  colors:\n")
	if len(r.Colors) > 0 {
		for _, c := range r.Colors {
			sb.WriteString(fmt.Sprintf("    - %q\n", c))
		}
	} else {
		sb.WriteString("    []\n")
	}
	sb.WriteString("\n")
	sb.WriteString("  # URLs for web assets owned by the brand.\n")
	sb.WriteString("  urls:\n")
	sb.WriteString("    domains:\n")
	if len(r.Domains) > 0 {
		for _, d := range r.Domains {
			sb.WriteString(fmt.Sprintf("      - %s\n", d))
		}
	} else {
		sb.WriteString("      []\n")
	}
	sb.WriteString("    social_media:\n")
	if len(r.SocialMedia) > 0 {
		for _, s := range r.SocialMedia {
			sb.WriteString(fmt.Sprintf("      - %s\n", s))
		}
	} else {
		sb.WriteString("      []\n")
	}
	sb.WriteString("    app_stores: []\n")
	sb.WriteString("    browser_extensions: []\n")
	sb.WriteString("    blogs: []\n")
	sb.WriteString("\n")
	sb.WriteString("# Detection source.\n")
	sb.WriteString("source: certstream\n")
	sb.WriteString("\n")
	sb.WriteString("# Rules configuration.\n")
	sb.WriteString("rules:\n")
	sb.WriteString("  favicon_match:\n")
	sb.WriteString("    enabled: true\n")
	sb.WriteString("    threshold: 5\n")
	sb.WriteString("  login_form:\n")
	sb.WriteString("    enabled: true\n")

	return sb.String()
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func dedup(items []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, item := range items {
		lower := strings.ToLower(strings.TrimSpace(item))
		if lower != "" && !seen[lower] {
			seen[lower] = true
			out = append(out, item)
		}
	}
	return out
}

func extractDomains(primaryDomain string, footerLinks, sitemapURLs []string) []string {
	domains := make(map[string]bool)
	domains[primaryDomain] = true

	// Extract unique domains from footer links
	for _, link := range footerLinks {
		parsed, err := url.Parse(link)
		if err != nil {
			continue
		}
		host := parsed.Hostname()
		if host == "" {
			continue
		}
		// Only include domains that seem related (share a word with primary)
		primaryBase := strings.TrimPrefix(primaryDomain, "www.")
		primaryParts := strings.Split(strings.Split(primaryBase, ".")[0], "-")
		for _, part := range primaryParts {
			if len(part) > 2 && strings.Contains(host, part) {
				domains[host] = true
				break
			}
		}
	}

	// Extract domains from sitemap
	for _, u := range sitemapURLs {
		parsed, err := url.Parse(u)
		if err != nil {
			continue
		}
		host := parsed.Hostname()
		if host != "" {
			domains[host] = true
		}
	}

	var out []string
	for d := range domains {
		out = append(out, d)
	}
	return out
}

// fetchGoogleFavicon downloads a favicon PNG from Google's favicon service.
func fetchGoogleFavicon(domain string) ([]byte, error) {
	apiURL := fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=128", url.QueryEscape(domain))
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetch favicon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch favicon: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read favicon: %w", err)
	}

	return data, nil
}
