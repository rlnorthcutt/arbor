package assets

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml"
)

// BundleManifest holds the full set of bundles from assets.toml.
type BundleManifest struct {
	Bundles []BundleConfig `toml:"bundle"`
}

// BundleConfig describes a single named bundle.
type BundleConfig struct {
	Name    string   `toml:"name"`
	Type    string   `toml:"type"` // "css" or "js"
	Sources []string `toml:"sources"`
}

// LoadManifest reads an assets.toml file and returns the parsed manifest.
func LoadManifest(path string) (*BundleManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m BundleManifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// DefaultBundles creates conventional bundles by scanning staticDir for CSS and
// JS files. CSS files in static/css/ become css/bundle.css; JS files in
// static/js/ become js/bundle.js. Files are sorted alphabetically.
func DefaultBundles(staticDir string) ([]BundleConfig, error) {
	var bundles []BundleConfig

	cssBundle, err := defaultBundle(staticDir, "css", "css/bundle.css")
	if err != nil {
		return nil, err
	}
	if cssBundle != nil {
		bundles = append(bundles, *cssBundle)
	}

	jsBundle, err := defaultBundle(staticDir, "js", "js/bundle.js")
	if err != nil {
		return nil, err
	}
	if jsBundle != nil {
		bundles = append(bundles, *jsBundle)
	}

	return bundles, nil
}

func defaultBundle(staticDir, subDir, bundleName string) (*BundleConfig, error) {
	dir := filepath.Join(staticDir, subDir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	ext := "." + subDir // ".css" or ".js"
	var sources []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ext) {
			sources = append(sources, filepath.Join(staticDir, subDir, e.Name()))
		}
	}
	sort.Strings(sources)

	if len(sources) == 0 {
		return nil, nil
	}

	return &BundleConfig{
		Name:    bundleName,
		Type:    subDir,
		Sources: sources,
	}, nil
}

// ListStaticAssets returns sorted slices of CSS and JS paths (relative to
// staticDir) found directly in static/css/ and static/js/.
func ListStaticAssets(staticDir string) (cssFiles, jsFiles []string) {
	cssDir := filepath.Join(staticDir, "css")
	if _, err := os.Stat(cssDir); err == nil {
		filepath.Walk(cssDir, func(path string, info os.FileInfo, err error) error { //nolint
			if err == nil && !info.IsDir() && strings.HasSuffix(path, ".css") {
				if rel, e := filepath.Rel(staticDir, path); e == nil {
					cssFiles = append(cssFiles, rel)
				}
			}
			return nil
		})
	}
	sort.Strings(cssFiles)

	jsDir := filepath.Join(staticDir, "js")
	if _, err := os.Stat(jsDir); err == nil {
		filepath.Walk(jsDir, func(path string, info os.FileInfo, err error) error { //nolint
			if err == nil && !info.IsDir() && strings.HasSuffix(path, ".js") {
				if rel, e := filepath.Rel(staticDir, path); e == nil {
					jsFiles = append(jsFiles, rel)
				}
			}
			return nil
		})
	}
	sort.Strings(jsFiles)

	return cssFiles, jsFiles
}
