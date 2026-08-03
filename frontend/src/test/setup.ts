// jsdom stubs the reka-ui primitives (Dialog/DropdownMenu/Select/Tooltip —
// all used across the KB forms and cards) need for their floating-ui-based
// positioning and viewport-collision checks — jsdom implements none of
// matchMedia/ResizeObserver/IntersectionObserver. Import-time side effects
// only; loaded via vitest.config.ts's `dom` project setupFiles, once per
// test file.
import { vi } from 'vitest'

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
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver
}

if (!window.IntersectionObserver) {
  window.IntersectionObserver = class {
    root = null
    rootMargin = ''
    thresholds: number[] = []
    observe() {}
    unobserve() {}
    disconnect() {}
    takeRecords() {
      return []
    }
  } as unknown as typeof IntersectionObserver
}
