import { ref, type Ref } from 'vue'
import { useIntersectionObserver } from '@vueuse/core'

// useRevealOnScroll ties a fade/slide-up reveal to whether `target` has
// entered the viewport once — the landing page's SectionShell and showcase
// panels toggle their `.landing-reveal.is-visible` class off this. Stops
// observing after the first hit (a reveal never re-hides on scroll back
// up). prefers-reduced-motion short-circuits straight to visible=true:
// motion is skipped, content is never hidden waiting for it.
export function useRevealOnScroll(target: Ref<HTMLElement | null | undefined>, threshold = 0.25) {
  const visible = ref(window.matchMedia('(prefers-reduced-motion: reduce)').matches)

  if (!visible.value) {
    const { stop } = useIntersectionObserver(
      target,
      ([entry]) => {
        if (entry?.isIntersecting) {
          visible.value = true
          stop()
        }
      },
      { threshold }
    )
  }

  return { visible }
}
