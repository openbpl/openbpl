// Package rules implements CLI commands for managing TypeScript rules.
package rules

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openbpl/openbpl/internal/bridge"
	"github.com/openbpl/openbpl/internal/config"
)

const ruleTemplate = `import { defineRule } from "@openbpl/sdk";

export default defineRule({
  name: "%s",
  description: "TODO: describe what this rule detects",

  evaluate({ evidence, brand }) {
    // TODO: implement detection logic
    //
    // Available evidence:
    //   evidence.domain  - the captured domain
    //   evidence.html    - full page HTML
    //   evidence.title   - page <title>
    //   evidence.screenshot - base64 PNG screenshot
    //
    // Available brand info:
    //   brand.name, brand.website, brand.keywords, brand.images, etc.
    //
    // Return null for no detection, or a Label:
    //   { name: "rule-name", confidence: 0.0-1.0, detail: "explanation" }

    return null;
  },
});
`

// Command dispatches rule subcommands.
func Command(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: openbpl rule <new|list|test> [args...]")
	}

	switch args[0] {
	case "new":
		if len(args) < 2 {
			return fmt.Errorf("usage: openbpl rule new <rule-name>")
		}
		return newRule(args[1])
	case "list":
		return listRules()
	case "test":
		var ruleName string
		if len(args) > 1 {
			ruleName = args[1]
		}
		return testRules(ruleName)
	default:
		return fmt.Errorf("unknown rule command: %s", args[0])
	}
}

func newRule(name string) error {
	rulesDir := "rules"
	if _, err := os.Stat(rulesDir); os.IsNotExist(err) {
		return fmt.Errorf("rules/ directory not found — are you in a project directory?")
	}

	filename := name + ".ts"
	path := filepath.Join(rulesDir, filename)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("rule %q already exists at %s", name, path)
	}

	content := fmt.Sprintf(ruleTemplate, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write rule: %w", err)
	}

	fmt.Printf("Created rule: %s\n", path)
	fmt.Printf("Edit the file to implement your detection logic.\n")
	return nil
}

func listRules() error {
	rulesDir := "rules"
	runtimePath := filepath.Join(rulesDir, "node_modules", "@openbpl", "sdk", "dist", "runtime.js")

	b, err := bridge.Start(runtimePath, rulesDir)
	if err != nil {
		return fmt.Errorf("start rules engine: %w", err)
	}
	defer b.Stop()

	rules, err := b.List()
	if err != nil {
		return fmt.Errorf("list rules: %w", err)
	}

	if len(rules) == 0 {
		fmt.Println("No rules found in rules/")
		return nil
	}

	fmt.Printf("Rules (%d):\n", len(rules))
	for _, r := range rules {
		desc := r.Description
		if desc == "" {
			desc = "(no description)"
		}
		fmt.Printf("  %-20s %s\n", r.Name, desc)
	}
	return nil
}

func testRules(ruleName string) error {
	rulesDir := "rules"
	runtimePath := filepath.Join(rulesDir, "node_modules", "@openbpl", "sdk", "dist", "runtime.js")

	b, err := bridge.Start(runtimePath, rulesDir)
	if err != nil {
		return fmt.Errorf("start rules engine: %w", err)
	}
	defer b.Stop()

	cfg, err := config.Load("config.yaml")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Find capture directories in data/
	dataDir := "data"
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return fmt.Errorf("read data dir: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No captures found in data/ — run 'openbpl start' first to capture pages.")
		return nil
	}

	brand := bridge.Brand{
		Name:        cfg.Brand.Name,
		Website:     cfg.Brand.Website,
		Description: cfg.Brand.Description,
		Industry:    cfg.Brand.Industry,
		Keywords: bridge.Keywords{
			Included: cfg.Brand.Keywords.Included,
			Excluded: cfg.Brand.Keywords.Excluded,
		},
		Images: cfg.Brand.Images,
		Colors: cfg.Brand.Colors,
		URLs: bridge.URLs{
			Domains:           cfg.Brand.URLs.Domains,
			SocialMedia:       cfg.Brand.URLs.SocialMedia,
			AppStores:         cfg.Brand.URLs.AppStores,
			BrowserExtensions: cfg.Brand.URLs.BrowserExtensions,
			Blogs:             cfg.Brand.URLs.Blogs,
		},
	}

	tested := 0
	flagged := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(dataDir, entry.Name())

		// Find HTML file
		htmlFiles, _ := filepath.Glob(filepath.Join(dir, "*.html"))
		if len(htmlFiles) == 0 {
			continue
		}

		htmlData, err := os.ReadFile(htmlFiles[0])
		if err != nil {
			continue
		}

		// Domain from HTML filename
		domain := strings.TrimSuffix(filepath.Base(htmlFiles[0]), ".html")

		// Screenshot
		screenshotPath := filepath.Join(dir, domain+".png")
		var screenshotB64 string
		if data, err := os.ReadFile(screenshotPath); err == nil {
			screenshotB64 = base64.StdEncoding.EncodeToString(data)
		}

		params := bridge.EvaluateParams{
			Evidence: bridge.Evidence{
				Domain:         domain,
				HTML:           string(htmlData),
				Title:          extractTitle(string(htmlData)),
				ScreenshotPath: screenshotPath,
				Screenshot:     screenshotB64,
			},
			Brand: brand,
		}

		labels, err := b.Evaluate(params)
		if err != nil {
			fmt.Printf("  ERROR %s: %v\n", domain, err)
			continue
		}

		tested++
		if len(labels) > 0 {
			flagged++
			fmt.Printf("  FLAGGED %s\n", domain)
			for _, l := range labels {
				if ruleName != "" && l.Name != ruleName {
					continue
				}
				fmt.Printf("    [%.2f] %s: %s\n", l.Confidence, l.Name, l.Detail)
			}
		}
	}

	fmt.Printf("\nTested %d captures, %d flagged.\n", tested, flagged)
	return nil
}

func extractTitle(html string) string {
	lower := strings.ToLower(html)
	start := strings.Index(lower, "<title")
	if start == -1 {
		return ""
	}
	gt := strings.Index(lower[start:], ">")
	if gt == -1 {
		return ""
	}
	contentStart := start + gt + 1
	end := strings.Index(lower[contentStart:], "</title>")
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(html[contentStart : contentStart+end])
}

// Ensure labels JSON output works for test command
func init() {
	_ = json.Marshal
}
