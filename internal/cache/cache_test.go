package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEmpty(t *testing.T) {
	dir := t.TempDir()
	c, err := Load(filepath.Join(dir, ".arbor-cache.json"))
	if err != nil {
		t.Fatalf("Load of non-existent cache failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
	if len(c.entries) != 0 {
		t.Errorf("expected empty cache, got %d entries", len(c.entries))
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, ".arbor-cache.json")

	// Create a test file
	testFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(cachePath)
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Update(testFile); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if err := c.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load again and verify
	c2, err := Load(cachePath)
	if err != nil {
		t.Fatalf("second Load failed: %v", err)
	}
	if len(c2.entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(c2.entries))
	}
	if _, ok := c2.entries[testFile]; !ok {
		t.Error("expected test file in loaded cache")
	}
}

func TestHasChanged(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, ".arbor-cache.json")
	testFile := filepath.Join(dir, "test.txt")

	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(cachePath)
	if err != nil {
		t.Fatal(err)
	}

	// File not in cache → HasChanged = true
	changed, err := c.HasChanged(testFile)
	if err != nil {
		t.Fatalf("HasChanged failed: %v", err)
	}
	if !changed {
		t.Error("expected HasChanged=true for uncached file")
	}

	// Update cache
	if err := c.Update(testFile); err != nil {
		t.Fatal(err)
	}

	// Same content → HasChanged = false
	changed, err = c.HasChanged(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("expected HasChanged=false after Update with same content")
	}

	// Modify the file
	if err := os.WriteFile(testFile, []byte("modified content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Modified content → HasChanged = true
	changed, err = c.HasChanged(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected HasChanged=true after file modification")
	}
}

func TestInvalidate(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, ".arbor-cache.json")
	testFile := filepath.Join(dir, "test.txt")

	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(cachePath)
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Update(testFile); err != nil {
		t.Fatal(err)
	}
	if len(c.entries) != 1 {
		t.Errorf("expected 1 entry before invalidate, got %d", len(c.entries))
	}

	c.Invalidate()
	if len(c.entries) != 0 {
		t.Errorf("expected 0 entries after Invalidate, got %d", len(c.entries))
	}

	// After invalidate, HasChanged should return true
	changed, err := c.HasChanged(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected HasChanged=true after Invalidate")
	}
}

func TestAtomicSave(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, ".arbor-cache.json")
	testFile := filepath.Join(dir, "test.txt")

	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Update(testFile); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify the cache file exists and is valid JSON
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("cache file not found after Save: %v", err)
	}
	if len(data) == 0 {
		t.Error("cache file is empty")
	}

	// Verify no temp files left behind
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != ".arbor-cache.json" && e.Name() != "test.txt" {
			t.Errorf("unexpected file left behind: %s", e.Name())
		}
	}
}
