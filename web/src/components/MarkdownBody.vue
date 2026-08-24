<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { renderMarkdown } from '../lib/markdown'
import { bindYoutubeFacades } from '../lib/youtubeFacade'
import { hydrateLinkCards } from '../lib/linkCard'
import { hydrateIcons } from '../lib/phosphorIcons'
import { externalLinksNewTabFromConfig } from '../lib/contentConfig'
import { fetchSiteConfig, getCachedSiteConfig } from '../lib/site'

const props = defineProps<{
  content: string
  inlineLinkCards?: boolean
}>()

const root = ref<HTMLElement | null>(null)
// Seeded from the (possibly stale, possibly absent) cached config so first
// paint doesn't wait on a fetch; corrected below once the real value loads,
// same pattern PostPage.vue already uses for avatarsEnabledFromConfig.
const externalLinksNewTab = ref(externalLinksNewTabFromConfig(getCachedSiteConfig()?.content))
const html = computed(() =>
  renderMarkdown(props.content, {
    inlineLinkCards: props.inlineLinkCards,
    externalLinksNewTab: externalLinksNewTab.value,
  }),
)

async function hydrate() {
  await nextTick()
  bindYoutubeFacades(root.value)
  await Promise.all([hydrateLinkCards(root.value), hydrateIcons(root.value)])
}

onMounted(async () => {
  hydrate()
  try {
    const config = await fetchSiteConfig()
    externalLinksNewTab.value = externalLinksNewTabFromConfig(config.content)
  } catch {
    // Keep whatever the cache seeded above — not worth failing rendering
    // over a config refresh.
  }
})
watch(html, hydrate)
</script>

<template>
  <div ref="root" class="markdown-body" v-html="html" />
</template>
