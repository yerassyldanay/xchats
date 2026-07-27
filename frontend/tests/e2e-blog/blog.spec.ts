import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'
import { DEFAULT_LOCALE, readAllArticles } from '../../scripts/lib/content'

// The whole point of a separate prerendered blog bundle is that crawlers and
// link-preview bots — which never execute JavaScript — still see real content.
// javaScriptEnabled: false is the literal automated form of that claim: if a
// test below only passes with JS on, the approach has failed.
test.use({ javaScriptEnabled: false })

const frontendRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
const articles = readAllArticles(frontendRoot)
if (articles.length === 0) {
  throw new Error('content/blog/ has no articles — nothing for blog.spec.ts to test against')
}
// Tests against whichever article happens first rather than a hardcoded
// slug/id, so this suite doesn't need editing when content changes.
const article = articles[0]
const ru = article.locales[DEFAULT_LOCALE]!
const en = article.locales.en

test('article renders its real heading with JS fully disabled', async ({ page }) => {
  await page.goto(`/blog/${article.id}/`)
  await expect(page.locator('h1')).toHaveText(ru.frontmatter.title)
})

test('article body text is present in the raw HTML, not injected by JS', async ({ page }) => {
  await page.goto(`/blog/${article.id}/`)
  await expect(page.locator('article')).toContainText(ru.frontmatter.description)
})

test('category and tags render as visible text — never as links — on the article page', async ({ page }) => {
  await page.goto(`/blog/${article.id}/`)
  await expect(page.locator('.badge--category')).toHaveText(/.+/)
  for (const tag of ru.frontmatter.tags ?? []) {
    await expect(page.locator('.badge--tag', { hasText: tag })).toBeVisible()
  }
  // Categories/tags are metadata, not navigation — see the redesign plan's
  // "no filter UI at all" decision. Nothing badge-shaped should be a link.
  await expect(page.locator('a.badge--category, a.badge--tag')).toHaveCount(0)
})

test('canonical and hreflang tags cover every locale this article actually has', async ({ page }) => {
  await page.goto(`/blog/${article.id}/`)
  await expect(page.locator('link[rel="canonical"]')).toHaveCount(1)
  const localeCount = Object.keys(article.locales).length
  // one hreflang per present locale, plus x-default.
  await expect(page.locator('link[rel="alternate"][hreflang]')).toHaveCount(localeCount + 1)
})

test('og:image is an absolute URL sized for a real social preview', async ({ page }) => {
  await page.goto(`/blog/${article.id}/`)
  const content = await page.locator('meta[property="og:image"]').getAttribute('content')
  expect(content).toMatch(/^https?:\/\//)
  const width = Number(await page.locator('meta[property="og:image:width"]').getAttribute('content'))
  const height = Number(await page.locator('meta[property="og:image:height"]').getAttribute('content'))
  expect(width).toBeGreaterThanOrEqual(1200)
  expect(height).toBeGreaterThanOrEqual(630)
})

test('English variant renders at /en/blog/<id>/ when en.md exists', async ({ page }) => {
  test.skip(!en, 'this article has no en.md yet')
  await page.goto(`/en/blog/${article.id}/`)
  await expect(page.locator('h1')).toHaveText(en!.frontmatter.title)
})

test('a non-existent article id gets a real 404, not the internal app shell', async ({ page }) => {
  const response = await page.goto('/blog/this-id-does-not-exist/')
  expect(response?.status()).toBe(404)
  // The *styled* 404 body (nginx's error_page substitution) only exists behind
  // real nginx, not vite preview's own bare 404 — see the redesign plan's
  // verification section for the real-nginx-container check that covers it.
})

test('the blog index lists every article under its real title, with a category label', async ({ page }) => {
  await page.goto('/blog/')
  for (const a of articles) {
    const ruLoc = a.locales[DEFAULT_LOCALE]!
    await expect(page.getByRole('link', { name: ruLoc.frontmatter.title })).toBeVisible()
  }
  await expect(page.locator('.article-card__category').first()).toHaveText(/.+/)
})

test('the hero renders a real article count and resolves its image via <picture>, WebP first', async ({ page }) => {
  await page.goto('/blog/')
  await expect(page.locator('.hero__stat-value').first()).toHaveText(String(articles.length))

  const source = page.locator('.hero__image-frame source[type="image/webp"]')
  await expect(source).toHaveAttribute('srcset', /\.webp$/)
  const img = page.locator('.hero__image-frame img')
  await expect(img).toHaveAttribute('width', /^\d+$/)
  await expect(img).toHaveAttribute('height', /^\d+$/)
})

test('the shipped stylesheet defines an automatic (prefers-color-scheme) dark mode', async ({ page, request }) => {
  await page.goto('/blog/')
  // Distinct from the Google Fonts <link>, which is also rel="stylesheet".
  const cssHref = await page.locator('link[rel="stylesheet"][href^="/assets/"]').getAttribute('href')
  const css = await (await request.get(cssHref!)).text()
  expect(css).toMatch(/prefers-color-scheme:\s*dark/)
})

test('blog pages carry the xchats favicon and full public-site chrome', async ({ page }) => {
  await page.goto('/blog/')

  await expect(page.locator('link[rel="icon"]')).toHaveAttribute('href', '/logo.png')
  await expect(page.locator('link[rel="apple-touch-icon"]')).toHaveAttribute('href', '/logo.png')
  await expect(page.locator('.site-nav__mark')).toHaveText('x')

  const footer = page.locator('.site-footer')
  await expect(footer.locator('.site-footer__brand')).toContainText('xchats')
  await expect(footer.locator('a[href="/#features"]')).toHaveCount(1)
  await expect(footer.locator('a[href="/blog/"][aria-current="page"]')).toHaveCount(1)
  await expect(footer.locator('a[href^="mailto:"]')).toBeVisible()
  await expect(footer.locator('a[href^="https://wa.me/"]')).toBeVisible()
})

test('public navigation, hero, and footer keep their mobile gutters', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/blog/')

  const heroBox = await page.locator('.hero__text').boundingBox()
  const footerBrandBox = await page.locator('.site-footer__brand').boundingBox()
  const emailBox = await page.locator('.site-footer__contact a[href^="mailto:"]').boundingBox()

  expect(heroBox?.x).toBeGreaterThanOrEqual(20)
  expect(footerBrandBox?.x).toBeGreaterThanOrEqual(20)
  expect((emailBox?.x ?? 0) + (emailBox?.width ?? 0)).toBeLessThanOrEqual(370)
})
