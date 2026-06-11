package shortcode

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/flosch/pongo2/v6"
)

// shortcodeEntry holds a parsed shortcode.
type shortcodeEntry struct {
	partial string
	params  map[string]string
	body    string // pre-rendered body, empty for self-closing
}

// Processor implements the two-pass shortcode processor.
type Processor struct {
	extracted []shortcodeEntry
}

// New creates a new Processor.
func New() *Processor {
	return &Processor{}
}

// tokenFor returns an HTML comment placeholder that Goldmark passes through
// unchanged (double-underscore tokens get mangled as Markdown bold).
func tokenFor(i int) string {
	return fmt.Sprintf("<!--ARBOR-SC-%d-->", i)
}

// reShortcode matches self-closing shortcodes: {{% partial "name" key=value %}}
var reShortcode = regexp.MustCompile(`\{\{%\s+partial\s+"([^"]+)"([^%]*?)%\}\}`)

// reShortcodeBlock matches block shortcodes: {{% partial "name" key=value %}}body{{% /partial %}}
var reShortcodeBlock = regexp.MustCompile(`(?s)\{\{%\s+partial\s+"([^"]+)"([^%]*?)%\}\}(.*?)\{\{%\s*/partial\s*%\}\}`)

// PreProcess scans markdown for shortcodes, replaces them with tokens,
// and stores the shortcode data for later rendering.
func (p *Processor) PreProcess(markdown string) string {
	p.extracted = nil

	// First pass: extract block shortcodes (with body)
	result := reShortcodeBlock.ReplaceAllStringFunc(markdown, func(match string) string {
		submatches := reShortcodeBlock.FindStringSubmatch(match)
		if len(submatches) < 4 {
			return match
		}
		idx := len(p.extracted)
		p.extracted = append(p.extracted, shortcodeEntry{
			partial: submatches[1],
			params:  parseParams(submatches[2]),
			body:    submatches[3],
		})
		return tokenFor(idx)
	})

	// Second pass: extract self-closing shortcodes
	result = reShortcode.ReplaceAllStringFunc(result, func(match string) string {
		submatches := reShortcode.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		idx := len(p.extracted)
		p.extracted = append(p.extracted, shortcodeEntry{
			partial: submatches[1],
			params:  parseParams(submatches[2]),
		})
		return tokenFor(idx)
	})

	return result
}

// PostProcess replaces tokens in rendered HTML with Pongo2 partial output.
// projectRoot is the site root directory; templates are resolved relative to it.
func (p *Processor) PostProcess(html string, projectRoot string, vars map[string]any) string {
	if len(p.extracted) == 0 {
		return html
	}

	loader := pongo2.MustNewLocalFileSystemLoader(projectRoot)
	set := pongo2.NewSet("shortcodes", loader)

	for i, sc := range p.extracted {
		token := tokenFor(i)
		if !strings.Contains(html, token) {
			continue
		}

		rendered, err := renderPartial(set, sc, vars)
		if err != nil {
			html = strings.ReplaceAll(html, token, fmt.Sprintf("<!-- shortcode error: %v -->", err))
			continue
		}

		html = strings.ReplaceAll(html, token, rendered)
	}

	return html
}

// renderPartial renders a Pongo2 partial template for a shortcode entry.
func renderPartial(set *pongo2.TemplateSet, sc shortcodeEntry, vars map[string]any) (string, error) {
	// Try with .html extension first, then bare path
	tmpl, err := set.FromFile(sc.partial + ".html")
	if err != nil {
		tmpl, err = set.FromFile(sc.partial)
		if err != nil {
			return "", fmt.Errorf("loading partial %q: %w", sc.partial, err)
		}
	}

	ctx := pongo2.Context{}
	for k, v := range vars {
		ctx[k] = v
	}
	for k, v := range sc.params {
		ctx[k] = v
	}
	if sc.body != "" {
		ctx["body"] = sc.body
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteWriter(ctx, &buf); err != nil {
		return "", fmt.Errorf("executing partial %q: %w", sc.partial, err)
	}

	return buf.String(), nil
}

// parseParams parses key=value pairs from a shortcode tag string.
func parseParams(s string) map[string]string {
	params := make(map[string]string)
	s = strings.TrimSpace(s)
	if s == "" {
		return params
	}

	re := regexp.MustCompile(`(\w+)=(?:"([^"]*)"|([\S]+))`)
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		val := m[2]
		if val == "" {
			val = m[3]
		}
		params[m[1]] = val
	}

	return params
}
