package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Brand holds all brand-related identity information.
type Brand struct {
	Name        string   `yaml:"name"`
	Website     string   `yaml:"website"`
	Description string   `yaml:"description"`
	Industry    string   `yaml:"industry"`
	Twitter     string   `yaml:"twitter"`
	GitHub      string   `yaml:"github"`
	Keywords    Keywords `yaml:"keywords"`
	Images      []string `yaml:"images"`
	Colors      []string `yaml:"colors"`
	URLs        URLs     `yaml:"urls"`
}

// Keywords specifies included and excluded terms for detection.
type Keywords struct {
	Included []string `yaml:"included"`
	Excluded []string `yaml:"excluded"`
}

// URLs holds the various web asset URLs owned by the brand.
type URLs struct {
	Domains           []string `yaml:"domains"`
	SocialMedia       []string `yaml:"social_media"`
	AppStores         []string `yaml:"app_stores"`
	BrowserExtensions []string `yaml:"browser_extensions"`
	Blogs             []string `yaml:"blogs"`
}

// Rules holds rule-specific configuration.
type Rules struct {
	FaviconMatch *RuleConfig `yaml:"favicon_match"`
	LoginForm    *RuleConfig `yaml:"login_form"`
}

// RuleConfig is generic on/off + threshold for a rule.
type RuleConfig struct {
	Enabled   bool `yaml:"enabled"`
	Threshold int  `yaml:"threshold,omitempty"`
}

// Config is the top-level configuration structure.
type Config struct {
	Brand  Brand  `yaml:"brand"`
	Source string `yaml:"source"`
	Rules  Rules  `yaml:"rules"`
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if len(cfg.Brand.Keywords.Included) == 0 {
		return nil, fmt.Errorf("config: brand.keywords.included must not be empty")
	}

	return &cfg, nil
}
