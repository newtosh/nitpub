import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { checkSession } from './lib/auth'
import BlogIndex from './views/BlogIndex.vue'
import PostPage from './views/PostPage.vue'
import LoginView from './views/LoginView.vue'
import AuthorView from './views/AuthorView.vue'
import ComposeView from './views/ComposeView.vue'
import AdminShellView from './views/AdminShellView.vue'
import ModerationView from './views/ModerationView.vue'
import EditPostView from './views/EditPostView.vue'
import WebAuthnEnrollView from './views/WebAuthnEnrollView.vue'
import PostsArchiveView from './views/PostsArchiveView.vue'
import SearchResultsView from './views/SearchResultsView.vue'
import CustomPageView from './views/CustomPageView.vue'

const routes: RouteRecordRaw[] = [
  { path: '/', name: 'home', component: BlogIndex },
  { path: '/posts', name: 'posts', component: PostsArchiveView },
  { path: '/search', name: 'search', component: SearchResultsView },
  { path: '/p/:slug', name: 'post', component: PostPage, props: true },
  { path: '/login', name: 'login', component: LoginView, meta: { guest: true } },
  {
    path: '/author',
    name: 'author',
    component: AuthorView,
    meta: { requiresAuth: true, title: 'Author' },
  },
  {
    path: '/author/compose',
    name: 'compose',
    component: ComposeView,
    meta: { requiresAuth: true, title: 'Compose' },
  },
  { path: '/author/enroll', name: 'webauthn-enroll', component: WebAuthnEnrollView },
  {
    path: '/author/edit/:slug',
    name: 'edit-post',
    component: EditPostView,
    props: true,
    meta: { requiresAuth: true, title: 'Edit post' },
  },
  { path: '/admin', redirect: '/admin/appearance' },
  {
    path: '/admin/moderation',
    name: 'admin-moderation',
    component: ModerationView,
    meta: { title: 'Moderation' },
  },
  {
    path: '/admin/:section',
    name: 'admin',
    component: AdminShellView,
    props: true,
    meta: { title: 'Admin' },
  },
  {
    path: '/:pagePath(.*)',
    name: 'custom-page',
    component: CustomPageView,
    props: (route) => ({ pagePath: route.params.pagePath as string }),
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
  // Vue Router doesn't reset scroll on navigation by default. iOS
  // Safari scrolls the page to keep a focused input (e.g. the login
  // form's password field) above the on-screen keyboard; that offset
  // survives the SPA redirect into /author since it's not a real page
  // load. Waiting a tick lets the new route's content mount first —
  // scrolling immediately, before layout has the new (usually taller)
  // page in place, is what made an earlier version of this fix land on
  // the wrong position instead of just failing to move at all.
  async scrollBehavior(to, from, savedPosition) {
    if (savedPosition) return savedPosition
    await new Promise((resolve) => requestAnimationFrame(resolve))
    return { top: 0 }
  },
})

router.beforeEach(async (to) => {
  const requiresAuth = to.matched.some((record) => record.meta.requiresAuth)
  const authed = await checkSession()

  if (requiresAuth && !authed) {
    return {
      path: '/login',
      query: { redirect: to.fullPath },
    }
  }

  if (to.meta.guest && authed) {
    const redirect =
      typeof to.query.redirect === 'string' &&
      to.query.redirect.startsWith('/') &&
      !to.query.redirect.startsWith('//')
        ? to.query.redirect
        : '/author'
    return redirect
  }
})
