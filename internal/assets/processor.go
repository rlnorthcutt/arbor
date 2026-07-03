package assets

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tdewolff/minify/v2"
	minCSS "github.com/tdewolff/minify/v2/css"
	minJS "github.com/tdewolff/minify/v2/js"
)

// Processor handles asset aggregation and minification.
type Processor struct {
	m *minify.M
}

// NewProcessor creates a Processor with CSS and JS minifiers registered.
func NewProcessor() *Processor {
	m := minify.New()
	m.AddFunc("text/css", minCSS.Minify)
	m.AddFunc("application/javascript", minJS.Minify)
	return &Processor{m: m}
}

// ProcessBundles reads each bundle's sources, concatenates them, optionally
// minifies the result, writes it to outputDir, and returns a map of
// bundle.Name → versioned URL (e.g. "/css/bundle.css?v=deadbeef").
func (p *Processor) ProcessBundles(bundles []BundleConfig, outputDir string, shouldMinify bool) (map[string]string, error) {
	result := make(map[string]string)

	for _, bundle := range bundles {
		var sb strings.Builder
		for _, src := range bundle.Sources {
			data, err := os.ReadFile(src)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", src, err)
			}
			sb.Write(data)
			sb.WriteByte('\n')
		}

		content := sb.String()

		if shouldMinify && content != "" {
			mimeType := mimeTypeFor(bundle.Type)
			minified, err := p.m.String(mimeType, content)
			if err != nil {
				return nil, fmt.Errorf("minifying bundle %s: %w", bundle.Name, err)
			}
			content = minified
		}

		outPath := filepath.Join(outputDir, filepath.FromSlash(bundle.Name))
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return nil, fmt.Errorf("creating output dir for bundle %s: %w", bundle.Name, err)
		}
		if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("writing bundle %s: %w", bundle.Name, err)
		}

		hash := sha256.Sum256([]byte(content))
		hashStr := fmt.Sprintf("%x", hash[:4])
		result[bundle.Name] = "/" + bundle.Name + "?v=" + hashStr
	}

	return result, nil
}

func mimeTypeFor(assetType string) string {
	if assetType == "css" {
		return "text/css"
	}
	return "application/javascript"
}
