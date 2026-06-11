package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml"
)

// Config is the top-level configuration for an Arbor project.
type Config struct {
	Site   SiteConfig   `toml:"site"`
	Build  BuildConfig  `toml:"build"`
	Author AuthorConfig `toml:"author"`
}

// SiteConfig holds site-wide metadata.
type SiteConfig struct {
	Title    string `toml:"title"`
	BaseURL  string `toml:"base_url"`
	Language string `toml:"language"`
	PageSize int    `toml:"page_size"`
}

// BuildConfig controls build behavior.
type BuildConfig struct {
	DraftMode bool   `toml:"draft_mode"`
	OutputDir string `toml:"output_dir"`
}

// AuthorConfig holds author information.
type AuthorConfig struct {
	Name  string `toml:"name"`
	Email string `toml:"email"`
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		Site: SiteConfig{
			Title:    "My Arbor Site",
			BaseURL:  "https://example.com",
			Language: "en",
			PageSize: 10,
		},
		Build: BuildConfig{
			DraftMode: false,
			OutputDir: "public",
		},
		Author: AuthorConfig{
			Name:  "Your Name",
			Email: "you@example.com",
		},
	}
}

// Load reads config.toml from rootDir and returns the parsed Config.
// Missing optional fields use default values.
func Load(rootDir string) (*Config, error) {
	cfgPath := filepath.Join(rootDir, "config.toml")

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("reading config.toml: %w", err)
	}

	cfg := Default()
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config.toml: %w", err)
	}

	// Apply defaults for zero values
	if cfg.Site.Language == "" {
		cfg.Site.Language = "en"
	}
	if cfg.Site.PageSize == 0 {
		cfg.Site.PageSize = 10
	}
	if cfg.Build.OutputDir == "" {
		cfg.Build.OutputDir = "public"
	}

	return cfg, nil
}
