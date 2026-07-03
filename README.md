# Arbor

A fast, convention-driven static site generator for people who want to ship sites, not configure tools.

Arbor sits between a Markdown converter and a full framework like Hugo. It trades some flexibility for a dramatically faster setup experience — sensible defaults, a clean templating system based on familiar Jinja2 syntax, and three ready-to-use site blueprints.

---

## Quick Start

```bash
# Install (build from source)
git clone https://github.com/rlnorthcutt/arbor
cd arbor
go build -o arbor .

# Create a new site
arbor init                        # blog (default)
arbor init --blueprint marketing  # landing page
arbor init --blueprint docs       # documentation site

# Write
arbor new blog my-first-post      # creates content/blog/my-first-post.md

# Preview with live reload
arbor preview

# Build for production
arbor build
```

---

## CLI Reference

```
arbor [OPTIONS] COMMAND

Commands:
  init     Initialize a new project in the current directory
           --blueprint  blog (default) | marketing | docs
  new      Create a content stub
           Usage: arbor new [TYPE] [NAME]
           Example: arbor new blog my-post → content/blog/my-post.md
  build    Build the site to public/
           --force         Ignore cache, full rebuild
           --no-minify     Disable CSS/JS minification (default: enabled)
           --no-aggregate  Disable CSS/JS bundling (default: enabled)
  preview  Build and serve locally with live reload
           --port          Port to serve on (default: 8080)
           --force         Force full rebuild before serving
           --no-minify     Disable minification during preview
           --no-aggregate  Disable bundling during preview
  check    Validate config, templates, and content without building
  version  Print the version
  help     Show this message

Global options:
  --root     Project root directory (default: ./)
  --verbose  Show per-file detail during builds
```

---

## Blueprints

Run `arbor init --blueprint <name>` to start from a purpose-built scaffold.

### `blog` (default)

Personal blog or journal. Gets you:

- Home page with recent posts
- Blog listing with pagination
- Individual post template with tags
- About page
- Card and teaser display templates

```bash
arbor init
arbor preview
# → http://localhost:8080
```

### `marketing`

Landing page for an open source project or small business. Gets you:

- Hero section with headline, tagline, and CTA
- Features grid (driven by `data/features.toml`)
- Call-to-action section
- About page
- Sticky header with dark-mode toggle

```bash
arbor init --blueprint marketing
arbor preview
```

### `docs`

Documentation site for a software project. Gets you:

- Persistent sidebar navigation
- Two-column docs layout
- Four starter pages (getting started, configuration, templates, CLI reference)
- Auto-generated docs listing

```bash
arbor init --blueprint docs
arbor preview
```

---

## Project Structure

```
.
├── config.toml          # Site settings
├── content/             # Markdown source files
│   ├── blog/
│   │   └── my-post.md   # Type inferred from first subdirectory
│   └── about.md         # Top-level → type "page"
├── templates/
│   ├── layouts/         # Base HTML wrappers (extend these)
│   ├── types/           # One template per content type
│   ├── displays/        # Reusable rendering modes (card, teaser, full)
│   └── partials/        # Structural fragments (header, footer, nav)
├── data/                # TOML data files, available as Data.filename
├── static/              # Copied to public/ as-is
└── public/              # Build output (git-ignored)
```

---

## Templates

Arbor uses [Pongo2](https://github.com/flosch/pongo2) — Jinja2/Django-compatible syntax.

### Template inheritance

```html
<!-- templates/types/blog.html -->
{% extends "templates/layouts/base.html" %}

{% block title %}{{ Page.Title }} | {{ Site.Config.Site.Title }}{% endblock %}

{% block body %}
<article>
  <h1>{{ Page.Title }}</h1>
  <time>{{ Page.Date|godate:"January 2, 2006" }}</time>
  {{ Page.HTMLContent|safe }}
</article>
{% endblock %}
```

### Template variables

Every template receives:

| Variable | Type | Description |
|---|---|---|
| `Site` | `SiteContext` | `Site.Config` (config.toml) + `Site.Index` (all content) |
| `Page` | `*ContentItem` | Current page being rendered |
| `Data` | `map[string]any` | All `data/*.toml` files by filename |
| `Items` | `[]*ContentItem` | All published items of the current type |
| `Pager` | `*Pager` | Pagination state on listing pages (nil otherwise) |
| `Assets` | `*AssetTags` | Pre-built `<link>` and `<script>` HTML strings (`Assets.CSS`, `Assets.JS`) |

### Useful patterns

```html
{{# Render HTML content without escaping #}}
{{ Page.HTMLContent|safe }}

{{# Format a date with Go's reference time #}}
{{ Page.Date|godate:"January 2, 2006" }}

{{# Access custom front matter fields #}}
{{ Page.Meta.Fields.hero_image }}

{{# Include a display template with a specific item #}}
{% for item in Items %}
  {% with item=item %}{% include "templates/displays/card.html" %}{% endwith %}
{% endfor %}

{{# Pagination controls #}}
{% if Pager %}
  {% if Pager.HasPrev %}<a href="{{ Pager.PrevURL }}">← Newer</a>{% endif %}
  {% if Pager.HasNext %}<a href="{{ Pager.NextURL }}">Older →</a>{% endif %}
{% endif %}

{{# Access data files #}}
{% for item in Data.nav.items %}
  <a href="{{ item.url }}">{{ item.label }}</a>
{% endfor %}
```

### Template resolution

For a content item, Arbor looks for templates in this order:

1. `frontmatter.template` if set (full path override)
2. `templates/types/{type}.html` where type = first subdirectory under `content/`
3. `templates/types/page.html` (final fallback)

For auto-generated listing pages:

1. `templates/types/{type}-list.html`
2. `templates/types/listing.html`

---

## Content & Front Matter

```toml
+++
title    = "My Post"
date     = 2025-06-15
tags     = ["go", "web"]
draft    = false
template = "templates/types/custom.html"  # optional override

[extra]
hero_image = "/img/hero.jpg"
show_toc   = true
+++

Your **Markdown** content here.
```

Draft files are excluded from builds unless `draft_mode = true` in `config.toml`.

Custom fields under `[extra]` are available in templates as `{{ Page.Meta.Fields.hero_image }}`.

---

## Shortcodes

Embed Pongo2 partials inside Markdown content:

```
{{% partial "templates/partials/callout" type="warning" %}}
Watch out for this thing.
{{% /partial %}}
```

The partial receives `type` as a template variable alongside `Site` and `Data`. A block shortcode's inner content arrives as `body`.

---

## Data Files

Any `.toml` file in `data/` is loaded and made available in templates as `Data.<filename>`:

```toml
# data/features.toml
[[items]]
title = "Fast"
desc  = "Generates in milliseconds."
```

```html
{% for f in Data.features.items %}
  <h3>{{ f.title }}</h3>
{% endfor %}
```

---

## `config.toml`

```toml
[site]
title     = "My Site"
base_url  = "https://example.com"
language  = "en"
page_size = 10          # items per listing page

[build]
draft_mode = false      # set true to include drafts
output_dir = "public"

[author]
name  = "Your Name"
email = "you@example.com"

[assets]
aggregate = true        # bundle CSS/JS (default: true for build, false for preview)
minify    = true        # minify bundles (default: true for build, false for preview)
```

---

## Incremental Builds

Arbor caches a SHA-256 hash of every source file in `.arbor-cache.json`. On subsequent builds it only re-renders what changed:

| Changed | Action |
|---|---|
| A content file | Re-render that page only |
| A type or display template | Re-render all pages using it |
| A partial | Full rebuild (partials can appear anywhere) |
| `config.toml` or any data file | Full rebuild |
| A static asset | Copy that file only |
| A CSS/JS source file (when aggregating) | Re-processed into the bundle on next build |

`arbor build --force` ignores the cache and rebuilds everything.

---

## Default CSS Stack

Every blueprint ships with [Ivy](https://github.com/rlnorthcutt/ivy) (design tokens) and [Lattice](https://github.com/rlnorthcutt/lattice) (utility classes), plus a `dark-mode-toggle` web component. Override tokens in `static/css/site.css`:

```css
:root {
  --color-primary-light: #2563eb;
  --color-primary-dark:  #93c5fd;
  --font-sans: "Your Font", system-ui, sans-serif;
  --lat-container-width: 1100px;
}
```

---

## Asset Aggregation & Minification

Arbor can bundle CSS and JS files into single outputs with cache-busting query strings, and optionally minify them for production.

**Default behavior:**

| Command | Aggregate | Minify |
|---|---|---|
| `arbor build` | yes | yes |
| `arbor preview` | no | no |

Override per-run with `--no-aggregate` / `--no-minify`, or set a project default in `config.toml`:

```toml
[assets]
aggregate = true   # bundle CSS/JS files into one output each
minify    = true   # minify the output
```

### How bundling works

When aggregation is enabled, Arbor:

1. Reads all `.css` files from `static/css/` (alphabetically) → writes `public/css/bundle.css`
2. Reads all `.js` files from `static/js/` (alphabetically) → writes `public/js/bundle.js`
3. Appends a short content hash as a query string (`?v=deadbeef`) for cache-busting
4. Skips copying the individual source files to `public/` (they are already bundled)

When aggregation is disabled, each file is copied to `public/` as-is and linked individually.

### Customising bundles with `assets.toml`

For full control over bundle order, multiple outputs, or non-default file paths, create an `assets.toml` at the project root:

```toml
[[bundle]]
name    = "css/bundle.css"
type    = "css"
sources = [
  "static/css/ivy.full.css",
  "static/css/lattice.full.css",
  "static/css/site.css",
]

[[bundle]]
name    = "js/bundle.js"
type    = "js"
sources = [
  "static/js/dark-mode-toggle.js",
  "static/js/app.js",
]
```

Each `[[bundle]]` entry controls one output file. `name` is the output path relative to `public/`. When `assets.toml` is present it takes precedence over the default scan of `static/css/` and `static/js/`.

### Using asset tags in templates

The `Assets` variable is available in every template as pre-built HTML strings:

```html
{{ Assets.CSS|safe }}   {{# renders <link rel="stylesheet" ...> tags #}}
{{ Assets.JS|safe }}    {{# renders <script src="..."> tags #}}
```

Blueprint base layouts already include these in `<head>`. If you build a custom layout, add them yourself:

```html
<head>
  {{ Assets.CSS|safe }}
  {{ Assets.JS|safe }}
</head>
```

---

## TODO

Items planned for future releases:

- **RSS feed generation** — auto-generate `feed.xml` for content types that opt in via config
- **Sitemap generation** — generate `sitemap.xml` on build for SEO
- **`arbor demo` command** — add example content to an existing project without re-running `init`
- **Tag listing pages** — auto-generate `public/tags/{tag}/index.html` for each tag in the content index
- **Image processing** — resize/optimize images during build (likely as a build hook)
- **Deploy integration** — `arbor deploy` command wrapping common targets (Netlify, Cloudflare Pages, GitHub Pages)
- **Plugin/hook system** — run shell commands before/after build steps for custom processing
