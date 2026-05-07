package rule

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/corona10/goimagehash"
)

var faviconLinkRe = regexp.MustCompile(`(?i)<link[^>]+rel=["'](?:shortcut )?icon["'][^>]*>`)
var hrefRe = regexp.MustCompile(`(?i)href=["']([^"']+)["']`)

type refIcon struct {
	name string
	hash *goimagehash.ImageHash
}

// FaviconMatch compares a captured page's favicon against a folder of
// reference brand icons using perceptual hashing.
type FaviconMatch struct {
	refs      []refIcon
	threshold int // max hamming distance to consider a match
}

// NewFaviconMatch loads specific image files and pre-computes their
// perceptual hashes. threshold is the max hamming distance (e.g. 10).
func NewFaviconMatch(paths []string, threshold int) (*FaviconMatch, error) {
	var refs []refIcon
	for _, path := range paths {
		h, err := hashFile(path)
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", path, err)
		}
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		refs = append(refs, refIcon{name: name, hash: h})
	}
	return &FaviconMatch{refs: refs, threshold: threshold}, nil
}

func (f *FaviconMatch) Name() string { return "favicon-match" }

func (f *FaviconMatch) Evaluate(ev Evidence) ([]Label, error) {
	if len(f.refs) == 0 {
		return nil, nil
	}

	faviconURL := extractFaviconURL(ev.HTML, ev.Domain)
	if faviconURL == "" {
		return nil, nil
	}

	data, err := fetchFavicon(faviconURL)
	if err != nil {
		return nil, nil
	}
	if len(data) < 100 {
		return nil, nil
	}

	hash, err := hashBytes(data)
	if err != nil {
		return nil, nil
	}

	var labels []Label
	for _, ref := range f.refs {
		dist, err := hash.Distance(ref.hash)
		if err != nil {
			continue
		}
		if dist <= f.threshold {
			confidence := 1.0 - float64(dist)/float64(f.threshold+1)
			labels = append(labels, Label{
				Rule:       f.Name(),
				Name:       "favicon-match",
				Confidence: confidence,
				Detail:     fmt.Sprintf("matches %s (hamming=%d)", ref.name, dist),
			})
		}
	}
	return labels, nil
}

func extractFaviconURL(html, domain string) string {
	m := faviconLinkRe.FindString(html)
	if m != "" {
		href := hrefRe.FindStringSubmatch(m)
		if len(href) > 1 {
			u := href[1]
			if strings.HasPrefix(u, "//") {
				return "https:" + u
			}
			if strings.HasPrefix(u, "/") {
				return "https://" + domain + u
			}
			if strings.HasPrefix(u, "http") {
				return u
			}
			return "https://" + domain + "/" + u
		}
	}
	return "https://" + domain + "/favicon.ico"
}

var httpClient = &http.Client{Timeout: 5 * time.Second}

func fetchFavicon(url string) ([]byte, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB cap
}

func hashFile(path string) (*goimagehash.ImageHash, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return goimagehash.PerceptionHash(img)
}

func hashBytes(data []byte) (*goimagehash.ImageHash, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return goimagehash.PerceptionHash(img)
}
