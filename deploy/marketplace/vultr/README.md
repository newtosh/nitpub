# nitpub — Vultr Marketplace submission

## Path used

Vultr's imageless "Build from Vendor Data" flow — their own recommended
approach ("For repeatability and ease of maintenance, we recommend
building imageless apps from Vendor Data whenever possible."). No Packer,
no snapshot pipeline: Cloud Manager's Builds tab takes `vendor-data.sh`
pasted directly and produces an "App Image."

## Submission checklist (Cloud Manager > Builds > Build from Vendor Data)

- **Base OS:** Debian 12 (matches the DigitalOcean image's target)
- **Vendor Data script:** `vendor-data.sh` in this directory
- **App name:** nitpub
- **Category:** confirm against Vultr's current category list at
  submission time — likely "Content Management" or "Developer Tools"
- **Description:** self-hosted ActivityPub blog — micro-notes and
  long-form posts, federated, custom domain, single Go binary
- **Logo:** use `docsite/docs/public/wordmark-light.svg` (or a square
  icon variant if Vultr requires one — check their asset spec at
  submission time)
- **Website / docs URL:** https://docs.nitpub.com

## Process

1. Apply for Verified Vendor status through Vultr's Cloud Manager.
2. Sign the Publisher Agreement.
3. Build the App Image from `vendor-data.sh` using the checklist above.
4. Submit for review.

No published review-timeline SLA.
