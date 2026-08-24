/** Best-effort @user@host handle derived from an ActivityPub actor URI
 * (e.g. "https://mastodon.example/users/alice" -> "@alice@mastodon.example"). */
export function actorHandle(actor: string): string {
  try {
    const url = new URL(actor)
    const user = url.pathname.split('/').filter(Boolean).pop()
    return user ? `@${user}@${url.hostname}` : actor
  } catch {
    return actor
  }
}
