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

async function send<T>(method: string, path: string, body?: unknown): Promise<T> {
  const opts: RequestInit = { method, credentials: 'include', headers: {} }
  if (body !== undefined) {
    opts.headers = { 'Content-Type': 'application/json' }
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(API_BASE + PREFIX + path, opts)
  return unwrap<T>(res, path)
}

export const api = {
  get: <T>(path: string) => send<T>('GET', path),
  post: <T>(path: string, body?: unknown) => send<T>('POST', path, body),
  del: <T>(path: string) => send<T>('DELETE', path),
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
  mediaURL: (urlPath: string) => API_BASE + urlPath,
  realtimeURL: () => API_BASE + PREFIX + '/realtime',
}
