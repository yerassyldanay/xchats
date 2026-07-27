<script setup lang="ts">
import {
  ArrowRight,
  Database,
  ShieldCheck,
  MessageSquare,
  Github,
  Mail,
  Instagram,
  Check,
  X,
  Code2,
  FolderOpen,
  CheckCircle2,
  Lock,
  Layers,
  Sparkles,
  Bot
} from 'lucide-vue-next'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'

const currentYear = new Date().getFullYear()
</script>

<template>
  <div class="min-h-screen bg-white text-slate-900 font-sans selection:bg-indigo-500/30 selection:text-indigo-900 overflow-y-auto">
    <!-- Navbar -->
    <header class="sticky top-0 z-50 border-b border-slate-200 bg-white/80 backdrop-blur-md">
      <div class="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between">
        <div class="flex items-center gap-2">
          <div class="w-8 h-8 rounded-lg bg-indigo-600 text-white grid place-items-center font-bold text-lg leading-none">
            x
          </div>
          <span class="font-bold text-xl tracking-tight">xchats</span>
        </div>
        <nav class="hidden md:flex items-center gap-8 text-sm font-medium text-slate-600">
          <a href="#features" class="hover:text-indigo-600 transition-colors">Возможности</a>
          <a href="#how-it-works" class="hover:text-indigo-600 transition-colors">Как это работает</a>
          <a href="#architecture" class="hover:text-indigo-600 transition-colors">Архитектура</a>
          <a href="/blog/" class="hover:text-indigo-600 transition-colors">Блог</a>
        </nav>
        <div class="flex items-center gap-4">
          <a href="https://github.com/yerassyldanay/xchats" target="_blank" class="hidden sm:flex text-slate-600 hover:text-slate-900 transition-colors items-center gap-2 text-sm font-medium">
            <Github class="w-5 h-5" /> GitHub
          </a>
          <RouterLink
            :to="{ name: 'login' }"
            class="text-sm font-medium bg-indigo-600 text-white px-5 py-2.5 rounded-lg hover:bg-indigo-700 transition-colors shadow-sm"
          >
            Попробовать
          </RouterLink>
        </div>
      </div>
    </header>

    <main>
      <!-- Hero Section -->
      <section class="bg-slate-50 text-slate-900 pt-20 pb-24 px-6 relative overflow-hidden">
        <div class="absolute inset-0 bg-[radial-gradient(ellipse_at_top_right,_var(--tw-gradient-stops))] from-indigo-100/50 via-slate-50 to-slate-50 -z-10"></div>
        <div class="max-w-7xl mx-auto flex flex-col lg:flex-row items-center gap-12">
          <div class="flex-1 space-y-8 z-10">
            <div class="inline-flex items-center gap-3">
              <div class="flex items-center gap-2 px-3 py-1 rounded-full bg-indigo-100 text-indigo-700 text-xs font-semibold border border-indigo-200">
                <Code2 class="w-3.5 h-3.5" /> OPEN SOURCE
              </div>
              <span class="text-sm text-slate-500 font-medium">100% Open Source • MIT License</span>
            </div>
            
            <h1 class="text-4xl md:text-5xl lg:text-6xl font-bold tracking-tight leading-[1.15]">
              Open-source<br />
              AI-чатбот для<br />
              <span class="text-indigo-600">вашего бизнеса</span>
            </h1>
            
            <p class="text-lg text-slate-600 max-w-xl leading-relaxed">
              Умный чат-бот для общения с клиентами в WhatsApp, Telegram и Instagram. Загружайте документы, аудио и видео — бот извлечет знания и будет отвечать точно, опираясь только на проверенные вами данные.
            </p>
            
            <div class="flex flex-wrap items-center gap-4 pt-2">
              <RouterLink
                :to="{ name: 'login' }"
                class="h-12 px-6 flex items-center justify-center gap-2 rounded-lg bg-indigo-600 text-white font-medium hover:bg-indigo-500 transition-all shadow-lg shadow-indigo-600/20"
              >
                Начать бесплатно
              </RouterLink>
              <a
                href="https://github.com/yerassyldanay/xchats"
                target="_blank"
                class="h-12 px-6 flex items-center justify-center gap-2 rounded-lg bg-white text-slate-700 font-medium hover:bg-slate-50 transition-all border border-slate-200 shadow-sm"
              >
                <Github class="w-5 h-5" /> GitHub
              </a>
            </div>
            
            <div class="flex flex-wrap items-center gap-6 text-sm text-slate-500 pt-4">
              <span class="flex items-center gap-2"><Github class="w-4 h-4" /> Открытый исходный код</span>
              <span class="flex items-center gap-2"><Database class="w-4 h-4" /> Самостоятельный хостинг</span>
              <span class="flex items-center gap-2"><ShieldCheck class="w-4 h-4" /> Прозрачность и контроль</span>
            </div>
          </div>
          
          <div class="flex-1 w-full relative">
            <div class="relative rounded-xl overflow-hidden border border-slate-200 shadow-2xl shadow-slate-200/50 bg-slate-900 aspect-[4/3] lg:aspect-auto lg:h-[500px]">
              <img src="/app-screenshot.png" alt="xchats Interface" class="w-full h-full object-cover object-left-top" />
            </div>
          </div>
        </div>
      </section>

      <!-- Workflow Section -->
      <section id="how-it-works" class="py-24 px-6 bg-slate-50">
        <div class="max-w-7xl mx-auto">
          <div class="text-center mb-16">
            <h2 class="text-3xl font-bold mb-4">Как это работает</h2>
            <p class="text-slate-500">От загрузки документов до умных ответов чат-бота</p>
          </div>
          
          <div class="flex flex-col md:flex-row items-start justify-between gap-4 md:gap-0 relative">
            <!-- Hidden border line for desktop -->
            <div class="hidden md:block absolute top-12 left-12 right-12 h-0.5 bg-slate-200 border-t-2 border-dashed border-slate-300 -z-10"></div>
            
            <div class="flex-1 flex flex-col items-center text-center px-2">
              <div class="w-24 h-24 bg-white rounded-2xl shadow-sm border border-slate-200 flex items-center justify-center mb-6">
                <FolderOpen class="w-10 h-10 text-slate-400" />
              </div>
              <h3 class="text-sm font-bold mb-2">1. Загрузите материалы</h3>
              <p class="text-xs text-slate-500">Обучите бота с помощью PDF, Excel, видео, аудио или обычного текста.</p>
            </div>

            <div class="flex-1 flex flex-col items-center text-center px-2">
              <div class="w-24 h-24 bg-white rounded-2xl shadow-sm border border-slate-200 flex items-center justify-center mb-6">
                <Sparkles class="w-10 h-10 text-indigo-500" />
              </div>
              <h3 class="text-sm font-bold mb-2">2. AI извлекает знания</h3>
              <p class="text-xs text-slate-500">Чат-бот анализирует каждый файл и структурирует данные для ответов.</p>
            </div>

            <div class="flex-1 flex flex-col items-center text-center px-2">
              <div class="w-24 h-24 bg-white rounded-2xl shadow-sm border border-slate-200 flex items-center justify-center mb-6 relative">
                <CheckCircle2 class="w-10 h-10 text-blue-500" />
              </div>
              <h3 class="text-sm font-bold mb-2">3. Проверьте поведение</h3>
              <p class="text-xs text-slate-500">Проверьте черновики бота и утвердите то, как он должен отвечать.</p>
            </div>

            <div class="flex-1 flex flex-col items-center text-center px-2">
              <div class="w-24 h-24 bg-white rounded-2xl shadow-sm border border-slate-200 flex items-center justify-center mb-6">
                <Database class="w-10 h-10 text-indigo-600" />
              </div>
              <h3 class="text-sm font-bold mb-2">4. База знаний</h3>
              <p class="text-xs text-slate-500">Знания бота сохраняются, чтобы он мог опираться на них в диалогах.</p>
            </div>

            <div class="flex-1 flex flex-col items-center text-center px-2">
              <div class="w-24 h-24 bg-white rounded-2xl shadow-sm border border-slate-200 flex items-center justify-center mb-6">
                <MessageSquare class="w-10 h-10 text-indigo-500 fill-indigo-100" />
              </div>
              <h3 class="text-sm font-bold mb-2">5. Чат-бот отвечает</h3>
              <p class="text-xs text-slate-500">Ваш AI-ассистент уверенно отвечает клиентам в WhatsApp и Instagram.</p>
            </div>
          </div>
        </div>
      </section>

      <!-- Key Features Section -->
      <section id="features" class="py-24 px-6">
        <div class="max-w-7xl mx-auto">
          <div class="text-center mb-16">
            <h2 class="text-3xl font-bold">Ключевые возможности чат-бота</h2>
          </div>
          
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            <div class="p-8 rounded-2xl bg-white border border-slate-200 shadow-sm flex flex-col items-center text-center hover:shadow-md transition-shadow">
              <div class="w-12 h-12 bg-indigo-50 text-indigo-600 rounded-xl flex items-center justify-center mb-6">
                <MessageSquare class="w-6 h-6" />
              </div>
              <h3 class="font-bold mb-3">Мульти-канальность</h3>
              <p class="text-sm text-slate-500">Единый бот для ответов в WhatsApp, Telegram, Instagram и других каналах.</p>
            </div>

            <div class="p-8 rounded-2xl bg-white border border-slate-200 shadow-sm flex flex-col items-center text-center hover:shadow-md transition-shadow">
              <div class="w-12 h-12 bg-indigo-50 text-indigo-600 rounded-xl flex items-center justify-center mb-6">
                <FolderOpen class="w-6 h-6" />
              </div>
              <h3 class="font-bold mb-3">Любые форматы</h3>
              <p class="text-sm text-slate-500">Чат-бот понимает PDF, Excel, изображения, видео, аудио и ссылки.</p>
            </div>

            <div class="p-8 rounded-2xl bg-white border border-slate-200 shadow-sm flex flex-col items-center text-center hover:shadow-md transition-shadow">
              <div class="w-12 h-12 bg-indigo-50 text-indigo-600 rounded-xl flex items-center justify-center mb-6">
                <ShieldCheck class="w-6 h-6" />
              </div>
              <h3 class="font-bold mb-3">Человеческое одобрение</h3>
              <p class="text-sm text-slate-500">Бот никогда не скажет лишнего — вы проверяете, чему он научился.</p>
            </div>

            <div class="p-8 rounded-2xl bg-white border border-slate-200 shadow-sm flex flex-col items-center text-center hover:shadow-md transition-shadow">
              <div class="w-12 h-12 bg-indigo-50 text-indigo-600 rounded-xl flex items-center justify-center mb-6">
                <Layers class="w-6 h-6" />
              </div>
              <h3 class="font-bold mb-3">Структурированная БЗ</h3>
              <p class="text-sm text-slate-500">Бот использует точные структурированные данные, а не сырые тексты.</p>
            </div>

            <div class="p-8 rounded-2xl bg-white border border-slate-200 shadow-sm flex flex-col items-center text-center hover:shadow-md transition-shadow">
              <div class="w-12 h-12 bg-indigo-50 text-indigo-600 rounded-xl flex items-center justify-center mb-6">
                <Lock class="w-6 h-6" />
              </div>
              <h3 class="font-bold mb-3">Безопасность и контроль</h3>
              <p class="text-sm text-slate-500">Данные ваших клиентов не уходят третьим лицам. Полный контроль чат-бота.</p>
            </div>

            <div class="p-8 rounded-2xl bg-white border border-slate-200 shadow-sm flex flex-col items-center text-center hover:shadow-md transition-shadow">
              <div class="w-12 h-12 bg-indigo-50 text-indigo-600 rounded-xl flex items-center justify-center mb-6">
                <Code2 class="w-6 h-6" />
              </div>
              <h3 class="font-bold mb-3">Открытый исходный код</h3>
              <p class="text-sm text-slate-500">MIT License. Прозрачный код, сообщество и возможность развивать вместе.</p>
            </div>
          </div>
        </div>
      </section>

      <!-- RAG Comparison Section -->
      <section class="py-24 px-6 bg-slate-50">
        <div class="max-w-6xl mx-auto">
          <div class="text-center mb-16">
            <h2 class="text-3xl font-bold">Почему xchats не использует обычный RAG</h2>
          </div>
          
          <div class="flex flex-col lg:flex-row items-stretch gap-8 relative">
            <div class="flex-1 bg-white p-8 rounded-2xl border border-slate-200 shadow-sm">
              <h3 class="font-bold text-lg mb-6 text-center">Традиционные AI чат-боты</h3>
              <ul class="space-y-4">
                <li class="flex items-start gap-3">
                  <X class="w-5 h-5 text-red-500 shrink-0 mt-0.5" />
                  <span class="text-sm text-slate-700">Ищут информацию в документах во время ответа</span>
                </li>
                <li class="flex items-start gap-3">
                  <X class="w-5 h-5 text-red-500 shrink-0 mt-0.5" />
                  <span class="text-sm text-slate-700">Могут выдавать неточные или выдуманные факты</span>
                </li>
                <li class="flex items-start gap-3">
                  <X class="w-5 h-5 text-red-500 shrink-0 mt-0.5" />
                  <span class="text-sm text-slate-700">Работают с сырыми документами и текстом</span>
                </li>
                <li class="flex items-start gap-3">
                  <X class="w-5 h-5 text-red-500 shrink-0 mt-0.5" />
                  <span class="text-sm text-slate-700">Сложно отслеживать источники и изменения</span>
                </li>
              </ul>
            </div>
            
            <div class="hidden lg:flex items-center justify-center font-bold text-slate-400 text-xl px-4">
              VS
            </div>

            <div class="flex-1 bg-white p-8 rounded-2xl border border-indigo-100 shadow-md ring-1 ring-indigo-500/10">
              <h3 class="font-bold text-lg mb-6 text-center">xchats</h3>
              <ul class="space-y-4">
                <li class="flex items-start gap-3">
                  <Check class="w-5 h-5 text-emerald-500 shrink-0 mt-0.5" />
                  <span class="text-sm text-slate-700">Бот использует только утвержденные структурированные знания</span>
                </li>
                <li class="flex items-start gap-3">
                  <Check class="w-5 h-5 text-emerald-500 shrink-0 mt-0.5" />
                  <span class="text-sm text-slate-700">Точные значения подставляются из базы, без галлюцинаций</span>
                </li>
                <li class="flex items-start gap-3">
                  <Check class="w-5 h-5 text-emerald-500 shrink-0 mt-0.5" />
                  <span class="text-sm text-slate-700">Структурированные данные в понятной, контролируемой форме</span>
                </li>
                <li class="flex items-start gap-3">
                  <Check class="w-5 h-5 text-emerald-500 shrink-0 mt-0.5" />
                  <span class="text-sm text-slate-700">Каждое изменение в поведении бота проверяется</span>
                </li>
              </ul>
            </div>
          </div>
        </div>
      </section>

      <!-- Architecture Section -->
      <section id="architecture" class="py-24 px-6">
        <div class="max-w-7xl mx-auto">
          <div class="text-center mb-16">
            <h2 class="text-3xl font-bold mb-12">Архитектура чат-бота</h2>
            
            <!-- Simplified Architecture Flow -->
            <div class="flex flex-col lg:flex-row items-center justify-center gap-4 lg:gap-2 overflow-x-auto pb-8">
              <div class="flex flex-col items-center justify-center p-4 border border-slate-200 rounded-xl min-w-[140px]">
                <FolderOpen class="w-8 h-8 text-indigo-500 mb-2" />
                <span class="text-sm font-bold">Обучение</span>
                <span class="text-[10px] text-slate-500 mt-1">PDF, Excel, Фото,<br/>Видео, Аудио, URL</span>
              </div>
              <ArrowRight class="text-slate-300 w-5 h-5 rotate-90 lg:rotate-0" />
              
              <div class="flex flex-col items-center justify-center p-4 border border-slate-200 rounded-xl min-w-[140px]">
                <Sparkles class="w-8 h-8 text-purple-500 mb-2" />
                <span class="text-sm font-bold">AI обработка</span>
                <span class="text-[10px] text-slate-500 mt-1">Извлечение и<br/>структурирование</span>
              </div>
              <ArrowRight class="text-slate-300 w-5 h-5 rotate-90 lg:rotate-0" />

              <div class="flex flex-col items-center justify-center p-4 border border-slate-200 rounded-xl min-w-[140px]">
                <ShieldCheck class="w-8 h-8 text-emerald-500 mb-2" />
                <span class="text-sm font-bold">Контроль</span>
                <span class="text-[10px] text-slate-500 mt-1">Проверка ответов<br/>человеком</span>
              </div>
              <ArrowRight class="text-slate-300 w-5 h-5 rotate-90 lg:rotate-0" />

              <div class="flex flex-col items-center justify-center p-4 border border-slate-200 rounded-xl min-w-[140px]">
                <Bot class="w-8 h-8 text-indigo-500 mb-2" />
                <span class="text-sm font-bold">Чат-бот</span>
                <span class="text-[10px] text-slate-500 mt-1">Отвечает клиентам</span>
              </div>
              <ArrowRight class="text-slate-300 w-5 h-5 rotate-90 lg:rotate-0" />

              <div class="flex flex-col items-center justify-center p-4 border border-slate-200 rounded-xl min-w-[140px]">
                <span class="text-sm font-bold mb-3">Каналы</span>
                <div class="flex gap-2 text-slate-400">
                  <WhatsappIcon class="w-5 h-5 text-emerald-500" />
                  <MessageSquare class="w-5 h-5 text-blue-500" />
                  <Instagram class="w-5 h-5 text-pink-500" />
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- Engineering & Evals Section -->
      <section class="py-24 px-6">
        <div class="max-w-6xl mx-auto">
          <div class="text-center mb-16">
            <h2 class="text-3xl font-bold">Открытая разработка и тестирование</h2>
            <p class="text-slate-500 mt-4">Мы строим надежный AI-продукт и открыто делимся нашими процессами.</p>
          </div>
          
          <div class="grid grid-cols-1 md:grid-cols-2 gap-8">
            <div class="bg-white p-10 rounded-3xl border border-slate-200 shadow-sm flex flex-col items-start hover:shadow-md transition-shadow">
              <div class="w-12 h-12 bg-indigo-50 text-indigo-600 rounded-xl flex items-center justify-center mb-6">
                <FileText class="w-6 h-6" />
              </div>
              <h3 class="text-xl font-bold mb-3">Инженерный блог</h3>
              <p class="text-slate-600 mb-8 flex-1">
                Читайте о том, как мы решаем сложные задачи, проектируем архитектуру и создаем надежные AI-инструменты без галлюцинаций.
              </p>
              <a href="/blog/" class="inline-flex items-center gap-2 text-indigo-600 font-bold hover:text-indigo-700 transition-colors">
                Читать статьи <ArrowRight class="w-4 h-4" />
              </a>
            </div>

            <div class="bg-white p-10 rounded-3xl border border-slate-200 shadow-sm flex flex-col items-start hover:shadow-md transition-shadow">
              <div class="w-12 h-12 bg-indigo-50 text-indigo-600 rounded-xl flex items-center justify-center mb-6">
                <CheckCircle2 class="w-6 h-6" />
              </div>
              <h3 class="text-xl font-bold mb-3">Открытые эвалы (Evals)</h3>
              <p class="text-slate-600 mb-8 flex-1">
                Мы регулярно тестируем нашего бота на реальных сценариях. Посмотрите метрики качества и убедитесь в надежности ответов.
              </p>
              <RouterLink :to="{ name: 'evals' }" class="inline-flex items-center gap-2 text-indigo-600 font-bold hover:text-indigo-700 transition-colors">
                Смотреть результаты <ArrowRight class="w-4 h-4" />
              </RouterLink>
            </div>
          </div>
        </div>
      </section>


      <!-- CTA Section -->
      <section class="py-20 px-6">
        <div class="max-w-5xl mx-auto bg-slate-950 rounded-3xl p-10 md:p-16 flex flex-col md:flex-row items-center justify-between gap-10 text-white shadow-2xl relative overflow-hidden">
          <div class="absolute inset-0 bg-[radial-gradient(ellipse_at_bottom_left,_var(--tw-gradient-stops))] from-indigo-900/40 via-transparent to-transparent -z-10"></div>
          <div class="flex-1 text-center md:text-left space-y-4">
            <h2 class="text-3xl font-bold">Готовы попробовать?</h2>
            <p class="text-slate-300">Запустите xchats за несколько минут и создайте своего умного чат-бота для общения с клиентами.</p>
          </div>
          <div class="flex flex-col sm:flex-row items-center gap-4 shrink-0">
            <RouterLink
              :to="{ name: 'login' }"
              class="h-12 px-6 flex items-center justify-center gap-2 rounded-lg bg-indigo-600 text-white font-medium hover:bg-indigo-500 transition-all shadow-lg w-full sm:w-auto"
            >
              Начать бесплатно
            </RouterLink>
            <div class="flex gap-4 w-full sm:w-auto">
              <a
                href="https://github.com/yerassyldanay/xchats"
                target="_blank"
                class="h-12 px-5 flex-1 flex items-center justify-center gap-2 rounded-lg bg-white/5 text-white font-medium hover:bg-white/10 transition-all border border-white/10"
              >
                <Github class="w-5 h-5" /> GitHub
              </a>
            </div>
          </div>
        </div>
      </section>
    </main>

    <!-- Footer -->
    <footer class="border-t border-slate-200 bg-white py-16 px-6">
      <div class="max-w-7xl mx-auto grid grid-cols-1 md:grid-cols-5 gap-10">
        <div class="md:col-span-2 space-y-6">
          <div class="flex items-center gap-2">
            <div class="w-6 h-6 rounded-md bg-indigo-600 text-white grid place-items-center font-bold text-sm leading-none">
              x
            </div>
            <span class="font-bold text-lg tracking-tight">xchats</span>
          </div>
          <p class="text-sm text-slate-500 max-w-sm">
            Open-source платформа для создания умных чат-ботов на основе ваших данных.
          </p>
          <p class="text-xs text-slate-400 pt-4">
            © {{ currentYear }} xchats. MIT License.
          </p>
        </div>
        
        <div>
          <h4 class="font-bold mb-4 text-sm text-slate-900">Продукт</h4>
          <ul class="space-y-3 text-sm text-slate-500">
            <li><a href="#features" class="hover:text-indigo-600 transition-colors">Возможности</a></li>
            <li><a href="#how-it-works" class="hover:text-indigo-600 transition-colors">Как это работает</a></li>
            <li><a href="#architecture" class="hover:text-indigo-600 transition-colors">Архитектура</a></li>
            <li><a href="/blog/" class="hover:text-indigo-600 transition-colors">Блог</a></li>
          </ul>
        </div>
        
        <div>
          <h4 class="font-bold mb-4 text-sm text-slate-900">Сообщество</h4>
          <ul class="space-y-3 text-sm text-slate-500">
            <li><a href="https://github.com/yerassyldanay/xchats" class="hover:text-indigo-600 transition-colors">GitHub</a></li>
          </ul>
        </div>
        
        <div class="space-y-6">
          <div>
            <h4 class="font-bold mb-4 text-sm text-slate-900">Свяжитесь с нами</h4>
            <div class="space-y-4">
              <a href="https://wa.me/77029766509" target="_blank" class="flex items-center gap-3 text-sm text-slate-600 hover:text-indigo-600 transition-colors">
                <WhatsappIcon class="w-5 h-5 text-emerald-500" />
                <span class="font-medium">+7 702 976 65 09</span>
              </a>
              <a href="mailto:yerassyl.danay.abc@gmail.com" class="flex items-center gap-3 text-sm text-slate-600 hover:text-indigo-600 transition-colors">
                <Mail class="w-5 h-5 text-blue-500" />
                <span class="font-medium">yerassyl.danay.abc@gmail.com</span>
              </a>
              <a href="https://www.instagram.com/xpayment.kz/" target="_blank" class="flex items-center gap-3 text-sm text-slate-600 hover:text-indigo-600 transition-colors">
                <Instagram class="w-5 h-5 text-pink-500" />
                <span class="font-medium">@xpayment.kz</span>
              </a>
            </div>
          </div>
        </div>
        
      </div>
    </footer>
  </div>
</template>
