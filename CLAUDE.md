# Arbor — Developer Guide

Arbor is a Go-based static site generator CLI. It is intentionally simpler than Hugo: convention-heavy, fast to set up, not trying to do everything.

## Commands

```bash
go build -o arbor .          # build the binary
go test ./...                # run all tests
go vet ./...                 # static analysis
./arbor init                 # scaffold a new site in ./
./arbor build                # build to public/
./arbor preview              # build + serve with live reload
```

## Project Layout

```
main.go                      # CLI entry point, flag parsing, command dispatch
internal/
  builder/    builder.go     # orchestrates the full build pipeline
  cache/      cache.go       # SHA-256 incremental build cache (.arbor-cache.json)
  config/     config.go      # config.toml loader → Config struct
  content/    content.go     # front matter parser, ContentItem, SiteIndex
  renderer/   renderer.go    # Pongo2 template rendering, template resolution
  scaffold/   scaffold.go    # arbor init and arbor new (content stub creation)
  blueprints/                # embedded site blueprints (blog, marketing, docs)
    base/                    # shared static assets for all blueprints
    blog/
    marketing/
    docs/
  server/     server.go      # preview HTTP server, fsnotify watcher, WebSocket
  shortcode/  shortcode.go   # two-pass shortcode pre/post processor
```

## Templating: Pongo2

Templates use **Pongo2** (Jinja2/Django syntax) with the `.html` extension. The renderer creates one `*pongo2.TemplateSet` at construction and reuses it — never create a new set per render call.

**Key syntax:**
```html
{% extends "templates/layouts/base.html" %}
{% block title %}{{ Page.Title }}{% endblock %}
{% block body %}
  {{ Page.HTMLContent|safe }}
  {{ Page.Date|date:"January 2, 2006" }}
  {% for item in Items %}
    {% with item=item %}{% include "templates/displays/teaser.html" %}{% endwith %}
  {% endfor %}
  {% if Pager.HasNext %}<a href="{{ Pager.PrevURL }}">Next</a>{% endif %}
{% endblock %}
```

**Template variables available in every render:**
- `Site` — `SiteContext{Config, Index}` 
- `Page` — `*ContentItem` (nil on auto-generated listing pages)
- `Data` — `map[string]any` keyed by data filename (`Data.nav`, `Data.team`)
- `Items` — `[]*ContentItem` for the current type
- `Pager` — `*Pager` (non-nil on paginated listing pages only)

HTML is auto-escaped by default. Use `{{ value|safe }}` for trusted HTML (e.g., `Page.HTMLContent`).

## Template Resolution

For a `ContentItem`, template lookup order:
1. `item.Template` (front matter override, full path)
2. `templates/types/{item.Type}.html`
3. `templates/types/page.html` (final fallback)

For auto-generated listing pages:
1. `templates/types/{type}-list.html`
2. `templates/types/listing.html`

## Blueprints

Blueprints are embedded in the binary via `//go:embed`. Each blueprint is a complete project scaffold. `arbor init --blueprint <name>` copies `base/` then the chosen blueprint on top, never overwriting existing files.

To add a new blueprint:
1. Create `internal/blueprints/<name>/` with `config.toml`, `content/`, `data/`, `templates/`
2. Ensure `arbor init && arbor preview` produces a navigable site with no config changes
3. Register the name in the `init` command's `--blueprint` flag validation

## Pongo2 Template Constraints

Pongo2 cannot call Go methods that take arguments from templates. Patterns to follow:

- `{{ Page.Meta.Fields.hero_image }}` ✓ — direct map access (`ContentMeta.Fields` is exported)
- `{{ Page.Meta.GetString "hero_image" }}` ✗ — not valid Pongo2 (method with argument)
- `{{ Page.HTMLContent|safe }}` — `|safe` marks HTML as trusted, prevents auto-escaping
- `{{ Page.Date|godate:"January 2, 2006" }}` — custom filter using Go reference time format

`ContentMeta.Fields` is an exported `map[string]any`. Pongo2 can traverse it with dot notation: `Page.Meta.Fields.my_key`. The `GetString`, `GetBool`, `GetInt` methods are still available for use in Go code only.

## Build Cache

`.arbor-cache.json` maps `filepath → sha256`. `cache.HasChanged(path)` returns true if the hash differs or the file is unseen. The cache is saved atomically only on a successful full build — an interrupted build does not corrupt it.

## Front Matter

Files use TOML front matter delimited by `+++`. The `date` field is a bare TOML local date (`2025-06-15`) — the parser pre-processes it to a quoted string before `go-toml` unmarshals it, since go-toml v1 does not support TOML local dates natively.

## Content Type Inference

A file's content type is its first subdirectory under `content/`:
- `content/blog/my-post.md` → type `blog`
- `content/about.md` → type `page`
- `content/blog/series/part-1.md` → type `blog` (first dir wins, nesting ignored)

## Shortcodes

Shortcodes use `{{% partial "path" key=value %}}` syntax, processed in two passes around Goldmark. The pre-pass extracts shortcodes and replaces them with tokens like `__ARBOR_SC_0__`; the post-pass renders the Pongo2 partial and substitutes. Block shortcodes wrap body text: `{{% partial "callout" %}}`...`{{% /partial %}}`.
