import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import type { FrameLocator, Page } from '@playwright/test'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

// The REAL, production kb-manager.html — read from its actual location
// (backend/internal/mcpserver/widget/) rather than a copy, so this harness
// tests exactly what production embeds and can never silently drift from it.
const WIDGET_HTML_PATH = path.resolve(
  __dirname,
  '../../../../backend/internal/mcpserver/widget/kb-manager.html',
)

const HOST_ORIGIN = 'https://mock-widget-host.internal'
export const WIDGET_URL = `${HOST_ORIGIN}/kb-manager.html`
export const UPLOAD_URL = `${HOST_ORIGIN}/upload-target`

function widgetHtml(): string {
  return fs.readFileSync(WIDGET_HTML_PATH, 'utf-8')
}

// toolResult builds a full tools/call result envelope — the real wire shape
// (see mcpserver/handlers.go's toolResult): content the model reads,
// structuredContent the widget's unwrapToolResult() actually returns to
// callers, and an optional _meta (e.g. xchats/reviewUrl).
export function toolResult(structuredContent: unknown, meta: Record<string, unknown> = {}) {
  return {
    content: [{ type: 'text', text: 'mock result' }],
    isError: false,
    structuredContent,
    _meta: meta,
  }
}

const DEFAULT_FRONTEND_BASE_URL = 'https://xchats.test'

function defaultKbInfo() {
  return toolResult({
    types: ['assistant', 'topic', 'product', 'tariff', 'contacts', 'policies', 'delivery_zone'],
    natural_key_main: ['assistant', 'contacts', 'policies'],
    media_field_kinds: {},
    frontend_base_url: DEFAULT_FRONTEND_BASE_URL,
  })
}

function defaultKbSummary() {
  return toolResult({ draft_version: 1, items: [] })
}

// hostHtml is the mock MCP host page: it implements just enough of the
// generic postMessage JSON-RPC bridge (plan/mcp.md's fallback path —
// window.openai isn't available outside a real ChatGPT/Claude runtime) to
// drive plan Task 18's scenarios. `seeded` is baked in BEFORE the widget's
// own script ever runs, so the widget's boot-time kb_info/kb_summary calls
// can never race a test's setToolResponse() call — see mountWidget's doc.
function hostHtml(seeded: Record<string, unknown[]>): string {
  return `<!doctype html>
<html><body style="margin:0">
<iframe id="widget" src="${WIDGET_URL}" style="width:100vw;height:100vh;border:0"></iframe>
<script>
  window.__calls = [];
  window.__toolResponses = ${JSON.stringify(seeded)};
  window.__uiInitializeReceived = false;
  window.addEventListener("message", function (ev) {
    var data = ev.data;
    if (!data || data.jsonrpc !== "2.0") return;
    if (data.method === "ui/initialize") {
      window.__uiInitializeReceived = true;
      return;
    }
    if (data.method !== "tools/call") return;
    window.__calls.push({ name: data.params.name, arguments: data.params.arguments });
    var queue = window.__toolResponses[data.params.name];
    var result;
    if (Array.isArray(queue) && queue.length) {
      result = queue.length > 1 ? queue.shift() : queue[0];
    } else {
      result = { content: [{ type: "text", text: "no mock configured for " + data.params.name }], isError: true };
    }
    ev.source.postMessage({ jsonrpc: "2.0", id: data.id, result: result }, "*");
  });
  // sendToWidget lets the test inject a message AS THE HOST (ev.source will
  // correctly be window.parent from the widget's perspective) — used for
  // tool-result notifications a model-driven tool call would trigger.
  window.__sendToWidget = function (message) {
    document.getElementById("widget").contentWindow.postMessage(message, "*");
  };
</script>
</body></html>`
}

// mountWidget serves the real widget file plus the mock host page above
// under one fixed, fully route-intercepted origin — no real server process,
// no file:// path-depth fragility.
//
// `responses` seeds window.__toolResponses BEFORE the page ever navigates,
// so it covers everything the widget's OWN boot sequence calls
// (kb_info, then whatever loadForView('all') needs) without racing against
// it — a setToolResponse() call made only after mountWidget() resolves can
// already be too late for those two. Reasonable defaults are provided for
// both so a test that doesn't care about them can omit them entirely.
// Anything else (kb_topic_upsert, kb_media_upload, a SECOND kb_summary
// page, ...) is only ever called in response to a Playwright-driven action
// later in the test, so setToolResponse() after mountWidget() is safe for
// those.
export async function mountWidget(
  page: Page,
  responses: Record<string, unknown[]> = {},
): Promise<FrameLocator> {
  const seeded: Record<string, unknown[]> = {
    kb_info: [defaultKbInfo()],
    kb_summary: [defaultKbSummary()],
    ...responses,
  }
  await page.route(WIDGET_URL, (route) => route.fulfill({ contentType: 'text/html', body: widgetHtml() }))
  await page.route(`${HOST_ORIGIN}/`, (route) => route.fulfill({ contentType: 'text/html', body: hostHtml(seeded) }))
  await page.goto(HOST_ORIGIN + '/')
  return page.frameLocator('#widget')
}

// setToolResponse queues one or more canned tools/call results for a tool
// name, consumed in order (the last one repeats once exhausted) — for calls
// made only in response to a later, test-driven action (see mountWidget's
// doc for why boot-time calls must be seeded there instead).
export async function setToolResponse(page: Page, name: string, ...results: unknown[]) {
  await page.evaluate(
    ({ name, results }) => {
      ;(window as any).__toolResponses[name] = results
    },
    { name, results },
  )
}

export async function toolCalls(page: Page): Promise<Array<{ name: string; arguments: unknown }>> {
  return page.evaluate(() => (window as any).__calls)
}

export async function receivedUiInitialize(page: Page): Promise<boolean> {
  return page.evaluate(() => (window as any).__uiInitializeReceived)
}

export async function sendToWidget(page: Page, message: unknown) {
  await page.evaluate((message) => (window as any).__sendToWidget(message), message)
}

// postFromImpostorWindow simulates "a message from another iframe/window"
// (plan Task 18) — a message whose ev.source, as seen by the widget, is
// NEITHER window.parent NOR anything the widget should trust. A plain
// page.evaluate() call from the top-level test page would itself BE
// window.parent (this page genuinely is the widget's parent), so this
// spins up a THROWAWAY sibling iframe and posts from inside ITS OWN
// execution context instead — a real "some other frame" sender.
export async function postFromImpostorWindow(page: Page, message: unknown) {
  await page.evaluate((message) => {
    const impostor = document.createElement('iframe')
    impostor.srcdoc = `<script>
      parent.document.getElementById("widget").contentWindow.postMessage(${JSON.stringify(message)}, "*");
    <\/script>`
    document.body.appendChild(impostor)
  }, message)
}
