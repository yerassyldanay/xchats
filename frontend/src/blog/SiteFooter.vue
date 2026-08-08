<script setup lang="ts">
import { Github, Instagram, Linkedin, Mail } from 'lucide-vue-next'
import { computed } from 'vue'
import WhatsappIcon from '../components/icons/WhatsappIcon.vue'

const props = defineProps<{
  siteTitle: string
  indexHref: string
  currentLocale: string
  year: number
  githubUrl: string
  githubLabel: string
  copyrightSuffix: string
}>()

const copy = computed(() => {
  if (props.currentLocale === 'en') {
    return {
      description: 'Open-source platform for building intelligent chatbots grounded in your own data.',
      product: 'Product',
      features: 'Features',
      howItWorks: 'How it works',
      architecture: 'Architecture',
      blog: 'Blog',
      community: 'Community',
      contact: 'Contact us',
    }
  }
  if (props.currentLocale === 'kk') {
    return {
      description: 'Өз деректеріңізге негізделген ақылды чат-боттарды жасауға арналған ашық платформа.',
      product: 'Өнім',
      features: 'Мүмкіндіктер',
      howItWorks: 'Қалай жұмыс істейді',
      architecture: 'Архитектура',
      blog: 'Блог',
      community: 'Қауымдастық',
      contact: 'Бізбен байланысыңыз',
    }
  }
  return {
    description: 'Open-source платформа для создания умных чат-ботов на основе ваших данных.',
    product: 'Продукт',
    features: 'Возможности',
    howItWorks: 'Как это работает',
    architecture: 'Архитектура',
    blog: 'Блог',
    community: 'Сообщество',
    contact: 'Свяжитесь с нами',
  }
})
</script>

<template>
  <footer class="site-footer">
    <div class="blog-shell site-footer__grid">
      <div class="site-footer__brand-column">
        <a href="/" class="site-footer__brand">
          <span class="site-footer__mark" aria-hidden="true">x</span>
          <span>{{ siteTitle }}</span>
        </a>
        <p class="site-footer__description">{{ copy.description }}</p>
        <p class="site-footer__copyright">© {{ year }} {{ siteTitle }}. {{ copyrightSuffix }}</p>
      </div>

      <nav :aria-label="copy.product">
        <h2 class="site-footer__heading">{{ copy.product }}</h2>
        <ul class="site-footer__links">
          <li><a href="/#features">{{ copy.features }}</a></li>
          <li><a href="/#how-it-works">{{ copy.howItWorks }}</a></li>
          <li><a href="/#architecture">{{ copy.architecture }}</a></li>
          <li><a :href="indexHref" aria-current="page">{{ copy.blog }}</a></li>
        </ul>
      </nav>

      <nav :aria-label="copy.community">
        <h2 class="site-footer__heading">{{ copy.community }}</h2>
        <ul class="site-footer__links">
          <li>
            <a :href="githubUrl" target="_blank" rel="noreferrer">
              <Github aria-hidden="true" />
              <span>{{ githubLabel }}</span>
            </a>
          </li>
        </ul>
      </nav>

      <div>
        <h2 class="site-footer__heading">{{ copy.contact }}</h2>
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
