// Build-time feature flags. Each is read through a function (not a module
// constant) so a test can flip it with vi.stubEnv without re-importing.

// The official Meta Graph API channels — WhatsApp Cloud API, Instagram Direct
// and Messenger — are switched off in the UI until they are ready to ship.
// Only the UI is gated: the backend (internal/meta, the OAuth routes), the
// stores and the API client are untouched, so setting
// VITE_ENABLE_META_CHANNELS=true brings the whole feature back.
export const META_CHANNELS = ['whatsapp_cloud', 'instagram', 'messenger'] as const

export function metaChannelsEnabled(): boolean {
  return import.meta.env.VITE_ENABLE_META_CHANNELS === 'true'
}

export function isMetaChannel(channel: string): boolean {
  return (META_CHANNELS as readonly string[]).includes(channel)
}
