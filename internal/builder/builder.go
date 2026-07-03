package builder

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/pelletier/go-toml"
	"github.com/rlnorthcutt/arbor/internal/assets"
	"github.com/rlnorthcutt/arbor/internal/cache"
	"github.com/rlnorthcutt/arbor/internal/config"
	"github.com/rlnorthcutt/arbor/internal/content"
	"github.com/rlnorthcutt/arbor/internal/renderer"
	"github.com/rlnorthcutt/arbor/internal/shortcode"
	"github.com/rlnorthcutt/cmdkit/logger"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
)

// Builder orchestrates the full site build pipeline.
type Builder struct {
	projectRoot     string
	config          *config.Config
	log             *logger.Logger
	cache           *cache.Cache
	minifyAssets    bool
	aggregateAssets bool
	bundledSources  map[string]bool   // absolute paths of static files included in a bundle
	processedAssets map[string]string // bundle.Name → versioned URL
}

// BuildOptions controls build behavior.
type BuildOptions struct {
	Force           bool // ignore cache, full rebuild
	MinifyAssets    bool
	AggregateAssets bool
}

// New creates a new Builder, loading config from the project root.
func New(projectRoot string, log *logger.Logger) (*Builder, error) {
	cfg, err := config.Load(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	cachePath := filepath.Join(projectRoot, ".arbor-cache.json")
	c, err := cache.Load(cachePath)
	if err != nil {
		return nil, fmt.Errorf("loading cache: %w", err)
	}

	return &Builder{
		projectRoot: projectRoot,
		config:      cfg,
		log:         log,
		cache:       c,
	}, nil
}

// Build runs the full build pipeline.
func (b *Builder) Build(ctx context.Context, opts BuildOptions) error {
	b.minifyAssets = opts.MinifyAssets
	b.aggregateAssets = opts.AggregateAssets
	b.bundledSources = make(map[string]bool)
	b.processedAssets = make(map[string]string)

	if opts.Force {
		b.cache.Invalidate()
		b.log.Info("Force rebuild: cache invalidated")
	}

	outputDir := filepath.Join(b.projectRoot, b.config.Build.OutputDir)

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory %s: %w", outputDir, err)
	}

	// Step 1: Load data files
	b.log.Info("Loading data files...")
	data, err := b.loadData()
	if err != nil {
		return fmt.Errorf("loading data: %w", err)
	}

	// Step 2: Scan content directory
	b.log.Info("Scanning content...")
	contentDir := filepath.Join(b.projectRoot, "content")
	index, err := content.BuildIndex(contentDir, outputDir, b.config.Build.DraftMode)
	if err != nil {
		return fmt.Errorf("building content index: %w", err)
	}
	b.log.Info("Found %d content items", len(index.All))

	// Step 3: Determine what needs rebuilding
	// Check if config or data changed → full rebuild needed
	needsFullRebuild, err := b.checkGlobalFilesChanged()
	if err != nil {
		b.log.Warn("Could not check global files: %v", err)
		needsFullRebuild = true
	}

	// Check if any partial changed → full rebuild
	partialsChanged, err := b.checkPartialsChanged()
	if err != nil {
		b.log.Warn("Could not check partials: %v", err)
		partialsChanged = true
	}

	if needsFullRebuild || partialsChanged {
		if needsFullRebuild {
			b.log.Info("Config or data changed: full rebuild")
		} else {
			b.log.Info("Partials changed: full rebuild")
		}
	}

	// Step 4: Compute asset context, then render content items
	staticDir := filepath.Join(b.projectRoot, "static")
	assetTags, err := b.computeAssetContext(staticDir, outputDir)
	if err != nil {
		b.log.Warn("Asset context computation failed: %v", err)
		assetTags = &renderer.AssetTags{}
	}

	r := renderer.New(b.projectRoot, b.log)
	site := &renderer.SiteContext{
		Config: b.config,
		Index:  index,
	}

	built := 0
	skipped := 0

	for _, item := range index.All {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Determine source file path for cache check
		srcPath := b.contentItemSourcePath(item, contentDir)

		// Determine if this item needs rendering
		needsRender := needsFullRebuild || partialsChanged
		if !needsRender {
			changed, err := b.cache.HasChanged(srcPath)
			if err != nil {
				b.log.Warn("Cache check failed for %s: %v", srcPath, err)
				needsRender = true
			} else {
				needsRender = changed
			}

			// Also check if its type template changed
			if !needsRender {
				typeChanged, _ := b.checkTypeTemplateChanged(item)
				needsRender = typeChanged
			}
		}

		if !needsRender {
			b.log.Detail("Skipping unchanged: %s", srcPath)
			skipped++
			continue
		}

		// Apply shortcode pre-processing when the content file uses {{% partial %}} tags.
		// This re-runs Goldmark on the pre-processed markdown so tokens survive into HTMLContent.
		sc := shortcode.New()
		if strings.Contains(item.RawContent, "{{% partial") {
			processed := sc.PreProcess(item.RawContent)
			var htmlBuf bytes.Buffer
			if err := goldmark.New(goldmark.WithRendererOptions(html.WithUnsafe())).Convert([]byte(processed), &htmlBuf); err != nil {
				b.log.Warn("Shortcode pre-processing failed for %s: %v", item.ID, err)
			} else {
				item.HTMLContent = htmlBuf.String()
			}
		}

		vars := renderer.TemplateVars{
			Site:   site,
			Page:   item,
			Data:   data,
			Items:  index.ByType[item.Type],
			Assets: assetTags,
		}

		html, err := r.Render(item, vars)
		if err != nil {
			return fmt.Errorf("rendering %s: %w", item.ID, err)
		}

		// Replace shortcode tokens with rendered Pongo2 partial output.
		html = sc.PostProcess(html, b.projectRoot, map[string]any{
			"Site": site,
			"Data": data,
		})

		// Ensure output directory exists
		outDir := filepath.Dir(item.OutputPath)
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("creating output dir %s: %w", outDir, err)
		}

		if err := os.WriteFile(item.OutputPath, []byte(html), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", item.OutputPath, err)
		}

		b.log.Success("Built %s", item.OutputPath)
		b.log.Detail("Rendering %s → %s", srcPath, item.OutputPath)

		// Update cache for this file
		if err := b.cache.Update(srcPath); err != nil {
			b.log.Warn("Could not update cache for %s: %v", srcPath, err)
		}

		built++
	}

	b.log.Info("Built %d pages, skipped %d unchanged", built, skipped)

	// Step 5: Auto-generate listing pages for types without a manual index
	listingBuilt, err := b.buildListingPages(ctx, index, r, site, data, assetTags)
	if err != nil {
		return fmt.Errorf("building listing pages: %w", err)
	}
	if listingBuilt > 0 {
		b.log.Info("Auto-generated %d listing page(s)", listingBuilt)
	}

	// Step 6: Copy static files (skips sources already written as bundles)
	if _, err := os.Stat(staticDir); err == nil {
		if err := b.copyStatic(staticDir, outputDir); err != nil {
			return fmt.Errorf("copying static files: %w", err)
		}
	}

	// Step 7: Update global file hashes and save cache
	b.updateGlobalFileHashes()
	b.updateTemplateHashes()
	if err := b.cache.Save(); err != nil {
		b.log.Warn("Could not save cache: %v", err)
	}

	return nil
}

// computeAssetContext builds the HTML tag strings for CSS and JS includes.
// When aggregating, it processes bundles immediately (writing files to outputDir)
// so that cache-busting hashes are available for templates. When not aggregating,
// it lists individual static files and builds individual include tags.
func (b *Builder) computeAssetContext(staticDir, outputDir string) (*renderer.AssetTags, error) {
	if !b.aggregateAssets {
		cssFiles, jsFiles := assets.ListStaticAssets(staticDir)
		var cssLinks, jsLinks strings.Builder
		for _, f := range cssFiles {
			cssLinks.WriteString(`<link rel="stylesheet" href="/` + filepath.ToSlash(f) + `">` + "\n")
		}
		for _, f := range jsFiles {
			jsLinks.WriteString(`<script src="/` + filepath.ToSlash(f) + `"></script>` + "\n")
		}
		return &renderer.AssetTags{
			CSS: strings.TrimRight(cssLinks.String(), "\n"),
			JS:  strings.TrimRight(jsLinks.String(), "\n"),
		}, nil
	}

	// Aggregate mode: load or compute bundles, then process them.
	bundles, err := b.loadAssetManifest(staticDir)
	if err != nil {
		return nil, err
	}

	proc := assets.NewProcessor()
	processed, err := proc.ProcessBundles(bundles, outputDir, b.minifyAssets)
	if err != nil {
		return nil, fmt.Errorf("processing asset bundles: %w", err)
	}
	b.processedAssets = processed

	// Track which source files were bundled so copyStatic can skip them.
	for _, bundle := range bundles {
		for _, src := range bundle.Sources {
			b.bundledSources[src] = true
		}
	}

	// Build tag HTML from processed bundle URLs.
	var cssLinks, jsLinks strings.Builder
	for _, bundle := range bundles {
		url, ok := processed[bundle.Name]
		if !ok {
			url = "/" + bundle.Name
		}
		switch bundle.Type {
		case "css":
			cssLinks.WriteString(`<link rel="stylesheet" href="` + url + `">` + "\n")
		case "js":
			jsLinks.WriteString(`<script src="` + url + `"></script>` + "\n")
		}
	}

	b.log.Info("Processed %d asset bundle(s)", len(bundles))

	return &renderer.AssetTags{
		CSS: strings.TrimRight(cssLinks.String(), "\n"),
		JS:  strings.TrimRight(jsLinks.String(), "\n"),
	}, nil
}

// loadAssetManifest returns bundles from assets.toml if present, otherwise
// builds default bundles by scanning static/css/ and static/js/.
func (b *Builder) loadAssetManifest(staticDir string) ([]assets.BundleConfig, error) {
	manifestPath := filepath.Join(b.projectRoot, "assets.toml")
	if _, err := os.Stat(manifestPath); err == nil {
		manifest, err := assets.LoadManifest(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("loading assets.toml: %w", err)
		}
		return manifest.Bundles, nil
	}
	return assets.DefaultBundles(staticDir)
}

// loadData reads all .toml files from the data/ directory.
func (b *Builder) loadData() (map[string]any, error) {
	data := make(map[string]any)
	dataDir := filepath.Join(b.projectRoot, "data")

	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return data, nil
	}

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, fmt.Errorf("reading data directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}

		path := filepath.Join(dataDir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading data file %s: %w", path, err)
		}

		var m map[string]any
		if err := toml.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("parsing data file %s: %w", path, err)
		}

		key := strings.TrimSuffix(entry.Name(), ".toml")
		data[key] = m
	}

	return data, nil
}

// copyStatic copies static files to the output directory, skipping any files
// that were aggregated into a bundle during this build.
func (b *Builder) copyStatic(staticDir, outputDir string) error {
	return filepath.Walk(staticDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Skip files that were included in an asset bundle.
		if b.bundledSources[path] {
			b.log.Detail("Skipping bundled source: %s", path)
			return nil
		}

		rel, err := filepath.Rel(staticDir, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(outputDir, rel)

		// Check cache to skip unchanged files
		changed, err := b.cache.HasChanged(path)
		if err != nil || changed {
			if err2 := copyFile(path, dst); err2 != nil {
				return fmt.Errorf("copying %s: %w", path, err2)
			}
			b.log.Detail("Copied static: %s → %s", path, dst)
			b.cache.Update(path) //nolint
		}

		return nil
	})
}

// checkGlobalFilesChanged returns true if config.toml or any data file changed.
func (b *Builder) checkGlobalFilesChanged() (bool, error) {
	// Check config.toml
	cfgPath := filepath.Join(b.projectRoot, "config.toml")
	changed, err := b.cache.HasChanged(cfgPath)
	if err != nil || changed {
		return true, err
	}

	// Check data/*.toml
	dataDir := filepath.Join(b.projectRoot, "data")
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return false, nil
	}

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		path := filepath.Join(dataDir, entry.Name())
		changed, err := b.cache.HasChanged(path)
		if err != nil || changed {
			return true, err
		}
	}

	return false, nil
}

// checkPartialsChanged returns true if any partial template changed.
func (b *Builder) checkPartialsChanged() (bool, error) {
	partialsDir := filepath.Join(b.projectRoot, "templates", "partials")
	if _, err := os.Stat(partialsDir); os.IsNotExist(err) {
		return false, nil
	}

	changed := false
	err := filepath.Walk(partialsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".html") {
			return nil
		}
		c, err := b.cache.HasChanged(path)
		if err != nil {
			return err
		}
		if c {
			changed = true
		}
		return nil
	})
	return changed, err
}

// checkTypeTemplateChanged checks if the type template for an item has changed.
func (b *Builder) checkTypeTemplateChanged(item *content.ContentItem) (bool, error) {
	tmplPath := filepath.Join(b.projectRoot, "templates", "types", item.Type+".html")
	if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
		tmplPath = filepath.Join(b.projectRoot, "templates", "types", "page.html")
	}
	return b.cache.HasChanged(tmplPath)
}

// updateGlobalFileHashes updates cache hashes for config and data files.
func (b *Builder) updateGlobalFileHashes() {
	cfgPath := filepath.Join(b.projectRoot, "config.toml")
	b.cache.Update(cfgPath) //nolint

	dataDir := filepath.Join(b.projectRoot, "data")
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return
	}
	entries, _ := os.ReadDir(dataDir)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".toml") {
			b.cache.Update(filepath.Join(dataDir, entry.Name())) //nolint
		}
	}
}

// updateTemplateHashes updates cache hashes for all template files.
func (b *Builder) updateTemplateHashes() {
	tmplDir := filepath.Join(b.projectRoot, "templates")
	filepath.Walk(tmplDir, func(path string, info os.FileInfo, err error) error { //nolint
		if err != nil || info.IsDir() {
			return err
		}
		if strings.HasSuffix(path, ".html") {
			b.cache.Update(path) //nolint
		}
		return nil
	})
}

// contentItemSourcePath reconstructs the source .md file path for a ContentItem.
func (b *Builder) contentItemSourcePath(item *content.ContentItem, contentDir string) string {
	// OutputPath encodes the full path info; we reconstruct from ID and type
	// For index.md style: ID = blog/my-post, Permalink ends with /
	if strings.HasSuffix(item.Permalink, "/") {
		return filepath.Join(contentDir, filepath.FromSlash(item.ID), "index.md")
	}
	return filepath.Join(contentDir, filepath.FromSlash(item.ID)+".md")
}

// buildListingPages auto-generates index.html pages for any content type that
// doesn't already have one. Pagination is supported: page 1 goes to
// outputDir/typeName/index.html, page N goes to outputDir/typeName/page/N/index.html.
func (b *Builder) buildListingPages(ctx context.Context, index *content.SiteIndex, r *renderer.Renderer, site *renderer.SiteContext, data map[string]any, assetTags *renderer.AssetTags) (int, error) {
	outputDir := filepath.Join(b.projectRoot, b.config.Build.OutputDir)
	built := 0

	for typeName, items := range index.ByType {
		select {
		case <-ctx.Done():
			return built, ctx.Err()
		default:
		}

		if typeName == "page" {
			continue // top-level pages don't need a collection listing
		}

		page1IndexPath := filepath.Join(outputDir, typeName, "index.html")

		// Skip if the regular build already produced an index for this type
		alreadyBuilt := false
		for _, item := range index.All {
			if item.OutputPath == page1IndexPath {
				alreadyBuilt = true
				break
			}
		}
		if alreadyBuilt {
			continue
		}

		pageSize := b.config.Site.PageSize
		if pageSize <= 0 {
			pageSize = 10
		}
		totalItems := len(items)
		totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))
		if totalPages == 0 {
			totalPages = 1
		}

		typePath := "/" + typeName

		for pageNum := 1; pageNum <= totalPages; pageNum++ {
			start := (pageNum - 1) * pageSize
			end := min(start+pageSize, totalItems)
			pageItems := items[start:end]

			pager := renderer.ComputePager(totalItems, pageSize, pageNum, typePath)

			var indexPath string
			if pageNum == 1 {
				indexPath = filepath.Join(outputDir, typeName, "index.html")
			} else {
				indexPath = filepath.Join(outputDir, typeName, "page", strconv.Itoa(pageNum), "index.html")
			}

			// Build a synthetic ContentItem for the listing page.
			title := titleCase(typeName)
			synthetic := &content.ContentItem{
				ID:         typeName,
				Type:       typeName,
				Title:      title,
				Permalink:  typePath + "/",
				OutputPath: indexPath,
			}

			vars := renderer.TemplateVars{
				Site:   site,
				Page:   synthetic,
				Data:   data,
				Items:  pageItems,
				Pager:  pager,
				Assets: assetTags,
			}

			html, err := r.RenderListing(synthetic, vars)
			if err != nil {
				b.log.Warn("Could not render listing for type %q: %v", typeName, err)
				break // non-fatal: missing listing template is acceptable
			}

			if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
				return built, fmt.Errorf("creating listing output dir for %s: %w", typeName, err)
			}

			if err := os.WriteFile(indexPath, []byte(html), 0644); err != nil {
				return built, fmt.Errorf("writing listing page for %s: %w", typeName, err)
			}

			b.log.Success("Built listing: %s", indexPath)
			built++
		}
	}

	return built, nil
}

// Check validates the project without building.
// It checks that config is valid, all content files have parseable front matter,
// and all templates compile without errors.
// Returns the number of issues found (0 = all clear).
func (b *Builder) Check() int {
	issues := 0

	// Config is already loaded and valid (New would have failed otherwise).
	b.log.Info("Config:    OK")

	// Check templates
	tmplDir := filepath.Join(b.projectRoot, "templates")
	if _, err := os.Stat(tmplDir); os.IsNotExist(err) {
		b.log.Warn("Templates directory not found — run 'arbor init' first")
		issues++
	} else {
		r := renderer.New(b.projectRoot, b.log)
		tmplIssues := r.CheckTemplates(tmplDir)
		tmplCount := countFiles(tmplDir, ".html")
		for rel, e := range tmplIssues {
			b.log.Error("Template  %s: %v", rel, e)
			issues++
		}
		if len(tmplIssues) == 0 {
			b.log.Info("Templates: OK (%d files)", tmplCount)
		} else {
			b.log.Warn("Templates: %d error(s) in %d files", len(tmplIssues), tmplCount)
		}
	}

	// Check content front matter
	contentDir := filepath.Join(b.projectRoot, "content")
	outputDir := filepath.Join(b.projectRoot, b.config.Build.OutputDir)
	if _, err := os.Stat(contentDir); os.IsNotExist(err) {
		b.log.Warn("Content directory not found — no content to check")
	} else {
		contentErrors := 0
		contentCount := 0
		filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error { //nolint
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
				return err
			}
			contentCount++
			if _, parseErr := content.ParseFile(path, contentDir, outputDir); parseErr != nil {
				b.log.Error("Content  %s: %v", path, parseErr)
				contentErrors++
				issues++
			}
			return nil
		})
		if contentErrors == 0 {
			b.log.Info("Content:   OK (%d files)", contentCount)
		} else {
			b.log.Warn("Content:   %d error(s) in %d files", contentErrors, contentCount)
		}
	}

	return issues
}

// countFiles returns the number of files with the given extension under dir.
func countFiles(dir, ext string) int {
	n := 0
	filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error { //nolint
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ext) {
			n++
		}
		return nil
	})
	return n
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// titleCase capitalises the first letter of s.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// copyFile copies a file from src to dst, creating parent directories as needed.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
