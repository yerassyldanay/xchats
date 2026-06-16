import type { Directive } from 'vue'

// v-autosize grows a <textarea> to fit ALL of its content — on first render, on
// typing, and when the value is set programmatically (a draft loads, or text is
// moved into the composer). No fixed row count, no inner scrollbar. A `max-height`
// utility class, if present, still caps it (the composer keeps a sane ceiling).
function resize(el: HTMLTextAreaElement) {
  el.style.height = 'auto'
  el.style.height = `${el.scrollHeight}px`
}

export const vAutosize: Directive<HTMLTextAreaElement> = {
  mounted(el) {
    resize(el)
    el.addEventListener('input', () => resize(el))
  },
  updated(el) {
    resize(el)
  },
}
