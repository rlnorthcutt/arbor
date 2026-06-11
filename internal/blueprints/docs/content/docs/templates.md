+++
title = "Templates"
date  = 2025-01-01
draft = false
+++

## Template Syntax

Arbor uses Pongo2 (Jinja2-compatible) templates with the `.html` extension.

## Template Variables

Every template receives:

- `Site` — site config and content index
- `Page` — the current content item
- `Data` — data files from `data/`
- `Items` — items of the current type
- `Pager` — pagination state (listing pages only)

## Rendering HTML Content

Use the `|safe` filter to render Markdown-generated HTML:

```html
{{ Page.HTMLContent|safe }}
```
