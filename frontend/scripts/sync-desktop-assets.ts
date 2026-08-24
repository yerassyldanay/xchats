// Mirror the built SPA into the desktop binary's go:embed directory.
//
// Why a copy instead of a second Vite output target: Go's go:embed cannot
// reach outside the package directory it is declared in, so the bundle has to
// physically sit under backend/internal/desktop/. Repointing Vite's outDir
// there would have changed frontend/Dockerfile and the web deployment, which
// desktop packaging is meant to leave untouched — so the web build stays
// exactly as it was and this script mirrors its output afterwards.
//
// Run by `npm run build:desktop`, which backend/cmd/xchats/wails.json wires
// to Wails' frontend:build step — so `wails dev` and `wails build` both get a
// fresh mirror without anyone remembering to run it.
import { access, cp, readdir, rm } from 'node:fs/promises'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const source = resolve(here, '..', 'dist')
const target = resolve(here, '..', '..', 'backend', 'internal', 'desktop', 'dist')

// .gitkeep is the one committed file in the target directory: it documents
// what lands there and keeps the go:embed path valid on a fresh clone, so a
// clean-out must step around it.
const keep = new Set(['.gitkeep'])

async function main(): Promise<void> {
  try {
    await access(join(source, 'index.html'))
  } catch {
    throw new Error(
      `no built frontend at ${source} — run \`npm run build\` in frontend/ before syncing desktop assets`,
    )
  }

  // Remove stale output first. Vite's filenames are content-hashed, so a
  // plain overwrite would accumulate every past build's chunks inside the
  // binary, growing it on every release with files nothing references.
  for (const entry of await readdir(target).catch(() => [])) {
    if (keep.has(entry)) continue
    await rm(join(target, entry), { recursive: true, force: true })
  }

  await cp(source, target, { recursive: true })
  console.log(`desktop assets: ${source} -> ${target}`)
}

main().catch((err: unknown) => {
  console.error(`sync-desktop-assets: ${err instanceof Error ? err.message : String(err)}`)
  process.exitCode = 1
})
