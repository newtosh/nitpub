<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/brand/wordmark-dark.png">
  <img alt="nitpub" src="assets/brand/wordmark-light.png" width="280">
</picture>

**ActivityPub blog for notes and long-form articles on your own domain** — one Go binary on a small VPS, without running Mastodon.

| | |
|---|---|
| **Product** | [www.nitpub.com](https://www.nitpub.com/) |
| **Docs (install)** | [docs.nitpub.com](https://docs.nitpub.com/) |
| **Live demo** | [nitpub.com](https://nitpub.com) (`@nit@nitpub.com`) |
| **Changelog** | [www.nitpub.com/changelog](https://www.nitpub.com/changelog/) |

## Who it’s for

Self-hosting indie bloggers who want ownership of notes + articles with federated replies, on a budget VPS — not a closed SaaS, and not Mastodon’s ops weight.

## Install

Follow the canonical guide: **[docs.nitpub.com/guide/install](https://docs.nitpub.com/guide/install)**

Quick start on a Debian/Ubuntu VPS as root:

```bash
curl -fsSL https://raw.githubusercontent.com/newtosh/nitpub/main/scripts/install.sh | bash
```

## Advanced / ops

Multi-instance, GoatCounter deep dive, Cloudflare Pages for www/docs, maintainer workflows: [`deploy/README.md`](deploy/README.md) · [`deploy/pages.md`](deploy/pages.md)

Local development:

```bash
make help
make build    # PWA + .build/nitpub
make test     # go test ./...
make run      # local HTTP
```

## Community

| Doc | Purpose |
|-----|---------|
| [LICENSE](LICENSE) | MIT |
| [SECURITY.md](SECURITY.md) | Vulnerability reports via GitHub Security Advisories only |
| [SUPPORT.md](SUPPORT.md) | Help via GitHub Issues (not for vulns) |

## License

[MIT](LICENSE)
