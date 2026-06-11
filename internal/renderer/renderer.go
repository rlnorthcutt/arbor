package renderer

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/rlnorthcutt/arbor/internal/config"
	"github.com/rlnorthcutt/arbor/internal/content"
	"github.com/rlnorthcutt/cmdkit/logger"
)

// SiteContext is the Site template variable.
type SiteContext struct {
	Config *config.Config
	Index  *content.SiteIndex
}

// Pager holds pagination state for listing templates.
type Pager struct {
	Current int
	Total   int
	HasPrev bool
	HasNext bool
	PrevURL string
	NextURL string
}

// TemplateVars holds all variables passed to a Pongo2 template.
type TemplateVars struct {
	Site  *SiteContext
	Page  *content.ContentItem
	Data  map[string]any
	Items []*content.ContentItem
	Pager *Pager
}

// Renderer handles Pongo2 template rendering.
type Renderer struct {
	projectRoot string
	set         *pongo2.TemplateSet
	log         *logger.Logger
}

var registerOnce sync.Once

// New creates a Renderer with a single TemplateSet reused for all renders.
func New(projectRoot string, log *logger.Logger) *Renderer {
	registerOnce.Do(registerFilters)
	loader := pongo2.MustNewLocalFileSystemLoader(projectRoot)
	set := pongo2.NewSet("arbor", loader)
	return &Renderer{
		projectRoot: projectRoot,
		set:         set,
		log:         log,
	}
}

// registerFilters registers Arbor's custom Pongo2 filters (once per process).
func registerFilters() {
	// godate formats a time.Time using Go's reference time format strings.
	// Usage in templates: {{ Page.Date|godate:"January 2, 2006" }}
	pongo2.RegisterFilter("godate", func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
		t, ok := in.Interface().(time.Time)
		if !ok {
			return in, nil
		}
		format := param.String()
		if format == "" {
			format = "January 2, 2006"
		}
		return pongo2.AsValue(t.Format(format)), nil
	})
}

// ResolveTemplate finds the template path for a ContentItem.
// Resolution order:
//  1. item.Template set → use that path directly
//  2. templates/types/{type}.html exists → use it
//  3. templates/types/page.html → final fallback
func (r *Renderer) ResolveTemplate(item *content.ContentItem) (string, error) {
	if item.Template != "" {
		fullPath := filepath.Join(r.projectRoot, item.Template)
		if _, err := os.Stat(fullPath); err == nil {
			return item.Template, nil
		}
		return "", fmt.Errorf("specified template not found: %s", item.Template)
	}

	typePath := filepath.Join("templates", "types", item.Type+".html")
	if _, err := os.Stat(filepath.Join(r.projectRoot, typePath)); err == nil {
		return typePath, nil
	}

	fallback := filepath.Join("templates", "types", "page.html")
	if _, err := os.Stat(filepath.Join(r.projectRoot, fallback)); err == nil {
		return fallback, nil
	}

	return "", fmt.Errorf("no template found for type %q (tried types/%s.html and types/page.html)", item.Type, item.Type)
}

// Render renders a ContentItem with its resolved template.
func (r *Renderer) Render(item *content.ContentItem, vars TemplateVars) (string, error) {
	templatePath, err := r.ResolveTemplate(item)
	if err != nil {
		return "", err
	}

	tmpl, err := r.set.FromFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("loading template %q: %w", templatePath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteWriter(r.buildContext(vars), &buf); err != nil {
		return "", fmt.Errorf("executing template %q: %w", templatePath, err)
	}

	return buf.String(), nil
}

// ResolveListingTemplate finds the template for an auto-generated listing page.
// Resolution order:
//  1. templates/types/{type}-list.html
//  2. templates/types/listing.html
func (r *Renderer) ResolveListingTemplate(typeName string) (string, error) {
	specific := filepath.Join("templates", "types", typeName+"-list.html")
	if _, err := os.Stat(filepath.Join(r.projectRoot, specific)); err == nil {
		return specific, nil
	}

	generic := filepath.Join("templates", "types", "listing.html")
	if _, err := os.Stat(filepath.Join(r.projectRoot, generic)); err == nil {
		return generic, nil
	}

	return "", fmt.Errorf("no listing template found for type %q", typeName)
}

// RenderListing renders an auto-generated listing page for a content type.
func (r *Renderer) RenderListing(item *content.ContentItem, vars TemplateVars) (string, error) {
	templatePath, err := r.ResolveListingTemplate(item.Type)
	if err != nil {
		return "", err
	}

	tmpl, err := r.set.FromFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("loading listing template %q: %w", templatePath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteWriter(r.buildContext(vars), &buf); err != nil {
		return "", fmt.Errorf("executing listing template %q: %w", templatePath, err)
	}

	return buf.String(), nil
}

// CheckTemplates parses every .html file under tmplDir and returns a map of
// relative path → parse error for any files that fail. An empty map means all
// templates are syntactically valid.
func (r *Renderer) CheckTemplates(tmplDir string) map[string]error {
	issues := make(map[string]error)
	filepath.Walk(tmplDir, func(path string, info os.FileInfo, err error) error { //nolint
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".html") {
			return err
		}
		rel, _ := filepath.Rel(r.projectRoot, path)
		if _, parseErr := r.set.FromFile(rel); parseErr != nil {
			issues[rel] = parseErr
		}
		return nil
	})
	return issues
}

// buildContext converts TemplateVars to a pongo2.Context.
func (r *Renderer) buildContext(vars TemplateVars) pongo2.Context {
	return pongo2.Context{
		"Site":  vars.Site,
		"Page":  vars.Page,
		"Data":  vars.Data,
		"Items": vars.Items,
		"Pager": vars.Pager,
	}
}

// ComputePager calculates pagination state for a collection.
// typePath is the URL base path, e.g. "/blog".
func ComputePager(totalItems, pageSize, currentPage int, typePath string) *Pager {
	if pageSize <= 0 {
		pageSize = 10
	}
	if currentPage <= 0 {
		currentPage = 1
	}

	total := int(math.Ceil(float64(totalItems) / float64(pageSize)))
	if total == 0 {
		total = 1
	}

	var prevURL, nextURL string
	if currentPage > 1 {
		if currentPage == 2 {
			prevURL = typePath + "/"
		} else {
			prevURL = fmt.Sprintf("%s/page/%d/", typePath, currentPage-1)
		}
	}
	if currentPage < total {
		nextURL = fmt.Sprintf("%s/page/%d/", typePath, currentPage+1)
	}

	return &Pager{
		Current: currentPage,
		Total:   total,
		HasPrev: currentPage > 1,
		HasNext: currentPage < total,
		PrevURL: prevURL,
		NextURL: nextURL,
	}
}
