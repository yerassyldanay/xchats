import { describe, it, expect, afterEach } from 'vitest'
import { desktopRuntime, isDesktop } from './desktop'

const g = globalThis as { window?: unknown }

function setWindow(value: unknown) {
  g.window = value
}

afterEach(() => {
  delete g.window
})

describe('desktopRuntime', () => {
  it('is null in a browser with no Wails runtime', () => {
    setWindow({})
    expect(desktopRuntime()).toBeNull()
    expect(isDesktop()).toBe(false)
  })

  it('is null when window itself is absent (SSR/node)', () => {
    delete g.window
    expect(desktopRuntime()).toBeNull()
  })

  it('is null when the injected runtime has no EventsOn', () => {
    // A partially-injected runtime must not be mistaken for a working one —
    // the realtime layer would silently subscribe to nothing.
    setWindow({ runtime: { Quit: () => {} } })
    expect(desktopRuntime()).toBeNull()
  })

  it('returns the runtime inside the desktop shell', () => {
    const runtime = { EventsOn: () => () => {} }
    setWindow({ runtime })
    expect(desktopRuntime()).toBe(runtime)
    expect(isDesktop()).toBe(true)
  })
})
