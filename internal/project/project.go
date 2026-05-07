package project

import (
	"fmt"
	"os"
	"path/filepath"
)

const defaultConfig = `# OpenBPL Configuration
# Keywords to monitor in certificate transparency logs.
keywords:
  - coinbase
  - metamask
  - paypal
  - binance
  - kraken
  - ledger
  - trezor

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
func Create(name string) error {
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

	files := map[string]string{
		filepath.Join(name, "config.yaml"):  defaultConfig,
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
