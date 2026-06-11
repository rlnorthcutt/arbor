package blueprints

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rlnorthcutt/cmdkit/logger"
)

func TestInit_ValidBlueprints(t *testing.T) {
	for _, bp := range []string{"blog", "marketing", "docs"} {
		t.Run(bp, func(t *testing.T) {
			dir := t.TempDir()
			log := logger.New(false)

			if err := Init(dir, bp, log); err != nil {
				t.Fatalf("Init(%q) failed: %v", bp, err)
			}

			// Every blueprint must produce these shared files
			required := []string{
				"config.toml",
				"static/css/ivy.full.css",
				"static/css/lattice.full.css",
				"static/js/dark-mode-toggle.js",
				"templates/layouts/base.html",
				"templates/partials/header.html",
				"templates/partials/footer.html",
				"templates/types/page.html",
			}
			for _, rel := range required {
				if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
					t.Errorf("Init(%q): missing expected file %s", bp, rel)
				}
			}
		})
	}
}

func TestInit_InvalidBlueprint(t *testing.T) {
	dir := t.TempDir()
	log := logger.New(false)

	err := Init(dir, "nonexistent", log)
	if err == nil {
		t.Error("expected error for unknown blueprint, got nil")
	}
}

func TestInit_SkipsExistingFiles(t *testing.T) {
	dir := t.TempDir()
	log := logger.New(false)

	// Pre-create config.toml with custom content
	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte("[site]\ntitle=\"My Custom Site\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Init(dir, "blog", log); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// The pre-existing config.toml must not be overwritten
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "[site]\ntitle=\"My Custom Site\"\n" {
		t.Error("Init overwrote an existing file — it should skip existing files")
	}
}

func TestInit_BlogHasContentFiles(t *testing.T) {
	dir := t.TempDir()
	log := logger.New(false)

	if err := Init(dir, "blog", log); err != nil {
		t.Fatalf("Init(blog) failed: %v", err)
	}

	blogFiles := []string{
		"content/blog/hello-world.md",
		"content/blog/getting-started.md",
		"content/about.md",
		"data/nav.toml",
		"templates/types/blog.html",
		"templates/types/home.html",
		"templates/types/listing.html",
		"templates/displays/card.html",
	}
	for _, rel := range blogFiles {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("Init(blog): missing expected file %s", rel)
		}
	}
}

func TestInit_DocsHasSidebarLayout(t *testing.T) {
	dir := t.TempDir()
	log := logger.New(false)

	if err := Init(dir, "docs", log); err != nil {
		t.Fatalf("Init(docs) failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "templates/layouts/docs.html")); err != nil {
		t.Error("Init(docs): missing sidebar layout templates/layouts/docs.html")
	}
}
