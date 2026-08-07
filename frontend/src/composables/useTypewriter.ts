import { onBeforeUnmount, ref, watch, type Ref } from 'vue'

// useTypewriter reveals `target` character by character once `start` turns
// true — TemplatesSection/McpSection use one instance per line of their
// staged chat demos, chaining stages off each other's `done` flag rather
// than this composable knowing anything about a multi-line sequence.
// prefers-reduced-motion snaps straight to the full string: the demo's END
// STATE is still shown, only the reveal motion is skipped.
export function useTypewriter(target: string, start: Ref<boolean>, speedMs = 26) {
  const text = ref('')
  const done = ref(false)
  const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  let timer: number | undefined

  function run() {
    if (reduceMotion) {
      text.value = target
      done.value = true
      return
    }
    let i = 0
    timer = window.setInterval(() => {
      i++
      text.value = target.slice(0, i)
      if (i >= target.length) {
        window.clearInterval(timer)
        done.value = true
      }
    }, speedMs)
  }

  watch(
    start,
    (v) => {
      if (v && !text.value && !done.value) run()
    },
    { immediate: true }
  )
  onBeforeUnmount(() => window.clearInterval(timer))

  return { text, done }
}
