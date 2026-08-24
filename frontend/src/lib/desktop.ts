// Desktop (Wails) detection.
//
// The same bundle ships to a browser and to the desktop app's WebView, so
// every desktop-specific branch in the frontend keys off one runtime check
// rather than a build flag: there is no separate desktop build of the SPA,
// and `npm run build` produces the artifact both deployments use.
//
// The shell (backend/internal/desktop) injects Wails' runtime into the page
// as `window.runtime` before any application code runs. Its presence is the
// signal, and its EventsOn is the only part of it this app uses — the API
// itself is reached over ordinary same-origin fetch, exactly as in a browser
// (see backend/internal/desktop/handler.go).

/** The slice of Wails' injected runtime the frontend depends on. */
export interface DesktopRuntime {
  /** Subscribes to a backend event; the returned function unsubscribes. */
  EventsOn(name: string, callback: (data: unknown) => void): () => void
}

declare global {
  interface Window {
    runtime?: Partial<DesktopRuntime>
  }
}

/**
 * Returns Wails' runtime when this bundle is running inside the desktop
 * shell, and null in a browser.
 *
 * Feature-detected on EventsOn rather than on the bare `window.runtime`
 * object: an older shell, or a page loaded before injection finished, would
 * otherwise look like a desktop app whose event subscription throws.
 */
export function desktopRuntime(): DesktopRuntime | null {
  const rt = typeof window === 'undefined' ? undefined : window.runtime
  if (rt && typeof rt.EventsOn === 'function') {
    return rt as DesktopRuntime
  }
  return null
}

/** True when running inside the xchats desktop app. */
export function isDesktop(): boolean {
  return desktopRuntime() !== null
}
