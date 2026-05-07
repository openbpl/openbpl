package wizard

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// BrandData holds information fetched from brand lookup services.
type BrandData struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Colors      []string `json:"colors"`
	LogoURL     string   `json:"logo_url"`
	Industry    string   `json:"industry"`
}

// FetchBrand looks up brand information using publicly available sources.
// It tries the Brandfetch unofficial endpoint and falls back to favicon.
func FetchBrand(websiteURL string) (*BrandData, error) {
	parsed, err := url.Parse(websiteURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	domain := parsed.Hostname()

	brand, err := fetchBrandfetch(domain)
	if err != nil {
		// Return empty brand data on failure; the LLM will fill in gaps
		return &BrandData{}, nil
	}
	return brand, nil
}

func fetchBrandfetch(domain string) (*BrandData, error) {
	apiURL := fmt.Sprintf("https://api.brandfetch.io/v2/brands/%s", domain)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("brandfetch returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var raw struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Links       []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"links"`
		Logos []struct {
			Type   string `json:"type"`
			Theme  string `json:"theme"`
			Formats []struct {
				Src string `json:"src"`
			} `json:"formats"`
		} `json:"logos"`
		Colors []struct {
			Hex  string `json:"hex"`
			Type string `json:"type"`
		} `json:"colors"`
		Industry string `json:"industry"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse brandfetch response: %w", err)
	}

	brand := &BrandData{
		Name:        raw.Name,
		Description: raw.Description,
		Industry:    raw.Industry,
	}

	for _, c := range raw.Colors {
		hex := c.Hex
		if hex != "" && !strings.HasPrefix(hex, "#") {
			hex = "#" + hex
		}
		if hex != "" {
			brand.Colors = append(brand.Colors, hex)
		}
	}

	for _, logo := range raw.Logos {
		if logo.Type == "logo" && len(logo.Formats) > 0 {
			brand.LogoURL = logo.Formats[0].Src
			break
		}
	}

	return brand, nil
}
