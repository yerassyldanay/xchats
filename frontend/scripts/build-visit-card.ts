import { copyFileSync, existsSync, mkdirSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { renderVisitCard, SHOWCASE_TABS } from './lib/visitCard.ts'

// Regenerates frontend/visit-card/ — the standalone page Netlify publishes
// (see the root netlify.toml). Run it after changing the landing page's
// stylesheet or copy, or after refreshing the screenshots:
//
//   npm run build:visit-card
//
// It writes index.html from scripts/visit-card.template.html (see
// scripts/lib/visitCard.ts for what is generated and why), and mirrors the
// logo and the nine showcase screenshots out of public/ so the folder stays
// self-contained: Netlify publishes it as-is, with no build step.

const frontendRoot = join(dirname(fileURLToPath(import.meta.url)), '..')
const out = join(frontendRoot, 'visit-card')

mkdirSync(join(out, 'screenshots'), { recursive: true })
writeFileSync(join(out, 'index.html'), renderVisitCard())
copyFileSync(join(frontendRoot, 'public', 'logo.png'), join(out, 'logo.png'))
for (const [, , img] of SHOWCASE_TABS) {
  const src = join(frontendRoot, 'public', 'screenshots', `${img}.png`)
  if (existsSync(src)) copyFileSync(src, join(out, 'screenshots', `${img}.png`))
  else console.warn(`visit-card: ${src} is missing — skipped`)
}
console.log('visit-card/ regenerated')
