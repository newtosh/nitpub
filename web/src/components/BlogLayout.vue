<script setup lang="ts">
import { CircleUserRound, Inbox, LogOut, RefreshCw, Rss, Settings, SquarePen } from '@lucide/vue'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import ColorSchemeToggle from './ColorSchemeToggle.vue'
import SearchBox from './SearchBox.vue'
import GithubIcon from './icons/GithubIcon.vue'
import { logout } from '../lib/auth'
import { navIcon } from '../lib/icons'
import { fetchSiteConfig, getCachedSiteConfig, type NavItem, type SiteConfig } from '../lib/site'
import { useSession } from '../composables/useSession'
import { useModerationBadge } from '../composables/useModerationBadge'

const route = useRoute()
const router = useRouter()
const { authed, refresh } = useSession()
const { pendingCount, refresh: refreshModerationBadge } = useModerationBadge()
// Seeded from localStorage (synchronous), not null — a hard refresh
// used to always start from nothing, showing the default title and an
// empty nav for a beat before the fetch below resolved and visibly
// snapped to the real config. Only a genuine first-ever visit (no
// cache yet) still starts null; the template shows a skeleton for that
// case rather than the wrong defaults.
const site = ref<SiteConfig | null>(getCachedSiteConfig())
const siteTitle = computed(() => site.value?.title || 'nitpub')

// footer.github_url is per-instance configurable — any self-hoster can
// point it at their own unrelated repo, not always nitpub's. Only the
// project's own canonical repo gets the brand color split; anything
// else stays plain muted text.
const footerRepoPath = computed(
  () => site.value?.footer?.github_url?.replace(/^https?:\/\/(www\.)?github\.com\//, '') ?? ''
)
const isNitpubRepo = computed(() => /^newtosh\/nitpub\/?$/i.test(footerRepoPath.value))

function applyTitleAndFavicon() {
  document.title = siteTitle.value
  if (site.value?.branding?.favicon_url) {
    let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    if (!link) {
      link = document.createElement('link')
      link.rel = 'icon'
      document.head.appendChild(link)
    }
    link.href = site.value.branding.favicon_url
  }
}
// Runs during setup, before mount/first paint — not just in onMounted
// below — so a cached title/favicon from a previous visit is already
// correct from the start instead of waiting a tick even when the seed
// data is already available.
applyTitleAndFavicon()

const mobilePageTitle = computed(() => route.meta.title as string | undefined)

// Explicit state, not route-derived: navigating away via an unrelated
// nav icon (Moderation, Admin) used to silently flip this back to
// "Compose" because it tracked route.path directly. It now only
// changes when the button itself is tapped.
const magicIconShowsBack = ref(
  route.path === '/author/compose' || route.path.startsWith('/author/edit/'),
)

function onMagicIconClick() {
  router.push(magicIconShowsBack.value ? '/author' : '/author/compose')
  magicIconShowsBack.value = !magicIconShowsBack.value
}

const navRoutesRef = ref<HTMLElement | null>(null)
const canScrollLeft = ref(false)
const canScrollRight = ref(false)

function updateFade() {
  const el = navRoutesRef.value
  if (!el) return
  canScrollLeft.value = el.scrollLeft > 0
  canScrollRight.value = el.scrollLeft + el.clientWidth < el.scrollWidth - 1
}

watch(site, () => {
  nextTick(updateFade)
})

function resetIOSZoom() {
  const viewport = document.querySelector('meta[name="viewport"]')
  if (!viewport) return
  const original = viewport.getAttribute('content')
  if (!original) return
  // Briefly constrain the viewport to scale=1 to snap Safari out of a stuck
  // pinch/auto-zoom, then restore the original content so pinch-zoom stays
  // available for accessibility. This is the reliable fix for iOS Safari
  // leaving the page zoomed in after an input blurs — a no-op scroll alone
  // doesn't reset the visual viewport's scale.
  viewport.setAttribute('content', `${original}, maximum-scale=1.0`)
  window.setTimeout(() => {
    viewport.setAttribute('content', original)
  }, 300)
}

function handleFocusOut(event: FocusEvent) {
  const target = event.target as HTMLElement | null
  if (!target || !(target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement)) return
  window.setTimeout(() => {
    window.scrollTo(window.scrollX, window.scrollY)
    resetIOSZoom()
  }, 50)
}


onMounted(async () => {
  await refresh()
  if (authed.value) refreshModerationBadge()
  try {
    site.value = await fetchSiteConfig()
  } catch {
    // Keep whatever we seeded from cache (possibly null) rather than
    // wiping it out — a failed revalidation shouldn't be worse than
    // just not having fetched yet.
  }
  applyTitleAndFavicon()
  await nextTick()
  updateFade()
  window.addEventListener('resize', updateFade)
  // handleFocusOut/resetIOSZoom temporarily disabled — isolating whether
  // this JS is the cause of, or an ineffective fix for, the mobile
  // Safari scroll-stuck-after-login bug.
  // document.addEventListener('focusout', handleFocusOut)
})

onUnmounted(() => {
  window.removeEventListener('resize', updateFade)
  // document.removeEventListener('focusout', handleFocusOut)
})

async function doLogout() {
  await logout()
  await refresh()
  if (route.path.startsWith('/author') || route.path.startsWith('/admin')) {
    await router.push('/')
  }
}

function navActive(item: NavItem) {
  return route.path === item.path || route.path.startsWith(item.path + '/')
}
</script>

<template>
  <div class="blog-layout">
    <div class="site-chrome">
      <header class="site-header">
      <div class="header-inner">
        <div class="header-brand-group">
          <RouterLink to="/" class="brand header-brand" aria-label="Home" :title="`Home — ${siteTitle}`">
            <img v-if="site?.branding?.logo_url" :src="site.branding.logo_url" class="brand-logo" alt="" />
            <span>{{ siteTitle }}</span>
          </RouterLink>
          <h1 v-if="mobilePageTitle" class="mobile-page-title">{{ mobilePageTitle }}</h1>
        </div>
        <!-- display:contents on desktop (both children act as direct grid
             items of .header-inner, keeping their own grid-area) but a real
             flex row on mobile, so routes can grow into whatever width
             .nav-admin isn't using instead of both fighting over a grid
             column track sized independently of which row is empty. -->
        <div class="nav-row2-group">
          <div class="nav-routes-wrap">
            <div class="nav-routes-fade nav-routes-fade-left" v-show="canScrollLeft" aria-hidden="true" />
            <nav
              v-if="site"
              ref="navRoutesRef"
              class="nav-routes cluster"
              aria-label="Site"
              @scroll="updateFade"
            >
              <RouterLink
                v-for="item in site.nav"
                :key="item.path"
                :to="item.path"
                class="nav-item"
                :class="{ active: navActive(item) }"
                :title="item.label"
              >
                <component
                  :is="navIcon(item.icon)"
                  v-if="navIcon(item.icon)"
                  :size="18"
                  :stroke-width="1.75"
                  aria-hidden="true"
                />
                <span class="nav-label">{{ item.label }}</span>
              </RouterLink>
            </nav>
            <!-- True first-ever visit only (no cached config yet) — an
                 empty nav for that one beat reads as broken/missing
                 routes rather than "loading," so show placeholder shapes
                 instead of nothing. -->
            <div v-else class="nav-routes cluster nav-routes-skeleton" aria-hidden="true">
              <span class="nav-item-skeleton" />
              <span class="nav-item-skeleton" />
              <span class="nav-item-skeleton" />
            </div>
            <div class="nav-routes-fade nav-routes-fade-right" v-show="canScrollRight" aria-hidden="true" />
          </div>
          <nav v-if="authed" class="nav-admin cluster" aria-label="Admin controls">
            <button
              type="button"
              class="nav-icon nav-icon-magic"
              :aria-label="magicIconShowsBack ? 'Back to Author' : 'Compose'"
              :title="magicIconShowsBack ? 'Back to Author list' : 'Compose — write and publish'"
              @click="onMagicIconClick"
            >
              <Transition name="magic-icon" mode="out-in">
                <svg
                  v-if="magicIconShowsBack"
                  key="back"
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.75"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                >
                  <rect width="18" height="18" x="3" y="3" rx="2" />
                  <path d="M7 8h8" />
                  <path d="M7 12h10" />
                  <path d="M7 16h6" />
                </svg>
                <SquarePen v-else key="compose" :size="20" :stroke-width="1.75" aria-hidden="true" />
              </Transition>
              <RefreshCw class="nav-icon-magic-badge" :size="15" :stroke-width="2.5" aria-hidden="true" />
            </button>
            <RouterLink
              to="/admin/moderation"
              class="nav-icon nav-icon-badge"
              :class="{ active: route.path.startsWith('/admin/moderation') }"
              :aria-label="pendingCount > 0 ? `Moderation — ${pendingCount} pending` : 'Moderation'"
              :title="pendingCount > 0 ? `Moderation — ${pendingCount} pending` : 'Moderation'"
            >
              <Inbox :size="20" :stroke-width="1.75" aria-hidden="true" />
              <span v-if="pendingCount > 0" class="badge-count">{{ pendingCount }}</span>
            </RouterLink>
            <RouterLink
              to="/admin"
              class="nav-icon"
              :class="{ active: route.path === '/admin' }"
              aria-label="Admin settings"
              title="Admin — instance settings"
            >
              <Settings :size="20" :stroke-width="1.75" aria-hidden="true" />
            </RouterLink>
          </nav>
        </div>
        <nav class="nav-utilities cluster" aria-label="Utilities">
          <template v-if="!authed">
            <span class="nav-group cluster">
              <RouterLink
                to="/login"
                class="nav-icon"
                :class="{ active: route.path === '/login' }"
                aria-label="Sign in"
                title="Sign in"
              >
                <CircleUserRound :size="20" :stroke-width="1.75" aria-hidden="true" />
              </RouterLink>
            </span>
            <span class="nav-sep" aria-hidden="true" />
          </template>
          <span class="nav-group cluster">
            <SearchBox v-if="site?.search.enabled !== false" :enabled="site?.search.enabled" />
            <a
              href="/feed.xml"
              class="nav-icon"
              aria-label="RSS feed"
              title="RSS feed — subscribe to posts"
            >
              <Rss :size="20" :stroke-width="1.75" aria-hidden="true" />
            </a>
            <ColorSchemeToggle />
          </span>
          <template v-if="authed">
            <span class="nav-sep" aria-hidden="true" />
            <button
              type="button"
              class="nav-icon"
              aria-label="Log out"
              title="Log out"
              @click="doLogout"
            >
              <LogOut :size="20" :stroke-width="1.75" aria-hidden="true" />
            </button>
          </template>
        </nav>
      </div>
      </header>
      <div id="layout-subnav" class="layout-subnav" />
    </div>
    <div class="site-scroll">
      <main class="site-main">
        <slot />
      </main>
      <footer class="site-footer">
        <div class="site-footer-row">
          <p>{{ site?.footer?.text || 'Powered by nitpub' }}</p>
          <div class="site-footer-meta">
            <span v-if="site?.version" class="site-footer-version">{{ site.version }}</span>
            <a
              v-if="site?.footer?.show_github_link && site?.footer?.github_url"
              :href="site.footer.github_url"
              class="site-footer-github"
              target="_blank"
              rel="noopener noreferrer"
            >
              <GithubIcon :size="14" />
              <span v-if="isNitpubRepo" class="site-footer-wordmark">newtosh/<span class="n">n</span><span class="i">i</span><span class="t">t</span><span class="pub">pub</span></span>
              <span v-else>{{ footerRepoPath }}</span>
            </a>
          </div>
        </div>
      </footer>
    </div>
  </div>
</template>

<style scoped>
.blog-layout {
  /* dvh, not vh — vh is the *largest* possible viewport (address bar
     hidden); iOS Safari's chrome collapsing/expanding on scroll then
     makes a 100vh shell taller than the actually-visible area. dvh
     tracks whichever is currently true. */
  height: 100dvh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.site-chrome {
  /* The document itself never scrolls in this layout (.site-scroll is
     the only scrolling box, see below), so sticky here isn't doing any
     scroll-tracking work — position:static would look identical. It's
     required anyway: Safari 26 ("Liquid Glass") dropped theme-color
     entirely and instead tints the status bar/toolbar by sampling the
     background-color of whichever position:fixed/sticky element sits
     at the viewport edge, read once at initial paint. No fixed/sticky
     element there at all (the state this had after the app-shell
     rewrite) means Safari falls back to some other default instead of
     matching the header. The color must be a solid value — Safari
     samples computed transparency too, so semi-transparent/color-mix
     backgrounds here would produce an inconsistent tint. */
  position: sticky;
  top: 0;
  z-index: 1;
  flex-shrink: 0;
  background: var(--surface);
  /* Extends the header's own background up under the notch/status bar
     instead of leaving that strip showing the page background — needs
     viewport-fit=cover (index.html) or env() resolves to 0 and this is
     a no-op. Padding, not just background bleed, so header-inner's
     content doesn't sit under the notch too. */
  padding-top: env(safe-area-inset-top);
}
.site-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  /* Always reserve the scrollbar's width, even on pages short enough
     that content doesn't overflow. Without this, .site-main (inside
     this scroll container) is ~15px wider on short pages than tall
     ones — the sticky header/subnav sits outside .site-scroll and
     never loses that width, so admin pages visibly shifted sideways
     depending on which section's content was tall enough to scroll. */
  scrollbar-gutter: stable;
  -webkit-overflow-scrolling: touch;
  display: flex;
  flex-direction: column;
}
.site-header {
  border-bottom: 1px solid var(--border);
  background: var(--surface);
}
.site-chrome:has(.layout-subnav:not(:empty)) .site-header {
  border-bottom: none;
}
.layout-subnav:not(:empty) {
  border-bottom: 1px solid var(--border);
}
/* Brand + utility icons share a top row (both short, single line); nav
   routes get their own dedicated row below. Routes used to share the
   top row too (1fr auto 1fr), which worked while there was only a
   nav item or two — with a full Posts/About/Projects/Contact nav it
   no longer fits, and .nav-utilities' own flex-wrap would silently
   wrap onto a second line that visually crowded into the routes row
   instead of forming a clean, separate one. Same two-row shape at
   every width now, not just the old mobile-only override. */
.header-inner {
  position: relative;
  max-width: var(--content-max);
  margin: 0 auto;
  /* No bottom padding — .nav-row2-band relies on row 2 ending exactly
     at this container's real bottom edge (see below). Padding there
     previously got canceled with a negative margin on the band instead,
     which desyncs a grid item's layout-height contribution from what
     it visually paints: .site-main (next sibling, normal flow) started
     at the shorter *layout* height while the band kept visually
     painting past it, so page content rendered underneath the header. */
  padding: var(--header-pad-y) var(--header-pad-x) 0;
  display: grid;
  grid-template-columns: 1fr auto auto;
  grid-template-areas:
    'brand admin utils'
    'routes routes routes';
  align-items: center;
  row-gap: var(--space-3);
  column-gap: var(--space-4);
  /* One step lighter than the header's own --surface, mixed from
     existing theme tokens (no hardcoded color) so it holds up across
     every palette — defined here (not on the band itself) so the
     scroll-fade overlays below, which aren't grid children of this
     row, can still read the same value. */
  --routes-band: color-mix(in srgb, var(--surface) 88%, var(--border));
}
.header-brand-group {
  grid-area: brand;
  justify-self: start;
  display: flex;
  align-items: baseline;
  gap: 0.6rem;
  min-width: 0;
}
.mobile-page-title {
  display: none;
  margin: 0;
  font: inherit;
}
.nav-admin {
  grid-area: admin;
  justify-self: end;
  gap: var(--space-2);
}
/* Desktop: invisible to layout, so .nav-routes-wrap and .nav-admin act
   as direct grid items of .header-inner (their own grid-area, as
   before nesting). Mobile: a real flex row instead — see below for
   why (grid columns are shared across rows, so a 2-column split here
   would keep reserving .nav-admin's width even on visits where it
   doesn't render, capping routes at less than the full row instead
   of it growing into that now-empty space). */
.nav-row2-group {
  display: contents;
}
/* The band lives directly on the real content box (.nav-routes-wrap
   here; .nav-row2-group takes over at mobile, see below) rather than
   a separate decorative element stretched to fill the row. A stretched
   *empty* grid item depends on the row's auto-sizing already being
   correct before it can fill it — content-driven sizing on a real box
   doesn't have that circular dependency and behaves the same across
   browsers. (A previous version used a negative margin to bleed an
   empty band element past this container's padding to reach the real
   border — that desynced the item's layout-height contribution from
   what it visually painted, so page content below started flowing
   before the header visually ended. Both issues, same root cause:
   asking an empty element to define a size nothing else confirms.) */
.nav-routes-wrap {
  grid-area: routes;
  position: relative;
  min-width: 0;
  justify-self: stretch;
  padding: 0.35rem var(--space-3);
  background: var(--routes-band);
  border-radius: var(--radius-md);
}
.nav-utilities {
  grid-area: utils;
  justify-self: end;
}
@media (max-width: 47.99rem) {
  /* Each nav-item icon+gap costs ~24px; on a 4-item nav sharing row 2
     with admin controls, that's ~96px of width spent on decoration
     instead of the labels themselves, on the row most likely to
     actually run out of room. Text-only here, not worth the space. */
  .nav-item svg {
    display: none;
  }
  /* Narrow screens: brand + the default icon set (search/rss/theme,
     plus sign-in when logged out or logout when logged in) share the
     top row — every visitor, logged in or not, gets that same icon
     set out of the routes row's way, so the nav labels ("Posts",
     "About", …) always get the full row instead of being squeezed
     down to a sliver next to icons. Signed-in admin controls
     (compose/moderation/settings) go the other way: down into row 2
     alongside routes, since those are extra to a logged-out visitor's
     default experience, not the baseline. */
  /* .nav-routes-wrap .nav-routes (not just .nav-routes) — same
     specificity trap as the header-row cascade bug: this media query
     is declared before the base .nav-routes rule in source order, so
     without the extra specificity the later, unconditional rule would
     keep winning even inside the media query. */
  .nav-routes-wrap .nav-routes {
    justify-content: flex-start;
  }
  .header-inner {
    grid-template-columns: 1fr auto;
    grid-template-areas:
      'brand icons'
      'row2 row2';
    row-gap: var(--space-2);
  }
  .nav-utilities {
    grid-area: icons;
    /* Row 2's content sits inset from the header's true right edge
       (the band's own padding) — match it here so both rows' rightmost
       icons line up instead of row 1 sitting flush against the edge
       while row 2 sits a few px short of it. */
    padding-right: var(--space-3);
  }
  .nav-row2-group {
    grid-area: row2;
    display: flex;
    align-items: center;
    min-width: 0;
    /* The band moves here from .nav-routes-wrap — at this width it's
       only part of row 2 (.nav-admin, when signed in, shares it), so
       the band needs to be the whole flex row, not just one child. */
    background: var(--routes-band);
    border-radius: var(--radius-md);
  }
  .nav-routes-wrap {
    flex: 1;
    min-width: 0;
    background: none;
  }
  .nav-admin {
    flex-shrink: 0;
    justify-self: auto;
    padding: 0.35rem var(--space-3);
    /* Natural wrap only — a forced max-width here wrapped this group
       to 2 rows unconditionally, even with a short nav (2 items) that
       had plenty of room and no reason to lose the single-row shape
       every other icon group keeps. Let it wrap only when routes
       genuinely need the width back, not as a permanent height tax. */
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: var(--space-2);
  }
  .mobile-page-title {
    display: inline;
    color: var(--accent);
    font-weight: 600;
    font-size: var(--text-sm);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
}
.brand {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-family: var(--font-serif);
  font-size: var(--text-xl);
  font-weight: 600;
  color: var(--text);
  text-decoration: none;
}
.brand-logo {
  width: 1.5rem;
  height: 1.5rem;
  object-fit: contain;
  border-radius: var(--radius-sm, 4px);
}
nav {
  font-size: var(--text-sm);
  align-items: center;
}
.nav-routes {
  gap: var(--space-3);
  min-width: 0;
  overflow-x: auto;
  flex-wrap: nowrap;
  /* Desktop-only (see media query below) — centering an *overflowing*
     scrollable row also centers its initial scroll position, which
     pushes the first item off the left edge instead of just visually
     centering content that fits. Mobile route rows can overflow
     (routes competing with admin icons for width when signed in), so
     they need to stay left-anchored instead. */
  justify-content: center;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
}
.nav-routes::-webkit-scrollbar {
  display: none;
}
.nav-routes-fade {
  position: absolute;
  inset-block: 0;
  width: 2rem;
  pointer-events: none;
  z-index: 1;
}
.nav-routes-fade-left {
  left: 0;
  background: linear-gradient(to right, var(--routes-band) 40%, transparent);
}
.nav-routes-fade-right {
  right: 0;
  background: linear-gradient(to left, var(--routes-band) 40%, transparent);
}
.nav-utilities {
  gap: var(--space-2);
  flex-wrap: wrap;
  justify-content: flex-end;
}
nav a {
  text-decoration: none;
}
.nav-item {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  gap: 0.35rem;
  color: var(--muted);
  padding: 0.2rem 0.35rem;
}
.nav-item:hover,
.nav-item.active {
  color: var(--accent);
}
.nav-routes-skeleton {
  gap: var(--space-3);
}
.nav-item-skeleton {
  display: inline-block;
  flex-shrink: 0;
  width: 3.5rem;
  height: 1.1em;
  margin: 0.2rem 0.35rem;
  border-radius: var(--radius-sm, 4px);
  background: linear-gradient(
    90deg,
    color-mix(in srgb, var(--muted) 18%, transparent) 25%,
    color-mix(in srgb, var(--muted) 30%, transparent) 50%,
    color-mix(in srgb, var(--muted) 18%, transparent) 75%
  );
  background-size: 200% 100%;
  animation: nav-skeleton-shimmer 1.4s ease-in-out infinite;
}
.nav-item-skeleton:nth-child(2) {
  width: 4.5rem;
}
.nav-item-skeleton:nth-child(3) {
  width: 3rem;
}
@keyframes nav-skeleton-shimmer {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -200% 0;
  }
}
@media (prefers-reduced-motion: reduce) {
  .nav-item-skeleton {
    animation: none;
  }
}
.nav-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 0;
  color: var(--muted);
}
/* Double class beats button.nav-icon's background/color reset below —
   this element is now a <button> (was a RouterLink/<a>), and
   button.nav-icon's element+class specificity would otherwise win
   regardless of source order. */
.nav-icon.nav-icon-magic {
  position: relative;
  width: 1.75rem;
  height: 1.75rem;
  border-radius: 50%;
  background: color-mix(in srgb, var(--accent) 16%, transparent);
  color: var(--accent);
}
.nav-icon.nav-icon-magic:hover {
  background: color-mix(in srgb, var(--accent) 28%, transparent);
  color: var(--accent);
}
/* Always-visible corner badge — the glyph inside changes (pencil/back
   arrow) to say what happens next, but this stays fixed to mark the
   button itself as a toggle, not two different actions. */
.nav-icon-magic-badge {
  position: absolute;
  right: -0.25rem;
  bottom: -0.25rem;
  padding: 0.12rem;
  border-radius: 50%;
  background: var(--surface);
  color: var(--muted);
  box-shadow: 0 0 0 1px var(--surface);
}
/* Compose ↔ Author-list glyph swap on the same icon slot — the badge
   stays fixed to mark this as a toggle, only the center glyph
   cross-fades/rotates to say what happens next. */
.magic-icon-enter-active,
.magic-icon-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}
.magic-icon-enter-from {
  opacity: 0;
  transform: rotate(-45deg) scale(0.7);
}
.magic-icon-leave-to {
  opacity: 0;
  transform: rotate(45deg) scale(0.7);
}
button.nav-icon {
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  color: var(--muted);
  font: inherit;
}
button.nav-icon:hover {
  color: var(--accent);
}
.nav-icon:hover,
.nav-icon.active,
nav a.router-link-active.nav-icon {
  color: var(--accent);
}
.nav-icon-badge {
  position: relative;
}
.badge-count {
  position: absolute;
  top: -0.4rem;
  right: -0.5rem;
  min-width: 1.05rem;
  height: 1.05rem;
  padding: 0 0.2rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: var(--accent);
  color: #fff;
  font-size: 0.62rem;
  font-weight: 700;
  line-height: 1;
}
.nav-sep {
  width: 1px;
  height: 1.15em;
  background: var(--border);
  flex-shrink: 0;
}
.nav-group {
  gap: var(--space-2);
}
.site-main {
  flex: 1;
  max-width: var(--content-max);
  width: 100%;
  margin: 0 auto;
  padding: var(--space-8) var(--header-pad-x) var(--space-12);
}
.site-footer {
  border-top: 1px solid var(--border);
  padding: var(--space-5);
  /* a touch brighter than --muted (used elsewhere for lower-priority UI
     like nav labels) — footer text is small and reads as too washed out
     at the plain --muted shade */
  color: color-mix(in srgb, var(--muted) 70%, var(--text) 30%);
  font-size: var(--text-xs);
}
.site-footer-row {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: var(--space-3);
}
.site-footer p {
  grid-column: 2;
  margin: 0;
  text-align: center;
}
.site-footer-meta {
  grid-column: 3;
  display: flex;
  align-items: center;
  justify-self: end;
  gap: var(--space-3);
}
@media (max-width: 30rem) {
  /* Not enough room for the three-column layout to keep the title
     genuinely centered once the GitHub link's width becomes
     significant relative to the footer — stack instead. */
  .site-footer-row {
    grid-template-columns: 1fr;
    justify-items: center;
  }
  .site-footer p {
    grid-column: 1;
  }
  .site-footer-meta {
    grid-column: 1;
    justify-self: center;
  }
}
.site-footer-github {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: inherit;
  text-decoration: none;
}
.site-footer-github:hover {
  color: var(--accent);
}
.site-footer-wordmark {
  font-weight: 700;
  letter-spacing: -0.01em;
}
.site-footer-wordmark .n {
  color: #5ea1e0;
}
.site-footer-wordmark .i {
  color: #3b82c4;
}
.site-footer-wordmark .t {
  color: #0969da;
}
.site-footer-wordmark .pub {
  color: #033d85;
}
@media (prefers-color-scheme: dark) {
  :global(html:not([data-scheme='light'])) .site-footer-wordmark .n {
    color: #9ecbff;
  }
  :global(html:not([data-scheme='light'])) .site-footer-wordmark .i {
    color: #79c0ff;
  }
  :global(html:not([data-scheme='light'])) .site-footer-wordmark .t {
    color: #4493f8;
  }
  :global(html:not([data-scheme='light'])) .site-footer-wordmark .pub {
    color: #1a56c4;
  }
}
:global(html[data-scheme='dark']) .site-footer-wordmark .n {
  color: #9ecbff;
}
:global(html[data-scheme='dark']) .site-footer-wordmark .i {
  color: #79c0ff;
}
:global(html[data-scheme='dark']) .site-footer-wordmark .t {
  color: #4493f8;
}
:global(html[data-scheme='dark']) .site-footer-wordmark .pub {
  color: #1a56c4;
}
.site-footer-version {
  font-variant-numeric: tabular-nums;
}
</style>
