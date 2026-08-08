import { describe, expect, it } from 'vitest'
import { usePlayground } from '@/stores/playground'
import { mountKb, testPinia } from '@/test/mount'
import MediaStrip from './MediaStrip.vue'
import type { DraftView, KbMaterial } from '@/types'

function material(over: Partial<KbMaterial> = {}): KbMaterial {
  return {
    id: 'mat-1', source_type: 'file', source_ref: '', blob_id: '', extracted_text: '',
    media_kind: '', status: 'ready', extraction: '{}', created_at: '2026-01-01', updated_at: '2026-01-02',
    filename: 'file.bin', mime_type: 'application/octet-stream', size_bytes: 1024,
    processing_status: 'parsed', customer_visibility: 'visible', visual_summary: '', transcript_text: '',
    operator_note: '', has_content: true,
    ...over,
  }
}

function mountWithMaterials(materials: KbMaterial[], props: { label: string; field: string; ids: string | string[] | null }) {
  const pinia = testPinia()
  const pg = usePlayground()
  pg.live = {
    config: {
      organization_id: 'org-1', persona: '', mission: '', guardrails: '', language_policy: '',
      reply_max_words: 120, draft: false, base_version: 0, updated_at: '',
    },
    topics: [], tariffs: [], products: [], contacts: [], policies: [], zones: [], materials, requests: [],
  } as DraftView
  return mountKb(MediaStrip, { pinia, props })
}

describe('MediaStrip — empty slot', () => {
  it('renders the label and empty text with an image icon for an image field', () => {
    const wrapper = mountWithMaterials([], { label: 'Галерея', field: 'gallery_images', ids: [] })
    expect(wrapper.text()).toContain('Галерея')
    expect(wrapper.text()).toContain('Не добавлено')
    expect(wrapper.find('svg.lucide-image').exists()).toBe(true)
  })

  it('is empty for a null singular id too', () => {
    const wrapper = mountWithMaterials([], { label: 'Изображение', field: 'featured_image', ids: null })
    expect(wrapper.text()).toContain('Не добавлено')
    expect(wrapper.find('svg.lucide-image').exists()).toBe(true)
  })

  it('uses a video icon for a video field', () => {
    const wrapper = mountWithMaterials([], { label: 'Видео', field: 'demo_videos', ids: [] })
    expect(wrapper.find('svg.lucide-video').exists()).toBe(true)
  })

  it('uses a document icon for a document field', () => {
    const wrapper = mountWithMaterials([], { label: 'Сертификаты', field: 'certificate_documents', ids: [] })
    expect(wrapper.find('svg.lucide-file-text').exists()).toBe(true)
  })

  it('renders no thumbnail/video/download row when empty', () => {
    const wrapper = mountWithMaterials([], { label: 'Галерея', field: 'gallery_images', ids: [] })
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.find('video').exists()).toBe(false)
    expect(wrapper.find('a').exists()).toBe(false)
  })
})

describe('MediaStrip — attached media', () => {
  it('renders an image thumbnail for an attached image material', () => {
    const wrapper = mountWithMaterials(
      [material({ id: 'img-1', mime_type: 'image/png', filename: 'photo.png' })],
      { label: 'Галерея', field: 'gallery_images', ids: ['img-1'] }
    )
    expect(wrapper.text()).not.toContain('Не добавлено')
    const img = wrapper.find('img')
    expect(img.exists()).toBe(true)
    expect(img.attributes('src')).toContain('/kb/materials/img-1/content')
  })

  it('renders an inline <video> for an attached video material', () => {
    const wrapper = mountWithMaterials(
      [material({ id: 'vid-1', mime_type: 'video/mp4', filename: 'clip.mp4' })],
      { label: 'Видео', field: 'demo_videos', ids: ['vid-1'] }
    )
    const video = wrapper.find('video')
    expect(video.exists()).toBe(true)
    expect(video.attributes('src')).toContain('/kb/materials/vid-1/content')
  })

  it('renders a download link for an attached document material', () => {
    const wrapper = mountWithMaterials(
      [material({ id: 'doc-1', mime_type: 'application/pdf', filename: 'terms.pdf' })],
      { label: 'Условия', field: 'terms_documents', ids: ['doc-1'] }
    )
    const link = wrapper.find('a')
    expect(link.exists()).toBe(true)
    expect(link.text()).toContain('terms.pdf')
    expect(link.attributes('href')).toContain('/kb/materials/doc-1/content')
  })
})
