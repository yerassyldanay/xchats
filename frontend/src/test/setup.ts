// jsdom stubs the 'dom' vitest project needs — reka-ui's primitives (Dialog,
// Tooltip, DropdownMenu, ...) all probe these at mount time, and jsdom
// implements none of them.
import { afterEach, vi } from 'vitest'
import { enableAutoUnmount } from '@vue/test-utils'

// A Dialog/Tooltip/DropdownMenu Teleports its content to document.body,
// OUTSIDE the mounted wrapper's own element — a raw `document.body.innerHTML
// = ''` reset (this file's previous approach) rips that DOM out from under
// Vue's still-running reactive effects instead of tearing them down, which
// surfaces as "Cannot read properties of null" from a LATER test. Auto-
// unmounting every wrapper properly runs onUnmounted/effect cleanup first.
enableAutoUnmount(afterEach)

if (!window.matchMedia) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}

if (!window.ResizeObserver) {
  window.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
}

if (!window.IntersectionObserver) {
  // @ts-expect-error minimal stub — jsdom has no real implementation either
  window.IntersectionObserver = class IntersectionObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
    takeRecords() {
      return []
    }
  }
}

for (const fn of ['scrollIntoView', 'hasPointerCapture', 'setPointerCapture', 'releasePointerCapture'] as const) {
  if (!(Element.prototype as any)[fn]) {
    ;(Element.prototype as any)[fn] = () => {}
  }
}

// jsdom has no EventSource — every KB page calls pg.startRealtime() from
// onMounted, which opens one via lib/sse.ts's connectRealtime(). A minimal
// stub (never actually connects) is enough for mount() to not throw.
if (!window.EventSource) {
  class EventSourceStub {
    static readonly CONNECTING = 0
    static readonly OPEN = 1
    static readonly CLOSED = 2
    onopen: (() => void) | null = null
    onerror: (() => void) | null = null
    onmessage: ((e: MessageEvent) => void) | null = null
    constructor(_url: string, _opts?: EventSourceInit) {}
    addEventListener() {}
    removeEventListener() {}
    close() {}
  }
  // @ts-expect-error minimal stub — assigned on both window and the bare
  // global, since application code refers to the unqualified identifier.
  window.EventSource = EventSourceStub
  // @ts-expect-error see above
  globalThis.EventSource = EventSourceStub
}
