package wizard

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// LLMResult holds synthesized brand information from an LLM.
type LLMResult struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Industry    string   `json:"industry"`
	Keywords    []string `json:"keywords"`
	Colors      []string `json:"colors"`
}

// SynthesizeWithLLM calls out to the claude CLI to fill in missing brand fields.
func SynthesizeWithLLM(siteData *SiteData, brandData *BrandData, websiteURL string) (*LLMResult, error) {
	prompt := buildPrompt(siteData, brandData, websiteURL)

	result, err := callClaude(prompt)
	if err != nil {
		// Try opencode as fallback
		result, err = callOpencode(prompt)
		if err != nil {
			return nil, fmt.Errorf("LLM synthesis failed: %w", err)
		}
	}

	var llmResult LLMResult
	// Extract JSON from response (may have surrounding text)
	jsonStr := extractJSON(result)
	if err := json.Unmarshal([]byte(jsonStr), &llmResult); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w (raw: %s)", err, result)
	}

	return &llmResult, nil
}

func buildPrompt(siteData *SiteData, brandData *BrandData, websiteURL string) string {
	var sb strings.Builder
	sb.WriteString("You are helping configure a brand phishing detection tool. Based on the following information about a brand's website, provide the missing fields.\n\n")
	sb.WriteString(fmt.Sprintf("Website URL: %s\n", websiteURL))

	if siteData != nil {
		sb.WriteString(fmt.Sprintf("Page Title: %s\n", siteData.Title))
		sb.WriteString(fmt.Sprintf("Meta Description: %s\n", siteData.Description))
		if len(siteData.Colors) > 0 {
			sb.WriteString(fmt.Sprintf("Colors found on site: %s\n", strings.Join(siteData.Colors, ", ")))
		}
		if len(siteData.SocialLinks) > 0 {
			sb.WriteString(fmt.Sprintf("Social links: %s\n", strings.Join(siteData.SocialLinks, ", ")))
		}
	}

	if brandData != nil {
		if brandData.Name != "" {
			sb.WriteString(fmt.Sprintf("Brand name (from lookup): %s\n", brandData.Name))
		}
		if brandData.Description != "" {
			sb.WriteString(fmt.Sprintf("Brand description (from lookup): %s\n", brandData.Description))
		}
		if brandData.Industry != "" {
			sb.WriteString(fmt.Sprintf("Industry (from lookup): %s\n", brandData.Industry))
		}
		if len(brandData.Colors) > 0 {
			sb.WriteString(fmt.Sprintf("Brand colors (from lookup): %s\n", strings.Join(brandData.Colors, ", ")))
		}
	}

	sb.WriteString(`
Respond with ONLY a JSON object (no markdown, no explanation) with these fields:
- "name": the brand name (short, official name)
- "description": a one-sentence description of what the brand/company does
- "industry": the industry category (e.g., "fintech", "social media", "e-commerce")
- "keywords": an array of 2-5 lowercase keywords that attackers might use in phishing domains (the brand name, common misspellings, abbreviations)
- "colors": an array of 3-6 hex color codes that represent the brand (e.g., ["#003087", "#009cde"])

JSON:`)

	return sb.String()
}

func callClaude(prompt string) (string, error) {
	cmd := exec.Command("claude", "--print", "--max-turns", "1", prompt)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func callOpencode(prompt string) (string, error) {
	cmd := exec.Command("opencode", "ask", prompt)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("opencode: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func extractJSON(s string) string {
	// Find the first { and last }
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
