import { describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { usePlayground } from '@/stores/playground'
import { mountKb, testPinia } from '@/test/mount'
import MediaFieldPicker from './MediaFieldPicker.vue'
import type { DraftView, KbMaterial } from '@/types'

function material(over: Partial<KbMaterial> = {}): KbMaterial {
  return {
    id: 'mat-1', source_type: 'file', source_ref: '', blob_id: '', extracted_text: '',
    media_kind: '', status: 'ready', extraction: '{}', created_at: '', updated_at: '',
    filename: 'photo.png', mime_type: 'image/png', size_bytes: 1024,
    processing_status: 'parsed', customer_visibility: 'visible', visual_summary: '', transcript_text: '',
    operator_note: '', has_content: true,
    ...over,
  }
}

function mountPicker(
  materials: KbMaterial[],
  props: { label: string; field: string; multiple: boolean; modelValue: string | string[] | null }
) {
  const pinia = testPinia()
  const pg = usePlayground()
  pg.live = {
    config: {
      organization_id: 'org-1', persona: '', mission: '', guardrails: '', language_policy: '',
      reply_max_words: 120, draft: false, base_version: 0, updated_at: '',
    },
    topics: [], tariffs: [], products: [], contacts: [], policies: [], zones: [], materials, requests: [],
  } as DraftView
  const wrapper = mountKb(MediaFieldPicker, { pinia, props })
  return { wrapper, pg }
}

// detachButtons filters by aria-label rather than plain `button[aria-label]`
// — MediaThumb's own "open image" button also carries an aria-label, so a
// bare attribute selector would match both.
function detachButtons(wrapper: ReturnType<typeof mountKb>) {
  return wrapper.findAll('button').filter((b) => b.attributes('aria-label') === 'Открепить')
}

describe('MediaFieldPicker — attach existing', () => {
  it('lists uploaded materials of the matching kind, excluding already-selected', () => {
    const { wrapper } = mountPicker(
      [material({ id: 'a', filename: 'a.png' }), material({ id: 'b', filename: 'b.png' })],
      { label: 'Галерея', field: 'gallery_images', multiple: true, modelValue: ['a'] }
    )
    const options = wrapper.findAll('option').map((o) => o.attributes('value'))
    expect(options).not.toContain('a')
    expect(options).toContain('b')
  })

  it('excludes materials with no bytes yet, the wrong kind, or invisible visibility', () => {
    const { wrapper } = mountPicker(
      [
        material({ id: 'pending', has_content: false }),
        material({ id: 'wrong-kind', mime_type: 'application/pdf' }),
        material({ id: 'invisible', customer_visibility: 'invisible' }),
        material({ id: 'ok' }),
      ],
      { label: 'Галерея', field: 'gallery_images', multiple: true, modelValue: [] }
    )
    const options = wrapper.findAll('option').map((o) => o.attributes('value')).filter(Boolean)
    expect(options).toEqual(['ok'])
  })

  it('selecting an option appends it for a plural field', async () => {
    const { wrapper } = mountPicker([material({ id: 'a' })], {
      label: 'Галерея', field: 'gallery_images', multiple: true, modelValue: ['existing'],
    })
    await wrapper.find('select').setValue('a')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([['existing', 'a']])
  })

  it('selecting an option REPLACES a singular field', async () => {
    const { wrapper } = mountPicker([material({ id: 'a' })], {
      label: 'Изображение', field: 'featured_image', multiple: false, modelValue: null,
    })
    await wrapper.find('select').setValue('a')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['a'])
  })

  it('renders no <select> at all when nothing is available to attach', () => {
    const { wrapper } = mountPicker([], { label: 'Галерея', field: 'gallery_images', multiple: true, modelValue: [] })
    expect(wrapper.find('select').exists()).toBe(false)
  })
})

describe('MediaFieldPicker — detach', () => {
  it('clicking X removes just that id from a plural field', async () => {
    const { wrapper } = mountPicker([material({ id: 'a' }), material({ id: 'b' })], {
      label: 'Галерея', field: 'gallery_images', multiple: true, modelValue: ['a', 'b'],
    })
    await detachButtons(wrapper)[0].trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([['b']])
  })

  it('clicking X on a singular field emits null', async () => {
    const { wrapper } = mountPicker([material({ id: 'a' })], {
      label: 'Изображение', field: 'featured_image', multiple: false, modelValue: 'a',
    })
    await detachButtons(wrapper)[0].trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([null])
  })
})

describe('MediaFieldPicker — upload', () => {
  it('uploads a file through the store and attaches the returned id, without setting busy', async () => {
    const { wrapper, pg } = mountPicker([], { label: 'Галерея', field: 'gallery_images', multiple: true, modelValue: [] })
    const uploadSpy = vi.spyOn(pg, 'uploadMaterial').mockResolvedValue('new-id')
    const file = new File(['data'], 'photo.png', { type: 'image/png' })
    const input = wrapper.find('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [file] })
    await input.trigger('change')
    await flushPromises()

    expect(uploadSpy).toHaveBeenCalledWith(file)
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([['new-id']])
    expect(pg.busy).toBe(false)
  })

  it('shows an error message and does not emit when upload fails', async () => {
    const { wrapper, pg } = mountPicker([], { label: 'Галерея', field: 'gallery_images', multiple: true, modelValue: [] })
    vi.spyOn(pg, 'uploadMaterial').mockImplementation(async () => {
      pg.error = 'Не удалось загрузить файл.'
      return undefined
    })
    const file = new File(['data'], 'photo.png', { type: 'image/png' })
    const input = wrapper.find('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [file] })
    await input.trigger('change')
    await flushPromises()

    expect(wrapper.text()).toContain('Не удалось загрузить файл.')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })
})
