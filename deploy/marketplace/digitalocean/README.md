# nitpub — DigitalOcean Marketplace image

Self-hosted ActivityPub blog. Single Go binary, systemd, optional Caddy.

## What this image does

`scripts/01-nitpub-install.sh` downloads and installs the latest `nitpub`
release binary at build time and writes a first-login MOTD. It does **not**
run `nitpub install` at build time — that needs a domain, which isn't known
until the customer deploys a real droplet from this image, and the
non-interactive Packer provisioner has no TTY for the install wizard's
interactive form anyway.

## First-boot config (customer-facing)

After deploying a Droplet from this image:

1. SSH in: `ssh root@YOUR_DROPLET_IP`
2. Run `nitpub install` — the interactive wizard asks for domain, federation
   actor, and admin password, then configures systemd + Caddy and creates
   the admin account.
3. Open `https://YOUR_DOMAIN/login`.

Full docs: https://docs.nitpub.com/guide/install

## Base image

Currently targets `debian-12-x64`. **Confirm this slug is on DigitalOcean's
Marketplace-eligible base image list before the first real submission** —
it was chosen to match the existing `deploy/startup-do-vultr.sh` path, not
independently verified against DO's accepted set for vendor images.

## Build

```bash
DIGITALOCEAN_TOKEN=... packer build marketplace-image.json
```

Requires a DigitalOcean API token scoped to image/droplet read-write (see
the repo's `.github/workflows/marketplace-digitalocean.yml` for the CI
path). Produces a snapshot in the token's DO account — submit its ID
through the DO Vendor Portal (`cloud.digitalocean.com/vendorportal`).
