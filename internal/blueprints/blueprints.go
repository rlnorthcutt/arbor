package blueprints

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rlnorthcutt/cmdkit/logger"
)

// all: prefix includes hidden files (e.g. .gitignore) that the default embed ignores.
//
//go:embed all:base all:blog all:marketing all:docs
var blueprintFiles embed.FS

var validBlueprints = map[string]bool{"blog": true, "marketing": true, "docs": true}

// Init initializes a new Arbor project in dir using the specified blueprint.
func Init(dir, blueprintName string, log *logger.Logger) error {
	if !validBlueprints[blueprintName] {
		return fmt.Errorf("unknown blueprint %q: choose blog, marketing, or docs", blueprintName)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating project directory: %w", err)
	}
	log.Info("Initializing new Arbor project in %s (blueprint: %s)", dir, blueprintName)

	totalSkipped := 0
	for _, prefix := range []string{"base", blueprintName} {
		skipped, err := copyEmbeddedDir(blueprintFiles, prefix, dir, log)
		if err != nil {
			return err
		}
		totalSkipped += skipped
	}

	if totalSkipped > 0 {
		log.Info("%d file(s) already existed and were skipped", totalSkipped)
	}
	log.Info("Project ready. Run 'arbor preview' to start the dev server.")
	return nil
}

func copyEmbeddedDir(fsys embed.FS, prefix, destRoot string, log *logger.Logger) (skipped int, err error) {
	walkErr := fs.WalkDir(fsys, prefix, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel := strings.TrimPrefix(path, prefix+"/")
		if rel == prefix || rel == "" {
			return nil
		}
		dst := filepath.Join(destRoot, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0755)
		}
		if _, statErr := os.Stat(dst); statErr == nil {
			log.Detail("Skipping existing: %s", dst)
			skipped++
			return nil
		}
		data, readErr := fsys.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if mkErr := os.MkdirAll(filepath.Dir(dst), 0755); mkErr != nil {
			return mkErr
		}
		if writeErr := os.WriteFile(dst, data, 0644); writeErr != nil {
			return writeErr
		}
		log.Success("Created %s", dst)
		return nil
	})
	return skipped, walkErr
}
