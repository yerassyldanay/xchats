import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import './style.css'

// Theme: default light; honor a persisted 'dark' preference. No UI toggle in v1 —
// flip localStorage 'theme' to 'dark' (or toggle .dark on <html> in devtools) to verify.
if (localStorage.getItem('theme') === 'dark') {
  document.documentElement.classList.add('dark')
}

createApp(App).use(createPinia()).use(router).use(i18n).mount('#app')
