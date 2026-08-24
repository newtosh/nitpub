import type { Post } from './posts'

export type FederationState = NonNullable<Post['federation']>

export type DeliveryBadge = {
  label: string
  icon: 'site' | 'fediverse'
  tone: 'default' | 'muted' | 'warn'
  title?: string
}

/** Legacy posts without federation metadata were site-only until explicitly shared. */
export function federationShared(post: Post): boolean {
  if (!post.federation) return false
  return post.federation.shared && !post.federation.error
}

export function federationFailed(post: Post): boolean {
  return federationShared(post) && !!post.federation?.error
}

export function deliveryBadges(post: Post): DeliveryBadge[] {
  const badges: DeliveryBadge[] = [{ label: 'Site', icon: 'site', tone: 'default' }]
  if (!federationShared(post)) {
    return badges
  }
  if (federationFailed(post)) {
    badges.push({
      label: 'Fediverse',
      icon: 'fediverse',
      tone: 'warn',
      title: post.federation?.error ?? 'Delivery failed',
    })
    return badges
  }
  badges.push({ label: 'Fediverse', icon: 'fediverse', tone: 'default' })
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
