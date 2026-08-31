# One-click deploy

<script setup>
import { ref, computed } from 'vue'

const domain = ref('')
const actor = ref('')

const domainPattern = /^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$/
const actorPattern = /^[a-zA-Z0-9_-]*$/

const domainValid = computed(() => domainPattern.test(domain.value))
const actorValid = computed(() => actorPattern.test(actor.value))
const canGenerate = computed(() => domainValid.value && actorValid.value)

const effectiveActor = computed(() => actor.value.trim() === '' ? 'user' : actor.value.trim())

const scriptTemplate = (d, a) => `#!/usr/bin/env bash
# Paste this into DigitalOcean's Droplet "User Data" field, or Vultr's
# instance "Startup Script" field, at instance creation. Runs as root on
# first boot; installs nitpub unattended via the public install.sh release
# path (see docsite/docs/guide/install.md for the interactive equivalent).
NITPUB_DOMAIN="${d}"
NITPUB_ACTOR="${a}"

set -euo pipefail

CONFIG_SECRET="$(openssl rand -hex 32)"

if [[ -f /root/nitpub-admin-password ]]; then
  ADMIN_PASSWORD="$(cat /root/nitpub-admin-password)"
else
  ADMIN_PASSWORD="$(openssl rand -hex 16)"
fi

(umask 077; echo "$ADMIN_PASSWORD" >/root/nitpub-admin-password)
chmod 600 /root/nitpub-admin-password

curl -fsSL https://raw.githubusercontent.com/newtosh/nitpub/main/scripts/install.sh | bash -s -- \\
  --yes \\
  --domain "$NITPUB_DOMAIN" \\
  --actor "$NITPUB_ACTOR" \\
  --secret "$CONFIG_SECRET" \\
  --password "$ADMIN_PASSWORD" \\
  --with-caddy \\
  --with-federation \\
  --no-analytics

echo "==> nitpub installed. Log in at https://\${NITPUB_DOMAIN}/login (admin password: /root/nitpub-admin-password)"
`

const generatedScript = computed(() => canGenerate.value ? scriptTemplate(domain.value.trim(), effectiveActor.value) : '')

const copyState = ref('idle') // idle | copied | error

function withTimeout(promise, ms) {
  return Promise.race([
    promise,
    new Promise((_, reject) => setTimeout(() => reject(new Error('timeout')), ms)),
  ])
}

async function copyScript() {
  try {
    // Bounded: the Clipboard API can hang indefinitely (permission prompt
    // never resolving, extension interference) rather than reject, which
    // would leave the button stuck with no feedback forever.
    await withTimeout(navigator.clipboard.writeText(generatedScript.value), 3000)
    copyState.value = 'copied'
  } catch (e) {
    copyState.value = 'error'
  }
  setTimeout(() => { copyState.value = 'idle' }, 2000)
}
</script>

[![Deploy on DigitalOcean](https://img.shields.io/badge/Deploy-DigitalOcean-0080FF)](#digitalocean)
[![Deploy on Vultr](https://img.shields.io/badge/Deploy-Vultr-007BFC)](#vultr)
[![Deploy on Linode](https://img.shields.io/badge/Deploy-Linode-00A95C)](https://cloud.linode.com/stackscripts/2203935)

Same result as [Quick install](/guide/install), skipping the SSH install step: create a VPS with a startup script pasted in, and nitpub installs itself on first boot. You'll still land on the same `/login` step either way — and you'll still need SSH afterward to retrieve the generated admin password (see "After it installs" below).

The provider links below are plain homepage/signup links for now — affiliate/referral codes are still TODO for all three (see the `TODO: ref code` note under each provider). No provider lets you deep-link domain/region/size into their create page either way; you fill those in yourself as usual, then paste the script.

## Before you start

Same prerequisites as [Quick install](/guide/install): a domain pointed at the instance you're about to create (A/AAAA), and a VPS-sized budget (≈1 GB RAM is enough for idle use).

## Generate your deploy script

For DigitalOcean and Vultr — fill in your domain and actor, copy the result, paste it into the provider's instance-creation form. Shared between both sections below since they take the identical script.

<div class="deploy-generator">
  <label>
    Domain
    <input v-model="domain" placeholder="blog.example.com" :class="{ invalid: domain && !domainValid }" />
  </label>
  <label>
    Federation actor <span class="muted">(optional, defaults to "user")</span>
    <input v-model="actor" placeholder="user" :class="{ invalid: actor && !actorValid }" />
  </label>

  <p v-if="domain && !domainValid" class="error">Enter a valid hostname (letters, numbers, hyphens, dots — no quotes, spaces, or symbols).</p>
  <p v-if="actor && !actorValid" class="error">Actor can only contain letters, numbers, hyphens, and underscores.</p>

  <button :disabled="!canGenerate" @click="copyScript">
    {{ copyState === 'copied' ? 'Copied!' : copyState === 'error' ? 'Copy failed — select and copy manually' : 'Copy generated script' }}
  </button>

  <pre v-if="canGenerate" class="generated-script">{{ generatedScript }}</pre>
  <p v-else class="muted">Enter a domain above to generate your script.</p>
</div>

## DigitalOcean

<!-- TODO: ref code — apply at https://www.digitalocean.com/affiliates -->

1. [Create a Droplet](https://www.digitalocean.com/) (any size ≥1 GB RAM).
2. In the Droplet creation form, open **Advanced Options → User Data** and paste the script generated above.
3. Create the Droplet. Wait a minute or two for first boot to finish installing.

## Vultr

<!-- TODO: ref code — apply at Vultr's affiliate program page -->

1. [Deploy a new instance](https://www.vultr.com/) (any plan ≥1 GB RAM).
2. Under **Startup Script**, choose "Add New" and paste the script generated above.
3. Deploy. Wait a minute or two for first boot to finish installing.

## Linode / Akamai Cloud

<!-- TODO: ref code — apply at Linode's Affiliate Sign-Up page -->

Closest thing to genuine one-click of the three: domain and actor are real form fields, not a pasted script.

1. Open the [nitpub StackScript deploy link](https://cloud.linode.com/stackscripts/2203935).
2. Fill in **Domain** and **Actor** in the StackScript Options form.
3. Choose a plan (≥1 GB RAM) and deploy. Wait a minute or two for first boot to finish installing.

## After it installs

Same as [Quick install](/guide/install)'s remaining steps:

- Admin password: `ssh root@YOUR_IP cat /root/nitpub-admin-password`
- Log in at `https://YOUR_DOMAIN/login`, publish a note, verify with `nitpub doctor` and the `/healthz` / webfinger checks in [Quick install](/guide/install#4-verify).

## Next

- [Quick install](/guide/install) if you'd rather SSH in directly
- [Manual download](/guide/manual-download) for the non-scripted path

<style>
.deploy-generator {
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  padding: 1.25rem;
  margin: 1.5rem 0;
}
.deploy-generator label {
  display: block;
  font-weight: 600;
  margin-bottom: 0.75rem;
}
.deploy-generator input {
  display: block;
  width: 100%;
  margin-top: 0.25rem;
  padding: 0.5rem;
  font-family: var(--vp-font-family-mono);
  border: 1px solid var(--vp-c-divider);
  border-radius: 6px;
  background: var(--vp-c-bg);
  color: var(--vp-c-text-1);
}
.deploy-generator input.invalid {
  border-color: var(--vp-c-danger-1);
}
.deploy-generator .muted {
  color: var(--vp-c-text-2);
  font-weight: normal;
  font-size: 0.85em;
}
.deploy-generator .error {
  color: var(--vp-c-danger-1);
  font-size: 0.85em;
  margin: -0.5rem 0 0.75rem;
}
.deploy-generator button {
  padding: 0.5rem 1rem;
  border-radius: 6px;
  border: 1px solid var(--vp-c-brand-1);
  background: var(--vp-c-brand-1);
  color: white;
  cursor: pointer;
}
.deploy-generator button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.generated-script {
  margin-top: 1rem;
  max-height: 300px;
  overflow: auto;
}
</style>
