<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import AdminSectionNav, { type AdminSection } from '../components/AdminSectionNav.vue'
import AnalyticsDashboard from '../components/AnalyticsDashboard.vue'
import FederationSettings from '../components/FederationSettings.vue'
import InfoTip from '../components/InfoTip.vue'
import SecurityCard from '../components/SecurityCard.vue'
import SiteEditor from '../components/SiteEditor.vue'
import ThemePicker from '../components/ThemePicker.vue'
import VersionCheck from '../components/VersionCheck.vue'
import { useSession } from '../composables/useSession'
import { fetchSiteConfig, getCachedSiteConfig } from '../lib/site'

// section comes straight from the route (/admin/:section) — each section is
// its own real page now, not a scroll position on one long page. That's a
// deliberate simplification: scroll-spy tracking which section is "current"
// (offset math, atBottom edge cases, listening on the right scroll
// container) kept breaking in ways a plain active-route comparison can't.
const props = defineProps<{ section: string }>()

const { authed, refresh } = useSession()

const baseSections: (AdminSection & { info: string })[] = [
  {
    id: 'appearance',
    title: 'Appearance',
    info: 'Choose a color theme and typography for how the site looks to visitors.',
  },
  {
    id: 'site',
    title: 'Site',
    info: 'Manage site settings, navigation, and custom pages.',
  },
  {
    id: 'federation',
    title: 'Federation',
    info: 'Configure ActivityPub delivery and moderation for federated replies.',
  },
  {
    id: 'security',
    title: 'Security',
    info: 'Manage the admin password, two-factor auth, and passkeys.',
  },
  {
    id: 'system',
    title: 'System',
    info: 'Check the running build against the latest release.',
  },
]

// analytics_enabled is a deploy-time flag (internal/config), so the
// Analytics entry only exists when it's on — seeded synchronously from the
// cached config (same pattern as MarkdownBody.vue) so a hard refresh
// doesn't flash the entry in/out, then corrected once fetchSiteConfig()
// resolves. Always appended last, after the five fixed sections above.
const analyticsEnabled = ref(getCachedSiteConfig()?.analytics_enabled ?? false)
const adminSections = computed<(AdminSection & { info: string })[]>(() =>
  analyticsEnabled.value
    ? [
        ...baseSections,
        { id: 'analytics', title: 'Analytics', info: 'Self-hosted visitor analytics via GoatCounter.' },
      ]
    : baseSections,
)

// An unrecognized :section (bad bookmark, typo) falls back to Appearance
// rather than rendering nothing — the URL itself isn't corrected, since
// that's a minor edge case not worth a redirect for.
const activeSection = computed(() => {
  const ids = adminSections.value.map((s) => s.id)
  return ids.includes(props.section) ? props.section : 'appearance'
})
const activeSectionMeta = computed(
  () => adminSections.value.find((s) => s.id === activeSection.value) ?? adminSections.value[0],
)

onMounted(async () => {
  refresh()
  try {
    const config = await fetchSiteConfig()
    analyticsEnabled.value = config.analytics_enabled ?? false
  } catch {
    // Cached/seeded value stands; the section nav just won't self-correct
    // this load if the fetch fails.
  }
})
</script>

<template>
  <section v-if="!authed" class="card stack">
    <p>Sign in to manage this instance.</p>
    <div class="form-actions">
      <RouterLink class="btn btn-primary" to="/login" title="Sign in" aria-label="Sign in">
        Sign in
      </RouterLink>
    </div>
  </section>

  <template v-else>
    <Teleport to="#layout-subnav">
      <AdminSectionNav :sections="adminSections" :active-id="activeSection" />
    </Teleport>

    <article class="card stack admin-section">
      <div class="admin-title-row">
        <h1>Admin</h1>
        <h2>{{ activeSectionMeta.title }}</h2>
        <InfoTip :label="activeSectionMeta.info" />
      </div>
      <ThemePicker v-if="activeSection === 'appearance'" />
      <SiteEditor v-else-if="activeSection === 'site'" />
      <FederationSettings v-else-if="activeSection === 'federation'" />
      <SecurityCard v-else-if="activeSection === 'security'" />
      <VersionCheck v-else-if="activeSection === 'system'" />
      <AnalyticsDashboard v-else-if="activeSection === 'analytics'" />
    </article>
  </template>
</template>

<style scoped>
.admin-title-row {
  display: flex;
  align-items: baseline;
  gap: 0.6rem;
}
.admin-title-row h1 {
  margin: 0;
  font-family: var(--font-serif);
  font-size: var(--text-lg);
  font-weight: 600;
}
.admin-title-row h2 {
  margin: 0;
  padding-left: 0.6rem;
  border-left: 1px solid var(--border);
  color: var(--accent);
  font-family: var(--font-serif);
  font-size: var(--text-lg);
  font-weight: 600;
}
.admin-title-row :deep(.info-tip) {
  margin-left: -0.2rem;
}
.admin-section p {
  margin: 0;
  color: var(--muted);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}
</style>
