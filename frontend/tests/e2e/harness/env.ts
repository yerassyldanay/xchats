import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

// Repo root: frontend/tests/e2e/harness/env.ts -> up four levels.
export const REPO_ROOT = path.resolve(__dirname, '../../../../')

// Minimal .env parser — KEY=VALUE per line, '#' comments, optional quotes
// stripped. Deliberately not the `dotenv` package: this file is the ONLY
// place the suite ever reads `.env` (see tests/e2e/README.md's "How .env
// flows through the test" — it is never --env-file'd into Docker, never
// exported to the backend), so a few lines of parsing beat a new
// devDependency for what is otherwise a one-shot cheat sheet lookup.
function loadDotEnv(filePath: string): Record<string, string> {
  const out: Record<string, string> = {}
  if (!fs.existsSync(filePath)) return out
  for (const rawLine of fs.readFileSync(filePath, 'utf-8').split('\n')) {
    const line = rawLine.trim()
    if (!line || line.startsWith('#')) continue
    const eq = line.indexOf('=')
    if (eq === -1) continue
    const key = line.slice(0, eq).trim()
    let value = line.slice(eq + 1).trim()
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
      value = value.slice(1, -1)
    }
    out[key] = value
  }
  return out
}

const fileEnv = loadDotEnv(path.resolve(REPO_ROOT, '.env'))

// process.env wins over the .env file, so a CI/shell override always takes
// precedence over the committed-nowhere local cheat sheet.
function envVar(key: string): string {
  return process.env[key] ?? fileEnv[key] ?? ''
}

// --- values to type into the Settings UI (never used to bypass it) --------

export const FIRECRAWL_KEY = envVar('FIRECRAWL_API_KEY')
export const LLAMAPARSE_KEY = envVar('LLAMA_PARSE_API_KEY')
export const LLM_KEY = envVar('LLM_API_KEY')
export const LLM_PROVIDER = envVar('LLM_PROVIDER') || 'openrouter'
export const LLM_MODEL = envVar('LLM_DEFAULT_MODEL') || 'google/gemini-2.5-flash'

export function missingCredentialKeys(): string[] {
  const missing: string[] = []
  if (!FIRECRAWL_KEY) missing.push('FIRECRAWL_API_KEY')
  if (!LLAMAPARSE_KEY) missing.push('LLAMA_PARSE_API_KEY')
  if (!LLM_KEY) missing.push('LLM_API_KEY')
  return missing
}

// --- fixtures ---------------------------------------------------------------

// example.pdf is a checked-in fixture, not a secret — the spec's prerequisite
// list says "repo root", but the current checkout carries it under scripts/
// (which this suite otherwise never reads from — see the plan's Step 0).
// Check both locations rather than hard-failing on a path that moved.
function resolvePdfPath(): string {
  const atRoot = path.resolve(REPO_ROOT, 'example.pdf')
  if (fs.existsSync(atRoot)) return atRoot
  const inScripts = path.resolve(REPO_ROOT, 'scripts', 'example.pdf')
  if (fs.existsSync(inScripts)) return inScripts
  return atRoot
}
export const PDF_PATH = resolvePdfPath()
export const PDF_EXISTS = fs.existsSync(PDF_PATH)

// --- import targets -----------------------------------------------------
// Real, public URLs for the two Ссылки imports (Phase 2). Kaspi is a known
// aggressive-anti-bot target (the plan itself anticipates the crawl being
// blocked) so a shop root is used rather than a specific, rot-prone product
// page; override either via env if you have a better-known-good page.

export const KASPI_URL = envVar('E2E_KASPI_URL') || 'https://kaspi.kz/shop/'
export const XPAYMENT_URL = envVar('E2E_XPAYMENT_URL') || 'https://xpayment.kz/'

// Mirrors playwright.config.ts's own BASE_URL resolution — needed here too
// because the journey spec opens its own browser context by hand (to share
// ONE page across many test() blocks in serial order) rather than using the
// per-test `page` fixture, so it must pass baseURL explicitly instead of
// inheriting the project's `use.baseURL`.
export const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:5173'
