<script setup lang="ts">
import { ArrowLeft, Pencil } from '@lucide/vue'
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import LinkCollectionPage from '../components/LinkCollectionPage.vue'
import MarkdownBody from '../components/MarkdownBody.vue'
import { fetchSitePage, type SitePage } from '../lib/site'

const props = defineProps<{ pagePath: string }>()

const page = ref<SitePage | null>(null)
const error = ref('')
const loading = ref(true)

// Only present when the viewer is authenticated and the page has a known
// backing file — the API omits `file` entirely for an unauthenticated
// request (see ServeSitePage), so this doubles as the auth check itself.
const editHref = computed(() =>
  page.value?.file
    ? { path: '/admin/site', query: { siteTab: 'files', file: page.value.file } }
    : null,
)

async function load(path: string) {
  loading.value = true
  error.value = ''
  page.value = null
  const urlPath = '/' + path.replace(/^\//, '')
  try {
    page.value = await fetchSitePage(urlPath)
  } catch (e) {
    error.value = e instanceof Error && e.message === 'not-found' ? 'Page not found.' : 'Could not load page.'
  } finally {
    loading.value = false
  }
}

onMounted(() => load(props.pagePath))
watch(() => props.pagePath, load)
</script>

<template>
  <div class="page-toolbar">
    <RouterLink to="/" class="text-link">
      <ArrowLeft :size="16" :stroke-width="1.75" aria-hidden="true" />
      <span>Home</span>
    </RouterLink>
    <RouterLink v-if="editHref" :to="editHref" class="text-link edit-link">
      <Pencil :size="16" :stroke-width="1.75" aria-hidden="true" />
      <span>Edit</span>
    </RouterLink>
  </div>

  <p v-if="loading" class="status">Loading…</p>
  <p v-else-if="error" class="status error">{{ error }}</p>

  <article v-else-if="page" class="custom-page">
    <template v-if="page.type === 'markdown'">
      <h1 v-if="page.title">{{ page.title }}</h1>
      <MarkdownBody :content="page.body" />
    </template>
    <LinkCollectionPage v-else :title="page.title" :links="page.links" />
  </article>
</template>

<style scoped>
.page-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 0 0 1.5rem;
  font-size: 0.9rem;
}
.page-toolbar a {
  color: var(--muted);
  text-decoration: none;
}
.text-link {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}
.page-toolbar a:hover {
  color: var(--accent);
}
.custom-page h1 {
  font-family: var(--font-serif);
  margin: 0 0 1rem;
}
.status {
  color: var(--muted);
}
.status.error {
  color: var(--danger);
}
</style>
