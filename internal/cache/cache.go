package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Cache tracks file hashes for incremental builds.
type Cache struct {
	entries map[string]string // path → sha256 hex
	path    string            // path to .arbor-cache.json
}

// Load reads the cache from disk. Returns an empty cache if the file doesn't exist.
func Load(path string) (*Cache, error) {
	c := &Cache{
		entries: make(map[string]string),
		path:    path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("reading cache file: %w", err)
	}

	if err := json.Unmarshal(data, &c.entries); err != nil {
		// Corrupted cache — start fresh
		c.entries = make(map[string]string)
		return c, nil
	}

	return c, nil
}

// HasChanged computes the current hash of path and compares it to the cached value.
// Returns true if the file has changed or is not in the cache.
func (c *Cache) HasChanged(path string) (bool, error) {
	current, err := hashFile(path)
	if err != nil {
		return false, fmt.Errorf("hashing %s: %w", path, err)
	}

	cached, ok := c.entries[path]
	if !ok {
		return true, nil
	}

	return current != cached, nil
}

// Update computes the current hash of path and stores it in memory.
func (c *Cache) Update(path string) error {
	h, err := hashFile(path)
	if err != nil {
		return fmt.Errorf("hashing %s: %w", path, err)
	}
	c.entries[path] = h
	return nil
}

// Save atomically writes the cache to disk.
func (c *Cache) Save() error {
	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cache: %w", err)
	}

	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	// Atomic write: write to temp file then rename
	tmp, err := os.CreateTemp(dir, ".arbor-cache-*.json.tmp")
	if err != nil {
		return fmt.Errorf("creating temp cache file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp cache file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp cache file: %w", err)
	}

	if err := os.Rename(tmpPath, c.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming cache file: %w", err)
	}

	return nil
}

// Invalidate clears all cache entries (for --force rebuild).
func (c *Cache) Invalidate() {
	c.entries = make(map[string]string)
}

// hashFile computes the SHA-256 hash of a file's contents.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
