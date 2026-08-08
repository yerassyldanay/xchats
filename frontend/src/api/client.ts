import { log } from '../lib/logfmt'

// Env-driven addressing: VITE_API_BASE_URL points at the backend. Empty means
// same-origin (dev uses the Vite proxy; prod fronts both behind one domain).
export const API_BASE = import.meta.env.VITE_API_BASE_URL || ''
const PREFIX = '/xchats/api/v1'

export class ApiError extends Error {
  constructor(public errcode: string, public status: number, message: string) {
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
    throw new ApiError(env.errcode, res.status, env.message)
  }
  return env.payload
}

type Headers = Record<string, string>

async function send<T>(method: string, path: string, body?: unknown, headers?: Headers): Promise<T> {
  const opts: RequestInit = { method, credentials: 'include', headers: { ...(headers || {}) } }
  if (body !== undefined) {
    opts.headers = { 'Content-Type': 'application/json', ...(headers || {}) }
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(API_BASE + PREFIX + path, opts)
  return unwrap<T>(res, path)
}

export const api = {
  get: <T>(path: string, headers?: Headers) => send<T>('GET', path, undefined, headers),
  post: <T>(path: string, body?: unknown, headers?: Headers) => send<T>('POST', path, body, headers),
  put: <T>(path: string, body?: unknown, headers?: Headers) => send<T>('PUT', path, body, headers),
  patch: <T>(path: string, body?: unknown, headers?: Headers) => send<T>('PATCH', path, body, headers),
  del: <T>(path: string, headers?: Headers) => send<T>('DELETE', path, undefined, headers),
  async upload(file: File): Promise<{ media_id: string; url: string; media_type: string }> {
    const form = new FormData()
    form.append('file', file)
    const res = await fetch(API_BASE + PREFIX + '/media', {
      method: 'POST',
      credentials: 'include',
      body: form,
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
}
