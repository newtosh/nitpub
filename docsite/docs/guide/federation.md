# Federation — visible federated reply

nitpub speaks ActivityPub; remote replies are moderated by default.

1. **Discoverability** — From another Fediverse client, resolve `@actor@YOUR_DOMAIN` (exact handle from config), not a fuzzy search.
2. **Optional preflight** — `DOMAIN=YOUR_DOMAIN ACTOR=youractor bash scripts/federation-interop-preflight.sh`
3. **Follow** — Follow the nitpub actor from a Mastodon (or compatible) account you control.
4. **Publish with federation** — Keep cross-post enabled (install gate default, or Admin → Federation). Publish a note from `/author/compose` so followers receive it.
5. **Reply from Mastodon** — Reply publicly to that post from your Mastodon account.
6. **Make the reply visible** — By default inbound replies are **pending**:
   - Open **Admin → Moderation** and **approve** the reply, **or**
   - Trust the remote actor, **or**
   - Turn moderation off in Admin → Federation (`moderation_enabled = false`)
7. **Confirm** — Open the permalink `/p/<slug>#replies` and check the reply is shown.

Full checklist and inspect scripts: [federation-interop.md](https://github.com/newtosh/nitpub/blob/main/docs/federation-interop.md). Ops notes: [deploy/README.md](https://github.com/newtosh/nitpub/blob/main/deploy/README.md#federation-interop-u9).
