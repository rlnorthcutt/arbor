+++
title = "CLI Reference"
date  = 2025-01-01
draft = false
+++

## Commands

### arbor init

Initialize a new project with the specified blueprint.

```bash
arbor init --blueprint blog      # personal blog (default)
arbor init --blueprint marketing # landing page
arbor init --blueprint docs      # documentation site
```

### arbor new

Create a new content file.

```bash
arbor new blog my-post     # creates content/blog/my-post.md
arbor new docs api-guide   # creates content/docs/api-guide.md
```

### arbor build

Build the site to `public/`.

```bash
arbor build           # incremental build
arbor build --force   # full rebuild
```

### arbor preview

Build and serve with live reload.

```bash
arbor preview           # serve on :8080
arbor preview --port 3000
```
