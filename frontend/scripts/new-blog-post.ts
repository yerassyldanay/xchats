import { existsSync, mkdirSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { customAlphabet } from 'nanoid'
import { allExistingIds } from './lib/content.ts'

// Ids are minted only here — never hand-typed into meta.yaml — so two authors
// working in parallel branches can't pick the same id. URL-safe alphabet,
// no ambiguous look-alike characters (0/O, 1/l/I) to keep ids easy to read aloud.
const nanoid = customAlphabet('23456789abcdefghjkmnpqrstuvwxyz', 8)

const frontendRoot = join(dirname(fileURLToPath(import.meta.url)), '..')

function mintId(existing: Set<string>): string {
  for (let i = 0; i < 10; i++) {
    const id = nanoid()
    if (!existing.has(id)) return id
  }
  throw new Error('could not mint a unique id after 10 attempts — check content/blog/ for corruption')
}

function main() {
  const dir = process.argv[2]
  if (!dir || !/^[a-z0-9-]+$/.test(dir)) {
    console.error('usage: npm run new-blog-post -- <human-readable-dir>')
    console.error('  <human-readable-dir> must be lowercase kebab-case, e.g. choosing-an-llm')
    process.exit(1)
  }

  const dirPath = join(frontendRoot, 'content', 'blog', dir)
  if (existsSync(dirPath)) {
    console.error(`content/blog/${dir} already exists`)
    process.exit(1)
  }

  const existing = allExistingIdsSafe()
  const id = mintId(existing)

  mkdirSync(dirPath, { recursive: true })
  writeFileSync(join(dirPath, 'meta.yaml'), `id: ${id}\n`)
  writeFileSync(
    join(dirPath, 'ru.md'),
    `---
title: ""
description: ""
date: "${new Date().toISOString().slice(0, 10)}"
cover: cover.png
tags: []
---

Напишите статью здесь.
`,
  )

  console.log(`created content/blog/${dir}/`)
  console.log(`  id: ${id}  ->  /blog/${id}/`)
  console.log('next steps:')
  console.log(`  1. fill in content/blog/${dir}/ru.md`)
  console.log(`  2. add content/blog/${dir}/cover.png (>= 1200x630)`)
  console.log(`  3. optionally add en.md / kk.md in the same folder`)
}

// readAllArticles() (used by allExistingIds) requires every existing dir to
// already have a valid meta.yaml + ru.md — true for anything previously
// scaffolded by this same script, but guard anyway so a first-ever run (empty
// content/blog/) or a mid-edit directory doesn't crash id minting.
function allExistingIdsSafe(): Set<string> {
  try {
    return allExistingIds(frontendRoot)
  } catch (err) {
    console.warn(`warning: could not fully scan existing articles (${(err as Error).message})`)
    console.warn('proceeding with an empty id set — double-check for collisions manually')
    return new Set()
  }
}

main()
