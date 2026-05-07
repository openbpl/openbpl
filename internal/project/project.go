package project

import (
	"fmt"
	"os"
	"path/filepath"
)

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

# Rules configuration.
rules:
  favicon_match:
    enabled: true
    threshold: 5
  login_form:
    enabled: true
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
		filepath.Join(name, "config.yaml"):  cfg,
		filepath.Join(name, "flagged.txt"):  "",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	// Create empty detections.db by touching the file.
	dbPath := filepath.Join(name, "detections.db")
	f, err := os.Create(dbPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", dbPath, err)
	}
	f.Close()

	fmt.Printf("Created project: %s/\n", name)
	fmt.Printf("  data/\n")
	fmt.Printf("  config.yaml\n")
	fmt.Printf("  detections.db\n")
	fmt.Printf("  flagged.txt\n")
	fmt.Printf("\nRun 'cd %s && openbpl start' to begin monitoring.\n", name)
	return nil
}
