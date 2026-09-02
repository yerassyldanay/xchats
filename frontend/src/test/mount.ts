import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { mount, type MountingOptions } from '@vue/test-utils'
import type { Component } from 'vue'
import { i18n } from '@/i18n'

// testPinia creates AND activates a fresh Pinia — call it before touching a
// store (usePlayground(), etc.) in test setup, so the store you seed is the
// same instance the component you mount reads from.
export function testPinia(): Pinia {
  const pinia = createPinia()
  setActivePinia(pinia)
  return pinia
}

// mountKb is the one mounting helper every *.dom.test.ts uses: real Pinia
// (a fresh one by default, or pass one you already seeded via testPinia()),
// the app's real i18n instance — so kb.* strings render exactly as
// production does, not a fake translator — and a RouterLink/RouterView stub,
// since KB components link to /draft or /knowledge-base but a full
// vue-router instance is unnecessary machinery for a component-level mount.
export function mountKb<T extends Component>(
  component: T,
  options: MountingOptions<any> & { pinia?: Pinia } = {}
) {
  const { pinia = testPinia(), global, ...rest } = options
  return mount(component as any, {
    ...rest,
    global: {
      ...global,
      plugins: [pinia, i18n, ...(global?.plugins ?? [])],
      stubs: {
        RouterLink: { template: '<a><slot /></a>' },
        RouterView: true,
        ...(global?.stubs ?? {}),
      },
    },
  })
}
