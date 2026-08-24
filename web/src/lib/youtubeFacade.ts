/** Click-to-play YouTube facade (thumbnail until play). */

function isBareIP(hostname: string): boolean {
  return /^\d{1,3}(\.\d{1,3}){3}$/.test(hostname) || hostname.startsWith('[')
}

/** YouTube rejects inline embeds from bare IPs — hostname required (not just HTTPS). */
export function canEmbedYoutubeInline(): boolean {
  if (typeof window === 'undefined') return true
  return !isBareIP(window.location.hostname)
}

/** Origin for embed URL — only on real hostnames. */
export function youtubeEmbedOrigin(): string | null {
  if (typeof window === 'undefined') return null
  const { hostname, origin } = window.location
  if (!origin || origin === 'null' || isBareIP(hostname)) return null
  return origin
}

export function youtubeWatchUrl(videoId: string): string {
  return `https://www.youtube.com/watch?v=${encodeURIComponent(videoId)}`
}

export function youtubeFacadeHtml(videoId: string): string {
  const id = videoId.replace(/[^A-Za-z0-9_-]/g, '')
  const external = !canEmbedYoutubeInline()
  const mode = external ? 'external' : 'inline'
  const label = external ? 'Watch on YouTube (opens in new tab)' : 'Play YouTube video'
  const badge = external
    ? `<span class="yt-facade-badge"><svg class="yt-facade-external-icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/></svg>YouTube</span>`
    : ''
  return `<div class="embed embed-video yt-facade yt-facade--${mode}" data-yt-id="${id}" data-yt-mode="${mode}">
    <button type="button" class="yt-facade-btn" aria-label="${label}">
      <img class="yt-facade-poster" src="https://i.ytimg.com/vi/${id}/hqdefault.jpg" alt="" loading="lazy" decoding="async" />
      <span class="yt-facade-play" aria-hidden="true"></span>
      ${badge}
    </button>
  </div>`
}

export function activateYoutubeFacade(facade: HTMLElement): void {
  const videoId = facade.dataset.ytId
  if (!videoId) return

  const external =
    facade.dataset.ytMode === 'external' || !canEmbedYoutubeInline()

  if (external) {
    window.open(youtubeWatchUrl(videoId), '_blank', 'noopener')
    return
  }

  if (facade.classList.contains('yt-facade--active')) return
  facade.classList.add('yt-facade--active')

  const params = new URLSearchParams({
    autoplay: '1',
    playsinline: '1',
    rel: '0',
  })
  const origin = youtubeEmbedOrigin()
  if (origin) params.set('origin', origin)

  const iframe = document.createElement('iframe')
  iframe.src = `https://www.youtube-nocookie.com/embed/${encodeURIComponent(videoId)}?${params}`
  iframe.title = 'YouTube video'
  iframe.allow =
    'accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share'
  iframe.allowFullscreen = true
  iframe.referrerPolicy = 'strict-origin-when-cross-origin'

  facade.replaceChildren(iframe)
}

export function bindYoutubeFacades(root: HTMLElement | null): void {
  if (!root) return
  root.querySelectorAll<HTMLElement>('.yt-facade').forEach((facade) => {
    const btn = facade.querySelector<HTMLButtonElement>('.yt-facade-btn')
    if (!btn || btn.dataset.bound === '1') return
    btn.dataset.bound = '1'
    btn.addEventListener('click', (e) => {
      e.preventDefault()
      activateYoutubeFacade(facade)
    })
  })
}
