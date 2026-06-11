+++
title = "Configuration"
date  = 2025-01-01
draft = false
+++

## config.toml

Arbor is configured with a single `config.toml` file in your project root.

```toml
[site]
title    = "My Site"
base_url = "https://example.com"

[build]
draft_mode = false
output_dir = "public"
```

## Site Options

| Key | Default | Description |
|---|---|---|
| `title` | "My Site" | Site title shown in templates |
| `base_url` | "" | Full URL for RSS/sitemaps |
| `language` | "en" | HTML lang attribute |
| `page_size` | 10 | Items per listing page |
