import { afterEach, describe, expect, it } from 'vitest'
import { mountKb, testPinia } from '@/test/mount'
import { i18n } from '@/i18n'
import Home from './Home.vue'

// i18n's locale is the real global singleton (mountKb wires up the app's
// actual i18n instance, not a fresh one per test) — reset it after any test
// that changes it, or the switch leaks into later tests in this file.
afterEach(() => {
  i18n.global.locale.value = 'ru'
})

describe('Home (landing page)', () => {
  it('renders the positioning headline and CTAs in Russian by default', () => {
    const wrapper = mountKb(Home, { pinia: testPinia() })
    expect(wrapper.text()).toContain('омниканальный инбокс с')
    expect(wrapper.text()).toContain('AI без галлюцинаций')
    expect(wrapper.text()).toContain('Начать бесплатно')
  })

  it('switching the language dropdown to English re-renders the hero in English', async () => {
    const wrapper = mountKb(Home, { pinia: testPinia() })
    const select = wrapper.find('.landing-lang-select select')
    expect(select.exists()).toBe(true)
    await select.setValue('en')

    expect(wrapper.text()).toContain('The self-hosted omnichannel inbox with')
    expect(wrapper.text()).toContain('zero-hallucination AI')
    expect(wrapper.text()).toContain('Get started free')
  })

  it('switching to Қазақша renders the hero title with no missing space around the highlighted phrase', async () => {
    const wrapper = mountKb(Home, { pinia: testPinia() })
    const select = wrapper.find('.landing-lang-select select')
    await select.setValue('kk')

    const heading = wrapper.find('.landing-hero__title')
    // Regression guard for the <template v-if> whitespace bug: the plain
    // prefix and the highlighted phrase must not run together.
    expect(heading.text()).toContain('AI-мен self-hosted')
    expect(heading.text()).not.toContain('AI-менself-hosted')
  })

  it('footer states the real license (AGPL-3.0), not the old MIT text', () => {
    const wrapper = mountKb(Home, { pinia: testPinia() })
    expect(wrapper.text()).toContain('AGPL-3.0 License.')
    expect(wrapper.text()).not.toContain('MIT License')
  })

  it('the evals link points at the public GitHub tree, never an auth-guarded internal route', () => {
    const wrapper = mountKb(Home, { pinia: testPinia() })
    const evalsLink = wrapper.findAll('a').find((a) => a.text().includes('Смотреть на GitHub'))
    expect(evalsLink).toBeTruthy()
    expect(evalsLink!.attributes('href')).toBe('https://github.com/yerassyldanay/xchats/tree/master/evals')
  })

  it('the blog link points at the prerendered static path', () => {
    const wrapper = mountKb(Home, { pinia: testPinia() })
    const blogLinks = wrapper.findAll('a[href="/blog/"]')
    expect(blogLinks.length).toBeGreaterThan(0)
  })

  it('the WhatsApp footnote carries the unofficial-connectivity warning', () => {
    const wrapper = mountKb(Home, { pinia: testPinia() })
    expect(wrapper.text()).toContain('WhatsApp-подключение неофициальное')
  })
})
