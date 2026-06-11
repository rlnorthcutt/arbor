package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rlnorthcutt/cmdkit/logger"
)

func TestInit(t *testing.T) {
	dir := t.TempDir()
	log := logger.New(false)

	if err := Init(dir, "blog", log); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Check that key files were created by the blog blueprint
	expectedFiles := []string{
		"config.toml",
		"data/nav.toml",
		"templates/layouts/base.html",
		"templates/types/page.html",
		"templates/types/blog.html",
		"templates/partials/header.html",
		"templates/partials/footer.html",
		"templates/displays/card.html",
		"templates/displays/teaser.html",
		"templates/displays/full.html",
		"static/css/ivy.full.css",
		"static/css/lattice.full.css",
		"static/css/site.css",
		"static/js/dark-mode-toggle.js",
		"content/about.md",
	}

	for _, rel := range expectedFiles {
		path := filepath.Join(dir, rel)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist after Init", rel)
		}
	}
}

func TestInitDoesNotOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	log := logger.New(false)

	// Create existing config.toml with custom content
	customContent := "[site]\ntitle = \"Custom Site\"\n"
	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(customContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Init(dir, "blog", log); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Existing file should not be overwritten
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != customContent {
		t.Error("Init should not overwrite existing config.toml")
	}
}

func TestInitInvalidBlueprint(t *testing.T) {
	dir := t.TempDir()
	log := logger.New(false)

	err := Init(dir, "nonexistent", log)
	if err == nil {
		t.Error("expected error for unknown blueprint")
	}
}

// newProjectDir creates a temp dir with a minimal config.toml so NewContent
// can validate it's being run inside an Arbor project.
func newProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[site]\ntitle = \"Test\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNewContent(t *testing.T) {
	dir := newProjectDir(t)
	log := logger.New(false)

	if err := NewContent(dir, "blog", "my-first-post", log); err != nil {
		t.Fatalf("NewContent failed: %v", err)
	}

	path := filepath.Join(dir, "content", "blog", "my-first-post.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected content file %s to exist", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "+++") {
		t.Error("content file should have front matter delimiters")
	}
	if !strings.Contains(content, `title = "My First Post"`) {
		t.Errorf("content file should have title, got:\n%s", content)
	}
	if !strings.Contains(content, "draft = true") {
		t.Error("new content should be a draft")
	}
	if !strings.Contains(content, "tags  = []") {
		t.Error("new content should have empty tags")
	}
}

func TestNewContentSlugNormalization(t *testing.T) {
	dir := newProjectDir(t)
	log := logger.New(false)

	// Name with spaces and uppercase
	if err := NewContent(dir, "blog", "Hello World Post", log); err != nil {
		t.Fatalf("NewContent failed: %v", err)
	}

	// Should be normalized to hello-world-post.md
	path := filepath.Join(dir, "content", "blog", "hello-world-post.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected slug-normalized file %s to exist", path)
	}
}

func TestNewContentExistingFile(t *testing.T) {
	dir := newProjectDir(t)
	log := logger.New(false)

	// Create first
	if err := NewContent(dir, "blog", "test-post", log); err != nil {
		t.Fatal(err)
	}

	// Try to create again — should fail
	err := NewContent(dir, "blog", "test-post", log)
	if err == nil {
		t.Error("expected error when content file already exists")
	}
}

func TestNewContentRequiresProject(t *testing.T) {
	dir := t.TempDir() // no config.toml
	log := logger.New(false)

	err := NewContent(dir, "blog", "test-post", log)
	if err == nil {
		t.Error("expected error when running outside an Arbor project")
	}
}

func TestToTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-first-post", "My First Post"},
		{"hello world", "Hello World"},
		{"golang", "Golang"},
		{"Hello World", "Hello World"},
	}

	for _, tc := range tests {
		result := toTitle(tc.input)
		if result != tc.expected {
			t.Errorf("toTitle(%q): expected %q, got %q", tc.input, tc.expected, result)
		}
	}
}
