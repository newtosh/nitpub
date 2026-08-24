import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import { loadColorSchemePreference } from './lib/color-scheme-preference'
import { fetchAppearance } from './lib/auth'
import { useTheme } from './composables/useTheme'
import './style.css'
import './styles/hljs.css'
import 'markdown-it-github-alerts/styles/github-colors-light.css'
import 'markdown-it-github-alerts/styles/github-base.css'
import './lib/markdown.css'

if ('scrollRestoration' in history) {
  // The default 'auto' lets the browser's own back/forward scroll
  // restoration fight with Vue Router's scrollBehavior below — on iOS
  // Safari specifically that produced a page stuck mid-scroll after the
  // login redirect instead of either behavior winning cleanly.
  history.scrollRestoration = 'manual'
}

async function bootstrap() {
  const { applyAppearance } = useTheme()
  const scheme = loadColorSchemePreference()
  const root = document.documentElement
  const fromHTML = root.dataset.theme
  if (fromHTML) {
    applyAppearance(fromHTML, scheme, root, false)
  } else {
    const appearance = await fetchAppearance()
    if (appearance?.theme_id) {
      applyAppearance(appearance.theme_id, scheme, root, false)
    } else {
      applyAppearance('github', scheme, root, false)
    }
  }
  createApp(App).use(router).mount('#app')
}

void bootstrap()
