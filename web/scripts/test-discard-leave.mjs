import { chromium } from 'playwright'

const BASE = process.env.BASE_URL || 'http://localhost:8080'

async function login(page) {
  await page.goto(`${BASE}/login`)
  await page.fill('input[name="username"]', 'admin')
  await page.fill('input[name="password"]', 'localdev')
  await page.click('button[type="submit"]')
  await page.waitForURL('**/author**', { timeout: 10000 })
}

async function openFirstEditPage(page) {
  await page.goto(`${BASE}/author`)
  const editLink = page.locator('a[href*="/author/edit/"]').first()
  await editLink.waitFor({ timeout: 10000 })
  const href = await editLink.getAttribute('href')
  if (!href) throw new Error('No edit link found')
  await page.goto(`${BASE}${href}`)
  await page.waitForSelector('h1:text("Edit post")')
  return href
}

async function discardAndExpectAuthor(page) {
  await page.getByRole('button', { name: 'Author' }).click()
  const dialog = page.getByRole('dialog')
  await dialog.waitFor()
  await page.getByRole('button', { name: 'Discard' }).click()
  await page.waitForURL('**/author', { timeout: 5000 })
  if (await dialog.isVisible().catch(() => false)) {
    throw new Error('Discard modal still visible after confirm')
  }
}

;(async () => {
  const browser = await chromium.launch({ headless: true })
  const page = await browser.newPage()

  try {
    await login(page)
    const editHref = await openFirstEditPage(page)

    const titleInput = page.locator('.article-title-input')
    if ((await titleInput.count()) > 0) {
      const val = await titleInput.inputValue()
      await titleInput.fill(`${val} modified`)
    } else {
      await page.locator('textarea').first().fill('modified note content for discard test')
    }

    await discardAndExpectAuthor(page)
    console.log('PASS: edited title then discarded')

    await openFirstEditPage(page)
    const slug = editHref.split('/').pop()
    const postKind = await page.locator('select').inputValue()
    const draftContent =
      postKind === 'article'
        ? 'Draft title\n\nDraft body from localStorage'
        : 'Draft note body from localStorage'
    await page.evaluate(
      ({ draftSlug, kind, content }) => {
        localStorage.setItem(
          `nitpub:draft:${draftSlug}`,
          JSON.stringify({
            kind,
            content,
            savedAt: new Date().toISOString(),
          }),
        )
      },
      { draftSlug: slug, kind: postKind, content: draftContent },
    )
    await page.reload()
    await page.waitForSelector('text=Restored unsaved draft')
    await discardAndExpectAuthor(page)
    console.log('PASS: restored draft then discarded')
  } finally {
    await browser.close()
  }
})().catch((err) => {
  console.error('FAIL:', err.message)
  process.exit(1)
})
