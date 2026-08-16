import { computed, ref, watch, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api, ApiError } from '@/api/client'
import type { KbImportProviderCapability, KbImportProviderFamily } from '@/types'

// --- MIME -> family, mirroring extractor.go's isTextlike/isImage (backend/
// internal/extractor/extractor.go:164-169) and capabilities.go's own probe
// families — kept here as plain structural checks rather than an exhaustive
// enum, same reasoning as the Go original. -------------------------------
const MIME_PDF = 'application/pdf'
const MIME_DOCX = 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'

function isTextlikeMime(mime: string): boolean {
  return mime.startsWith('text/') || mime === 'application/csv'
}
function isImageMime(mime: string): boolean {
  return mime.startsWith('image/')
}

// familyOfMime maps one staged File's declared type to the family
// GET /kb/import/providers' capability matrix is keyed by — null for a MIME
// no provider is known to read, so a caller never claims false confidence
// about an unrecognized type.
export function familyOfMime(mime: string): KbImportProviderFamily | null {
  if (mime === MIME_PDF) return 'pdf'
  if (mime === MIME_DOCX) return 'docx'
  if (isImageMime(mime)) return 'image'
  if (isTextlikeMime(mime)) return 'text'
  return null
}

const FILE_FAMILIES: KbImportProviderFamily[] = ['docx', 'pdf', 'text', 'image']

// --- module-level cache: the capability matrix is fixed backend data plus
// slow-changing credential state, not per-run — every KbImportCard instance
// (Ссылки AND Файлы, both always mounted under KbIngestPanel) shares one
// fetch instead of issuing it twice. -------------------------------------
const cachedProviders = ref<KbImportProviderCapability[] | null>(null)
let inFlight: Promise<KbImportProviderCapability[]> | null = null

async function loadProviders(): Promise<KbImportProviderCapability[]> {
  if (cachedProviders.value) return cachedProviders.value
  if (!inFlight) {
    inFlight = api.getKbImportProviders().finally(() => {
      inFlight = null
    })
  }
  const providers = await inFlight
  cachedProviders.value = providers
  return providers
}

// resetImportProvidersCache forces the next call to re-fetch — test-only:
// production has no "a credential just changed, refetch now" trigger yet,
// a full page load already starts from an empty cache.
export function resetImportProvidersCache() {
  cachedProviders.value = null
  inFlight = null
}

export interface ImportProviderOption {
  id: string
  displayName: string
  capsHint: string
  available: boolean
  reason: string // '' when available — already translated, ready to render
  // reasonKey is reason's untranslated source ('' | 'files' | 'pdf' | 'urls'
  // | 'noCredential') — exposed so a caller can single out the credential
  // case (to render a RouterLink to Settings) without string-matching
  // translated text.
  reasonKey: string
}

// useImportProviders drives one KbImportCard's parser <Select>: which of
// the (up to three) providers GET /kb/import/providers returns are actually
// usable for `kind`'s tab and whatever `stagedFiles` currently holds, plus
// why not for the rest — so the dropdown can never offer a combination
// Submit's own precheck (kbimport.Service.Submit) would 400 on.
//
// Availability is a two-tier gate, matching Submit's own per-file loop
// (enqueue.go): a BASELINE check — can this provider read this TAB's kind
// of content at all ("не читает файлы"/"не читает ссылки" — structural and
// staged-content-independent), then a REFINEMENT check once something is
// actually staged in the Файлы tab — does the provider cover EVERY staged
// family, not just one (surfaces e.g. "не читает PDF" the moment an
// incompatible file joins an otherwise-fine batch). A credential gate is
// independent of both.
//
// selectedProvider is corrected automatically the moment it stops being
// available (e.g. a PDF gets staged while `native` was selected) — falling
// back to the first still-available option rather than silently letting a
// caller submit an invalid pair.
export function useImportProviders(kind: Ref<'url' | 'file'>, stagedFiles: Ref<File[]>, selectedProvider: Ref<string>) {
  const { t } = useI18n()
  const loading = ref(false)
  const error = ref('')
  const providers = ref<KbImportProviderCapability[]>(cachedProviders.value ?? [])

  loading.value = !cachedProviders.value
  loadProviders()
    .then((p) => {
      providers.value = p
    })
    .catch((e) => {
      error.value = e instanceof ApiError ? e.message : t('kb.import.loadProvidersFailed')
    })
    .finally(() => {
      loading.value = false
    })

  const stagedFamilies = computed<KbImportProviderFamily[]>(() => {
    if (kind.value !== 'file') return []
    const set = new Set<KbImportProviderFamily>()
    for (const f of stagedFiles.value) {
      const fam = familyOfMime(f.type)
      if (fam) set.add(fam)
    }
    return [...set]
  })

  const hasStagedImage = computed(() => stagedFamilies.value.includes('image'))

  function reasonKeyFor(p: KbImportProviderCapability): string {
    if (kind.value === 'url') {
      if (!p.families.includes('url')) return 'urls'
    } else {
      if (!FILE_FAMILIES.some((f) => p.families.includes(f))) return 'files'
      const missing = stagedFamilies.value.find((f) => !p.families.includes(f))
      if (missing === 'pdf') return 'pdf'
      if (missing) return 'files'
    }
    if (p.requires_credential && !p.configured) return 'noCredential'
    return ''
  }

  const options = computed<ImportProviderOption[]>(() =>
    providers.value.map((p) => {
      const reasonKey = reasonKeyFor(p)
      return {
        id: p.id,
        displayName: t(`kb.import.providers.${p.id}`),
        capsHint: t(`kb.import.providerCaps.${p.id}`),
        available: reasonKey === '',
        reason: reasonKey ? t(`kb.import.unavailable.${reasonKey}`) : '',
        reasonKey,
      }
    })
  )

  watch(options, (opts) => {
    if (!opts.length) return
    const current = opts.find((o) => o.id === selectedProvider.value)
    if (current && current.available) return
    const fallback = opts.find((o) => o.available)
    if (fallback) selectedProvider.value = fallback.id
  })

  return { loading, error, options, hasStagedImage }
}
