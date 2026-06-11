package builder

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rlnorthcutt/cmdkit/logger"
)

// setupTestProject creates a minimal project structure for testing.
func setupTestProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// config.toml
	cfg := `
[site]
title    = "Test Site"
base_url = "https://test.example.com"
language = "en"
page_size = 10

[build]
draft_mode = false
output_dir = "public"

[author]
name  = "Tester"
email = "test@example.com"
`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	// Templates
	dirs := []string{
		filepath.Join(dir, "templates", "layouts"),
		filepath.Join(dir, "templates", "types"),
		filepath.Join(dir, "templates", "partials"),
		filepath.Join(dir, "content", "blog"),
		filepath.Join(dir, "static", "css"),
		filepath.Join(dir, "data"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// page.html template (pongo2 syntax)
	pageTmpl := `<html><head><title>{{ Page.Title }}</title></head><body>{{ Page.HTMLContent|safe }}</body></html>`
	if err := os.WriteFile(filepath.Join(dir, "templates", "types", "page.html"), []byte(pageTmpl), 0644); err != nil {
		t.Fatal(err)
	}

	// blog.html template
	blogTmpl := `<html><head><title>{{ Page.Title }} | Blog</title></head><body>{{ Page.HTMLContent|safe }}</body></html>`
	if err := os.WriteFile(filepath.Join(dir, "templates", "types", "blog.html"), []byte(blogTmpl), 0644); err != nil {
		t.Fatal(err)
	}

	// Content files
	aboutMD := "+++\ntitle = \"About\"\ndate = 2025-01-01\n+++\n\nAbout page."
	if err := os.WriteFile(filepath.Join(dir, "content", "about.md"), []byte(aboutMD), 0644); err != nil {
		t.Fatal(err)
	}

	blogMD := "+++\ntitle = \"Hello Blog\"\ndate = 2025-06-01\ntags = [\"golang\"]\n+++\n\nBlog content."
	if err := os.WriteFile(filepath.Join(dir, "content", "blog", "hello.md"), []byte(blogMD), 0644); err != nil {
		t.Fatal(err)
	}

	// Static file
	if err := os.WriteFile(filepath.Join(dir, "static", "css", "site.css"), []byte("body { color: red; }"), 0644); err != nil {
		t.Fatal(err)
	}

	// Data file
	navTOML := "[[items]]\nlabel = \"Home\"\nurl = \"/\"\n"
	if err := os.WriteFile(filepath.Join(dir, "data", "nav.toml"), []byte(navTOML), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestNew(t *testing.T) {
	dir := setupTestProject(t)

	log := logger.New(false)
	b, err := New(dir, log)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil builder")
	}
}

func TestBuild(t *testing.T) {
	dir := setupTestProject(t)
	log := logger.New(false)

	b, err := New(dir, log)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()
	if err := b.Build(ctx, BuildOptions{}); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Check output files exist
	expectedFiles := []string{
		filepath.Join(dir, "public", "about.html"),
		filepath.Join(dir, "public", "blog", "hello.html"),
		filepath.Join(dir, "public", "css", "site.css"),
	}
	for _, f := range expectedFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("expected output file %s to exist", f)
		}
	}

	// Check cache file was created
	if _, err := os.Stat(filepath.Join(dir, ".arbor-cache.json")); os.IsNotExist(err) {
		t.Error("expected .arbor-cache.json to exist after build")
	}
}

func TestBuildForce(t *testing.T) {
	dir := setupTestProject(t)
	log := logger.New(false)

	b, err := New(dir, log)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// First build
	if err := b.Build(ctx, BuildOptions{}); err != nil {
		t.Fatalf("first build failed: %v", err)
	}

	// Second build with force
	b2, err := New(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	if err := b2.Build(ctx, BuildOptions{Force: true}); err != nil {
		t.Fatalf("force build failed: %v", err)
	}

	// Output should still exist
	if _, err := os.Stat(filepath.Join(dir, "public", "about.html")); os.IsNotExist(err) {
		t.Error("about.html should exist after force rebuild")
	}
}

func TestBuildIncrementalCache(t *testing.T) {
	dir := setupTestProject(t)
	log := logger.New(false)

	ctx := context.Background()

	// First build
	b, err := New(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Build(ctx, BuildOptions{}); err != nil {
		t.Fatalf("first build failed: %v", err)
	}

	// Verify output exists
	outPath := filepath.Join(dir, "public", "about.html")
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Fatal("about.html should exist after first build")
	}

	// Second build without changes — should succeed
	b2, err := New(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	if err := b2.Build(ctx, BuildOptions{}); err != nil {
		t.Fatalf("second build failed: %v", err)
	}
}

func TestLoadData(t *testing.T) {
	dir := setupTestProject(t)
	log := logger.New(false)

	b, err := New(dir, log)
	if err != nil {
		t.Fatal(err)
	}

	data, err := b.loadData()
	if err != nil {
		t.Fatalf("loadData failed: %v", err)
	}

	if _, ok := data["nav"]; !ok {
		t.Error("expected 'nav' key in data")
	}
}

func TestBuildMissingConfig(t *testing.T) {
	dir := t.TempDir()
	log := logger.New(false)

	_, err := New(dir, log)
	if err == nil {
		t.Error("expected error when config.toml is missing")
	}
}
