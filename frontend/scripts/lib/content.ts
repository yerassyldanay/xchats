import { type Dirent, readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'
import matter from 'gray-matter'
import { parse as parseYaml } from 'yaml'

// The blog's locale set. Russian is the default and unprefixed in URLs; see
// buildUrl() in build-blog.ts. Order here also drives hreflang emission order.
export const LOCALES = ['ru', 'en', 'kk'] as const
export type Locale = (typeof LOCALES)[number]
export const DEFAULT_LOCALE: Locale = 'ru'

// Categories are metadata only — displayed on cards and article pages, never
// a filter or a route (deliberate: see the redesign plan's "no filter UI").
// The key is what authors write in frontmatter; labels are what readers see.
export const CATEGORIES = {
  models: { ru: 'Модели', en: 'Models', kk: 'Модельдер' },
  evals: { ru: 'Эвалы', en: 'Evaluations', kk: 'Бағалаулар' },
  product: { ru: 'Продукт', en: 'Product', kk: 'Өнім' },
  engineering: { ru: 'Инженерия', en: 'Engineering', kk: 'Инженерия' },
  lessons: { ru: 'Опыт', en: 'Lessons', kk: 'Тәжірибе' },
} as const satisfies Record<string, Record<Locale, string>>
export type CategoryKey = keyof typeof CATEGORIES

export interface ArticleFrontmatter {
  title: string
  description: string
  date: string
  updated?: string
  cover?: string
  category: CategoryKey
  tags?: string[]
}

export interface ArticleLocale {
  locale: Locale
  frontmatter: ArticleFrontmatter
  body: string // raw markdown, not yet rendered
}

export interface Article {
  /** Directory name under content/blog/ — human-readable, repo-only, never the URL. */
  dir: string
  dirPath: string
  /** Opaque id from meta.yaml — the only thing that becomes part of the URL. */
  id: string
  locales: Partial<Record<Locale, ArticleLocale>>
}

/**
 * meta.yaml carries only the id. It is deliberately the single source of truth,
 * shared by every locale file of one article, so three per-locale frontmatter
 * blocks can never drift out of sync on the one field that must never change
 * after publish.
 */
interface MetaYaml {
  id: string
}

export function contentRoot(frontendRoot: string): string {
  return join(frontendRoot, 'content', 'blog')
}

function readMeta(dirPath: string): MetaYaml {
  const raw = readFileSync(join(dirPath, 'meta.yaml'), 'utf-8')
  const parsed = parseYaml(raw) as MetaYaml | undefined
  if (!parsed?.id || typeof parsed.id !== 'string') {
    throw new Error(`${dirPath}/meta.yaml is missing a string "id" field`)
  }
  return parsed
}

function readLocale(dirPath: string, locale: Locale): ArticleLocale | undefined {
  const path = join(dirPath, `${locale}.md`)
  try {
    statSync(path)
  } catch {
    return undefined
  }
  const raw = readFileSync(path, 'utf-8')
  const { data, content } = matter(raw)
  const fm = data as Partial<ArticleFrontmatter>
  for (const field of ['title', 'description', 'date', 'category'] as const) {
    if (!fm[field]) {
      throw new Error(`${path} is missing required frontmatter field "${field}"`)
    }
  }
  if (!(fm.category! in CATEGORIES)) {
    const known = Object.keys(CATEGORIES).join(', ')
    throw new Error(`${path} has unknown category "${fm.category}" — must be one of: ${known}`)
  }
  return {
    locale,
    frontmatter: fm as ArticleFrontmatter,
    body: content,
  }
}

const WORDS_PER_MINUTE = 200

/** Words ÷ 200, rounded up, minimum 1 — a stated estimate, not a precise measurement. */
export function estimateReadingMinutes(markdownBody: string): number {
  const wordCount = markdownBody
    .replace(/```[\s\S]*?```/g, ' ') // fenced code blocks aren't prose
    .split(/\s+/)
    .filter(Boolean).length
  return Math.max(1, Math.ceil(wordCount / WORDS_PER_MINUTE))
}

/** Reads every content/blog/<dir>/ article: meta.yaml + whichever locale .md files exist. */
export function readAllArticles(frontendRoot: string): Article[] {
  const root = contentRoot(frontendRoot)
  let entries: Dirent[]
  try {
    entries = readdirSync(root, { withFileTypes: true })
  } catch {
    return [] // no content/blog/ yet — not an error, just nothing to build
  }
  const dirs = entries.filter((e) => e.isDirectory()).map((e) => e.name)

  const articles: Article[] = dirs.map((dir) => {
    const dirPath = join(root, dir)
    const meta = readMeta(dirPath)
    const locales: Partial<Record<Locale, ArticleLocale>> = {}
    for (const locale of LOCALES) {
      const l = readLocale(dirPath, locale)
      if (l) locales[locale] = l
    }
    if (!locales[DEFAULT_LOCALE]) {
      throw new Error(`${dirPath} has no ${DEFAULT_LOCALE}.md — the default locale is required`)
    }
    return { dir, dirPath, id: meta.id, locales }
  })

  assertUniqueIds(articles)
  return articles
}

function assertUniqueIds(articles: Article[]): void {
  const seen = new Map<string, string>() // id -> dir
  for (const a of articles) {
    const collision = seen.get(a.id)
    if (collision) {
      throw new Error(
        `duplicate blog id "${a.id}": content/blog/${collision} and content/blog/${a.dir} both use it. ` +
          `Ids are permanent — fix the newer article's meta.yaml with a freshly minted id.`,
      )
    }
    seen.set(a.id, a.dir)
  }
}

/** All ids currently in use, for new-blog-post.ts's collision check when minting a fresh one. */
export function allExistingIds(frontendRoot: string): Set<string> {
  return new Set(readAllArticles(frontendRoot).map((a) => a.id))
}
