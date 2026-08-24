<script setup lang="ts">
import { ArrowRight } from '@lucide/vue'
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import PostCard from '../components/PostCard.vue'
import { fetchPosts, fetchPostsPage, type Post } from '../lib/posts'
import { fetchSiteConfig, type SiteConfig } from '../lib/site'

const posts = ref<Post[]>([])
const site = ref<SiteConfig | null>(null)
const error = ref('')
const loading = ref(true)
const total = ref(0)

onMounted(async () => {
  try {
    site.value = await fetchSiteConfig()
    const limit = site.value.home.recent_count
    if (limit > 0) {
      const page = await fetchPostsPage(limit, 0)
      posts.value = page.posts
      total.value = page.total
    } else {
      posts.value = await fetchPosts()
      total.value = posts.value.length
    }
  } catch {
    error.value = 'Could not load posts.'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <header class="page-header">
    <h1>Latest</h1>
    <p>Notes and long-form articles from this instance.</p>
  </header>

  <p v-if="loading" class="status">Loading…</p>
  <p v-else-if="error" class="status error">{{ error }}</p>
  <p v-else-if="posts.length === 0" class="status">No posts yet.</p>
  <template v-else>
    <section class="post-list">
      <PostCard v-for="post in posts" :key="post.id" :post="post" />
    </section>
    <p v-if="site && site.home.recent_count > 0 && total > posts.length" class="archive-link">
      <RouterLink to="/posts" class="text-link">
        <span>View all {{ total }} posts</span>
        <ArrowRight :size="16" :stroke-width="1.75" aria-hidden="true" />
      </RouterLink>
    </p>
  </template>
</template>

<style scoped>
.page-header h1 {
  font-family: var(--font-serif);
  font-size: 2rem;
  margin: 0 0 0.35rem;
}
.page-header p {
  margin: 0 0 1.5rem;
  color: var(--muted);
}
.status {
  color: var(--muted);
}
.status.error {
  color: var(--danger);
}
.archive-link {
  margin-top: var(--space-6);
  text-align: center;
}
.archive-link a {
  color: var(--accent);
  text-decoration: none;
}
.text-link {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}
</style>
