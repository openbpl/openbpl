package project

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed templates/rules/*
var templateRules embed.FS

const defaultConfig = `# OpenBPL Configuration

# Brand details for phishing detection.
brand:
  name: ""
  website: ""
  description: ""
  industry: ""
  twitter: ""
  github: ""

  # Keywords used for detection.
  keywords:
    included:
      - example
    excluded: []

  # Brand images (paths on disk for favicon/logo matching).
  images: []

  # Brand colors (hex values).
  colors: []

  # URLs for web assets owned by the brand.
  urls:
    domains: []
    social_media: []
    app_stores: []
    browser_extensions: []
    blogs: []

# Detection source.
source: certstream
`

// Create scaffolds a new OpenBPL project directory.
// If configContent is non-empty, it's used instead of the default template.
func Create(name string, configContent string) error {
	if _, err := os.Stat(name); err == nil {
		return fmt.Errorf("directory %q already exists", name)
	}

	dirs := []string{
		name,
		filepath.Join(name, "data"),
		filepath.Join(name, "rules"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
	}

	cfg := defaultConfig
	if configContent != "" {
		cfg = configContent
	}

	files := map[string]string{
		filepath.Join(name, "config.yaml"): cfg,
		filepath.Join(name, "flagged.txt"): "",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	// Copy default rule templates into rules/
	if err := fs.WalkDir(templateRules, "templates/rules", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := templateRules.ReadFile(path)
		if err != nil {
			return err
		}
		dest := filepath.Join(name, "rules", d.Name())
		return os.WriteFile(dest, data, 0o644)
	}); err != nil {
		return fmt.Errorf("copy rule templates: %w", err)
	}

	// Write rules/package.json for the Node.js runtime
	pkgJSON := fmt.Sprintf(`{
  "name": "%s-rules",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "dependencies": {
    "@openbpl/sdk": "^0.1.0"
  }
}
`, name)
	if err := os.WriteFile(filepath.Join(name, "rules", "package.json"), []byte(pkgJSON), 0o644); err != nil {
		return fmt.Errorf("write rules/package.json: %w", err)
	}

	// Create empty detections.db by touching the file.
	dbPath := filepath.Join(name, "detections.db")
	f, err := os.Create(dbPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", dbPath, err)
	}
	f.Close()

	fmt.Printf("Created project: %s/\n", name)
	fmt.Printf("  config.yaml\n")
	fmt.Printf("  data/\n")
	fmt.Printf("  rules/\n")
	fmt.Printf("  detections.db\n")
	fmt.Printf("  flagged.txt\n")
	fmt.Printf("\nRun 'cd %s/rules && npm install' to install rule dependencies.\n", name)
	fmt.Printf("Then 'cd %s && openbpl start' to begin monitoring.\n", name)
	return nil
}
