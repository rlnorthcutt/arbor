package assets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	toml := `
[[bundle]]
name = "css/bundle.css"
type = "css"
sources = ["static/css/a.css", "static/css/b.css"]

[[bundle]]
name = "js/bundle.js"
type = "js"
sources = ["static/js/app.js"]
`
	path := filepath.Join(dir, "assets.toml")
	if err := os.WriteFile(path, []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(m.Bundles) != 2 {
		t.Fatalf("expected 2 bundles, got %d", len(m.Bundles))
	}
	if m.Bundles[0].Name != "css/bundle.css" {
		t.Errorf("bundle[0].Name = %q, want css/bundle.css", m.Bundles[0].Name)
	}
	if m.Bundles[0].Type != "css" {
		t.Errorf("bundle[0].Type = %q, want css", m.Bundles[0].Type)
	}
	if len(m.Bundles[0].Sources) != 2 {
		t.Errorf("bundle[0].Sources len = %d, want 2", len(m.Bundles[0].Sources))
	}
}

func TestLoadManifest_Missing(t *testing.T) {
	_, err := LoadManifest("/nonexistent/assets.toml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestDefaultBundles(t *testing.T) {
	dir := t.TempDir()
	cssDir := filepath.Join(dir, "css")
	jsDir := filepath.Join(dir, "js")
	os.MkdirAll(cssDir, 0755)
	os.MkdirAll(jsDir, 0755)

	os.WriteFile(filepath.Join(cssDir, "b.css"), []byte("b{}"), 0644)
	os.WriteFile(filepath.Join(cssDir, "a.css"), []byte("a{}"), 0644)
	os.WriteFile(filepath.Join(jsDir, "app.js"), []byte("var x=1;"), 0644)

	bundles, err := DefaultBundles(dir)
	if err != nil {
		t.Fatalf("DefaultBundles: %v", err)
	}
	if len(bundles) != 2 {
		t.Fatalf("expected 2 bundles, got %d", len(bundles))
	}

	css := bundles[0]
	if css.Type != "css" {
		t.Errorf("bundle[0].Type = %q, want css", css.Type)
	}
	if css.Name != "css/bundle.css" {
		t.Errorf("bundle[0].Name = %q, want css/bundle.css", css.Name)
	}
	if len(css.Sources) != 2 {
		t.Errorf("css sources len = %d, want 2", len(css.Sources))
	}
	// Sources must be sorted alphabetically.
	if filepath.Base(css.Sources[0]) != "a.css" {
		t.Errorf("first CSS source = %q, want a.css", filepath.Base(css.Sources[0]))
	}

	js := bundles[1]
	if js.Type != "js" {
		t.Errorf("bundle[1].Type = %q, want js", js.Type)
	}
}

func TestDefaultBundles_Empty(t *testing.T) {
	dir := t.TempDir()
	bundles, err := DefaultBundles(dir)
	if err != nil {
		t.Fatalf("DefaultBundles on empty dir: %v", err)
	}
	if len(bundles) != 0 {
		t.Errorf("expected 0 bundles for empty static dir, got %d", len(bundles))
	}
}

func TestListStaticAssets(t *testing.T) {
	dir := t.TempDir()
	cssDir := filepath.Join(dir, "css")
	jsDir := filepath.Join(dir, "js")
	os.MkdirAll(cssDir, 0755)
	os.MkdirAll(jsDir, 0755)

	os.WriteFile(filepath.Join(cssDir, "z.css"), []byte("z{}"), 0644)
	os.WriteFile(filepath.Join(cssDir, "a.css"), []byte("a{}"), 0644)
	os.WriteFile(filepath.Join(jsDir, "app.js"), []byte("var x;"), 0644)
	os.WriteFile(filepath.Join(jsDir, "main.js"), []byte("var y;"), 0644)

	css, js := ListStaticAssets(dir)

	if len(css) != 2 {
		t.Fatalf("css files len = %d, want 2", len(css))
	}
	if css[0] != "css/a.css" {
		t.Errorf("css[0] = %q, want css/a.css", css[0])
	}
	if len(js) != 2 {
		t.Fatalf("js files len = %d, want 2", len(js))
	}
	if js[0] != "js/app.js" {
		t.Errorf("js[0] = %q, want js/app.js", js[0])
	}
}
