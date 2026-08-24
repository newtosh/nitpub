<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import MarkdownEditor from './MarkdownEditor.vue'
import NavBarPreview from './NavBarPreview.vue'
import InfoTip from './InfoTip.vue'
import { allowedIcons } from '../lib/icons'
import {
  fetchAdminSite,
  importPosts,
  saveManifest,
  saveSiteFile,
  type AdminSiteResponse,
} from '../lib/adminSite'
import { clearSiteConfigCache } from '../lib/site'

const DEFAULT_TAGLINE = 'nitpub personal blog'

const SITE_TABS = ['settings', 'structure', 'files', 'import'] as const
type SiteTab = (typeof SITE_TABS)[number]

function isSiteTab(value: string): value is SiteTab {
  return (SITE_TABS as readonly string[]).includes(value)
}

function tabFromRoute(query: Record<string, unknown>): SiteTab {
  const raw = query.siteTab
  if (typeof raw === 'string' && isSiteTab(raw)) return raw
  return 'settings'
}

const route = useRoute()
const router = useRouter()

const loading = ref(true)
const saving = ref(false)
const error = ref('')
const message = ref('')
const tab = ref<SiteTab>(tabFromRoute(route.query))
const data = ref<AdminSiteResponse | null>(null)
const selectedFile = ref('')
const fileContent = ref('')
const importKind = ref('article')
const importErrors = ref<string[]>([])

// null/undefined means "on" (see FooterConfig.GithubLinkEnabled) — the
// checkbox needs a concrete boolean, and once touched it should stay
// explicit rather than snapping back to null.
const footerShowGithub = computed<boolean>({
  get: () => data.value?.manifest.footer?.show_github_link ?? true,
  set: (v) => {
    if (data.value) data.value.manifest.footer.show_github_link = v
  },
})

// null/undefined means "on" (see ContentConfig.ExternalLinksOpenNewTab) —
// same reasoning as footerShowGithub above.
const contentExternalLinksNewTab = computed<boolean>({
  get: () => data.value?.manifest.content?.external_links_new_tab ?? true,
  set: (v) => {
    if (data.value) {
      if (!data.value.manifest.content) data.value.manifest.content = {}
      data.value.manifest.content.external_links_new_tab = v
    }
  },
})

const brandingUploadError = ref('')
const brandingUploading = ref<'favicon_url' | 'logo_url' | null>(null)

async function onBrandingImageSelected(event: Event, field: 'favicon_url' | 'logo_url') {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file || !data.value) return
  brandingUploadError.value = ''
  brandingUploading.value = field
  const form = new FormData()
  form.append('file', file)
  try {
    const res = await fetch('/api/media', {
      method: 'POST',
      credentials: 'include',
      body: form,
    })
    if (!res.ok) {
      brandingUploadError.value = (await res.text()) || 'Upload failed'
      return
    }
    const uploaded = (await res.json()) as { url: string }
    data.value.manifest.branding[field] = uploaded.url
  } catch {
    brandingUploadError.value = 'Upload failed'
  } finally {
    brandingUploading.value = null
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    data.value = await fetchAdminSite()
    if (!data.value.manifest.nav) data.value.manifest.nav = []
    if (!data.value.manifest.pages) data.value.manifest.pages = []
    if (!data.value.manifest.footer) data.value.manifest.footer = { text: '' }
    if (!data.value.manifest.branding) data.value.manifest.branding = {}
    if (!selectedFile.value && data.value.files.length) {
      // A deep link from a public page's Edit button (?file=pages/x.md)
      // pre-selects that exact file; otherwise fall back to the first one.
      const requested = typeof route.query.file === 'string' ? route.query.file : ''
      const match = requested && data.value.files.some((f) => f.path === requested)
      selectFile(match ? requested : data.value.files[0].path)
    }
  } catch {
    error.value = 'Could not load site files.'
  } finally {
    loading.value = false
  }
}

function selectFile(path: string) {
  selectedFile.value = path
  const f = data.value?.files.find((x) => x.path === path)
  fileContent.value = f?.content ?? ''
}

function slugFromPath(path: string): string {
  return path.replace(/^\//, '').trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'page'
}

function starterForPage(type: 'markdown' | 'links', slug: string): string {
  if (type === 'links') {
    return `title = "${slug}"\n\n[[links]]\ntitle = "Example"\nurl = "https://example.com"\n`
  }
  const title = slug.replace(/-/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
  return `# ${title}\n\n`
}

function syncPageFile(page: { path: string; type: string; file: string }) {
  const slug = slugFromPath(page.path)
  page.file = page.type === 'links' ? `pages/${slug}.links.toml` : `pages/${slug}.md`
}

function addPage(type: 'markdown' | 'links') {
  if (!data.value) return
  const n = data.value.manifest.pages!.length + 1
  const slug = `page-${n}`
  const path = `/${slug}`
  const file = type === 'markdown' ? `pages/${slug}.md` : `pages/${slug}.links.toml`
  const content = starterForPage(type, slug)
  data.value.manifest.pages!.push({ path, type, file })
  if (!data.value.files.some((f) => f.path === file)) {
    data.value.files.push({ path: file, content })
  }
  selectedFile.value = file
  fileContent.value = content
}

function removePage(i: number) {
  data.value?.manifest.pages?.splice(i, 1)
}

function onPagePathChange(page: { path: string; type: string; file: string }) {
  syncPageFile(page)
  const existing = data.value?.files.find((f) => f.path === page.file)
  if (!existing && data.value) {
    const type = page.type === 'links' ? 'links' : 'markdown'
    data.value.files.push({
      path: page.file,
      content: starterForPage(type, slugFromPath(page.path)),
    })
  }
  selectFile(page.file)
}

async function saveNewPageFiles() {
  if (!data.value) return
  for (const page of data.value.manifest.pages ?? []) {
    const f = data.value.files.find((x) => x.path === page.file)
    if (!f) continue
    try {
      await saveSiteFile(f.path, f.content)
    } catch (e) {
      throw new Error(`Could not save ${f.path}: ${e instanceof Error ? e.message : 'error'}`)
    }
  }
}

async function saveManifestForm() {
  if (!data.value) return
  saving.value = true
  message.value = ''
  error.value = ''
  try {
    await saveNewPageFiles()
    await saveManifest(data.value.manifest)
    clearSiteConfigCache()
    message.value = 'Site configuration saved.'
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Save failed.'
  } finally {
    saving.value = false
  }
}

async function saveSelectedFile() {
  if (!selectedFile.value) return
  saving.value = true
  message.value = ''
  error.value = ''
  try {
    await saveSiteFile(selectedFile.value, fileContent.value)
    clearSiteConfigCache()
    message.value = 'File saved.'
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Save failed.'
  } finally {
    saving.value = false
  }
}

function addNavItem() {
  if (!data.value) return
  data.value.manifest.nav.push({ label: 'New', path: '/new', icon: 'link' })
}

function removeNavItem(i: number) {
  data.value?.manifest.nav.splice(i, 1)
}

function syncTabToRoute(next: SiteTab) {
  const query = { ...route.query }
  if (next === 'settings') {
    delete query.siteTab
  } else {
    query.siteTab = next
  }
  const sameTab =
    (next === 'settings' && route.query.siteTab === undefined) ||
    route.query.siteTab === next
  if (sameTab && route.path === '/admin/site') return
  router.replace({ path: '/admin/site', query })
}

function selectTab(next: SiteTab) {
  tab.value = next
}

watch(tab, (next) => syncTabToRoute(next))

watch(
  () => route.query.siteTab,
  (raw) => {
    const next = typeof raw === 'string' && isSiteTab(raw) ? raw : 'settings'
    if (next !== tab.value) tab.value = next
  },
)

async function onImportFiles(ev: Event) {
  const input = ev.target as HTMLInputElement
  if (!input.files?.length) return
  saving.value = true
  importErrors.value = []
  message.value = ''
  try {
    const res = await importPosts(input.files, importKind.value)
    message.value = `Imported ${res.imported} post(s).`
    importErrors.value = res.errors
    input.value = ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Import failed.'
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  await load()
  if (typeof route.query.siteTab === 'string' && isSiteTab(route.query.siteTab)) {
    syncTabToRoute(route.query.siteTab)
    requestAnimationFrame(() => {
      document.getElementById('site')?.scrollIntoView({ behavior: 'auto', block: 'start' })
    })
  }
})
</script>

<template>
  <div class="site-editor stack">
    <nav class="tabs cluster" aria-label="Site editor sections">
      <button type="button" :class="{ active: tab === 'settings' }" @click="selectTab('settings')">Settings</button>
      <button type="button" :class="{ active: tab === 'structure' }" @click="selectTab('structure')">Pages &amp; navigation</button>
      <button type="button" :class="{ active: tab === 'files' }" @click="selectTab('files')">Page files</button>
      <button type="button" :class="{ active: tab === 'import' }" @click="selectTab('import')">Import posts</button>
    </nav>

    <p v-if="loading" class="status">Loading…</p>
    <p v-else-if="error" class="status error">{{ error }}</p>
    <p v-if="message" class="status ok">{{ message }}</p>

    <template v-if="!loading && data">
      <section v-show="tab === 'settings'" class="stack">
        <h3 class="section-title">
          Branding
          <InfoTip label="Brand icon shown beside the site title in the header, and the browser-tab favicon. Both optional — leave unset to use the default." />
        </h3>
        <div class="branding-row">
          <label class="field branding-field">
            <span>Brand icon</span>
            <div class="branding-upload">
              <img v-if="data.manifest.branding.logo_url" :src="data.manifest.branding.logo_url" class="branding-preview" alt="" />
              <input type="file" accept="image/*,.ico" :disabled="brandingUploading === 'logo_url'" @change="onBrandingImageSelected($event, 'logo_url')" />
              <button
                v-if="data.manifest.branding.logo_url"
                type="button"
                class="btn btn-ghost"
                @click="data.manifest.branding.logo_url = ''"
              >
                Clear
              </button>
            </div>
          </label>
          <label class="field branding-field">
            <span>Favicon</span>
            <div class="branding-upload">
              <img v-if="data.manifest.branding.favicon_url" :src="data.manifest.branding.favicon_url" class="branding-preview" alt="" />
              <input type="file" accept="image/*,.ico" :disabled="brandingUploading === 'favicon_url'" @change="onBrandingImageSelected($event, 'favicon_url')" />
              <button
                v-if="data.manifest.branding.favicon_url"
                type="button"
                class="btn btn-ghost"
                @click="data.manifest.branding.favicon_url = ''"
              >
                Clear
              </button>
            </div>
          </label>
        </div>
        <p v-if="brandingUploadError" class="status error">{{ brandingUploadError }}</p>

        <h3 class="section-title">
          Fediverse bio
          <InfoTip label="Profile description shown when this site's account is viewed on Mastodon and other fediverse apps." />
        </h3>
        <label class="field">
          <span>Tagline</span>
          <input v-model="data.manifest.branding.tagline" :placeholder="DEFAULT_TAGLINE" />
        </label>

        <h3 class="section-title">
          Footer
          <InfoTip label="Shown site-wide at the bottom of every page." />
        </h3>
        <label class="field">
          <span>Footer text</span>
          <input v-model="data.manifest.footer.text" placeholder="Self-hosted notes & articles" />
        </label>
        <label class="field checkbox">
          <input v-model="footerShowGithub" type="checkbox" />
          <span>Link to the nitpub project on GitHub</span>
        </label>
        <h3 class="section-title">
          Content
          <InfoTip label="Applies to links rendered from markdown in articles, notes, and custom pages. Same-site links always stay in the current tab regardless." />
        </h3>
        <label class="field checkbox">
          <input v-model="contentExternalLinksNewTab" type="checkbox" />
          <span>Open external links in a new tab</span>
        </label>
        <h3 class="section-title">
          Home
          <InfoTip label="How many recent posts appear on the home page. Use 0 to show all posts." />
        </h3>
        <label class="field">
          <span>Recent post count (0 = show all)</span>
          <input v-model.number="data.manifest.home.recent_count" type="number" min="0" />
        </label>
        <h3 class="section-title">
          Archive
          <InfoTip label="How the /posts archive loads: paginated pages or infinite scroll." />
        </h3>
        <label class="field">
          <span>Archive mode</span>
          <select v-model="data.manifest.archive.mode">
            <option value="pagination">Pagination</option>
            <option value="infinite">Infinite scroll</option>
          </select>
        </label>
        <label class="field">
          <span>Archive page size</span>
          <input v-model.number="data.manifest.archive.page_size" type="number" min="1" />
        </label>
        <h3 class="section-title">
          Search
          <InfoTip label="Shows the search icon in the site header and enables /search." />
        </h3>
        <label class="field checkbox">
          <input v-model="data.manifest.search.enabled" type="checkbox" />
          <span>Enable search</span>
        </label>
        <div class="form-actions">
          <button type="button" class="btn btn-primary" :disabled="saving" @click="saveManifestForm">
            {{ saving ? 'Saving…' : 'Save settings' }}
          </button>
        </div>
      </section>

      <section v-show="tab === 'structure'" class="stack">
        <NavBarPreview
          :nav="data.manifest.nav"
          :pages="data.manifest.pages"
          :search-enabled="data.manifest.search?.enabled !== false"
        />

        <div class="nav-editor">
          <h3>
            Pages
            <InfoTip label="Maps a URL to a content file under site/pages/. New files are created when you save." />
          </h3>
          <div v-for="(page, i) in data.manifest.pages" :key="i" class="page-row">
            <input v-model="page.path" placeholder="/about" @change="onPagePathChange(page)" />
            <select v-model="page.type" @change="onPagePathChange(page)">
              <option value="markdown">Markdown</option>
              <option value="links">Links</option>
            </select>
            <input v-model="page.file" placeholder="pages/about.md" spellcheck="false" />
            <button type="button" class="btn" @click="removePage(i)">Remove</button>
          </div>
          <div class="form-actions">
            <button type="button" class="btn" @click="addPage('markdown')">Add markdown page</button>
            <button type="button" class="btn" @click="addPage('links')">Add link collection</button>
          </div>
        </div>

        <div class="nav-editor">
          <h3>
            Navigation
            <InfoTip label="Links shown in the site header. Custom links should point at routes registered in Pages." />
          </h3>
          <div v-for="(item, i) in data.manifest.nav" :key="i" class="nav-row">
            <input v-model="item.label" placeholder="Label" />
            <input v-model="item.path" placeholder="/path" />
            <select v-model="item.icon">
              <option value="">No icon</option>
              <option v-for="icon in allowedIcons" :key="icon" :value="icon">{{ icon }}</option>
            </select>
            <button type="button" class="btn" @click="removeNavItem(i)">Remove</button>
          </div>
          <div class="form-actions">
            <button type="button" class="btn" @click="addNavItem">Add nav item</button>
          </div>
        </div>

        <div class="form-actions">
          <button type="button" class="btn btn-primary" :disabled="saving" @click="saveManifestForm">
            {{ saving ? 'Saving…' : 'Save pages &amp; navigation' }}
          </button>
        </div>
      </section>

      <section v-show="tab === 'files'" class="stack">
        <template v-if="data.files.length">
          <label class="field">
            <span>File</span>
            <select v-model="selectedFile" @change="selectFile(selectedFile)">
              <option v-for="f in data.files" :key="f.path" :value="f.path">{{ f.path }}</option>
            </select>
          </label>
          <label v-if="selectedFile.endsWith('.md')" class="field">
            <span>Content</span>
            <MarkdownEditor v-model="fileContent" :rows="16" profile="article" :show-hint="false" />
          </label>
          <label v-else class="field">
            <span>Content</span>
            <textarea v-model="fileContent" rows="16" spellcheck="false" />
          </label>
          <div class="form-actions">
            <button type="button" class="btn" @click="addPage('markdown'); selectTab('files')">
              New markdown file
            </button>
            <button type="button" class="btn btn-primary" :disabled="saving" @click="saveSelectedFile">
              {{ saving ? 'Saving…' : 'Save file' }}
            </button>
          </div>
        </template>
        <template v-else>
          <p class="status">
            No page files yet. Add a page in Pages &amp; navigation or create one below.
          </p>
          <div class="form-actions">
            <button type="button" class="btn" @click="addPage('markdown'); selectTab('files')">
              New markdown file
            </button>
          </div>
        </template>
      </section>

      <section v-show="tab === 'import'" class="stack">
        <p class="hint">Upload <code>.md</code> files to import as posts. Optional frontmatter: <code>kind: note</code> or <code>kind: article</code>.</p>
        <label class="field">
          <span>Default kind</span>
          <select v-model="importKind">
            <option value="article">Article</option>
            <option value="note">Note</option>
          </select>
        </label>
        <label class="field">
          <span>Markdown files</span>
          <input type="file" accept=".md,text/markdown" multiple @change="onImportFiles" />
        </label>
        <ul v-if="importErrors.length" class="errors">
          <li v-for="(e, i) in importErrors" :key="i">{{ e }}</li>
        </ul>
      </section>
    </template>
  </div>
</template>

<style scoped>
.tabs {
  gap: var(--space-2);
  margin-bottom: var(--space-4);
}
.tabs button {
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface);
  color: var(--text);
  cursor: pointer;
  font: inherit;
}
.tabs button.active {
  border-color: var(--accent);
  color: var(--accent);
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  font-size: var(--text-sm);
}
.field input,
.field select,
.field textarea {
  padding: var(--space-2);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface);
  color: var(--text);
  font: inherit;
}
.branding-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-4);
}
.branding-field {
  flex: 1 1 14rem;
}
.branding-upload {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
.branding-preview {
  width: 2rem;
  height: 2rem;
  object-fit: contain;
  border-radius: var(--radius-sm, 4px);
  border: 1px solid var(--border);
  background: var(--surface);
  flex-shrink: 0;
}
.field.checkbox {
  flex-direction: row;
  align-items: center;
}
.section-title,
.nav-editor h3 {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: var(--space-4) 0 var(--space-2);
  font-size: var(--text-base);
}
.section-title:first-of-type {
  margin-top: 0;
}
.nav-row,
.page-row {
  display: grid;
  grid-template-columns: 1fr 1fr 8rem auto;
  gap: var(--space-2);
  margin-bottom: var(--space-2);
}
.page-row {
  grid-template-columns: 1fr 7rem 1.2fr auto;
}
.hint {
  color: var(--muted);
  font-size: var(--text-sm);
}
.status.ok {
  color: var(--accent);
}
.status.error {
  color: var(--danger);
}
.errors {
  color: var(--danger);
  font-size: var(--text-sm);
}
@media (max-width: 47.99rem) {
  /* iOS Safari auto-zooms on focus when an input's effective font-size
     is under 16px — .field's 0.9rem inherits down via font:inherit,
     and the page-row/nav-row inputs aren't covered by .field at all. */
  input,
  textarea,
  select {
    font-size: 16px;
  }
}
</style>
