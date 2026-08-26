# Cloudflare Pages (www + docs)

nitpub’s marketing and docs sites deploy from the **public** repo [`newtosh/nitpub`](https://github.com/newtosh/nitpub) via **Cloudflare Pages** (two projects, monorepo roots). The federated demo stays on the VPS at **apex** `nitpub.com` (`@nit@nitpub.com`).

## Projects

| Pages project | Root directory | Build command | Output directory | Custom domain |
|---|---|---|---|---|
| `nitpub-www` | `www` | `npm ci && npm run build` | `dist` | `www.nitpub.com` |
| `nitpub-docs` | `docsite` | `npm ci && npm run build` | `docs/.vitepress/dist` | `docs.nitpub.com` |

### www build notes

- `/changelog` is a `_redirects` rule (`www/public/_redirects`) to `docs.nitpub.com/changelog` — the changelog itself lives on the docs site now, not here.

### docs build notes

- VitePress app under `docsite/` (`vitepress build docs`).
- `llms.txt` ships at `https://docs.nitpub.com/llms.txt` via `docsite/docs/public/llms.txt`.
- `/changelog` is a VitePress data loader (`docsite/docs/changelog.data.ts`) that fetches GitHub Releases at build time — same approach www's old script used, now owned by docs instead of duplicated. Unauthenticated (60 req/hr GitHub limit is plenty for a build-time call); add a `GITHUB_TOKEN` Pages secret only if builds start hitting it.

## DNS

1. Attach custom domains in each Pages project (Cloudflare will show required CNAME/A records).
2. Point **`www`** and **`docs`** at Pages — **not** at the VPS.
3. Keep apex **`nitpub.com`** (and federation) on the VPS A/AAAA.

## VPS Caddy after cutover

If the live Caddy site block lists `nitpub.com, www.nitpub.com`, remove `www` so only the apex reverse-proxies the blog (Pages owns www). Do not change actor/`domain` config.

## Local preview

```bash
cd www && npm ci && npm run dev
cd docsite && npm ci && npm run dev
```
