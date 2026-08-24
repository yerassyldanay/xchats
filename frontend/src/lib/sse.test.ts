import { describe, it, expect, afterEach } from 'vitest'
import { connectRealtime } from './sse'

const g = globalThis as { window?: unknown }

afterEach(() => {
  delete g.window
})

// A stand-in for Wails' injected runtime: records subscriptions and lets the
// test push an event the way the Go shell's EventsEmit would.
function fakeRuntime() {
  const subs = new Map<string, Array<(d: unknown) => void>>()
  let unsubscribed = 0
  return {
    unsubscribed: () => unsubscribed,
    names: () => [...subs.keys()],
    emit(name: string, data: unknown) {
      for (const cb of subs.get(name) ?? []) cb(data)
    },
    runtime: {
      EventsOn(name: string, cb: (d: unknown) => void) {
        subs.set(name, [...(subs.get(name) ?? []), cb])
        return () => {
          unsubscribed++
        }
      },
    },
  }
}

describe('connectRealtime in the desktop app', () => {
  it('delivers hub events to the matching handler without an EventSource', () => {
    // No EventSource is defined in this environment, so constructing one
    // would throw — which is exactly the assertion that matters: the desktop
    // path must never open an SSE stream.
    const f = fakeRuntime()
    g.window = { runtime: f.runtime }

    const seen: unknown[] = []
    const disconnect = connectRealtime({
      messageCreated: (m) => seen.push(m),
    })

    f.emit('message.created', { id: 'm1', body: 'hi' })
    expect(seen).toEqual([{ id: 'm1', body: 'hi' }])

    disconnect()
    expect(f.unsubscribed()).toBe(1)
  })

  it('subscribes only to the events a caller asked for', () => {
    const f = fakeRuntime()
    g.window = { runtime: f.runtime }

    connectRealtime({ kbRowChanged: () => {}, chatUpdated: () => {} })
    expect(f.names().sort()).toEqual(['chat.updated', 'kb.row.changed'])
  })
})
