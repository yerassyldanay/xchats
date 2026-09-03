<script setup lang="ts">
// Structurally distinct from the blog's SiteFooter.vue (SSR props-driven,
// no i18n runtime) but built on the same chrome.css .site-footer* classes
// and the SAME real contact details — see SiteFooter.vue, which carries
// the identical WhatsApp/email/Instagram values.
import { useI18n } from 'vue-i18n'
import { Github, Instagram, Linkedin, Mail } from 'lucide-vue-next'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'

const { t } = useI18n()
const year = new Date().getUTCFullYear()
</script>

<template>
  <footer class="site-footer">
    <div class="blog-shell site-footer__grid">
      <div class="site-footer__brand-column">
        <RouterLink :to="{ name: 'home' }" class="site-footer__brand">
          <span class="site-footer__mark" aria-hidden="true">x</span>
          <span>xchats</span>
        </RouterLink>
        <p class="site-footer__description">{{ t('landing.footer.description') }}</p>
        <p class="site-footer__copyright">© {{ year }} xchats. {{ t('landing.footer.copyrightSuffix') }}</p>
      </div>

      <nav :aria-label="t('landing.footer.productHeading')">
        <h2 class="site-footer__heading">{{ t('landing.footer.productHeading') }}</h2>
        <ul class="site-footer__links">
          <li><a href="#platforms">{{ t('landing.nav.features') }}</a></li>
          <li><a href="#showcase">{{ t('landing.nav.tour') }}</a></li>
          <li><a href="#desktop">{{ t('landing.nav.desktop') }}</a></li>
          <li><a href="#architecture">{{ t('landing.nav.architecture') }}</a></li>
          <li>
            <a href="https://github.com/yerassyldanay/xchats/tree/master/docs" target="_blank" rel="noreferrer">{{ t('landing.nav.docs') }}</a>
          </li>
          <li><a href="/blog/">{{ t('landing.nav.blog') }}</a></li>
        </ul>
      </nav>

      <nav :aria-label="t('landing.footer.communityHeading')">
        <h2 class="site-footer__heading">{{ t('landing.footer.communityHeading') }}</h2>
        <ul class="site-footer__links">
          <li>
            <a href="https://github.com/yerassyldanay/xchats" target="_blank" rel="noreferrer">
              <Github aria-hidden="true" />
              <span>{{ t('landing.nav.github') }}</span>
            </a>
          </li>
        </ul>
      </nav>

      <div>
        <h2 class="site-footer__heading">{{ t('landing.footer.contactsHeading') }}</h2>
        <address class="site-footer__contact">
          <a href="https://wa.me/77029766509" target="_blank" rel="noreferrer">
            <WhatsappIcon class="site-footer__contact-icon site-footer__contact-icon--whatsapp" aria-hidden="true" />
            <span>+7 702 976 65 09</span>
          </a>
          <a href="mailto:yerassyl.danay.abc@gmail.com">
            <Mail class="site-footer__contact-icon site-footer__contact-icon--email" aria-hidden="true" />
            <span>yerassyl.danay.abc@gmail.com</span>
          </a>
          <a href="https://www.instagram.com/yerassyl.danay/" target="_blank" rel="noreferrer">
            <Instagram class="site-footer__contact-icon site-footer__contact-icon--instagram" aria-hidden="true" />
            <span>@yerassyl.danay</span>
          </a>
          <a href="https://www.linkedin.com/in/yerassyldanay/" target="_blank" rel="noreferrer">
            <Linkedin class="site-footer__contact-icon site-footer__contact-icon--linkedin" aria-hidden="true" />
            <span>yerassyldanay</span>
          </a>
        </address>
      </div>
    </div>
  </footer>
</template>
