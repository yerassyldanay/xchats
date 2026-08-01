// mountKb wraps @vue/test-utils' mount with the real plumbing every KB
// component needs: a Pinia (Pinia's global "active instance" mechanic means
// usePlayground() calls made later in the same test see this SAME store)
// and the app's own i18n instance — kb.* strings render for real, so a
// wrong/missing key shows up as a rendered "kb.foo.bar" instead of silently
// passing. The API layer is mocked per-test via vi.mock('@/api/client');
// the store's own logic stays real and under test.
import { DOMWrapper, mount, type ComponentMountingOptions } from '@vue/test-utils'
import { createPinia, type Pinia } from 'pinia'
import type { Component } from 'vue'
import { i18n } from '@/i18n'

// Renders :to's route name (or a plain string) as href, so a test can assert
// "this link points at knowledge-base/playground" without a real router.
const RouterLinkStub = {
  name: 'RouterLink',
  props: ['to'],
  template: '<a :href="typeof to === \'string\' ? to : to?.name"><slot /></a>',
}

// A test that needs to seed store/modal state BEFORE the component's first
// render (e.g. useKbModal().openEdit(...) so a form's immediate watch sees
// an already-open session) must use the SAME Pinia instance for both the
// pre-mount setup and the mount itself — pass it here rather than letting
// mountKb create its own, disconnected one.
export function mountKb<T extends Component>(component: T, options: ComponentMountingOptions<T> & { pinia?: Pinia } = {}) {
  const { global: userGlobal, pinia = createPinia(), ...rest } = options
  return mount(component, {
    ...rest,
    global: {
      ...userGlobal,
      plugins: [pinia, i18n, ...(userGlobal?.plugins ?? [])],
      stubs: { RouterLink: RouterLinkStub, ...(userGlobal?.stubs ?? {}) },
    },
  } as ComponentMountingOptions<T>)
}

// A Dialog/DropdownMenu/Tooltip Teleports its content to document.body,
// OUTSIDE the mounted wrapper's own root element — wrapper.find() searches
// only the wrapper's own subtree, so it can never see teleported content.
// bodyWrapper() gives a DOMWrapper over document.body itself, with the same
// find()/findAll()/trigger() API, for asserting on/interacting with an open
// Dialog's rendered content.
export function bodyWrapper(): DOMWrapper<HTMLElement> {
  return new DOMWrapper(document.body)
}
