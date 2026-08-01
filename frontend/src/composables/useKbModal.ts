// useKbModal is База знаний's one shared modal session — every «Добавить …» /
// «Изменить» button opens the SAME underlying session (module-scoped state,
// not per-call-site), keyed by a stable id that survives the store replacing
// its underlying objects on every SSE reload (see this file's stale-submit
// flow below and §2.6 of the redesign notes).
import { computed, ref, toRaw } from 'vue'
import { usePlayground } from '@/stores/playground'
import type { ChangeKind, ConfigField, KbRow } from './draftChanges'
import type { DraftConfig } from '@/types'
import type { KbFormPayload } from '@/components/kb/forms/payloads'

export type ModalMode = 'create' | 'edit'
// A config edit only ever needs ONE field's current value — ConfigChangeGroup
// re-editing an already-staged field seeds from that field's PENDING value,
// not the published baseline, so a bare Partial is the honest shape (never a
// full DraftConfig assembled just to satisfy a wider type).
export type KbSnapshot = KbRow | Partial<DraftConfig>
export interface ModalSession {
  id: string // stable for the life of the dialog
  kind: ChangeKind
  mode: ModalMode
  key?: string
  field?: ConfigField
  snapshot: KbSnapshot | null // deep copy taken at open
}

let nextSessionId = 0
const session = ref<ModalSession | null>(null)
const localError = ref('')
const localStale = ref(false)

export function useKbModal() {
  const pg = usePlayground()

  const isOpen = computed(() => session.value !== null)
  const busy = computed(() => pg.busy)
  const error = computed(() => localError.value)
  const stale = computed(() => localStale.value)

  function openCreate(kind: ChangeKind) {
    localError.value = ''
    localStale.value = false
    session.value = { id: 'kb-modal-' + nextSessionId++, kind, mode: 'create', snapshot: null }
  }

  // On open, deep-copy the record into the form's own buffer — a shallow
  // spread is NOT sufficient: media columns are arrays and would stay
  // shared with the store's own object, so a later SSE reload replacing
  // that store object would silently mutate what looks like "our" buffer.
  // toRaw() first: `row` is almost always read straight off pg.live/
  // pg.changes (a Pinia-reactive Proxy), and structuredClone() throws
  // DataCloneError on those — toRaw() only needs to strip the outer proxy
  // since Vue's reactive() never writes proxies back into the target's own
  // stored properties, only wraps them on access.
  function openEdit(kind: ChangeKind, row: KbSnapshot, opts?: { field?: ConfigField }) {
    localError.value = ''
    localStale.value = false
    const key = 'id' in row ? (row as KbRow).id : undefined
    session.value = {
      id: 'kb-modal-' + nextSessionId++,
      kind,
      mode: 'edit',
      key,
      field: opts?.field,
      snapshot: structuredClone(toRaw(row)),
    }
  }

  function close() {
    session.value = null
    localError.value = ''
    localStale.value = false
  }

  async function stage(payload: KbFormPayload): Promise<boolean> {
    const s = session.value
    if (!s) return false
    const ok = await pg.stageChange(s.kind, payload)
    if (ok) {
      close()
      return true
    }
    // Closes ONLY on success — a stale or invalid submit leaves the dialog
    // open with every typed value intact (the form component owns the
    // buffer; this composable never touches it).
    localStale.value = pg.draftStale
    localError.value = pg.error
    return false
  }

  function submit(payload: KbFormPayload): Promise<boolean> {
    return stage(payload)
  }

  // reloadAndRetry: reload the change set (picking up whatever moved it),
  // keep the caller's buffer exactly as typed, resubmit against the fresh
  // version.
  async function reloadAndRetry(payload: KbFormPayload): Promise<boolean> {
    localStale.value = false
    await pg.load()
    return stage(payload)
  }

  return { session, isOpen, busy, error, stale, openCreate, openEdit, close, submit, reloadAndRetry }
}
