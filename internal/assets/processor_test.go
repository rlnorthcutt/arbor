package assets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

func TestProcessBundles_Aggregate(t *testing.T) {
	src := t.TempDir()
	out := t.TempDir()

	writeFile(t, filepath.Join(src, "a.css"), "body { color: red; }")
	writeFile(t, filepath.Join(src, "b.css"), "p { margin: 0; }")

	bundles := []BundleConfig{
		{
			Name:    "css/bundle.css",
			Type:    "css",
			Sources: []string{filepath.Join(src, "a.css"), filepath.Join(src, "b.css")},
		},
	}

	proc := NewProcessor()
	result, err := proc.ProcessBundles(bundles, out, false)
	if err != nil {
		t.Fatalf("ProcessBundles: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}

	url, ok := result["css/bundle.css"]
	if !ok {
		t.Fatal("missing result for css/bundle.css")
	}
	if !strings.HasPrefix(url, "/css/bundle.css?v=") {
		t.Errorf("url = %q, want prefix /css/bundle.css?v=", url)
	}

	// Verify the output file was written and contains both sources.
	data, err := os.ReadFile(filepath.Join(out, "css", "bundle.css"))
	if err != nil {
		t.Fatalf("reading output bundle: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "color: red") {
		t.Error("bundle missing content from a.css")
	}
	if !strings.Contains(content, "margin: 0") {
		t.Error("bundle missing content from b.css")
	}
}

func TestProcessBundles_Minify(t *testing.T) {
	src := t.TempDir()
	out := t.TempDir()

	writeFile(t, filepath.Join(src, "main.css"), "body   {   color:   red;   }")

	bundles := []BundleConfig{
		{
			Name:    "css/bundle.css",
			Type:    "css",
			Sources: []string{filepath.Join(src, "main.css")},
		},
	}

	proc := NewProcessor()
	_, err := proc.ProcessBundles(bundles, out, true)
	if err != nil {
		t.Fatalf("ProcessBundles (minify): %v", err)
	}

	data, err := os.ReadFile(filepath.Join(out, "css", "bundle.css"))
	if err != nil {
		t.Fatalf("reading minified bundle: %v", err)
	}
	// Minified CSS should not contain multiple spaces.
	if strings.Contains(string(data), "   ") {
		t.Errorf("minified output still contains extra spaces: %q", string(data))
	}
}

func TestProcessBundles_JS(t *testing.T) {
	src := t.TempDir()
	out := t.TempDir()

	writeFile(t, filepath.Join(src, "app.js"), "var x = 1;")

	bundles := []BundleConfig{
		{
			Name:    "js/bundle.js",
			Type:    "js",
			Sources: []string{filepath.Join(src, "app.js")},
		},
	}

	proc := NewProcessor()
	result, err := proc.ProcessBundles(bundles, out, false)
	if err != nil {
		t.Fatalf("ProcessBundles JS: %v", err)
	}

	url := result["js/bundle.js"]
	if !strings.HasPrefix(url, "/js/bundle.js?v=") {
		t.Errorf("js url = %q", url)
	}
}

func TestProcessBundles_MissingSource(t *testing.T) {
	out := t.TempDir()

	bundles := []BundleConfig{
		{
			Name:    "css/bundle.css",
			Type:    "css",
			Sources: []string{"/nonexistent/file.css"},
		},
	}

	proc := NewProcessor()
	_, err := proc.ProcessBundles(bundles, out, false)
	if err == nil {
		t.Fatal("expected error for missing source, got nil")
	}
}

func TestProcessBundles_HashDiffersOnContent(t *testing.T) {
	src1 := t.TempDir()
	out1 := t.TempDir()
	src2 := t.TempDir()
	out2 := t.TempDir()

	writeFile(t, filepath.Join(src1, "a.css"), "a{color:red}")
	writeFile(t, filepath.Join(src2, "a.css"), "a{color:blue}")

	proc := NewProcessor()

	b1 := []BundleConfig{{Name: "css/bundle.css", Type: "css", Sources: []string{filepath.Join(src1, "a.css")}}}
	r1, _ := proc.ProcessBundles(b1, out1, false)

	b2 := []BundleConfig{{Name: "css/bundle.css", Type: "css", Sources: []string{filepath.Join(src2, "a.css")}}}
	r2, _ := proc.ProcessBundles(b2, out2, false)

	if r1["css/bundle.css"] == r2["css/bundle.css"] {
		t.Error("expected different cache-busting hashes for different content")
	}
}
