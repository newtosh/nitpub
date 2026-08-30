# One-click deploy

Same result as [Quick install](/guide/install), skipping the SSH install step: create a VPS with a startup script pasted in, and nitpub installs itself on first boot. You'll still land on the same `/login` step either way — and you'll still need SSH afterward to retrieve the generated admin password (see "After it installs" below).

The provider links below are plain homepage/signup links for now — affiliate/referral codes are still TODO for all three (see the `TODO: ref code` note under each provider). No provider lets you deep-link domain/region/size into their create page either way; you fill those in yourself as usual, then paste the script.

## Before you start

Same prerequisites as [Quick install](/guide/install): a domain pointed at the instance you're about to create (A/AAAA), and a VPS-sized budget (≈1 GB RAM is enough for idle use).

## DigitalOcean

<!-- TODO: ref code — apply at https://www.digitalocean.com/affiliates -->

1. [Create a Droplet](https://www.digitalocean.com/) (any size ≥1 GB RAM).
2. In the Droplet creation form, open **Advanced Options → User Data** and paste [`deploy/startup-do-vultr.sh`](https://github.com/newtosh/nitpub/blob/main/deploy/startup-do-vultr.sh), editing `NITPUB_DOMAIN` and `NITPUB_ACTOR` at the top first.
3. Create the Droplet. Wait a minute or two for first boot to finish installing.

## Vultr

<!-- TODO: ref code — apply at Vultr's affiliate program page -->

1. [Deploy a new instance](https://www.vultr.com/) (any plan ≥1 GB RAM).
2. Under **Startup Script**, choose "Add New" and paste [`deploy/startup-do-vultr.sh`](https://github.com/newtosh/nitpub/blob/main/deploy/startup-do-vultr.sh), editing `NITPUB_DOMAIN` and `NITPUB_ACTOR` at the top first.
3. Deploy. Wait a minute or two for first boot to finish installing.

## Linode / Akamai Cloud

<!-- TODO: ref code — apply at Linode's Affiliate Sign-Up page -->

Closest thing to genuine one-click of the three: domain and actor are real form fields, not a pasted script.

1. Open the nitpub StackScript deploy link *(TODO: publish `deploy/startup-linode.stackscript.sh` as a public StackScript and link it here)*.
2. Fill in **Domain** and **Actor** in the StackScript Options form.
3. Choose a plan (≥1 GB RAM) and deploy. Wait a minute or two for first boot to finish installing.

## After it installs

Same as [Quick install](/guide/install)'s remaining steps:

- Admin password: `ssh root@YOUR_IP cat /root/nitpub-admin-password`
- Log in at `https://YOUR_DOMAIN/login`, publish a note, verify with `nitpub doctor` and the `/healthz` / webfinger checks in [Quick install](/guide/install#4-verify).

## Next

- [Quick install](/guide/install) if you'd rather SSH in directly
- [Manual download](/guide/manual-download) for the non-scripted path
