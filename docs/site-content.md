# Site content (direct edit)

Site structure lives under **`{data_dir}/site/`** — the same files the admin UI edits at **Admin → Site**.

## Layout

```
{data_dir}/site/
  site.toml              # manifest: nav, home, archive, search, page registry
  pages/
    about.md             # markdown custom page
    projects.links.toml  # link collection page
```

Posts stay in **bbolt** (`nitpub.db`). Only site pages and manifest use this directory.

## `site.toml`

```toml
[home]
recent_count = 10   # posts on / ; 0 = show all

[archive]
mode = "pagination"   # or "infinite"
page_size = 20

[search]
enabled = true

[[nav]]
label = "Posts"
path = "/posts"
icon = "newspaper"

[[nav]]
label = "About"
path = "/about"
icon = "user"

[[pages]]
path = "/about"
type = "markdown"
file = "pages/about.md"

[[pages]]
path = "/projects"
type = "links"
file = "pages/projects.links.toml"
```

### Reserved paths

Do not register custom pages on these paths (validation rejects them):

`api`, `actor`, `inbox`, `outbox`, `.well-known`, `feed.xml`, `healthz`, `author`, `admin`, `login`, `p`, `posts`, `search`, `assets`, `media`

### Nav icons

Use Lucide icon slugs from this allowlist:

`user`, `newspaper`, `link`, `links`, `folder`, `book`, `book-open`, `home`, `rss`, `github`, `globe`, `mail`, `info`, `file-text`, `briefcase`, `code`, `search`, `external-link`

Invalid names render as text-only in the nav.

## Markdown pages

Register the route in `[[pages]]` and add the body file. First `# heading` becomes the page title. Rendering uses the same markdown pipeline as posts.

Example `pages/about.md`:

```markdown
# About

Hello from nitpub.
```

## Link collection pages

Use a `*.links.toml` file:

```toml
title = "Projects"

[[links]]
title = "nitpub"
url = "https://github.com/newtosh/nitpub"
description = "Source repo"
icon = "github"
```

## Import type registry (convention)

| Extension       | Handler        | Purpose              |
|----------------|----------------|----------------------|
| `.md` (pages)  | `page`         | Custom markdown page |
| `.links.toml`  | `links`        | Link collection      |
| `.md` (import) | `post_import`  | Bulk post import     |

## Post import (markdown → bbolt)

Posts are **not** file-backed after import. Use CLI or admin upload.

### CLI

```bash
nitpub import posts /path/to/markdown-dir [--kind note|article] [--offline]
```

Optional YAML-style frontmatter:

```markdown
---
kind: note
---

# My note

Body text.
```

Without frontmatter, kind is inferred: `# title` → note; `title line` + body → article.

### Admin

**Admin → Site → Import posts** — upload one or more `.md` files.

Imported posts appear on `/`, `/posts`, RSS, and search. Federation follows the normal create/delivery path.

## Git workflow

You can track `{data_dir}/site/` in git. Restart is not required; changes are read on the next API request (search index rebuilds after admin saves and post imports).

## Example seed (local dev)

```bash
./scripts/seed-site-example.sh
```

Copies example `site/` into your configured `data_dir`.
