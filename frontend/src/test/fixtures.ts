// Shared row/change-set fixture factories — every KB row type has enough
// required fields that inlining literals in each test file would drown the
// actual assertions. Every factory takes overrides so a test states only
// what it cares about.
import type {
  ContactRow, DeliveryZoneRow, DraftChangeSet, DraftConfig, DraftView, PolicyRow, ProductRow, PromptView, TariffRow, TopicRow,
} from '@/types'

export function topicRow(overrides: Partial<TopicRow> = {}): TopicRow {
  return {
    id: 't1', slug: 't1', title: 'Title', body_md: 'Body.',
    featured_image: null, illustration_images: [], explainer_videos: [], narration_audio_files: [], reference_documents: [],
    draft: false, updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

export function tariffRow(overrides: Partial<TariffRow> = {}): TariffRow {
  return {
    id: 'r1', ref: 'r1', name: 'Tariff', price: '1000', limit_text: '', fee: '', summary: '',
    pricing_type: 'fixed', advantages: '', disadvantages: '', sales_status: 'active',
    featured_image: null, pricing_images: [], explainer_videos: [], terms_documents: [],
    draft: false, updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

export function productRow(overrides: Partial<ProductRow> = {}): ProductRow {
  return {
    id: 'p1', ref: 'p1', name: 'Product', price: '1000', description: '', category: '',
    in_stock: true, sales_status: 'active',
    featured_image: null, gallery_images: [], demo_videos: [], audio_description_files: [],
    certificate_documents: [], manual_documents: [], guarantee_documents: [], specification_documents: [],
    draft: false, updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

export function zoneRow(overrides: Partial<DeliveryZoneRow> = {}): DeliveryZoneRow {
  return {
    id: 'z1', ref: 'z1', name: 'Zone', zone_level: 'city', parent_ref: '',
    delivery_available: true, delivery_cost: '500', delivery_in_days: '1', notes: '', sales_status: 'active',
    draft: false, updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

export function contactRow(overrides: Partial<ContactRow> = {}): ContactRow {
  return {
    id: 'support', slug: 'support', whatsapp: '', email: '', address: '', legal_information: '',
    callback_time: '', working_hours: '', phone: '', website: '', instagram: '',
    contact_card_image: null, location_map_image: null, company_legal_documents: [],
    draft: false, updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

export function policyRow(overrides: Partial<PolicyRow> = {}): PolicyRow {
  return {
    id: 'main', slug: 'main', delivery_cost: '', delivery_in_days: '', free_delivery_from: '', min_order: '',
    prepayment: '', installment: '', return_period_in_days: '', warranty: '', outside_zones_note: '',
    commerce_policy_documents: [], draft: false, updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

export function draftConfig(overrides: Partial<DraftConfig> = {}): DraftConfig {
  return {
    organization_id: 'org1', persona: '', mission: '', guardrails: '', language_policy: '', reply_max_words: 120,
    draft: false, base_version: 1, updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

export function emptyChangeSet(overrides: Partial<DraftChangeSet> = {}): DraftChangeSet {
  return {
    base_version: 1, updated_at: '2026-01-01T00:00:00Z', config: null,
    topics: [], tariffs: [], products: [], contacts: [], policies: [], zones: [], deletes: [],
    ...overrides,
  }
}

export function emptyLiveView(overrides: Partial<DraftView> = {}): DraftView {
  return {
    config: draftConfig(),
    topics: [], tariffs: [], products: [], contacts: [], policies: [], zones: [], materials: [], requests: [],
    ...overrides,
  }
}

export function emptyPromptView(overrides: Partial<PromptView> = {}): PromptView {
  return {
    prompt_ref: 'p1', rendered_text: '', frame_text: '', char_count: 0, approx_tokens: 0,
    built_at: '2026-01-01T00:00:00Z', status: 'ok',
    section_counts: { topics: 0, products: 0, tariffs: 0, zones: 0, contacts: 0, policies: 0 },
    ...overrides,
  }
}
