# Cloudflare Pages (www + docs)

nitpub’s marketing and docs sites deploy from the **public** repo [`newtosh/nitpub`](https://github.com/newtosh/nitpub) via **Cloudflare Pages** (two projects, monorepo roots). The federated demo stays on the VPS at **apex** `nitpub.com` (`@nit@nitpub.com`).

## Projects

| Pages project | Root directory | Build command | Output directory | Custom domain |
|---|---|---|---|---|
| `nitpub-www` | `www` | `npm ci && npm run build` | `dist` | `www.nitpub.com` |
| `nitpub-docs` | `docsite` | `npm ci && npm run build` | `docs/.vitepress/dist` | `docs.nitpub.com` |

### www build notes

- `npm run build` runs `scripts/fetch-releases.mjs --require` (GitHub Releases → `data/releases.json`).
- Set Pages secret **`GITHUB_TOKEN`** (classic or fine-grained read on public repo) so CF shared egress does not rate-limit the API.
- Changelog is a static MPA path: `/changelog/` → `changelog/index.html`.

### docs build notes

- VitePress app under `docsite/` (`vitepress build docs`).
- `llms.txt` ships at `https://docs.nitpub.com/llms.txt` via `docsite/docs/public/llms.txt`.

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
