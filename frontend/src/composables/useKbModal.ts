import { computed, ref, toRaw } from 'vue'
import { nanoid } from 'nanoid'
import { usePlayground } from '@/stores/playground'
import type { KbFormPayload } from '@/components/kb/forms/payloads'
import type { DraftConfig } from '@/types'
import type { ChangeKind, ConfigField, KbRow } from './draftChanges'

export type ModalMode = 'create' | 'edit'

export interface ModalSession {
  id: string // stable for the life of the dialog — never the store row's own reference
  kind: ChangeKind
  mode: ModalMode
  key?: string
  field?: ConfigField // config only — which of the 5 sections this edit targets
  snapshot: KbRow | DraftConfig | null // a DEEP copy taken at open; create mode has none
}

// Module-level (not inside the function body) so every component calling
// useKbModal() shares the SAME session — there is only ever one modal open
// at a time, driven from wherever the user clicked «Изменить»/«Добавить».
const session = ref<ModalSession | null>(null)
const busyRef = ref(false)
const errorRef = ref('')
const staleRef = ref(false)
// successCount increments on every successful submit — a page (Знаний база,
// for its DraftBanner) watches this rather than isOpen, because isOpen also
// transitions to false on a plain Cancel/Escape, which must NOT show
// "change staged".
const successCountRef = ref(0)

// useKbModal owns the whole "open a form, survive a background refresh,
// handle a stale submit" flow (§2.6) for BOTH pages: /knowledge-base opens
// it to create/edit a published row, /playground opens it to tweak a
// pending row's value before publishing — either way, submit() always
// stages into the draft (stores.stageChange), never writes live directly.
export function useKbModal() {
  const pg = usePlayground()

  const isOpen = computed(() => session.value !== null)
  const busy = computed(() => busyRef.value)
  const error = computed(() => errorRef.value)
  const stale = computed(() => staleRef.value)
  const successCount = computed(() => successCountRef.value)

  function reset() {
    errorRef.value = ''
    staleRef.value = false
  }

  function openCreate(kind: ChangeKind) {
    session.value = { id: nanoid(), kind, mode: 'create', snapshot: null }
    reset()
  }

  // openEdit deep-copies `row` on the spot — structuredClone, not a shallow
  // spread, because media columns are arrays (illustration_images,
  // gallery_images, commerce_policy_documents, …) that a shallow copy would
  // still share by reference with whatever the store holds. The session is
  // keyed by a freshly minted id, never the row's own reference: the store
  // replaces that object wholesale on every reload (setChanges/loadLive
  // reassign, they never mutate in place), so keying on it would remount
  // the dialog and discard anything the operator had typed.
  //
  // toRaw() first: `row` normally comes straight off Pinia state (pg.live.
  // topics[i], etc.), which is a Vue reactive Proxy — structuredClone()
  // throws DataCloneError on a Proxy directly ("could not be cloned"). toRaw
  // unwraps to the underlying plain object structuredClone can actually
  // handle; it is a no-op on an already-plain object (e.g. in tests), so
  // this is safe regardless of what called openEdit.
  function openEdit(kind: ChangeKind, row: KbRow | DraftConfig, opts?: { field?: ConfigField }) {
    const raw = toRaw(row)
    session.value = {
      id: nanoid(),
      kind,
      mode: 'edit',
      key: 'id' in raw ? raw.id : undefined,
      field: opts?.field,
      snapshot: structuredClone(raw),
    }
    reset()
  }

  function close() {
    session.value = null
    reset()
  }

  // markSuccess is for AssistantFieldForm.vue only: config edits bypass
  // submit() entirely (stores.patchConfig, not stageChange — see its own
  // doc comment), so it reports success here instead, keeping successCount
  // meaningful for every kind, not just the six KbFormPayload ones.
  function markSuccess() {
    successCountRef.value++
  }

  // submit stages payload and closes ONLY on success. A DRAFT_STALE
  // conflict — detected via pg.draftStale, not by parsing pg.error's text —
  // leaves the dialog open with everything the operator typed intact and
  // sets `stale`, so the caller can offer «Обновить и повторить» instead of
  // silently overwriting their input with whatever just arrived.
  async function submit(payload: KbFormPayload): Promise<boolean> {
    if (!session.value) return false
    busyRef.value = true
    reset()
    try {
      const ok = await pg.stageChange(session.value.kind as Exclude<ChangeKind, 'config'>, payload)
      if (ok) {
        close()
        successCountRef.value++
        return true
      }
      staleRef.value = pg.draftStale
      errorRef.value = pg.error
      return false
    } finally {
      busyRef.value = false
    }
  }

  // reloadAndRetry is submit() again, named for what the button reads: by
  // the time submit() first failed on staleness, stores.write() had already
  // reloaded `changes` to the fresh version (see its own doc comment), so
  // this retry's ifMatch() picks up the new base_version automatically —
  // there is no separate reload step to perform here.
  function reloadAndRetry(payload: KbFormPayload): Promise<boolean> {
    return submit(payload)
  }

  return { session, isOpen, busy, error, stale, successCount, openCreate, openEdit, close, submit, reloadAndRetry, markSuccess }
}
