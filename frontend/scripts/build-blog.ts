import { copyFileSync, existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { basename, dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { imageSize } from 'image-size'
import MarkdownIt from 'markdown-it'
import { build as viteBuild } from 'vite'
import {
  type Article,
  type ArticleLocale,
  CATEGORIES,
  type CategoryKey,
  DEFAULT_LOCALE,
  estimateReadingMinutes,
  type Locale,
  LOCALES,
  readAllArticles,
} from './lib/content.ts'

// Runs after the main `vite build` (see package.json's "build" script) — builds
// the blog's own standalone stylesheet and its own SSR bundle of
// src/blog/entry-server.ts, then renders every article to plain static HTML.
// No hydration bundle is produced or referenced anywhere: the blog ships zero
// client JS by design — crawlers and link-preview bots must see real content
// without executing anything.

const frontendRoot = join(dirname(fileURLToPath(import.meta.url)), '..')
const distDir = join(frontendRoot, 'dist')
const ssrOutDir = join(frontendRoot, 'dist-ssr')

const SITE_TITLE = 'XChats'
const GITHUB_URL = 'https://github.com/yerassyldanay/xchats'
// .example is IANA-reserved for documentation/placeholder use — never resolves
// to a real host, so local builds can't accidentally emit a real-looking URL.
const FALLBACK_ORIGIN = 'https://xchats.example'
const SITE_ORIGIN = (process.env.SITE_ORIGIN ?? '').replace(/\/$/, '')

if (!SITE_ORIGIN && process.env.CI) {
  throw new Error(
    'SITE_ORIGIN must be set in CI — canonical/OG/sitemap URLs need a real absolute origin, ' +
      'not the local placeholder',
  )
}
if (!SITE_ORIGIN) {
  console.warn(`SITE_ORIGIN not set — using placeholder "${FALLBACK_ORIGIN}" for this build`)
}
const ORIGIN = SITE_ORIGIN || FALLBACK_ORIGIN

const LOCALE_LABEL: Record<Locale, string> = { ru: 'Русский', en: 'English', kk: 'Қазақша' }
const LOCALE_HTML_LANG: Record<Locale, string> = { ru: 'ru', en: 'en', kk: 'kk' }
const OG_LOCALE: Record<Locale, string> = { ru: 'ru_RU', en: 'en_US', kk: 'kk_KZ' }
const INTL_LOCALE: Record<Locale, string> = { ru: 'ru-RU', en: 'en-US', kk: 'kk-KZ' }
const GITHUB_LABEL: Record<Locale, string> = { ru: 'GitHub', en: 'GitHub', kk: 'GitHub' }
const INDEX_TITLE: Record<Locale, string> = { ru: 'Блог', en: 'Blog', kk: 'Блог' }
const INDEX_DESCRIPTION: Record<Locale, string> = {
  ru: 'Исследования, тесты, архитектурные решения и практические выводы нашей команды.',
  en: 'Research, tests, architecture decisions, and practical lessons from our team.',
  kk: 'Біздің команданың зерттеулері, тесттері және практикалық қорытындылары.',
}
const HERO_EYEBROW: Record<Locale, string> = { ru: 'Инженерный блог', en: 'Engineering Blog', kk: 'Инженерлік блог' }
const HERO_TITLE: Record<Locale, string> = {
  ru: 'Делимся опытом и решениями',
  en: 'Sharing research, experiments and decisions',
  kk: 'Тәжірибе мен шешімдермен бөлісеміз',
}
const HERO_DESCRIPTION: Record<Locale, string> = {
  ru: 'Всё, что мы узнаём, создавая AI-ассистентов, пайплайны оценки и внутренние инструменты.',
  en: 'Everything we learn while building AI assistants, evaluation pipelines, and internal tooling.',
  kk: 'AI-ассистенттерді, бағалау құбырларын және ішкі құралдарды жасау кезінде білгеніміздің бәрі.',
}
const SECTION_TITLE: Record<Locale, string> = { ru: 'Все статьи', en: 'All articles', kk: 'Барлық мақалалар' }
const END_OF_LIST_TITLE: Record<Locale, string> = {
  ru: 'Пока это все статьи',
  en: "That's all for now",
  kk: 'Қазірше осы ғана',
}
const END_OF_LIST_DESCRIPTION: Record<Locale, string> = {
  ru: 'Мы регулярно публикуем новые материалы. Следите за обновлениями!',
  en: "We're publishing engineering articles as we discover interesting problems.",
  kk: 'Біз қызықты мәселелерді тапқан сайын жаңа мақалалар жариялап отырамыз.',
}
const COPYRIGHT_SUFFIX: Record<Locale, string> = {
  ru: 'Все права защищены.',
  en: 'All rights reserved.',
  kk: 'Барлық құқықтар қорғалған.',
}
const NOT_FOUND_TITLE: Record<Locale, string> = {
  ru: 'Страница не найдена',
  en: 'Page not found',
  kk: 'Бет табылмады',
}
const NOT_FOUND_DESCRIPTION: Record<Locale, string> = {
  ru: 'Такой статьи не существует или она была перемещена.',
  en: "This article doesn't exist or it may have moved.",
  kk: 'Мұндай мақала жоқ немесе ол жылжытылған.',
}
const NOT_FOUND_BACK_LABEL: Record<Locale, string> = {
  ru: 'Вернуться в блог',
  en: 'Back to the blog',
  kk: 'Блогқа оралу',
}
const FONT_LINKS = `<link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&display=swap" rel="stylesheet" />`

function categoryLabel(key: CategoryKey, locale: Locale): string {
  return CATEGORIES[key][locale]
}

function readingTimeDisplay(locale: Locale, minutes: number): string {
  if (locale === 'ru') return `${minutes} ${pluralRu(minutes, 'минута', 'минуты', 'минут')} чтения`
  if (locale === 'kk') return `${minutes} мин оқу`
  return `${minutes} min read`
}

/** Russian plural selection: 1 статья, 2–4 статьи, 5+ (and the 11–14 exception) статей. */
function pluralRu(n: number, one: string, few: string, many: string): string {
  const mod10 = n % 10
  const mod100 = n % 100
  if (mod10 === 1 && mod100 !== 11) return one
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return few
  return many
}

function articleCountUnit(locale: Locale, n: number): string {
  if (locale === 'ru') return pluralRu(n, 'статья', 'статьи', 'статей')
  if (locale === 'kk') return 'мақала'
  return n === 1 ? 'article' : 'articles'
}

function updatedLabel(locale: Locale): string {
  if (locale === 'ru') return 'обновлено'
  if (locale === 'kk') return 'жаңартылды'
  return 'updated'
}

function formatDateShort(locale: Locale, iso: string): string {
  return new Intl.DateTimeFormat(INTL_LOCALE[locale], { dateStyle: 'medium' }).format(new Date(iso))
}

function localePrefix(locale: Locale): string {
  return locale === DEFAULT_LOCALE ? '' : `/${locale}`
}
function articlePath(locale: Locale, id: string): string {
  return `${localePrefix(locale)}/blog/${id}/`
}
function indexPath(locale: Locale): string {
  return `${localePrefix(locale)}/blog/`
}
function notFoundPath(locale: Locale): string {
  return `${localePrefix(locale)}/blog/404.html`
}
function abs(path: string): string {
  return `${ORIGIN}${path}`
}

interface ImageDims {
  width: number
  height: number
}

function measureImage(filePath: string): ImageDims {
  const dims = imageSize(readFileSync(filePath))
  if (!dims.width || !dims.height) {
    throw new Error(`could not read dimensions of ${filePath}`)
  }
  return { width: dims.width, height: dims.height }
}

const MIN_COVER_WIDTH = 1200
const MIN_COVER_HEIGHT = 630

function resolveCover(article: Article, loc: ArticleLocale): ImageDims & { filename: string } {
  const filename = loc.frontmatter.cover
  if (!filename) {
    throw new Error(`content/blog/${article.dir}/${loc.locale}.md has no "cover" in its frontmatter`)
  }
  const filePath = join(article.dirPath, filename)
  if (!existsSync(filePath)) {
    throw new Error(
      `content/blog/${article.dir}/${loc.locale}.md declares cover "${filename}" but that file does not exist`,
    )
  }
  const dims = measureImage(filePath)
  if (dims.width < MIN_COVER_WIDTH || dims.height < MIN_COVER_HEIGHT) {
    throw new Error(
      `content/blog/${article.dir}/${filename} is ${dims.width}x${dims.height}, below the required ` +
        `${MIN_COVER_WIDTH}x${MIN_COVER_HEIGHT} minimum for a usable social preview image`,
    )
  }
  return { filename, ...dims }
}

// Markdown renderer scoped to one article: rewrites inline image src to the
// public /blog/img/<id>/ path, reads real width/height off disk (prevents
// layout shift), and marks them loading=lazy. Tracks which files it touched so
// the caller copies exactly those into dist/, alongside the cover.
function renderMarkdown(article: Article, body: string, usedImages: Set<string>): string {
  const md = new MarkdownIt({ html: false, linkify: true, typographer: false })
  md.renderer.rules.image = (tokens, idx, options, _env, self) => {
    const token = tokens[idx]
    const src = token.attrGet('src') ?? ''
    if (!/^https?:\/\//.test(src)) {
      const filename = basename(src)
      const filePath = join(article.dirPath, filename)
      if (!existsSync(filePath)) {
        throw new Error(
          `content/blog/${article.dir}/: markdown references image "${src}" which does not exist in that folder`,
        )
      }
      const dims = measureImage(filePath)
      token.attrSet('src', `/blog/img/${article.id}/${filename}`)
      token.attrSet('width', String(dims.width))
      token.attrSet('height', String(dims.height))
      token.attrSet('loading', 'lazy')
      usedImages.add(filename)
    }
    return self.renderToken(tokens, idx, options)
  }
  return md.render(body)
}

function copyArticleImages(article: Article, filenames: Set<string>): void {
  const outDir = join(distDir, 'blog', 'img', article.id)
  mkdirSync(outDir, { recursive: true })
  for (const filename of filenames) {
    copyFileSync(join(article.dirPath, filename), join(outDir, filename))
  }
}

// The hero illustration is a hand-picked brand asset, not user content — it's
// pre-optimized once and committed (src/blog/assets/hero.{webp,png}), not
// processed at build time. Regenerate with (from frontend/):
//   convert src/blog/assets/hero-source.png -resize 900x -strip /tmp/h.png
//   convert /tmp/h.png -quality 82 src/blog/assets/hero.webp
//   convert /tmp/h.png -strip -define png:compression-level=9 src/blog/assets/hero.png
function copyHeroImage(): { webp: string; png: string; width: number; height: number } {
  const assetsDir = join(frontendRoot, 'src', 'blog', 'assets')
  const outDir = join(distDir, 'blog', 'img')
  mkdirSync(outDir, { recursive: true })
  copyFileSync(join(assetsDir, 'hero.webp'), join(outDir, 'hero.webp'))
  copyFileSync(join(assetsDir, 'hero.png'), join(outDir, 'hero.png'))
  const dims = measureImage(join(assetsDir, 'hero.png'))
  return { webp: '/blog/img/hero.webp', png: '/blog/img/hero.png', ...dims }
}

function formatDate(locale: Locale, iso: string): string {
  return new Intl.DateTimeFormat(INTL_LOCALE[locale], { dateStyle: 'long' }).format(new Date(iso))
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

interface HeadInput {
  title: string
  description: string
  path: string // site-relative, e.g. /blog/<id>/
  locale: Locale
  /** Every locale this exact page exists in, including itself. Omit for a non-indexable page. */
  alternates?: Array<{ locale: Locale; path: string }>
  type: 'article' | 'website' | 'error'
  image?: { url: string; width: number; height: number }
  publishedIso?: string
  updatedIso?: string
}

function renderHead(input: HeadInput, cssHref: string): string {
  const url = abs(input.path)
  const tags: string[] = []
  tags.push('<meta charset="UTF-8" />')
  tags.push('<meta name="viewport" content="width=device-width, initial-scale=1.0" />')
  // The site-name suffix belongs in <title> only — og:title/JSON-LD headline
  // use the bare title, since og:site_name already carries the site name.
  tags.push(`<title>${escapeHtml(input.title)} — ${SITE_TITLE}</title>`)

  if (input.type === 'error') {
    // A 404 page is intentionally not indexed and carries no canonical/OG/JSON-LD
    // — there is nothing here to rank, and the real HTTP status is still 404
    // (nginx's error_page preserves it; see nginx.conf).
    tags.push('<meta name="robots" content="noindex" />')
    tags.push(`<link rel="stylesheet" href="${cssHref}" />`)
    tags.push(FONT_LINKS)
    return tags.join('\n    ')
  }

  const alternates = input.alternates ?? []
  tags.push(`<meta name="description" content="${escapeHtml(input.description)}" />`)
  tags.push(`<link rel="canonical" href="${url}" />`)
  for (const alt of alternates) {
    tags.push(`<link rel="alternate" hreflang="${LOCALE_HTML_LANG[alt.locale]}" href="${abs(alt.path)}" />`)
  }
  const defaultAlt = alternates.find((a) => a.locale === DEFAULT_LOCALE)
  if (defaultAlt) {
    tags.push(`<link rel="alternate" hreflang="x-default" href="${abs(defaultAlt.path)}" />`)
  }
  tags.push(`<meta property="og:type" content="${input.type}" />`)
  tags.push(`<meta property="og:title" content="${escapeHtml(input.title)}" />`)
  tags.push(`<meta property="og:description" content="${escapeHtml(input.description)}" />`)
  tags.push(`<meta property="og:url" content="${url}" />`)
  tags.push(`<meta property="og:site_name" content="${SITE_TITLE}" />`)
  tags.push(`<meta property="og:locale" content="${OG_LOCALE[input.locale]}" />`)
  for (const alt of alternates) {
    if (alt.locale === input.locale) continue
    tags.push(`<meta property="og:locale:alternate" content="${OG_LOCALE[alt.locale]}" />`)
  }
  if (input.image) {
    tags.push(`<meta property="og:image" content="${input.image.url}" />`)
    tags.push(`<meta property="og:image:width" content="${input.image.width}" />`)
    tags.push(`<meta property="og:image:height" content="${input.image.height}" />`)
    tags.push('<meta name="twitter:card" content="summary_large_image" />')
  } else {
    tags.push('<meta name="twitter:card" content="summary" />')
  }
  if (input.type === 'article') {
    if (input.publishedIso) tags.push(`<meta property="article:published_time" content="${input.publishedIso}" />`)
    if (input.updatedIso) tags.push(`<meta property="article:modified_time" content="${input.updatedIso}" />`)
    const jsonLd = {
      '@context': 'https://schema.org',
      '@type': 'BlogPosting',
      headline: input.title,
      description: input.description,
      inLanguage: LOCALE_HTML_LANG[input.locale],
      datePublished: input.publishedIso,
      dateModified: input.updatedIso ?? input.publishedIso,
      url,
      ...(input.image ? { image: input.image.url } : {}),
    }
    tags.push(`<script type="application/ld+json">${JSON.stringify(jsonLd)}</script>`)
  }
  tags.push(`<link rel="stylesheet" href="${cssHref}" />`)
  tags.push(FONT_LINKS)
  return tags.join('\n    ')
}

function renderPage(headHtml: string, bodyHtml: string, lang: string): string {
  return `<!doctype html>
<html lang="${lang}">
  <head>
    ${headHtml}
  </head>
  <body>${bodyHtml}</body>
</html>
`
}

/** Builds the blog's standalone CSS (src/blog/blog.css) and returns its hashed href. */
async function buildBlogCss(): Promise<string> {
  await viteBuild({
    configFile: false,
    root: frontendRoot,
    build: {
      outDir: 'dist',
      emptyOutDir: false, // dist/ already holds the SPA build — do not wipe it
      manifest: true,
      cssMinify: true,
      rollupOptions: { input: join(frontendRoot, 'src/blog/blog.css') },
    },
    logLevel: 'warn',
  })
  const manifestPath = join(distDir, '.vite', 'manifest.json')
  const manifest = JSON.parse(readFileSync(manifestPath, 'utf-8')) as Record<string, { file: string }>
  const entry = manifest['src/blog/blog.css']
  if (!entry?.file) {
    throw new Error(`${manifestPath} has no entry for src/blog/blog.css — check the blog CSS build`)
  }
  return `/${entry.file}`
}

interface SsrBundle {
  renderArticle: (props: Record<string, unknown>) => Promise<string>
  renderIndex: (props: Record<string, unknown>) => Promise<string>
  renderNotFound: (props: Record<string, unknown>) => Promise<string>
}

/** A dedicated Vite SSR build of entry-server.ts — separate from the client build, no hydration output. */
async function buildSsrBundle(): Promise<SsrBundle> {
  await viteBuild({
    configFile: false,
    root: frontendRoot,
    plugins: [vue()],
    resolve: { alias: { '@': join(frontendRoot, 'src') } },
    build: {
      ssr: join(frontendRoot, 'src/blog/entry-server.ts'),
      outDir: 'dist-ssr',
      emptyOutDir: true,
      rollupOptions: { output: { entryFileNames: 'entry-server.mjs', format: 'es' } },
      minify: false,
    },
    logLevel: 'warn',
  })
  return import(pathToFileURL(join(ssrOutDir, 'entry-server.mjs')).href) as Promise<SsrBundle>
}

interface SitemapEntry {
  path: string
  lastmod: string
  alternates: Array<{ locale: Locale; path: string }>
}

function renderSitemap(entries: SitemapEntry[]): string {
  const urls = entries
    .map((e) => {
      const altTags = e.alternates
        .map((a) => `<xhtml:link rel="alternate" hreflang="${LOCALE_HTML_LANG[a.locale]}" href="${abs(a.path)}" />`)
        .join('')
      const defaultAlt = e.alternates.find((a) => a.locale === DEFAULT_LOCALE)
      const xdefault = defaultAlt
        ? `<xhtml:link rel="alternate" hreflang="x-default" href="${abs(defaultAlt.path)}" />`
        : ''
      return `  <url>\n    <loc>${abs(e.path)}</loc>\n    <lastmod>${e.lastmod}</lastmod>\n    ${altTags}${xdefault}\n  </url>`
    })
    .join('\n')
  return `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">\n${urls}\n</urlset>\n`
}

function renderRobots(): string {
  return [
    'User-agent: *',
    'Allow: /blog/',
    'Allow: /en/blog/',
    'Allow: /kk/blog/',
    'Disallow: /',
    '',
    `Sitemap: ${abs('/sitemap.xml')}`,
    '',
  ].join('\n')
}

interface IndexArticleEntry {
  href: string
  title: string
  description: string
  dateIso: string
  dateDisplay: string
  readingTimeDisplay: string
  categoryLabel: string
  categoryKey: CategoryKey
}

async function main() {
  mkdirSync(distDir, { recursive: true })
  const cssHref = await buildBlogCss()
  const { renderArticle, renderIndex, renderNotFound } = await buildSsrBundle()
  const buildYear = new Date().getUTCFullYear()
  const heroImage = copyHeroImage()

  const articles = readAllArticles(frontendRoot)
  if (articles.length === 0) {
    console.warn('blog: no articles in content/blog/ — writing an empty sitemap/robots.txt only')
    writeFileSync(join(distDir, 'robots.txt'), renderRobots())
    writeFileSync(join(distDir, 'sitemap.xml'), renderSitemap([]))
    return
  }

  const sitemapEntries: SitemapEntry[] = []
  const indexArticlesByLocale: Record<Locale, IndexArticleEntry[]> = { ru: [], en: [], kk: [] }

  for (const article of articles) {
    const presentLocales = LOCALES.filter((l) => article.locales[l])
    const alternates = presentLocales.map((l) => ({ locale: l, path: articlePath(l, article.id) }))

    for (const locale of presentLocales) {
      const loc = article.locales[locale] as ArticleLocale
      const usedImages = new Set<string>()
      const bodyHtml = renderMarkdown(article, loc.body, usedImages)
      const cover = resolveCover(article, loc)
      usedImages.add(cover.filename)

      const publishedIso = new Date(loc.frontmatter.date).toISOString()
      const updatedIso = loc.frontmatter.updated ? new Date(loc.frontmatter.updated).toISOString() : undefined
      const coverUrl = `/blog/img/${article.id}/${cover.filename}`
      const minutes = estimateReadingMinutes(loc.body)
      const category = loc.frontmatter.category

      const headHtml = renderHead(
        {
          title: loc.frontmatter.title,
          description: loc.frontmatter.description,
          path: articlePath(locale, article.id),
          locale,
          alternates,
          type: 'article',
          image: { url: abs(coverUrl), width: cover.width, height: cover.height },
          publishedIso,
          updatedIso,
        },
        cssHref,
      )

      const appHtml = await renderArticle({
        siteTitle: SITE_TITLE,
        indexHref: indexPath(locale),
        githubUrl: GITHUB_URL,
        githubLabel: GITHUB_LABEL[locale],
        langLinks: alternates.map((a) => ({
          locale: a.locale,
          href: articlePath(a.locale, article.id),
          label: LOCALE_LABEL[a.locale],
        })),
        currentLocale: locale,
        year: buildYear,
        copyrightSuffix: COPYRIGHT_SUFFIX[locale],
        title: loc.frontmatter.title,
        description: loc.frontmatter.description,
        dateDisplay: formatDate(locale, loc.frontmatter.date),
        updatedDisplay: loc.frontmatter.updated
          ? `${updatedLabel(locale)} ${formatDate(locale, loc.frontmatter.updated)}`
          : undefined,
        readingTimeDisplay: readingTimeDisplay(locale, minutes),
        categoryLabel: categoryLabel(category, locale),
        categoryKey: category,
        tags: loc.frontmatter.tags,
        // cover.png is never shown in the body — the category icon is (same
        // as the index card, for visual consistency). cover/coverUrl above
        // are still used for og:image in renderHead(), just above this call.
        bodyHtml,
      })

      const outDir = join(distDir, articlePath(locale, article.id).replace(/^\//, ''))
      mkdirSync(outDir, { recursive: true })
      writeFileSync(join(outDir, 'index.html'), renderPage(headHtml, appHtml, LOCALE_HTML_LANG[locale]))
      copyArticleImages(article, usedImages)

      indexArticlesByLocale[locale].push({
        href: articlePath(locale, article.id),
        title: loc.frontmatter.title,
        description: loc.frontmatter.description,
        dateIso: publishedIso,
        dateDisplay: formatDate(locale, loc.frontmatter.date),
        readingTimeDisplay: readingTimeDisplay(locale, minutes),
        categoryLabel: categoryLabel(category, locale),
        categoryKey: category,
      })

      sitemapEntries.push({
        path: articlePath(locale, article.id),
        lastmod: (updatedIso ?? publishedIso).slice(0, 10),
        alternates,
      })
    }
  }

  // Index pages only for locales with at least one article — no empty kk index
  // while kk has zero reviewed articles.
  const localesWithContent = LOCALES.filter((l) => indexArticlesByLocale[l].length > 0)
  const indexAlternates = localesWithContent.map((l) => ({ locale: l, path: indexPath(l) }))

  for (const locale of localesWithContent) {
    const list = [...indexArticlesByLocale[locale]].sort((a, b) => (a.dateIso < b.dateIso ? 1 : -1))
    const mostRecentIso = list[0]?.dateIso ?? new Date().toISOString()
    const langLinks = indexAlternates.map((a) => ({
      locale: a.locale,
      href: indexPath(a.locale),
      label: LOCALE_LABEL[a.locale],
    }))

    const headHtml = renderHead(
      {
        title: INDEX_TITLE[locale],
        description: INDEX_DESCRIPTION[locale],
        path: indexPath(locale),
        locale,
        alternates: indexAlternates,
        type: 'website',
      },
      cssHref,
    )
    const appHtml = await renderIndex({
      siteTitle: SITE_TITLE,
      indexHref: indexPath(locale),
      githubUrl: GITHUB_URL,
      githubLabel: GITHUB_LABEL[locale],
      langLinks,
      currentLocale: locale,
      year: buildYear,
      copyrightSuffix: COPYRIGHT_SUFFIX[locale],
      heroEyebrow: HERO_EYEBROW[locale],
      heroTitle: HERO_TITLE[locale],
      heroDescription: HERO_DESCRIPTION[locale],
      heroStats: [
        { value: String(list.length), label: articleCountUnit(locale, list.length) },
        { value: formatDateShort(locale, mostRecentIso), label: updatedLabel(locale) },
      ],
      heroImage: {
        webp: heroImage.webp,
        png: heroImage.png,
        width: heroImage.width,
        height: heroImage.height,
        alt: SITE_TITLE,
      },
      sectionTitle: SECTION_TITLE[locale],
      endOfListTitle: END_OF_LIST_TITLE[locale],
      endOfListDescription: END_OF_LIST_DESCRIPTION[locale],
      articles: list.map(({ dateIso: _dateIso, ...rest }) => rest),
    })
    const outDir = join(distDir, indexPath(locale).replace(/^\//, ''))
    mkdirSync(outDir, { recursive: true })
    writeFileSync(join(outDir, 'index.html'), renderPage(headHtml, appHtml, LOCALE_HTML_LANG[locale]))
    sitemapEntries.push({
      path: indexPath(locale),
      lastmod: new Date().toISOString().slice(0, 10),
      alternates: indexAlternates,
    })

    // Styled 404 — nginx's error_page points here per locale (see nginx.conf).
    // noindex, no hreflang/OG: this page is never meant to be found, only seen.
    const notFoundHead = renderHead(
      { title: NOT_FOUND_TITLE[locale], description: '', path: notFoundPath(locale), locale, type: 'error' },
      cssHref,
    )
    const notFoundHtml = await renderNotFound({
      siteTitle: SITE_TITLE,
      indexHref: indexPath(locale),
      githubUrl: GITHUB_URL,
      githubLabel: GITHUB_LABEL[locale],
      langLinks,
      currentLocale: locale,
      year: buildYear,
      copyrightSuffix: COPYRIGHT_SUFFIX[locale],
      title: NOT_FOUND_TITLE[locale],
      description: NOT_FOUND_DESCRIPTION[locale],
      backLinkLabel: NOT_FOUND_BACK_LABEL[locale],
    })
    writeFileSync(join(outDir, '404.html'), renderPage(notFoundHead, notFoundHtml, LOCALE_HTML_LANG[locale]))
  }

  writeFileSync(join(distDir, 'sitemap.xml'), renderSitemap(sitemapEntries))
  writeFileSync(join(distDir, 'robots.txt'), renderRobots())

  console.log(`blog: wrote ${articles.length} article(s) across ${localesWithContent.length} locale(s)`)
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
