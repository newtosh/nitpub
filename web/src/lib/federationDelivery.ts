import type { Post } from './posts'

export type FederationState = NonNullable<Post['federation']>
export type BlueskyState = NonNullable<Post['bluesky']>

export type DeliveryBadge = {
  label: string
  icon: 'site' | 'fediverse' | 'bluesky'
  tone: 'default' | 'muted' | 'warn'
  title?: string
  // Only Bluesky's "posted" badge sets this (linking out to the post's
  // uri) — Fediverse's "Shared" badge has never linked out here (see
  // PostPage.vue's separate "View on Mastodon" link instead), so this
  // stays unset for it, same as before.
  href?: string
}

/** Legacy posts without federation metadata were site-only until explicitly shared. */
export function federationShared(post: Post): boolean {
  if (!post.federation) return false
  return post.federation.shared && !post.federation.error
}

export function federationFailed(post: Post): boolean {
  return federationShared(post) && !!post.federation?.error
}

/** Bluesky's badge, independent of post.federation (R8) — null when no
 * Bluesky delivery has ever been attempted for this post. */
export function blueskyBadge(post: Post): DeliveryBadge | null {
  const state = post.bluesky
  if (!state) return null
  if (state.status === 'pending') {
    return { label: 'Crossposting…', icon: 'bluesky', tone: 'muted' }
  }
  if (state.status === 'error') {
    return { label: 'Bluesky', icon: 'bluesky', tone: 'warn', title: state.error || 'Delivery failed' }
  }
  return { label: 'Bluesky', icon: 'bluesky', tone: 'default', href: state.uri || undefined }
}

export function deliveryBadges(post: Post): DeliveryBadge[] {
  const badges: DeliveryBadge[] = [{ label: 'Site', icon: 'site', tone: 'default' }]
  if (federationShared(post)) {
    if (federationFailed(post)) {
      badges.push({
        label: 'Fediverse',
        icon: 'fediverse',
        tone: 'warn',
        title: post.federation?.error ?? 'Delivery failed',
      })
    } else {
      badges.push({ label: 'Fediverse', icon: 'fediverse', tone: 'default' })
    }
  }
  const bluesky = blueskyBadge(post)
  if (bluesky) badges.push(bluesky)
  return badges
}

export function crossPostDefaultFromConfig(
  federation?: { cross_post_default?: boolean | null },
): boolean {
  if (federation?.cross_post_default === undefined || federation.cross_post_default === null) {
    return true
  }
  return federation.cross_post_default
}

export function avatarsEnabledFromConfig(
  federation?: { show_avatars_default?: boolean | null },
): boolean {
  if (federation?.show_avatars_default === undefined || federation.show_avatars_default === null) {
    return true
  }
  return federation.show_avatars_default
}

export function moderationEnabledFromConfig(
  federation?: { moderation_enabled?: boolean | null },
): boolean {
  if (federation?.moderation_enabled === undefined || federation.moderation_enabled === null) {
    return true
  }
  return federation.moderation_enabled
}

export function repliesCollapsedByDefaultFromConfig(
  federation?: { replies_collapsed_default?: boolean | null },
): boolean {
  if (
    federation?.replies_collapsed_default === undefined ||
    federation.replies_collapsed_default === null
  ) {
    return true
  }
  return federation.replies_collapsed_default
}
