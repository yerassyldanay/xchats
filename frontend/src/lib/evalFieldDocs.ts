// Structure only — no display text. The canonical yaml authoring order for a test
// case's fields. Display text for every key here lives in i18n/locales/{ru,en}.ts
// under evalCatalog.fields.* — kept in a separate module (not Vue-aware) so components
// derive message keys from this array instead of hardcoding the key list twice.
export const TEST_FIELD_KEYS = [
  'id',
  'source',
  'message',
  'history',
  'requires',
  'escalate',
  'language',
  'must_not_contain',
  'must_contain_any',
  'forbid_tokens',
  'media',
  'outcomes',
  'stock_check',
  'llm_checks',
] as const

export type TestFieldKey = (typeof TEST_FIELD_KEYS)[number]
