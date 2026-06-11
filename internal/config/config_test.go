package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Site.PageSize != 10 {
		t.Errorf("expected default PageSize=10, got %d", cfg.Site.PageSize)
	}
	if cfg.Site.Language != "en" {
		t.Errorf("expected default Language=en, got %s", cfg.Site.Language)
	}
	if cfg.Build.OutputDir != "public" {
		t.Errorf("expected default OutputDir=public, got %s", cfg.Build.OutputDir)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `
[site]
title    = "Test Site"
base_url = "https://test.example.com"
language = "fr"
page_size = 5

[build]
draft_mode = true
output_dir = "dist"

[author]
name  = "Test Author"
email = "test@example.com"
`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Site.Title != "Test Site" {
		t.Errorf("expected Title='Test Site', got '%s'", cfg.Site.Title)
	}
	if cfg.Site.BaseURL != "https://test.example.com" {
		t.Errorf("expected BaseURL='https://test.example.com', got '%s'", cfg.Site.BaseURL)
	}
	if cfg.Site.Language != "fr" {
		t.Errorf("expected Language='fr', got '%s'", cfg.Site.Language)
	}
	if cfg.Site.PageSize != 5 {
		t.Errorf("expected PageSize=5, got %d", cfg.Site.PageSize)
	}
	if !cfg.Build.DraftMode {
		t.Error("expected DraftMode=true")
	}
	if cfg.Build.OutputDir != "dist" {
		t.Errorf("expected OutputDir='dist', got '%s'", cfg.Build.OutputDir)
	}
	if cfg.Author.Name != "Test Author" {
		t.Errorf("expected Author.Name='Test Author', got '%s'", cfg.Author.Name)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path")
	if err == nil {
		t.Error("expected error for missing config.toml")
	}
}

func TestLoadDefaults_PartialConfig(t *testing.T) {
	dir := t.TempDir()
	// Only set title — other fields should use defaults
	cfgContent := `
[site]
title = "Minimal Site"
`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Site.PageSize != 10 {
		t.Errorf("expected default PageSize=10, got %d", cfg.Site.PageSize)
	}
	if cfg.Site.Language != "en" {
		t.Errorf("expected default Language=en, got '%s'", cfg.Site.Language)
	}
	if cfg.Build.OutputDir != "public" {
		t.Errorf("expected default OutputDir=public, got '%s'", cfg.Build.OutputDir)
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("not valid toml ]["), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}
