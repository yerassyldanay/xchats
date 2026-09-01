import { ref, watch, type Ref } from 'vue'

// usePanelCollapsed backs INB-02's collapse toggles: a per-panel boolean
// that survives a reload, guarded the same way i18n/index.ts guards its own
// localStorage read/write — this module is imported by components unit-
// tested under vitest's `node` project, where there is no localStorage at
// all, and a private-mode quota error must not crash the toggle either.
export function usePanelCollapsed(key: string): Ref<boolean> {
  const storageKey = `xchats.panel.${key}.collapsed`
  const collapsed = ref(readStored(storageKey))
  watch(collapsed, (v) => writeStored(storageKey, v))
  return collapsed
}

function readStored(storageKey: string): boolean {
  try {
    return localStorage.getItem(storageKey) === '1'
  } catch {
    return false
  }
}

function writeStored(storageKey: string, value: boolean): void {
  try {
    localStorage.setItem(storageKey, value ? '1' : '0')
  } catch {
    // Private-mode quota errors and non-browser environments both land
    // here; the choice still applies for this session, it just isn't
    // remembered — matches i18n/index.ts's own writeStoredLocale.
  }
}
