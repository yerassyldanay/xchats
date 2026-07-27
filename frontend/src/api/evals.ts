import { log } from '../lib/logfmt'
import type { CatalogFile, ExecutionsFile, LaunchManifest, RunsFile } from '../types'

// Eval comparison UI's data client — deliberately SEPARATE from api/client.ts:
// these are plain static JSON files served directly by nginx from evals/runs/
// (frontend/nginx.conf's /evals-data/ location, mounted only in
// docker-compose.override.yaml — local dev only), never the backend's
// envelope-wrapped {payload,errcode,message} API. Same-origin in production;
// vite.config.ts proxies /evals-data in dev, same pattern as the existing /xchats
// proxy, so no separate base URL constant is needed here.
const PREFIX = '/evals-data'

// EvalsUnavailableError distinguishes "the eval data mount isn't there at all"
// (fresh clone, or a deploy that never mounted evals/runs/ — the local-dev-only
// guard working as intended) from a genuine fetch failure, so the UI can show
// "run `harness export -all`" instead of a generic error.
export class EvalsUnavailableError extends Error {
  constructor(path: string) {
    super(`eval data not found: ${path}`)
  }
}

async function getJSON<T>(path: string): Promise<T> {
  // Exported eval files are mutable even though nginx serves them as static
  // files. Never let an already-open browser tab reuse an older runs/catalog
  // snapshot after the harness rewrites it.
  const res = await fetch(PREFIX + path, { cache: 'no-store' })
  log.info('evals fetch', { path, status: res.status })
  if (res.status === 404) throw new EvalsUnavailableError(path)
  if (!res.ok) throw new Error(`eval data fetch failed: ${path} (${res.status})`)
  return (await res.json()) as T
}

export const evalsApi = {
  // fetchRuns loads the launches-list page's one data source. Throws
  // EvalsUnavailableError on a fresh clone (no `harness export -all` run yet) or an
  // unmounted /evals-data/ (production posture) — callers render the "no data" state.
  fetchRuns(): Promise<RunsFile> {
    return getJSON<RunsFile>('/runs.json')
  },

  // fetchExecutions loads one run's full execution list (the comparison matrix +
  // drill-down's one data source).
  fetchExecutions(runID: string): Promise<ExecutionsFile> {
    return getJSON<ExecutionsFile>(`/${encodeURIComponent(runID)}/executions.json`)
  },

  // fetchCatalog loads the requirements catalog (runs/catalog.json) — every
  // scenario's and extraction case's requirements, resolved to real values, as of
  // the last `harness export -all`. Throws EvalsUnavailableError on a checkout that
  // predates this export or hasn't run it yet.
  fetchCatalog(): Promise<CatalogFile> {
    return getJSON<CatalogFile>('/catalog.json')
  },

  // catalogImageURL builds the served path for one extraction case's PROCESSED
  // image (catalog.go copies it through the same preprocess() pipeline a real
  // extraction call uses, so what's shown is exactly what a model would receive).
  catalogImageURL(image: string): string {
    return `${PREFIX}/${image}`
  },

  // fetchLaunch loads a launch's own manifest (status, planned families, expected
  // call count) — best-effort by design: a run started directly via `run`/`extract`
  // (no `harness launch`) has no launches/<id>.json at all, which is a normal,
  // common case, not an error. Callers get `null` for that case specifically.
  async fetchLaunch(launchID: string): Promise<LaunchManifest | null> {
    try {
      return await getJSON<LaunchManifest>(`/launches/${encodeURIComponent(launchID)}.json`)
    } catch (e) {
      if (e instanceof EvalsUnavailableError) return null
      throw e
    }
  },

  // probeAvailable checks whether /evals-data/ is mounted at all, WITHOUT throwing —
  // used by NavRail to hide the "Эвалы" nav item entirely when the mount is absent
  // (review amendment 7: no mount -> no nav item), rather than showing a link that
  // always 404s.
  async probeAvailable(): Promise<boolean> {
    return evalsApi.probeFile(PREFIX + '/runs.json')
  },

  // probeFile HEAD-checks whether an arbitrary URL actually resolves, without
  // throwing — the general form probeAvailable is built on. Used to gate the launch
  // detail page's export links (SUMMARY.md/CONTRACT.md/EXTRACT.md): a run that
  // never produced one of these (e.g. the hand-authored sample launch, which has no
  // Markdown reports at all) must not offer a link that only ever 404s.
  async probeFile(url: string): Promise<boolean> {
    try {
      const res = await fetch(url, { method: 'HEAD', cache: 'no-store' })
      return res.ok
    } catch {
      return false
    }
  },

  // inputImageURL / promptFileURL / reportFileURL build direct links into the
  // static mount — the extraction case's captured input image, the "view exact
  // prompt" link to a scenario's snapshotted, fully-rendered prompt.txt (see evals
  // README's "Comparison metadata" section for why this is prompt.txt, not
  // frame.txt: it's the ACTUAL rendered text a run's model calls saw), and a run's
  // human-readable reports.
  inputImageURL(runID: string, caseID: string): string {
    return `${PREFIX}/${encodeURIComponent(runID)}/inputs/${encodeURIComponent(caseID)}.jpg`
  },
  promptFileURL(runID: string, scenario: string): string {
    return `${PREFIX}/${encodeURIComponent(runID)}/snapshots/${encodeURIComponent(scenario)}/prompt.txt`
  },
  reportFileURL(runID: string, filename: string): string {
    return `${PREFIX}/${encodeURIComponent(runID)}/${encodeURIComponent(filename)}`
  },

  // fetchPromptText loads a scenario's exact rendered prompt as plain text — the
  // strategy dialog's "Просмотр промпта" view. Throws EvalsUnavailableError on 404,
  // which covers BOTH a legacy run with no snapshot at all AND an extraction setup
  // (extraction prompts live in evals/prompts/, never snapshotted into the run dir —
  // see the launch-detail plan's correction 2) — callers render the same "prompt
  // text isn't available for this run" note for either case, which is honest: from
  // the frontend's point of view they're indistinguishable.
  async fetchPromptText(runID: string, scenario: string): Promise<string> {
    const url = evalsApi.promptFileURL(runID, scenario)
    const res = await fetch(url, { cache: 'no-store' })
    log.info('evals fetch', { path: url, status: res.status })
    if (res.status === 404) throw new EvalsUnavailableError(url)
    if (!res.ok) throw new Error(`prompt fetch failed: ${url} (${res.status})`)
    return res.text()
  },
}
