import { log } from '../lib/logfmt'
import type { KbImportProviderCapability, KbImportRun, KbImportRunPage, CampaignRecipientPreviewResult } from '../types'

// Env-driven addressing: VITE_API_BASE_URL points at the backend. Empty means
// same-origin (dev uses the Vite proxy; prod fronts both behind one domain).
export const API_BASE = import.meta.env.VITE_API_BASE_URL || ''
const PREFIX = '/xchats/api/v1'

export class ApiError extends Error {
  // payload is failWithPayload's own optional "something useful alongside
  // the failure" body (server.go) — undefined for the ordinary fail() case
  // (nearly every error). KB-09's gate 422 is the one caller that reads
  // it (payload.reasons); everything else keeps using .message as before.
  constructor(public errcode: string, public status: number, message: string, public payload?: unknown) {
    super(message || errcode)
  }
}

interface Envelope<T> {
  payload: T
  errcode: string
  message: string
}

async function unwrap<T>(res: Response, path: string): Promise<T> {
  const env = (await res.json()) as Envelope<T>
  log.info('api call', { path, status: res.status, errcode: env.errcode })
  if (env.errcode !== 'OK') {
    throw new ApiError(env.errcode, res.status, env.message, env.payload)
  }
  return env.payload
}

type Headers = Record<string, string>

// isAbortError recognizes a fetch aborted via AbortController.abort() — every
// caller that races a request against a newer one (INB-08) must swallow this
// silently instead of surfacing it as a load/save failure.
export function isAbortError(e: unknown): boolean {
  return e instanceof DOMException && e.name === 'AbortError'
}

async function send<T>(method: string, path: string, body?: unknown, headers?: Headers, signal?: AbortSignal): Promise<T> {
  const opts: RequestInit = { method, credentials: 'include', headers: { ...(headers || {}) }, signal }
  if (body !== undefined) {
    opts.headers = { 'Content-Type': 'application/json', ...(headers || {}) }
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(API_BASE + PREFIX + path, opts)
  return unwrap<T>(res, path)
}

export const api = {
  get: <T>(path: string, headers?: Headers, signal?: AbortSignal) => send<T>('GET', path, undefined, headers, signal),
  post: <T>(path: string, body?: unknown, headers?: Headers, signal?: AbortSignal) => send<T>('POST', path, body, headers, signal),
  put: <T>(path: string, body?: unknown, headers?: Headers, signal?: AbortSignal) => send<T>('PUT', path, body, headers, signal),
  patch: <T>(path: string, body?: unknown, headers?: Headers, signal?: AbortSignal) => send<T>('PATCH', path, body, headers, signal),
  del: <T>(path: string, headers?: Headers, signal?: AbortSignal) => send<T>('DELETE', path, undefined, headers, signal),
  async upload(file: File, signal?: AbortSignal): Promise<{ media_id: string; url: string; media_type: string }> {
    const form = new FormData()
    form.append('file', file)
    const res = await fetch(API_BASE + PREFIX + '/media', {
      method: 'POST',
      credentials: 'include',
      body: form,
      signal,
    })
    return unwrap(res, '/media')
  },
  // uploadKbMaterial is upload's KB counterpart (POST /kb/materials,
  // kb_media.go) — session-authenticated, single-shot: the returned
  // material_id is immediately attachable from a draft-lane upsert
  // (playground.ts's stageChange), no separate signed-URL step like the MCP
  // kb_media_upload flow needs for its cross-origin widget caller.
  async uploadKbMaterial(file: File): Promise<{ material_id: string; processing_status: string }> {
    const form = new FormData()
    form.append('file', file)
    const res = await fetch(API_BASE + PREFIX + '/kb/materials', {
      method: 'POST',
      credentials: 'include',
      body: form,
    })
    return unwrap(res, '/kb/materials')
  },
  mediaURL: (urlPath: string) => API_BASE + urlPath,
  realtimeURL: () => API_BASE + PREFIX + '/realtime',

  // submitKbImport is POST /kb/imports' multipart body: any mix of URLs and
  // files in one request (kbimport.SubmitInput, httpapi/kb_import.go). Field
  // names match the backend's multipart.Form keys exactly — repeated `url`,
  // repeated `file`, single `provider`/`target_type`/`guidance`.
  async submitKbImport(input: {
    provider: string
    targetType: string
    guidance: string
    urls: string[]
    files: File[]
  }): Promise<KbImportRun> {
    const form = new FormData()
    form.append('provider', input.provider)
    form.append('target_type', input.targetType)
    if (input.guidance) form.append('guidance', input.guidance)
    for (const u of input.urls) form.append('url', u)
    for (const f of input.files) form.append('file', f)
    const res = await fetch(API_BASE + PREFIX + '/kb/imports', {
      method: 'POST',
      credentials: 'include',
      body: form,
    })
    return unwrap(res, '/kb/imports')
  },
  getKbImportRun: (runId: string) => send<KbImportRun>('GET', '/kb/imports/' + encodeURIComponent(runId)),
  // listKbImportRuns covers both callers of GET /kb/imports: the ingestion
  // panel's "just the latest run" hydration (limit=1, offset omitted) and
  // KB-14's paginated history list (limit=pageSize, offset=(page-1)*
  // pageSize) — the response's total is the org's full run count, not just
  // runs.length.
  listKbImportRuns: (limit?: number, offset?: number) => {
    const params = new URLSearchParams()
    if (limit) params.set('limit', String(limit))
    if (offset) params.set('offset', String(offset))
    const qs = params.toString()
    return send<KbImportRunPage>('GET', '/kb/imports' + (qs ? `?${qs}` : ''))
  },
  // cancelKbImportRun is POST /kb/imports/:id/cancel (KB-04) — only valid
  // while pass 1 is still running (KbImportRun.cancelable); refused with a
  // 409 once synthesis has been claimed.
  cancelKbImportRun: (runId: string) => send<KbImportRun>('POST', '/kb/imports/' + encodeURIComponent(runId) + '/cancel'),
  // getKbImportProviders is GET /kb/import/providers (internal/httpapi/
  // kb_import.go) — the one source of truth useImportProviders fetches
  // once and caches, so the parser dropdown can never disagree with
  // Submit's own precheck.
  async getKbImportProviders(): Promise<KbImportProviderCapability[]> {
    const res = await send<{ providers: KbImportProviderCapability[] }>('GET', '/kb/import/providers')
    return res.providers
  },

  // campaignRecipientsBody is POST .../preview and PUT .../recipients'
  // shared multipart body (backend/internal/httpapi/campaigns.go's
  // campaignRecipientsInput): a pasted "text" field, an uploaded "file"
  // field, or both (the backend prefers the file when both are present).
  async previewCampaignRecipients(campaignId: string, input: { text?: string; file?: File }): Promise<CampaignRecipientPreviewResult> {
    return campaignRecipientsRequest(`/campaigns/${campaignId}/preview`, input)
  },
  async replaceCampaignRecipients(campaignId: string, input: { text?: string; file?: File }): Promise<CampaignRecipientPreviewResult> {
    return campaignRecipientsRequest(`/campaigns/${campaignId}/recipients`, input, 'PUT')
  },
}

async function campaignRecipientsRequest(
  path: string,
  input: { text?: string; file?: File },
  method: 'POST' | 'PUT' = 'POST',
): Promise<CampaignRecipientPreviewResult> {
  const form = new FormData()
  if (input.file) form.append('file', input.file)
  else if (input.text) form.append('text', input.text)
  const res = await fetch(API_BASE + PREFIX + path, { method, credentials: 'include', body: form })
  return unwrap(res, path)
}
