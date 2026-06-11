package content

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestContentMetaAccessors(t *testing.T) {
	meta := ContentMeta{Fields: map[string]any{
		"hero_image": "/img/hero.jpg",
		"show_toc":   true,
		"count":      int64(42),
		"rating":     float64(9),
	}}

	if v := meta.GetString("hero_image"); v != "/img/hero.jpg" {
		t.Errorf("GetString: expected '/img/hero.jpg', got '%s'", v)
	}
	if v := meta.GetString("missing"); v != "" {
		t.Errorf("GetString missing: expected '', got '%s'", v)
	}
	if !meta.GetBool("show_toc") {
		t.Error("GetBool: expected true")
	}
	if meta.GetBool("missing") {
		t.Error("GetBool missing: expected false")
	}
	if v := meta.GetInt("count"); v != 42 {
		t.Errorf("GetInt int64: expected 42, got %d", v)
	}
	if v := meta.GetInt("rating"); v != 9 {
		t.Errorf("GetInt float64: expected 9, got %d", v)
	}
	if meta.GetInt("missing") != 0 {
		t.Error("GetInt missing: expected 0")
	}
	if !meta.Has("hero_image") {
		t.Error("Has: expected true for existing key")
	}
	if meta.Has("missing") {
		t.Error("Has: expected false for missing key")
	}

	v, ok := meta.Get("hero_image")
	if !ok || v != "/img/hero.jpg" {
		t.Errorf("Get: expected ('/img/hero.jpg', true), got (%v, %v)", v, ok)
	}
	_, ok = meta.Get("missing")
	if ok {
		t.Error("Get missing: expected false")
	}
}

func TestContentMetaEmptyFields(t *testing.T) {
	meta := ContentMeta{}
	if meta.GetString("key") != "" {
		t.Error("nil fields: GetString should return ''")
	}
	if meta.GetBool("key") {
		t.Error("nil fields: GetBool should return false")
	}
	if meta.GetInt("key") != 0 {
		t.Error("nil fields: GetInt should return 0")
	}
}

func TestSplitFrontMatter(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectFM     string
		expectMD     string
		expectErr    bool
	}{
		{
			name: "with front matter",
			input: "+++\ntitle = \"Hello\"\n+++\n\nContent here.",
			expectFM: "title = \"Hello\"",
			expectMD: "\nContent here.",
		},
		{
			name:     "no front matter",
			input:    "Just markdown content.",
			expectFM: "",
			expectMD: "Just markdown content.",
		},
		{
			name:      "unclosed front matter",
			input:     "+++\ntitle = \"Hello\"\n",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fm, md, err := splitFrontMatter(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if fm != tc.expectFM {
				t.Errorf("front matter: expected %q, got %q", tc.expectFM, fm)
			}
			if md != tc.expectMD {
				t.Errorf("markdown: expected %q, got %q", tc.expectMD, md)
			}
		})
	}
}

func TestComputePathFields(t *testing.T) {
	tests := []struct {
		name            string
		filePath        string
		contentDir      string
		outputDir       string
		expectedID      string
		expectedType    string
		expectedPerma   string
	}{
		{
			name:          "top-level file",
			filePath:      "/site/content/about.md",
			contentDir:    "/site/content",
			outputDir:     "/site/public",
			expectedID:    "about",
			expectedType:  "page",
			expectedPerma: "/about.html",
		},
		{
			name:          "blog post",
			filePath:      "/site/content/blog/my-post.md",
			contentDir:    "/site/content",
			outputDir:     "/site/public",
			expectedID:    "blog/my-post",
			expectedType:  "blog",
			expectedPerma: "/blog/my-post.html",
		},
		{
			name:          "index file",
			filePath:      "/site/content/blog/my-post/index.md",
			contentDir:    "/site/content",
			outputDir:     "/site/public",
			expectedID:    "blog/my-post",
			expectedType:  "blog",
			expectedPerma: "/blog/my-post/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, contentType, permalink, _ := computePathFields(tc.filePath, tc.contentDir, tc.outputDir)
			if id != tc.expectedID {
				t.Errorf("ID: expected %q, got %q", tc.expectedID, id)
			}
			if contentType != tc.expectedType {
				t.Errorf("Type: expected %q, got %q", tc.expectedType, contentType)
			}
			if permalink != tc.expectedPerma {
				t.Errorf("Permalink: expected %q, got %q", tc.expectedPerma, permalink)
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	contentDir := filepath.Join(dir, "content")
	outputDir := filepath.Join(dir, "public")
	if err := os.MkdirAll(filepath.Join(contentDir, "blog"), 0755); err != nil {
		t.Fatal(err)
	}

	mdContent := `+++
title = "My Blog Post"
date  = 2025-06-15
tags  = ["golang", "ssg"]
draft = false

[extra]
hero_image = "/img/hero.jpg"
+++

# Hello

This is **bold** content.
`
	postPath := filepath.Join(contentDir, "blog", "my-post.md")
	if err := os.WriteFile(postPath, []byte(mdContent), 0644); err != nil {
		t.Fatal(err)
	}

	item, err := ParseFile(postPath, contentDir, outputDir)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if item.ID != "blog/my-post" {
		t.Errorf("ID: expected 'blog/my-post', got '%s'", item.ID)
	}
	if item.Type != "blog" {
		t.Errorf("Type: expected 'blog', got '%s'", item.Type)
	}
	if item.Title != "My Blog Post" {
		t.Errorf("Title: expected 'My Blog Post', got '%s'", item.Title)
	}
	if item.Date.Year() != 2025 || item.Date.Month() != time.June || item.Date.Day() != 15 {
		t.Errorf("Date: expected 2025-06-15, got %v", item.Date)
	}
	if len(item.Tags) != 2 || item.Tags[0] != "golang" {
		t.Errorf("Tags: expected [golang ssg], got %v", item.Tags)
	}
	if item.Draft {
		t.Error("Draft: expected false")
	}
	if item.Permalink != "/blog/my-post.html" {
		t.Errorf("Permalink: expected '/blog/my-post.html', got '%s'", item.Permalink)
	}
	if item.Meta.GetString("hero_image") != "/img/hero.jpg" {
		t.Errorf("Meta.hero_image: expected '/img/hero.jpg', got '%s'", item.Meta.GetString("hero_image"))
	}
	if item.HTMLContent == "" {
		t.Error("HTMLContent should not be empty")
	}
}

func TestBuildIndex(t *testing.T) {
	dir := t.TempDir()
	contentDir := filepath.Join(dir, "content")
	outputDir := filepath.Join(dir, "public")
	if err := os.MkdirAll(filepath.Join(contentDir, "blog"), 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		filepath.Join(contentDir, "about.md"): "+++\ntitle = \"About\"\ndate = 2025-01-01\n+++\n\nAbout page.",
		filepath.Join(contentDir, "blog", "post1.md"): "+++\ntitle = \"Post 1\"\ndate = 2025-06-01\ntags = [\"golang\"]\n+++\n\nPost 1.",
		filepath.Join(contentDir, "blog", "draft.md"): "+++\ntitle = \"Draft\"\ndate = 2025-06-02\ndraft = true\n+++\n\nDraft post.",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Without drafts
	index, err := BuildIndex(contentDir, outputDir, false)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}
	if len(index.All) != 2 {
		t.Errorf("expected 2 items (no drafts), got %d", len(index.All))
	}
	if len(index.ByType["blog"]) != 1 {
		t.Errorf("expected 1 blog item, got %d", len(index.ByType["blog"]))
	}
	if len(index.ByType["page"]) != 1 {
		t.Errorf("expected 1 page item, got %d", len(index.ByType["page"]))
	}
	if len(index.ByTag["golang"]) != 1 {
		t.Errorf("expected 1 golang-tagged item, got %d", len(index.ByTag["golang"]))
	}
	if _, ok := index.BySlug["blog/post1"]; !ok {
		t.Error("expected 'blog/post1' in BySlug")
	}

	// With drafts
	indexWithDrafts, err := BuildIndex(contentDir, outputDir, true)
	if err != nil {
		t.Fatalf("BuildIndex with drafts failed: %v", err)
	}
	if len(indexWithDrafts.All) != 3 {
		t.Errorf("expected 3 items (with drafts), got %d", len(indexWithDrafts.All))
	}
}
