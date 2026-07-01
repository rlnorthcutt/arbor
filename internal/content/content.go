package content

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// reLocalDate matches unquoted TOML local date values (YYYY-MM-DD) in front matter.
// go-toml v1 does not support TOML local dates via Unmarshal, so we convert to strings.
var reLocalDate = regexp.MustCompile(`(?m)^(date\s*=\s*)(\d{4}-\d{2}-\d{2})\s*$`)

// ContentItem is the canonical representation of a single piece of content.
type ContentItem struct {
	ID          string
	Type        string
	Title       string
	Date        time.Time
	Tags        []string
	Draft       bool
	Template    string
	Permalink   string
	OutputPath  string
	RawContent  string
	HTMLContent string
	Meta        ContentMeta
}

// ContentMeta wraps custom front matter fields with non-panicking accessors.
// Fields is exported so Pongo2 templates can access values directly:
// {{ Page.Meta.Fields.hero_image }}
type ContentMeta struct {
	Fields map[string]any
}

// Get returns the raw value for a key and whether it exists.
func (m ContentMeta) Get(key string) (any, bool) {
	if m.Fields == nil {
		return nil, false
	}
	v, ok := m.Fields[key]
	return v, ok
}

// GetString returns the string value for key, or "" if missing or wrong type.
func (m ContentMeta) GetString(key string) string {
	v, ok := m.Fields[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// GetBool returns the bool value for key, or false if missing or wrong type.
func (m ContentMeta) GetBool(key string) bool {
	v, ok := m.Fields[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// GetInt returns the int value for key, or 0 if missing or wrong type.
func (m ContentMeta) GetInt(key string) int {
	v, ok := m.Fields[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// Has returns true if the key exists in the meta.
func (m ContentMeta) Has(key string) bool {
	_, ok := m.Fields[key]
	return ok
}

// SiteIndex is the complete content graph passed to every template.
type SiteIndex struct {
	All    []*ContentItem
	ByType map[string][]*ContentItem
	ByTag  map[string][]*ContentItem
	BySlug map[string]*ContentItem
}

// frontMatterRaw holds the raw TOML fields from the front matter.
// Date is stored as a string and parsed separately because go-toml v1
// does not support TOML local date (date-only without time) via Unmarshal.
type frontMatterRaw struct {
	Title    string         `toml:"title"`
	Date     string         `toml:"date"`
	Tags     []string       `toml:"tags"`
	Draft    bool           `toml:"draft"`
	Template string         `toml:"template"`
	Extra    map[string]any `toml:"extra"`
}

// ParseFile parses a markdown file and returns a ContentItem.
// contentDir is the root content directory (for computing ID/Type).
// outputDir is the directory where output (public/) lives.
func ParseFile(path, contentDir, outputDir string) (*ContentItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	raw, markdown, err := splitFrontMatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("parsing front matter in %s: %w", path, err)
	}

	var fm frontMatterRaw
	if raw != "" {
		// Pre-process: convert TOML local dates (2006-01-02) to quoted strings
		// because go-toml v1 cannot Unmarshal TOML local dates.
		processed := reLocalDate.ReplaceAllString(raw, `${1}"${2}"`)
		if err := toml.Unmarshal([]byte(processed), &fm); err != nil {
			return nil, fmt.Errorf("invalid TOML front matter in %s: %w", path, err)
		}
	}

	// Compute ID, Type, Permalink, OutputPath
	id, contentType, permalink, outputPath := computePathFields(path, contentDir, outputDir)

	// Render Markdown → HTML
	var htmlBuf bytes.Buffer
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
			extension.Footnote,
			extension.DefinitionList,
			extension.Linkify,
		),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
	if err := md.Convert([]byte(markdown), &htmlBuf); err != nil {
		return nil, fmt.Errorf("rendering markdown in %s: %w", path, err)
	}

	meta := ContentMeta{Fields: fm.Extra}
	if meta.Fields == nil {
		meta.Fields = make(map[string]any)
	}

	var date time.Time
	if fm.Date != "" {
		// Support both "2006-01-02" and full RFC3339 formats
		if t, err := time.Parse("2006-01-02", fm.Date); err == nil {
			date = t
		} else if t, err := time.Parse(time.RFC3339, fm.Date); err == nil {
			date = t
		}
		// If parsing fails, date stays zero value (no error)
	}

	item := &ContentItem{
		ID:          id,
		Type:        contentType,
		Title:       fm.Title,
		Date:        date,
		Tags:        fm.Tags,
		Draft:       fm.Draft,
		Template:    fm.Template,
		Permalink:   permalink,
		OutputPath:  outputPath,
		RawContent:  markdown,
		HTMLContent: htmlBuf.String(),
		Meta:        meta,
	}

	return item, nil
}

// splitFrontMatter separates TOML front matter from markdown content.
// Front matter is delimited by +++ on its own line.
func splitFrontMatter(content string) (frontMatter, markdown string, err error) {
	content = strings.TrimLeft(content, "\r\n")
	if !strings.HasPrefix(content, "+++") {
		return "", content, nil
	}

	// Find the closing +++
	rest := content[3:]
	// skip the newline after opening +++
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	idx := strings.Index(rest, "\n+++")
	if idx == -1 {
		return "", "", fmt.Errorf("unclosed front matter: no closing +++")
	}

	frontMatter = rest[:idx]
	markdown = rest[idx+4:] // skip \n+++
	// skip the newline after closing +++
	if len(markdown) > 0 && markdown[0] == '\n' {
		markdown = markdown[1:]
	} else if len(markdown) > 1 && markdown[0] == '\r' && markdown[1] == '\n' {
		markdown = markdown[2:]
	}

	return frontMatter, markdown, nil
}

// computePathFields derives ID, Type, Permalink, and OutputPath from the file path.
func computePathFields(filePath, contentDir, outputDir string) (id, contentType, permalink, outputPath string) {
	// Get relative path from contentDir
	rel, err := filepath.Rel(contentDir, filePath)
	if err != nil {
		rel = filepath.Base(filePath)
	}
	rel = filepath.ToSlash(rel)

	parts := strings.Split(rel, "/")

	if len(parts) == 1 {
		// File directly in content/ → type=page
		name := strings.TrimSuffix(parts[0], ".md")
		id = name
		contentType = "page"
		permalink = "/" + name + ".html"
		outputPath = filepath.Join(outputDir, name+".html")
		return
	}

	// File in subdirectory
	contentType = parts[0] // first subdirectory is the type

	// Check if it's an index.md
	filename := parts[len(parts)-1]
	dirParts := parts[:len(parts)-1]

	if filename == "index.md" {
		// content/blog/my-post/index.md → ID: blog/my-post, permalink: /blog/my-post/
		id = strings.Join(dirParts, "/")
		permalink = "/" + id + "/"
		outputPath = filepath.Join(outputDir, filepath.Join(dirParts...), "index.html")
	} else {
		// content/blog/my-post.md → ID: blog/my-post, permalink: /blog/my-post.html
		name := strings.TrimSuffix(filename, ".md")
		idParts := append(dirParts, name)
		id = strings.Join(idParts, "/")
		permalink = "/" + id + ".html"
		outputPath = filepath.Join(outputDir, filepath.Join(idParts...)+".html")
	}

	return
}

// BuildIndex scans contentDir for all .md files and builds a SiteIndex.
// If includeDrafts is false, draft items are excluded.
func BuildIndex(contentDir, outputDir string, includeDrafts bool) (*SiteIndex, error) {
	index := &SiteIndex{
		All:    make([]*ContentItem, 0),
		ByType: make(map[string][]*ContentItem),
		ByTag:  make(map[string][]*ContentItem),
		BySlug: make(map[string]*ContentItem),
	}

	err := filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		item, err := ParseFile(path, contentDir, outputDir)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}

		if item.Draft && !includeDrafts {
			return nil
		}

		index.All = append(index.All, item)
		index.ByType[item.Type] = append(index.ByType[item.Type], item)
		index.BySlug[item.ID] = item
		for _, tag := range item.Tags {
			index.ByTag[tag] = append(index.ByTag[tag], item)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("scanning content directory: %w", err)
	}

	return index, nil
}
