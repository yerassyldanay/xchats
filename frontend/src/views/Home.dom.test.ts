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
    expect(wrapper.text()).toContain('AI-ассистент, который')
    expect(wrapper.text()).toContain('никогда не выдумывает')
    expect(wrapper.text()).toContain('Начать бесплатно')
  })

  it('switching the language switcher to English re-renders the hero in English', async () => {
    const wrapper = mountKb(Home, { pinia: testPinia() })
    const enBtn = wrapper.findAll('.lang-links__link').find((b) => b.text() === 'English')
    expect(enBtn).toBeTruthy()
    await enBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('The AI assistant that')
    expect(wrapper.text()).toContain('never makes things up')
    expect(wrapper.text()).toContain('Get started free')
  })

  it('switching to Қазақша renders the hero title with no missing space around the highlighted phrase', async () => {
    const wrapper = mountKb(Home, { pinia: testPinia() })
    const kkBtn = wrapper.findAll('.lang-links__link').find((b) => b.text() === 'Қазақша')
    await kkBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    const heading = wrapper.find('.landing-hero__title')
    // Regression guard for the <template v-if> whitespace bug: the
    // highlighted phrase and the trailing suffix must not run together.
    expect(heading.text()).toContain('шығармайтын AI-көмекші')
    expect(heading.text()).not.toContain('шығармайтынAI')
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
