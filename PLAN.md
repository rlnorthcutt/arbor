# Arbor — Static Site Generator

---

## 1. Project Philosophy

Arbor sits between a Markdown-to-HTML converter and a full framework like Hugo. The guiding principles:

- **Convention over configuration** — sensible defaults that work without explicit setup
- **No magic** — template resolution, data access, and output paths are predictable and traceable
- **Good errors, not strict errors** — warn clearly and continue where possible; fail hard only when continuing would produce broken output
- **Speed and simplicity over flexibility** — Arbor trades some configurability for a much faster setup experience; it is not trying to be Hugo
- **Your stack, your rules** — Arbor ships with Ivy + Lattice as its default CSS foundation, but imposes nothing on the user's own templates

---

## 2. Technology Stack

| Concern | Library | Notes |
|---|---|---|
| CLI, logging, config | `github.com/rlnorthcutt/cmdkit` | `logger`, `ui`, `sys` packages |
| Configuration / Front Matter | `github.com/pelletier/go-toml` | TOML only |
| Markdown Rendering | `github.com/yuin/goldmark` | CommonMark compliant |
| Templating | `github.com/flosch/pongo2/v6` | Jinja2/Django-style syntax |
| File Watching | `github.com/fsnotify/fsnotify` | Preview mode only |
| Build Cache | Custom | SHA-256 hash index, `.arbor-cache.json` |
| Default CSS | Ivy + Lattice + dark-mode-toggle | Bundled into each blueprint |

**Why Pongo2 over Jet:** Pongo2 uses Jinja2/Django template syntax (`{% block %}`, `{% extends %}`, `{% include %}`, `{{ variable }}`) that is immediately familiar to a large population of web developers and requires no ramp-up. It has HTML auto-escaping on by default (use `|safe` for trusted HTML), rich built-in filters (`|date`, `|truncatewords`, `|striptags`, etc.), and a well-understood template inheritance model. Template files use the `.html` extension, which editors handle natively.

---

## 3. cmdkit Integration

Arbor uses all three cmdkit packages. No globals — everything flows through instantiated structs.

```go
func main() {
    root := flag.String("root", "./", "Project root directory")
    verbose := flag.Bool("verbose", false, "Enable verbose logging")
    flag.Parse()

    log := logger.New(*verbose)
    userUI := ui.New(false).
        WithLogger(log).
        WithInterrupt(context.Background())
    defer userUI.StopSignal()

    args := flag.Args()
    // ...
}
```

**Logger usage throughout Arbor:**

| Situation | Method |
|---|---|
| Normal build progress | `log.Info("Building %d pages...", count)` |
| File written successfully | `log.Success("Built %s", outputPath)` |
| Per-file details (verbose only) | `log.Detail("Rendering %s → %s", src, out)` |
| Skipped draft, missing optional field | `log.Warn("Skipping draft: %s", path)` |
| Missing required template, bad TOML | `log.Fatal("Template not found: %s", path)` |
| Dev-only debugging | `log.Debug(...)` — remove before shipping |

`log.Fatal` calls `os.Exit(1)` after printing — use it only for the hard-fail cases in §17.

**Signal handling:** `userUI.WithInterrupt` is registered in the root command. The preview server's watcher loop and `http.Server` both receive `userUI.Ctx` so that Ctrl+C shuts everything down cleanly.

---

## 4. Directory Structure

```text
.
├── config.toml              # Global site settings
├── .arbor-cache.json        # Incremental build cache (gitignore)
├── data/                    # Structured data files
│   ├── nav.toml
│   └── team.toml
├── content/                 # All source content
│   ├── blog/
│   │   └── my-post.md
│   └── about.md
├── templates/
│   ├── layouts/             # Base HTML wrappers
│   │   └── base.html
│   ├── types/               # One template per content type
│   │   ├── blog.html
│   │   └── page.html        # Default fallback
│   ├── displays/            # Content rendering modes
│   │   ├── card.html
│   │   ├── teaser.html
│   │   └── full.html
│   └── partials/            # Structural UI shell fragments
│       ├── header.html
│       ├── footer.html
│       └── nav.html
├── static/                  # Copied to /public as-is
│   ├── css/
│   │   ├── ivy.full.css
│   │   ├── lattice.full.css
│   │   └── site.css         # User's own overrides
│   └── js/
│       └── dark-mode-toggle.js
└── public/                  # Build output (gitignore)
```

All template files use the `.html` extension. Pongo2 resolves them from the project root.

---

## 5. Templates: Displays vs. Partials

This is a load-bearing distinction.

**Partials** (`templates/partials/`) are **structural UI shell fragments** — they build the page's chrome and have no specific knowledge of a `ContentItem`. They receive `Site` and `Data`. Examples: site header, nav bar, footer, cookie banner. You never pass a content item to a partial.

**Displays** (`templates/displays/`) are **content rendering modes** — they receive a `ContentItem` and render it in a particular visual format. This is the Drupal "view mode" concept: the same blog post can be rendered as a `card` (image + title + excerpt), a `teaser` (title + date + first paragraph), or `full` (complete rendered HTML).

**Calling a display from a type template (Pongo2):**
```html
{% for item in Items %}
  {% with item=item %}
    {% include "templates/displays/teaser.html" %}
  {% endwith %}
{% endfor %}
```

The `{% with %}` block makes the loop variable available as a named binding inside the included display template. Displays and partials are never interchangeable.

---

## 6. Ivy + Lattice: The Default CSS Stack

Each blueprint scaffolds `static/css/` with the Ivy + Lattice files and a starter `site.css`. The default `base.html` layout already wires them in using the correct load order.

**Default `templates/layouts/base.html`:**
```html
<!DOCTYPE html>
<html lang="{{ Site.Config.Site.Language }}">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{% block title %}{{ Site.Config.Site.Title }}{% endblock %}</title>
  <link rel="stylesheet" href="/css/ivy.full.css">
  <link rel="stylesheet" href="/css/lattice.full.css">
  <link rel="stylesheet" href="/css/site.css">
  <script type="module" src="/js/dark-mode-toggle.js"></script>
</head>
<body class="lattice">
  {% include "templates/partials/header.html" %}
  <main class="container">
    {% block body %}{% endblock %}
  </main>
  {% include "templates/partials/footer.html" %}
</body>
</html>
```

**Default `templates/partials/header.html`:**
```html
<header class="full-width">
  <div class="grid md-col-2 items-center p-3">
    <div><a href="/">{{ Site.Config.Site.Title }}</a></div>
    <nav class="d-flex justify-end gap-3">
      {% for item in Data.nav.items %}
        <a href="{{ item.url }}">{{ item.label }}</a>
      {% endfor %}
      <dark-mode-toggle></dark-mode-toggle>
    </nav>
  </div>
</header>
```

**Ivy token overrides** go in `static/css/site.css`. Always use the `-light`/`-dark` variant pairs:
```css
:root {
  --color-primary-light: #2563eb;
  --color-primary-dark:  #93c5fd;
  --font-sans: "Your Font", system-ui, sans-serif;
}
```

**Lattice container width** override also in `site.css`:
```css
:root {
  --lat-container-width: 1100px;
}
```

---

## 7. Content Item & Site Index Structs

```go
// ContentItem is the canonical representation of a single piece of content.
type ContentItem struct {
    ID          string       // Slug derived from path: "blog/my-post"
    Type        string       // Inferred from top-level content dir, or front matter override
    Title       string
    Date        time.Time
    Tags        []string
    Draft       bool
    Template    string       // Optional front matter override (full path)
    Permalink   string       // Computed URL: "/blog/my-post.html"
    OutputPath  string       // Absolute path in /public
    RawContent  string       // Original Markdown
    HTMLContent string       // Goldmark-rendered HTML
    Meta        ContentMeta  // Safe accessor for arbitrary front matter fields
}

// ContentMeta wraps custom front matter fields with non-panicking accessors.
type ContentMeta struct {
    fields map[string]any
}

func (m ContentMeta) Get(key string) (any, bool)
func (m ContentMeta) GetString(key string) string  // "" if missing or wrong type
func (m ContentMeta) GetBool(key string) bool
func (m ContentMeta) GetInt(key string) int
func (m ContentMeta) Has(key string) bool

// SiteIndex is the complete content graph passed to every template.
type SiteIndex struct {
    All    []*ContentItem
    ByType map[string][]*ContentItem  // "blog" → [...]
    ByTag  map[string][]*ContentItem  // "golang" → [...]
    BySlug map[string]*ContentItem    // "blog/my-post" → &ContentItem{}
}
```

**Template usage:**
- `{{ Page.Meta.Fields.hero_image }}` — direct map access (preferred in templates; `Fields` is exported)
- `{{ Page.Meta.GetString("hero_image") }}` — safe accessor from Go code; returns `""` if missing
- `{{ Page.HTMLContent|safe }}` — render Goldmark HTML without escaping
- `{{ Page.Date|godate:"January 2, 2006" }}` — formatted date using the custom `godate` filter with Go's reference time format

Note: Pongo2 cannot call Go methods with arguments from templates. Use `Page.Meta.Fields.key` for direct map access in templates, and `meta.GetString("key")` in Go code.

---

## 8. Global Template Context

Every template receives these top-level variables:

| Variable | Type | Contents |
|---|---|---|
| `Site` | `SiteContext` | `config.toml` fields at `Site.Config` + the full `SiteIndex` at `Site.Index` |
| `Page` | `*ContentItem` | Current page (nil on virtual listing pages) |
| `Data` | `map[string]any` | All `/data/*.toml` files, keyed by filename: `Data.nav`, `Data.team` |
| `Items` | `[]*ContentItem` | Published items for the current page (for listing pages: the current page's slice) |
| `Pager` | `*Pager` | Pagination state (nil for non-listing pages) |

Note: Pongo2 context variables are accessed without a leading dot — `Site.Config.Site.Title`, not `.Site.Config.Site.Title`.

---

## 9. Front Matter Format

```toml
+++
title    = "My First Post"
date     = 2025-06-15
tags     = ["golang", "ssg"]
draft    = false
template = "templates/types/custom.html"   # optional full-path override

[extra]
hero_image = "/static/img/hero.jpg"
show_toc   = true
+++

Your **Markdown** content starts here.
```

Fields under `[extra]` land in `ContentItem.Meta` and are accessed via `{{ Page.Meta.GetString("hero_image") }}`.

---

## 10. Template Resolution Cascade

For any `ContentItem`, the template resolves in this exact order — no exceptions:

1. `frontmatter.template` is set → use that path directly
2. `templates/types/{type}.html` exists → use it (`type` = first subdirectory under `content/`)
3. `templates/types/page.html` → always the final fallback

For nested paths like `content/blog/series/post.md`, the type is always inferred from the **first subdirectory** (`blog`). Nesting depth does not affect resolution.

---

## 11. Output Path Convention

| Content File | Output File | URL |
|---|---|---|
| `content/about.md` | `public/about.html` | `/about.html` |
| `content/blog/my-post.md` | `public/blog/my-post.html` | `/blog/my-post.html` |
| `content/blog/my-post/index.md` | `public/blog/my-post/index.html` | `/blog/my-post/` |

The `index.md` pattern is the recommended way to get clean URLs. No permalink configuration in v1.

---

## 12. Shortcode / Inline Component Syntax

Two forms, both using `{{% %}}` delimiters to avoid collision with Goldmark:

**Self-closing:**
```
{{% partial "displays/card" item=.Page %}}
```

**With body:**
```
{{% partial "partials/callout" type="warning" %}}
Watch out for this thing.
{{% /partial %}}
```

**Implementation:** a two-pass processor wraps Goldmark.

1. **Pre-pass (before Goldmark):** scan for `{{% ... %}}` patterns, extract and store each shortcode call with its parameters into a map keyed by a unique placeholder token (`__ARBOR_SC_0__`, etc.), and replace the shortcode syntax in the Markdown source with the token.
2. **Goldmark renders** the Markdown with tokens in place (they survive as plain text inside the HTML).
3. **Post-pass (after Goldmark):** scan rendered HTML for placeholder tokens, render each corresponding Pongo2 partial, substitute.

Parameters are `key=value` pairs parsed from the shortcode tag. The called partial receives them as template variables alongside `Site` and `Data`. If a body was provided, it arrives as `body` (pre-rendered HTML string).

Template files referenced in shortcodes use the `.html` extension and are resolved from the project root.

---

## 13. Pagination

Pagination is pre-computed by the renderer before the template executes. Listing pages receive `Items` (the slice for the current page only) and `Pager` (pagination state) as template variables — no function call is required inside the template.

The builder generates one output file per pagination page for each content type.

**Example listing template (`templates/types/blog.html`):**
```html
{% extends "templates/layouts/base.html" %}

{% block title %}Blog{% endblock %}

{% block body %}
<h1>Blog Posts</h1>

{% for item in Items %}
  {% with item=item %}
    {% include "templates/displays/teaser.html" %}
  {% endwith %}
{% endfor %}

<nav class="pagination">
  {% if Pager.HasPrev %}<a href="{{ Pager.PrevURL }}">← Previous</a>{% endif %}
  {% if Pager.HasNext %}<a href="{{ Pager.NextURL }}">Next →</a>{% endif %}
</nav>
{% endblock %}
```

**`Pager` fields:** `Current` (1-indexed), `Total`, `HasPrev`, `HasNext`, `PrevURL`, `NextURL`.

Output convention: `public/blog/index.html`, `public/blog/page/2/index.html`, etc.

`PageSize` defaults to `10`, configurable in `config.toml`. The first page is always the canonical index, never `/page/1/`.

---

## 14. Incremental Build Cache

Arbor maintains `.arbor-cache.json` — a map of `source_path → sha256_hash` for every file that contributes to the build.

**Re-render rules:**

| Changed file | Action |
|---|---|
| A content file | Re-render that `ContentItem` only |
| A type or display template (`.html`) | Re-render all items using that template |
| A partial (`.html`) | Re-render all items (partials can be used anywhere) |
| `config.toml` or any `data/*.toml` | Full rebuild (global context changed) |
| A static asset | Copy that file only |

**Full rebuild:** `arbor build --force` ignores the cache entirely.

**Cache write:** updated atomically on successful build completion. A failed build does not update the cache, so interrupted builds are safe to resume.

---

## 15. CLI Reference

```
arbor [OPTIONS] COMMAND

Commands:
  init          Initialize a new Arbor project in the current directory
                Flags: --blueprint  Site blueprint to use (default: blog)
                                    Options: blog, marketing, docs
  new           Create a new content file with scaffolded front matter
                Usage: arbor new [CONTENTTYPE] [FILENAME]
                Example: arbor new blog my-first-post
                Creates: content/blog/my-first-post.md
  build         Build the site to /public
                Flags: --force    Ignore cache, full rebuild
  preview       Build, serve locally, and live-reload on changes
                Flags: --port     Local port (default: 8080)
  help          Show this help message

Global Options:
  -r, --root    Project root directory (default: ./)
      --verbose Enable verbose logging (shows Detail-level output)
```

---

## 16. Preview Server & Live Reload

`arbor preview`:

1. Runs a full build (respecting the cache)
2. Starts `http.FileServer` on `public/` at `--port`
3. Starts `fsnotify` watcher on `content/`, `templates/`, `data/`, `config.toml`
4. Injects a `<script>` block into every HTML response (preview mode only) that opens a WebSocket to the Arbor dev server
5. On any file change: runs incremental build → sends `reload` over the WebSocket

The injected script is ~15 lines of vanilla JS and is **never written to `public/`** — it is added only to HTTP responses in preview mode, leaving the built output clean. The WebSocket server runs on `--port + 1` by default.

Both the file server and WebSocket server receive `userUI.Ctx`, so Ctrl+C triggers a clean shutdown via cmdkit's signal handling.

---

## 17. Error Handling

**Warn and continue:**
- Front matter field missing or wrong type → use zero value, `log.Warn` with file path
- `ContentMeta.GetString()` called on a missing key → return `""` silently
- Draft file encountered with `draft_mode = false` → skip, `log.Detail` in verbose mode
- Static file already exists in output and is unchanged → skip silently

**Fail hard (`log.Fatal` → exit 1):**
- `frontmatter.template` path does not exist on disk
- TOML front matter is syntactically invalid
- A Pongo2 template has a parse error (not a missing variable — a broken template)
- `config.toml` is missing or unparseable
- Output directory cannot be created or written to

All errors include the source file path. In `--verbose` mode, add the triggering file path and, where available, a line number. No stack traces in normal mode.

---

## 18. `config.toml` Reference

```toml
[site]
title       = "My Site"
base_url    = "https://example.com"
language    = "en"
page_size   = 10

[build]
draft_mode  = false
output_dir  = "public"

[author]
name        = "Your Name"
email       = "you@example.com"
```

---

## 19. Site Blueprints

A **blueprint** is a complete project scaffold optimized for a specific use case. Blueprints are embedded in the `arbor` binary and deployed by `arbor init --blueprint <name>`. The default blueprint is `blog`.

### Available Blueprints

| Blueprint | Use Case | Key Characteristics |
|---|---|---|
| `blog` | Personal blog or journal | Post listing, tags, RSS-friendly structure, author bio |
| `marketing` | Open source project homepage or small business | Hero section, features grid, CTA, testimonials, minimal nav |
| `docs` | Open source project documentation | Sidebar nav, versioned sections, previous/next links, code-heavy layouts |

### Blueprint Structure

Blueprints live in `internal/blueprints/`:

```text
internal/blueprints/
├── base/                   # Shared static assets copied by every blueprint
│   └── static/
│       ├── css/            # ivy.full.css, lattice.full.css, site.css
│       └── js/             # dark-mode-toggle.js
├── blog/
│   ├── config.toml
│   ├── content/            # Example posts demonstrating typical content
│   ├── data/               # nav.toml
│   └── templates/          # Layouts and types designed for a blog
├── marketing/
│   ├── config.toml
│   ├── content/            # Hero, features, CTA as content items
│   ├── data/               # nav.toml, features.toml
│   └── templates/          # Landing page layout, section types
└── docs/
    ├── config.toml
    ├── content/            # Getting started, API reference examples
    ├── data/               # nav.toml with sidebar hierarchy
    └── templates/          # Doc layout with sidebar, prev/next nav
```

### `arbor init` Behavior

`arbor init` (or `arbor init --blueprint blog`):

1. Copies `base/` static assets to the destination
2. Copies blueprint-specific files on top (blueprint files take precedence over base files)
3. Never overwrites files that already exist in the destination directory
4. Reports each created file with `log.Success`
5. Prints a getting-started message at completion

### Blueprint Design Principles

- **Runnable out of the box** — `arbor init && arbor preview` should produce a complete, navigable site with no additional configuration
- **Real content, not Lorem Ipsum** — example content should reflect the actual use case (a blog post about blogging, a docs page about getting started)
- **Annotated templates** — each template has a short top comment explaining its role and the variables it expects
- **Minimal CSS customization** — start from Ivy + Lattice defaults; override only what the use case demands via `static/css/site.css`

---

## 20. Pongo2 Renderer

The renderer holds a single `*pongo2.TemplateSet` created once at construction time and reused for all renders — not per-call.

```go
type Renderer struct {
    projectRoot string
    set         *pongo2.TemplateSet
    log         *logger.Logger
}

func New(projectRoot string, log *logger.Logger) *Renderer {
    loader := pongo2.NewSandboxedFilesystemLoader(projectRoot)
    set := pongo2.NewSet("arbor", loader)
    return &Renderer{projectRoot: projectRoot, set: set, log: log}
}
```

Templates are loaded via `set.FromCache()`, which parses and caches on first access:

```go
tmpl, err := r.set.FromCache("templates/types/blog.html")
```

The template context is a `pongo2.Context` (which is `map[string]interface{}`):

```go
ctx := pongo2.Context{
    "Site":  vars.Site,
    "Page":  vars.Page,
    "Data":  vars.Data,
    "Items": vars.Items,
    "Pager": vars.Pager,
}
var buf bytes.Buffer
err = tmpl.ExecuteWriter(ctx, &buf)
```

Pongo2 uses the `SandboxedFilesystemLoader` to prevent template path traversal outside the project root.

---

## 21. Development Phases

| Phase | Focus | Key Deliverables |
|---|---|---|
| 1 | Foundation | `cmdkit` wiring, `init` command, config loader, directory scaffold, Ivy+Lattice copy |
| 2 | Parser & Indexer | Front matter extraction, Goldmark, `ContentItem` + `SiteIndex` build |
| 3 | Pongo2 Renderer | Template resolution cascade, Pongo2 context injection, `build` command working end-to-end |
| 4 | Preview Server | `fsnotify` watcher, `http.FileServer`, WebSocket live reload, incremental cache |
| 5 | Shortcodes | Two-pass pre/post-processor, param passing to Pongo2 partials |
| 6 | Pagination | Pre-computed `Pager` struct, multi-page listing generation |
| 7 | `new` Command + Polish | Content scaffolding, `--force`, error message refinement |
| 8 | Site Blueprints | `blog`, `marketing`, `docs` blueprints, `--blueprint` flag on `init`, `base/` shared assets |

---

## 22. `go.mod` Reference

```go
module github.com/rlnorthcutt/arbor

go 1.22

require (
    github.com/rlnorthcutt/cmdkit   v0.0.0-...
    github.com/pelletier/go-toml    v1.9.5
    github.com/yuin/goldmark        v1.7.4
    github.com/flosch/pongo2/v6     v6.0.0
    github.com/fsnotify/fsnotify    v1.7.0
    github.com/gorilla/websocket    v1.5.1
)
```

---
