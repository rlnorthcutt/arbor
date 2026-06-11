package renderer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rlnorthcutt/arbor/internal/config"
	"github.com/rlnorthcutt/arbor/internal/content"
	"github.com/rlnorthcutt/cmdkit/logger"
)

func newTestRenderer(t *testing.T) (*Renderer, string) {
	t.Helper()
	dir := t.TempDir()

	dirs := []string{
		filepath.Join(dir, "templates", "types"),
		filepath.Join(dir, "templates", "layouts"),
		filepath.Join(dir, "templates", "partials"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	log := logger.New(false)
	return New(dir, log), dir
}

func TestResolveTemplate_Explicit(t *testing.T) {
	r, dir := newTestRenderer(t)

	customPath := filepath.Join(dir, "templates", "types", "custom.html")
	if err := os.WriteFile(customPath, []byte(`<custom/>`), 0644); err != nil {
		t.Fatal(err)
	}

	item := &content.ContentItem{
		Type:     "blog",
		Template: "templates/types/custom.html",
	}
	resolved, err := r.ResolveTemplate(item)
	if err != nil {
		t.Fatalf("ResolveTemplate failed: %v", err)
	}
	if resolved != "templates/types/custom.html" {
		t.Errorf("expected 'templates/types/custom.html', got '%s'", resolved)
	}
}

func TestResolveTemplate_ByType(t *testing.T) {
	r, dir := newTestRenderer(t)

	blogPath := filepath.Join(dir, "templates", "types", "blog.html")
	if err := os.WriteFile(blogPath, []byte(`<blog/>`), 0644); err != nil {
		t.Fatal(err)
	}

	item := &content.ContentItem{Type: "blog"}
	resolved, err := r.ResolveTemplate(item)
	if err != nil {
		t.Fatalf("ResolveTemplate failed: %v", err)
	}
	if resolved != filepath.Join("templates", "types", "blog.html") {
		t.Errorf("expected blog template, got '%s'", resolved)
	}
}

func TestResolveTemplate_Fallback(t *testing.T) {
	r, dir := newTestRenderer(t)

	pagePath := filepath.Join(dir, "templates", "types", "page.html")
	if err := os.WriteFile(pagePath, []byte(`<page/>`), 0644); err != nil {
		t.Fatal(err)
	}

	item := &content.ContentItem{Type: "blog"}
	resolved, err := r.ResolveTemplate(item)
	if err != nil {
		t.Fatalf("ResolveTemplate failed: %v", err)
	}
	if resolved != filepath.Join("templates", "types", "page.html") {
		t.Errorf("expected fallback page.html, got '%s'", resolved)
	}
}

func TestResolveTemplate_MissingExplicit(t *testing.T) {
	r, _ := newTestRenderer(t)

	item := &content.ContentItem{
		Template: "templates/types/nonexistent.html",
	}
	_, err := r.ResolveTemplate(item)
	if err == nil {
		t.Error("expected error for missing explicit template")
	}
}

func TestRender(t *testing.T) {
	r, dir := newTestRenderer(t)

	// Pongo2 syntax: |safe renders HTML without escaping
	tmplContent := `<html><head><title>{{ Page.Title }}</title></head><body>{{ Page.HTMLContent|safe }}</body></html>`
	pagePath := filepath.Join(dir, "templates", "types", "page.html")
	if err := os.WriteFile(pagePath, []byte(tmplContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	site := &SiteContext{
		Config: cfg,
		Index:  &content.SiteIndex{},
	}
	item := &content.ContentItem{
		Type:        "page",
		Title:       "Test Page",
		HTMLContent: "<p>Hello world</p>",
		Date:        time.Now(),
	}

	vars := TemplateVars{
		Site: site,
		Page: item,
		Data: map[string]any{},
	}

	out, err := r.Render(item, vars)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(out, "Test Page") {
		t.Error("rendered output should contain page title")
	}
	if !strings.Contains(out, "<p>Hello world</p>") {
		t.Error("rendered output should contain HTML content")
	}
}

func TestComputePager(t *testing.T) {
	// 25 items, page size 10 → 3 pages
	p1 := ComputePager(25, 10, 1, "/blog")
	if p1.Current != 1 {
		t.Errorf("page 1: Current should be 1, got %d", p1.Current)
	}
	if p1.Total != 3 {
		t.Errorf("page 1: Total should be 3, got %d", p1.Total)
	}
	if p1.HasPrev {
		t.Error("page 1: HasPrev should be false")
	}
	if !p1.HasNext {
		t.Error("page 1: HasNext should be true")
	}
	if p1.NextURL != "/blog/page/2/" {
		t.Errorf("page 1: NextURL should be '/blog/page/2/', got %q", p1.NextURL)
	}

	p2 := ComputePager(25, 10, 2, "/blog")
	if !p2.HasPrev {
		t.Error("page 2: HasPrev should be true")
	}
	if p2.PrevURL != "/blog/" {
		t.Errorf("page 2: PrevURL should be '/blog/', got %q", p2.PrevURL)
	}
	if !p2.HasNext {
		t.Error("page 2: HasNext should be true")
	}

	p3 := ComputePager(25, 10, 3, "/blog")
	if !p3.HasPrev {
		t.Error("page 3: HasPrev should be true")
	}
	if p3.PrevURL != "/blog/page/2/" {
		t.Errorf("page 3: PrevURL should be '/blog/page/2/', got %q", p3.PrevURL)
	}
	if p3.HasNext {
		t.Error("page 3: HasNext should be false")
	}

	// Empty collection
	pEmpty := ComputePager(0, 10, 1, "/blog")
	if pEmpty.Total != 1 {
		t.Errorf("empty: Total should be 1, got %d", pEmpty.Total)
	}
	if pEmpty.HasPrev || pEmpty.HasNext {
		t.Error("empty: should have no prev/next")
	}
}
