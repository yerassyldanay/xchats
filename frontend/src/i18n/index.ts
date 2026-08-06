import { createI18n } from 'vue-i18n'
import { watch } from 'vue'
import ru from './locales/ru'
import en from './locales/en'

// App-wide i18n foundation — vue-i18n over a one-off toggle because this is
// meant to grow beyond the catalog page. Composition API only (legacy:
// false) — no Options API $t() usage anywhere.
const stored = localStorage.getItem('locale')
const initialLocale = stored === 'en' ? 'en' : 'ru' // garbage-safe: anything else falls back to ru

export const i18n = createI18n({
  legacy: false,
  locale: initialLocale,
  fallbackLocale: 'ru',
  messages: { ru, en },
})

watch(i18n.global.locale, (v) => localStorage.setItem('locale', v))
