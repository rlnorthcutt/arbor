package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rlnorthcutt/arbor/internal/blueprints"
	"github.com/rlnorthcutt/cmdkit/logger"
)

// Init creates a new Arbor project in the given directory using the specified blueprint.
func Init(dir, blueprint string, log *logger.Logger) error {
	return blueprints.Init(dir, blueprint, log)
}

// NewContent creates a new content file at content/{contentType}/{name}.md
// with scaffolded front matter.
func NewContent(projectRoot, contentType, name string, log *logger.Logger) error {
	// Validate that this looks like an Arbor project root.
	if _, err := os.Stat(filepath.Join(projectRoot, "config.toml")); os.IsNotExist(err) {
		return fmt.Errorf("no Arbor project found at %s (config.toml missing)\nRun 'arbor init' first", projectRoot)
	}

	// Normalize name: replace spaces with hyphens, lowercase
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))

	contentDir := filepath.Join(projectRoot, "content", contentType)
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		return fmt.Errorf("creating content directory: %w", err)
	}

	filePath := filepath.Join(contentDir, slug+".md")
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("file already exists: %s", filePath)
	}

	today := time.Now().Format("2006-01-02")
	title := toTitle(name)

	frontMatter := fmt.Sprintf(`+++
title = %q
date  = %s
draft = true
tags  = []
+++

Content goes here.
`, title, today)

	if err := os.WriteFile(filePath, []byte(frontMatter), 0644); err != nil {
		return fmt.Errorf("writing content file: %w", err)
	}

	log.Success("Created %s", filePath)
	return nil
}

// toTitle converts a slug or name to a title-cased string.
func toTitle(s string) string {
	words := strings.Fields(strings.ReplaceAll(s, "-", " "))
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
