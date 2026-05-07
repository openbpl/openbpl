// Package sdk embeds the @openbpl/sdk npm package and provides
// a function to extract it into a project's node_modules.
package sdk

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed dist/*
var distFS embed.FS

//go:embed package.json
var packageJSON []byte

// Ensure extracts the embedded SDK into <projectRoot>/node_modules/@openbpl/sdk/
// if it doesn't already exist or is outdated.
func Ensure(projectRoot string) error {
	sdkDir := filepath.Join(projectRoot, "node_modules", "@openbpl", "sdk")
	distDir := filepath.Join(sdkDir, "dist")

	// Check if already extracted by looking for runtime.js
	marker := filepath.Join(distDir, "runtime.js")
	if _, err := os.Stat(marker); err == nil {
		return nil // already present
	}

	// Create directories
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		return fmt.Errorf("create sdk dir: %w", err)
	}

	// Write package.json
	if err := os.WriteFile(filepath.Join(sdkDir, "package.json"), packageJSON, 0o644); err != nil {
		return fmt.Errorf("write sdk package.json: %w", err)
	}

	// Write dist files
	if err := fs.WalkDir(distFS, "dist", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := distFS.ReadFile(path)
		if err != nil {
			return err
		}
		dest := filepath.Join(sdkDir, path)
		return os.WriteFile(dest, data, 0o644)
	}); err != nil {
		return fmt.Errorf("extract sdk: %w", err)
	}

	return nil
}
