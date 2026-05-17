<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings from appStore
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const loginPath = computed(() => authStore.isAuthenticated ? dashboardPath.value : '/login')

// Intersection Observer for reveal animations
let observer: IntersectionObserver | null = null

const setupRevealAnimations = () => {
  observer = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          entry.target.classList.add('is-visible')
        }
      })
    },
    {
      threshold: 0.1,
      rootMargin: '0px 0px -50px 0px'
    }
  )

  document.querySelectorAll('.reveal-section, .reveal-item').forEach((el) => {
    observer?.observe(el)
  })
}

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

// Features data
const features = computed(() => [
  {
    icon: 'globe',
    title: t('home.homeNew.features.noProxy.title'),
    description: t('home.homeNew.features.noProxy.description'),
    tags: [t('home.homeNew.features.noProxy.tag1'), t('home.homeNew.features.noProxy.tag2')],
    highlight: false,
    iconBg: 'bg-primary-100 text-primary-600 dark:bg-primary-500/15 dark:text-primary-300'
  },
  {
    icon: 'bolt',
    title: t('home.homeNew.features.speed.title'),
    description: t('home.homeNew.features.speed.description'),
    tags: [t('home.homeNew.features.speed.tag1'), t('home.homeNew.features.speed.tag2')],
    highlight: true,
    iconBg: 'bg-primary-100 text-primary-600 dark:bg-primary-500/15 dark:text-primary-300'
  },
  {
    icon: 'shield',
    title: t('home.homeNew.features.stable.title'),
    description: t('home.homeNew.features.stable.description'),
    tags: [t('home.homeNew.features.stable.tag1'), t('home.homeNew.features.stable.tag2')],
    highlight: false,
    iconBg: 'bg-lime-100 text-lime-700 dark:bg-lime-500/15 dark:text-lime-300'
  },
  {
    icon: 'lock',
    title: t('home.homeNew.features.security.title'),
    description: t('home.homeNew.features.security.description'),
    tags: [t('home.homeNew.features.security.tag1'), t('home.homeNew.features.security.tag2')],
    highlight: false,
    iconBg: 'bg-primary-100 text-primary-600 dark:bg-primary-500/15 dark:text-primary-400'
  }
])

// Pricing plans (placeholder - should be fetched from backend)
const pricingPlans = computed(() => [
  {
    name: t('home.homeNew.pricing.basic.name'),
    price: t('home.homeNew.pricing.basic.price'),
    period: t('home.homeNew.pricing.basic.period'),
    badge: t('home.homeNew.pricing.basic.badge'),
    badgeClass: 'bg-primary-100 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300',
    description: t('home.homeNew.pricing.basic.description'),
    limits: [
      { label: t('home.homeNew.pricing.basic.limit1Label'), value: t('home.homeNew.pricing.basic.limit1Value') },
      { label: t('home.homeNew.pricing.basic.limit2Label'), value: t('home.homeNew.pricing.basic.limit2Value') }
    ],
    features: [t('home.homeNew.pricing.basic.feature1'), t('home.homeNew.pricing.basic.feature2')],
    highlighted: true
  },
  {
    name: t('home.homeNew.pricing.pro.name'),
    price: t('home.homeNew.pricing.pro.price'),
    period: t('home.homeNew.pricing.pro.period'),
    badge: t('home.homeNew.pricing.pro.badge'),
    badgeClass: 'bg-teal-100 text-teal-700 dark:bg-teal-500/10 dark:text-teal-300',
    description: t('home.homeNew.pricing.pro.description'),
    limits: [
      { label: t('home.homeNew.pricing.pro.limit1Label'), value: t('home.homeNew.pricing.pro.limit1Value') },
      { label: t('home.homeNew.pricing.pro.limit2Label'), value: t('home.homeNew.pricing.pro.limit2Value') }
    ],
    features: [t('home.homeNew.pricing.pro.feature1'), t('home.homeNew.pricing.pro.feature2')],
    highlighted: false
  }
])

// Steps data
const steps = computed(() => [
  {
    step: '01',
    title: t('home.homeNew.steps.register.title'),
    description: t('home.homeNew.steps.register.description'),
    points: [t('home.homeNew.steps.register.point1'), t('home.homeNew.steps.register.point2')],
    reverse: false,
    demoType: 'register'
  },
  {
    step: '02',
    title: t('home.homeNew.steps.apikey.title'),
    description: t('home.homeNew.steps.apikey.description'),
    points: [t('home.homeNew.steps.apikey.point1'), t('home.homeNew.steps.apikey.point2'), t('home.homeNew.steps.apikey.point3')],
    reverse: true,
    demoType: 'apikey'
  },
  {
    step: '03',
    title: t('home.homeNew.steps.start.title'),
    description: t('home.homeNew.steps.start.description'),
    points: [t('home.homeNew.steps.start.point1'), t('home.homeNew.steps.start.point2')],
    reverse: false,
    demoType: 'terminal'
  }
])

// Stats data
const stats = computed(() => [
  {
    icon: 'bolt',
    value: '99.9%',
    title: t('home.homeNew.stats.stability.title'),
    description: t('home.homeNew.stats.stability.description'),
    progress: 100,
    progressColor: 'bg-lime-400',
    iconBg: 'bg-lime-500/15 text-lime-700 dark:text-lime-300',
    tags: [t('home.homeNew.stats.stability.tag1'), t('home.homeNew.stats.stability.tag2')]
  },
  {
    icon: 'clock',
    value: '<50ms',
    title: t('home.homeNew.stats.latency.title'),
    description: t('home.homeNew.stats.latency.description'),
    progress: 30,
    progressColor: 'bg-primary-500',
    iconBg: 'bg-primary-500/15 text-primary-600 dark:text-primary-300',
    tags: [t('home.homeNew.stats.latency.tag1'), t('home.homeNew.stats.latency.tag2')]
  },
  {
    icon: 'code',
    value: '10min',
    title: t('home.homeNew.stats.onboard.title'),
    description: t('home.homeNew.stats.onboard.description'),
    progress: 15,
    progressColor: 'bg-sky-500',
    iconBg: 'bg-sky-500/15 text-sky-600 dark:text-sky-300',
    tags: [t('home.homeNew.stats.onboard.tag1'), t('home.homeNew.stats.onboard.tag2')]
  }
])

onMounted(() => {
  initTheme()
  authStore.checkAuth()

  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }

  setTimeout(() => {
    setupRevealAnimations()
  }, 100)
})

onUnmounted(() => {
  if (observer) {
    observer.disconnect()
  }
})
</script>

<template>
  <div class="relative min-h-screen overflow-hidden bg-gradient-to-br from-primary-50/30 to-gray-100 text-slate-900 dark:from-dark-950 dark:via-dark-900 dark:to-dark-950 dark:text-primary-50">
    <!-- Background decorations -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div class="absolute left-[-10rem] top-[3rem] h-[24rem] w-[24rem] rounded-full bg-primary-200/45 blur-3xl dark:bg-primary-500/16"></div>
      <div class="absolute right-[-6rem] top-[7rem] h-[18rem] w-[18rem] rounded-full bg-lime-100/55 blur-3xl dark:bg-lime-300/10"></div>
      <div class="absolute bottom-[12%] left-[35%] h-[16rem] w-[16rem] rounded-full bg-teal-200/25 blur-3xl dark:bg-teal-300/10"></div>
      <div class="absolute inset-0 bg-[linear-gradient(rgba(20,184,166,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(20,184,166,0.03)_1px,transparent_1px)] bg-[size:48px_48px]"></div>
      <div class="absolute inset-0 bg-[radial-gradient(circle_at_top,rgba(255,255,255,0.45),transparent_45%)] dark:bg-[radial-gradient(circle_at_top,rgba(255,255,255,0.04),transparent_45%)]"></div>
    </div>

    <!-- Header -->
    <header class="sticky top-0 z-30 transition-all duration-300 bg-transparent">
      <nav class="mx-auto flex h-16 w-full max-w-6xl items-center justify-between px-6">
        <div class="flex items-center gap-3">
          <div v-if="siteLogo" class="h-10 w-10 overflow-hidden rounded-xl shadow-md">
            <img :src="siteLogo" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <div v-else class="leading-none">
            <div class="text-[1.35rem] font-black tracking-[-0.08em]">
              <span class="text-primary-600 dark:text-primary-300">{{ siteName.slice(0, 2) }}</span>
              <span>{{ siteName.slice(2) }}</span>
            </div>
            <div class="mt-1 text-[11px] uppercase tracking-[0.24em] text-stone-500 dark:text-stone-400">
              AI API Gateway
            </div>
          </div>
        </div>

        <div class="flex items-center gap-2 sm:gap-3">
          <!-- Language selector -->
          <LocaleSwitcher />

          <!-- Doc Link -->
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-stone-200/70 bg-white/70 text-stone-600 transition hover:border-stone-300 hover:text-stone-900 hover:translate-y-[-2px] dark:border-stone-800 dark:bg-stone-900/70 dark:text-stone-300 dark:hover:border-stone-700 dark:hover:text-white"
            :title="t('home.viewDocs')"
          >
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.61 0 4.99-.896 7.042-2.429A8.967 8.967 0 0118 21.75c1.052 0 2.062-.18 3-.512V6.042A8.967 8.967 0 0018 3.75c-1.052 0-2.062.18-3 .512"></path>
            </svg>
          </a>

          <!-- Dark mode toggle -->
          <button
            @click="toggleTheme"
            class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-stone-200/70 bg-white/70 text-stone-600 transition hover:border-stone-300 hover:text-stone-900 hover:translate-y-[-2px] dark:border-stone-800 dark:bg-stone-900/70 dark:text-stone-300 dark:hover:border-stone-700 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <svg v-if="isDark" class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 3v2.25m6.364.386l-1.591 1.591M21 12h-2.25m-.386 6.364l-1.591-1.591M12 18.75V21m-4.773-4.227l-1.591 1.591M5.25 12H3m.386-6.364l1.591 1.591M12 12l-1.5-1.5m0 3l1.5-1.5"></path>
            </svg>
            <svg v-else class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
              <path stroke-linecap="round" stroke-linejoin="round" d="M21.752 15.002A9.718 9.718 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z"></path>
            </svg>
          </button>

          <!-- Login/Dashboard button -->
          <router-link
            :to="loginPath"
            class="inline-flex items-center rounded-full bg-primary-950 px-4 py-2 text-sm font-medium text-white transition hover:bg-primary-800 hover:translate-y-[-2px] dark:bg-primary-300 dark:text-primary-950 dark:hover:bg-primary-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="relative z-10">
      <!-- Hero Section -->
      <section class="mx-auto grid max-w-6xl gap-14 px-6 pb-10 pt-12 lg:grid-cols-[1.02fr_0.98fr] lg:items-center lg:pt-20">
        <div class="reveal-section max-w-2xl">
          <div class="animate-hero-wave reveal-item inline-flex items-center gap-2 rounded-full border border-primary-200/80 bg-white/80 px-4 py-2 text-sm font-semibold text-primary-700 shadow-sm dark:border-primary-500/20 dark:bg-slate-950 dark:text-primary-300">
            <span class="h-2 w-2 rounded-full bg-primary-500"></span>
            {{ t('home.homeNew.hero.chip') }}
          </div>

          <h1 class="mt-6 max-w-3xl text-[clamp(2.4rem,6vw,4.85rem)] font-black leading-[0.92] tracking-[-0.065em] text-stone-950 dark:text-stone-50">
            <span class="reveal-item block text-stone-500 dark:text-stone-400">{{ t('home.homeNew.hero.title1') }}</span>
            <span class="animate-hero-wave-text mt-2 inline-block bg-gradient-to-r from-primary-600 via-lime-400 to-teal-300 bg-clip-text text-transparent dark:from-primary-200 dark:via-lime-200 dark:to-primary-500 reveal-item">{{ t('home.homeNew.hero.title2') }}</span>
          </h1>

          <div class="mt-6 max-w-xl space-y-4">
            <p class="reveal-item text-base leading-8 text-stone-600 dark:text-stone-300 md:text-lg">
              {{ siteSubtitle }}
            </p>
            <p class="reveal-item max-w-lg text-sm font-semibold uppercase tracking-[0.18em] text-stone-700 dark:text-stone-200 md:text-[0.95rem]">
              {{ t('home.homeNew.hero.subtitle') }}
            </p>
          </div>

          <div class="reveal-item mt-8 flex flex-wrap items-center gap-4">
            <router-link
              :to="loginPath"
              class="inline-flex items-center gap-2 rounded-full bg-primary-400 px-6 py-3 text-sm font-semibold text-primary-950 shadow-[0_18px_45px_rgba(52,211,153,0.28)] transition hover:bg-primary-300 hover:translate-y-[-2px]"
            >
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.homeNew.hero.cta') }}
              <svg class="h-4 w-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.9">
                <path stroke-linecap="round" stroke-linejoin="round" d="M4 10h12M12 6l4 4-4 4"></path>
              </svg>
            </router-link>
          </div>
        </div>

        <!-- Terminal Demo -->
        <div class="reveal-section">
          <div class="reveal-item relative mx-auto max-w-[34rem]">
            <div class="overflow-hidden rounded-[2rem] border border-stone-200/80 bg-[#18120f] shadow-[0_30px_90px_rgba(28,25,23,0.28)] dark:border-stone-800">
              <div class="flex items-center gap-2 border-b border-white/5 bg-white/5 px-5 py-4">
                <span class="h-3 w-3 rounded-full bg-[#ff5f56]"></span>
                <span class="h-3 w-3 rounded-full bg-[#ffbd2e]"></span>
                <span class="h-3 w-3 rounded-full bg-[#27c93f]"></span>
                <span class="ml-3 text-xs uppercase tracking-[0.24em] text-stone-400">api</span>
              </div>
              <div class="space-y-4 px-5 py-6 font-mono text-sm text-stone-200">
                <div class="text-stone-500">
                  <span class="mr-2 text-primary-300">$</span> curl -X POST <span class="text-stone-100">{{ siteName.toLowerCase() }}/v1/chat/completions</span>
                </div>
                <div class="text-stone-500">
                  <span class="mr-2 text-primary-300">$</span> <span class="text-stone-400"># {{ t('home.homeNew.terminal.routing') }}</span>
                </div>
                <div class="rounded-2xl border border-white/5 bg-white/5 px-4 py-3 text-stone-300">
                  <div class="text-primary-300">200 OK</div>
                  <div class="mt-1 text-stone-400">{{ t('home.homeNew.terminal.response') }}</div>
                </div>
                <div>
                  <span class="mr-2 text-primary-300">$</span>
                  <span class="animate-cursor-blink inline-block h-5 w-2 rounded-sm bg-primary-300/80 align-middle"></span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- Features Section -->
      <section class="reveal-section mx-auto max-w-6xl px-6 py-10">
        <div class="reveal-item text-center">
          <div class="text-xs font-semibold uppercase tracking-[0.3em] text-stone-500 dark:text-stone-400">{{ t('home.homeNew.features.sectionTitle') }}</div>
          <h2 class="mt-4 text-3xl font-black tracking-[-0.05em] md:text-4xl">
            {{ t('home.homeNew.features.sectionHeading') }}<span class="text-primary-600 dark:text-primary-300">{{ t('home.homeNew.features.sectionHighlight') }}</span>？
          </h2>
        </div>

        <div class="mt-10 grid gap-5 md:grid-cols-2 lg:grid-cols-4">
          <article
            v-for="(feature, index) in features"
            :key="feature.title"
            class="reveal-item relative overflow-hidden rounded-[1.75rem] border p-6 shadow-[0_18px_60px_rgba(28,25,23,0.08)] backdrop-blur-sm transition hover:translate-y-[-4px]"
            :class="[
              feature.highlight
                ? 'border-primary-300/60 bg-white/78 dark:border-primary-500/30 dark:bg-slate-950 shadow-[0_18px_60px_rgba(52,211,153,0.12)]'
                : 'border-stone-200/80 bg-white/78 dark:border-stone-800 dark:bg-stone-900'
            ]"
            :style="{ transitionDelay: `${70 + index * 70}ms` }"
          >
            <div v-if="feature.highlight" class="absolute inset-x-8 top-0 h-px bg-gradient-to-r from-transparent via-primary-300 to-transparent"></div>

            <div class="mb-5 inline-flex h-12 w-12 items-center justify-center rounded-2xl" :class="feature.iconBg">
              <svg v-if="feature.icon === 'globe'" class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418"></path>
              </svg>
              <svg v-if="feature.icon === 'bolt'" class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
                <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z"></path>
              </svg>
              <svg v-if="feature.icon === 'shield'" class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
                <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z"></path>
              </svg>
              <svg v-if="feature.icon === 'lock'" class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
                <path stroke-linecap="round" stroke-linejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z"></path>
              </svg>
            </div>

            <h3 class="text-lg font-black tracking-[-0.03em]">{{ feature.title }}</h3>
            <p class="mt-3 text-sm leading-7 text-stone-600 dark:text-stone-300">{{ feature.description }}</p>

            <div class="mt-5 flex flex-wrap gap-2">
              <span
                v-for="tag in feature.tags"
                :key="tag"
                class="rounded-full px-3 py-1 text-xs font-medium"
                :class="feature.highlight
                  ? 'border border-primary-300/80 bg-primary-50 text-primary-700 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-300'
                  : 'border border-stone-200/80 text-stone-500 dark:border-stone-700 dark:text-stone-400'"
              >{{ tag }}</span>
            </div>
          </article>
        </div>
      </section>

      <!-- Pricing Section -->
      <section class="reveal-section mx-auto max-w-6xl px-6 py-10">
        <div class="reveal-item text-center">
          <div class="text-xs font-semibold uppercase tracking-[0.3em] text-stone-500 dark:text-stone-400">{{ t('home.homeNew.pricing.sectionTitle') }}</div>
        </div>

        <div class="mx-auto mt-8 grid max-w-5xl gap-5 lg:grid-cols-2">
          <article
            v-for="(plan, index) in pricingPlans"
            :key="plan.name"
            class="reveal-item relative overflow-hidden rounded-[1.75rem] border bg-white/80 p-5 shadow-[0_18px_60px_rgba(28,25,23,0.08)] backdrop-blur-sm transition hover:translate-y-[-4px] dark:bg-stone-900 dark:shadow-[0_18px_60px_rgba(0,0,0,0.3)] md:p-6"
            :class="[
              plan.highlighted
                ? 'border-primary-300/80 dark:border-primary-500/30 ring-2 ring-primary-300/50 dark:ring-primary-400/30'
                : 'border-teal-200/80 dark:border-teal-500/20'
            ]"
            :style="{ transitionDelay: `${70 + index * 70}ms` }"
          >
            <div v-if="plan.highlighted" class="absolute inset-x-8 top-0 h-px bg-gradient-to-r from-transparent via-primary-300 to-transparent"></div>

            <div class="inline-flex rounded-full px-3 py-1 text-xs font-semibold" :class="plan.badgeClass">{{ plan.badge }}</div>

            <div class="mt-3 text-3xl font-black tracking-[-0.05em]">{{ plan.name }}</div>

            <div class="mt-6 flex items-end justify-center gap-1.5 text-center">
              <span class="text-[2.8rem] font-black tracking-[-0.07em]">{{ plan.price }}</span>
              <span class="pb-1 text-sm font-semibold text-stone-500 dark:text-stone-400">{{ plan.period }}</span>
            </div>

            <p class="mx-auto mt-2 max-w-sm text-center text-xs leading-5 text-stone-600 dark:text-stone-300">
              {{ plan.description }}
            </p>

            <div class="mt-6 border-t border-stone-200/80 pt-5 dark:border-stone-800">
              <div v-for="limit in plan.limits" :key="limit.label" class="grid grid-cols-[auto_1fr] items-start gap-4 py-2 text-sm">
                <div class="text-stone-500 dark:text-stone-400">{{ limit.label }}</div>
                <div class="text-right font-medium text-stone-800 dark:text-stone-100">{{ limit.value }}</div>
              </div>
            </div>

            <div class="mt-5 border-t border-stone-200/80 pt-5 dark:border-stone-800">
              <div v-for="feature in plan.features" :key="feature" class="flex items-center gap-3 py-2 text-sm font-medium text-stone-800 dark:text-stone-100">
                <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-primary-500 text-white">
                  <svg class="h-3.5 w-3.5" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M4 10l4 4 8-8"></path>
                  </svg>
                </span>
                <span>{{ feature }}</span>
              </div>
            </div>

            <router-link
              :to="loginPath"
              class="mt-6 inline-flex w-full items-center justify-center rounded-full border border-primary-300 bg-primary-400 px-6 py-2.5 text-sm font-semibold text-primary-950 shadow-[0_18px_40px_rgba(52,211,153,0.25)] transition hover:bg-primary-300 hover:translate-y-[-2px]"
            >
              {{ t('home.homeNew.pricing.cta') }}
            </router-link>
          </article>
        </div>
      </section>

      <!-- Steps Section -->
      <section class="reveal-section mx-auto max-w-6xl px-6 py-10">
        <div class="reveal-item text-center">
          <div class="text-xs font-semibold uppercase tracking-[0.3em] text-stone-500 dark:text-stone-400">{{ t('home.homeNew.steps.sectionTitle') }}</div>
          <h2 class="mt-4 text-3xl font-black tracking-[-0.05em] md:text-4xl">{{ t('home.homeNew.steps.sectionHeading') }}</h2>
          <p class="mt-3 text-base text-stone-600 dark:text-stone-300">{{ t('home.homeNew.steps.sectionSubtitle') }}</p>
        </div>

        <div class="mt-10 space-y-6">
          <article
            v-for="(step, index) in steps"
            :key="step.step"
            class="reveal-item flex flex-col gap-6 rounded-[2rem] border border-stone-200/80 bg-white/78 p-6 shadow-[0_18px_60px_rgba(28,25,23,0.08)] backdrop-blur-sm dark:border-stone-800 dark:bg-stone-900 md:p-8"
            :class="{ 'md:flex-row-reverse': step.reverse, 'md:flex-row': !step.reverse }"
            :style="{ transitionDelay: `${70 + index * 70}ms` }"
          >
            <div class="flex-1">
              <div class="inline-flex rounded-full border border-stone-200/90 bg-stone-50 px-3 py-1 text-xs font-semibold uppercase tracking-[0.2em] text-stone-500 dark:border-stone-700 dark:bg-stone-950 dark:text-stone-400">
                {{ t('home.homeNew.steps.stepLabel') }} {{ step.step }}
              </div>
              <h3 class="mt-4 text-2xl font-black tracking-[-0.04em]">{{ step.title }}</h3>
              <p class="mt-3 max-w-xl text-sm leading-7 text-stone-600 dark:text-stone-300">{{ step.description }}</p>
              <ul class="mt-5 space-y-3">
                <li v-for="point in step.points" :key="point" class="flex items-start gap-3 text-sm text-stone-700 dark:text-stone-200">
                  <span class="mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full bg-primary-500/15 text-xs font-bold text-primary-600 dark:text-primary-300">✓</span>
                  <span>{{ point }}</span>
                </li>
              </ul>
            </div>
            <div class="flex-1">
              <!-- Register Demo -->
              <div v-if="step.demoType === 'register'" class="overflow-hidden rounded-[1.7rem] border border-stone-200/80 bg-stone-50 shadow-inner dark:border-stone-800 dark:bg-stone-950/60">
                <div class="flex items-center gap-2 border-b border-stone-200/80 px-4 py-3 text-xs text-stone-500 dark:border-stone-800 dark:text-stone-400">
                  <span class="h-2.5 w-2.5 rounded-full bg-stone-300 dark:bg-stone-700"></span>
                  <span class="h-2.5 w-2.5 rounded-full bg-stone-300 dark:bg-stone-700"></span>
                  <span class="h-2.5 w-2.5 rounded-full bg-stone-300 dark:bg-stone-700"></span>
                  <span class="ml-2">{{ siteName }}/register</span>
                </div>
                <div class="space-y-4 p-5">
                  <div class="text-2xl font-black tracking-[-0.06em]">
                    <span class="text-primary-600 dark:text-primary-300">{{ siteName.slice(0, 2) }}</span>{{ siteName.slice(2) }}
                  </div>
                  <div>
                    <div class="mb-2 text-xs uppercase tracking-[0.18em] text-stone-400">{{ t('home.homeNew.steps.registerDemo.emailLabel') }}</div>
                    <div class="rounded-2xl border border-stone-200/80 bg-white px-4 py-3 text-sm text-stone-600 dark:border-stone-800 dark:bg-stone-900 dark:text-stone-300">your@email.com</div>
                  </div>
                  <div>
                    <div class="mb-2 text-xs uppercase tracking-[0.18em] text-stone-400">{{ t('home.homeNew.steps.registerDemo.passwordLabel') }}</div>
                    <div class="rounded-2xl border border-stone-200/80 bg-white px-4 py-3 text-sm text-stone-600 dark:border-stone-800 dark:bg-stone-900 dark:text-stone-300">••••••••••</div>
                  </div>
                  <div class="rounded-2xl bg-primary-400 px-4 py-3 text-center text-sm font-semibold text-primary-950">{{ t('home.homeNew.steps.registerDemo.submitBtn') }}</div>
                </div>
              </div>

              <!-- API Key Demo -->
              <div v-if="step.demoType === 'apikey'" class="overflow-hidden rounded-[1.7rem] border border-stone-200/80 bg-stone-50 shadow-inner dark:border-stone-800 dark:bg-stone-950/60">
                <div class="flex items-center justify-between border-b border-stone-200/80 px-4 py-3 dark:border-stone-800">
                  <span class="text-sm font-semibold">API Keys</span>
                  <span class="rounded-full bg-primary-400 px-3 py-1 text-xs font-semibold text-primary-950">+ {{ t('home.homeNew.steps.apikeyDemo.newKey') }}</span>
                </div>
                <div class="space-y-3 p-4">
                  <div class="flex items-center justify-between rounded-2xl border border-stone-200/80 bg-white px-4 py-3 dark:border-stone-800 dark:bg-stone-900">
                    <div>
                      <div class="text-sm font-semibold">{{ t('home.homeNew.steps.apikeyDemo.keyName') }}</div>
                      <div class="mt-1 text-xs text-stone-500 dark:text-stone-400">sk-{{ siteName.toLowerCase() }}-Kx9m••••••••••••</div>
                    </div>
                    <span class="rounded-full border border-stone-200 px-3 py-1 text-xs text-stone-500 dark:border-stone-700 dark:text-stone-400">{{ t('home.homeNew.steps.apikeyDemo.copy') }}</span>
                  </div>
                  <div class="flex items-center justify-between rounded-2xl border border-stone-200/80 bg-white px-4 py-3 dark:border-stone-800 dark:bg-stone-900">
                    <div>
                      <div class="text-sm font-semibold">{{ t('home.homeNew.steps.apikeyDemo.keyName2') }}</div>
                      <div class="mt-1 text-xs text-stone-500 dark:text-stone-400">sk-{{ siteName.toLowerCase() }}-Rp3n••••••••••••</div>
                    </div>
                    <span class="rounded-full border border-stone-200 px-3 py-1 text-xs text-stone-500 dark:border-stone-700 dark:text-stone-400">{{ t('home.homeNew.steps.apikeyDemo.copy') }}</span>
                  </div>
                </div>
              </div>

              <!-- Terminal Demo -->
              <div v-if="step.demoType === 'terminal'" class="overflow-hidden rounded-[1.7rem] border border-stone-200/80 bg-[#18120f] shadow-[0_20px_60px_rgba(28,25,23,0.22)] dark:border-stone-800">
                <div class="flex items-center gap-2 border-b border-white/5 px-4 py-3 text-xs text-stone-400">
                  <span class="h-2.5 w-2.5 rounded-full bg-[#ff5f56]"></span>
                  <span class="h-2.5 w-2.5 rounded-full bg-[#ffbd2e]"></span>
                  <span class="h-2.5 w-2.5 rounded-full bg-[#27c93f]"></span>
                  <span class="ml-2">terminal</span>
                </div>
                <div class="space-y-3 px-4 py-5 font-mono text-sm text-stone-200">
                  <div class="text-stone-500">
                    <span class="mr-2 text-primary-300">$</span>export API_BASE_URL="{{ siteName.toLowerCase() }}"
                  </div>
                  <div class="text-stone-500">
                    <span class="mr-2 text-primary-300">$</span>{{ t('home.homeNew.steps.terminalDemo.command') }}
                  </div>
                  <div class="text-primary-300">{{ t('home.homeNew.steps.terminalDemo.ready') }}</div>
                  <div class="text-stone-400">{{ t('home.homeNew.steps.terminalDemo.connected') }}</div>
                  <div class="text-stone-400">
                    <span class="mr-2 text-primary-300">›</span>{{ t('home.homeNew.steps.terminalDemo.prompt') }}
                  </div>
                  <div>
                    <span class="mr-2 text-primary-300">$</span>
                    <span class="animate-cursor-blink inline-block h-5 w-2 rounded-sm bg-primary-300/80 align-middle"></span>
                  </div>
                </div>
              </div>
            </div>
          </article>
        </div>
      </section>

      <!-- Stats Section -->
      <section class="reveal-section mx-auto max-w-6xl px-6 py-10">
        <div class="reveal-item text-center">
          <div class="text-xs font-semibold uppercase tracking-[0.3em] text-stone-500 dark:text-stone-400">{{ t('home.homeNew.stats.sectionTitle') }}</div>
          <h2 class="mt-4 text-3xl font-black tracking-[-0.05em] md:text-4xl">{{ t('home.homeNew.stats.sectionHeading') }}</h2>
          <p class="mt-3 text-base text-stone-600 dark:text-stone-300">{{ t('home.homeNew.stats.sectionSubtitle') }}</p>
        </div>

        <div class="mt-10 grid gap-6 lg:grid-cols-3">
          <article
            v-for="(stat, index) in stats"
            :key="stat.title"
            class="reveal-item overflow-hidden rounded-[2rem] border border-stone-200/80 bg-white/78 p-6 shadow-[0_18px_60px_rgba(28,25,23,0.08)] backdrop-blur-sm dark:border-stone-800 dark:bg-stone-900"
            :style="{ transitionDelay: `${70 + index * 70}ms` }"
          >
            <div class="flex items-start justify-between gap-4">
              <div class="inline-flex h-12 w-12 items-center justify-center rounded-2xl" :class="stat.iconBg">
                <svg v-if="stat.icon === 'bolt'" class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z"></path>
                </svg>
                <svg v-if="stat.icon === 'clock'" class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                </svg>
                <svg v-if="stat.icon === 'code'" class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6.75 7.5l3 2.25-3 2.25m4.5 0h3m-9 8.25h13.5A2.25 2.25 0 0021 18V6a2.25 2.25 0 00-2.25-2.25H5.25A2.25 2.25 0 003 6v12a2.25 2.25 0 002.25 2.25z"></path>
                </svg>
              </div>
              <div class="text-4xl font-black tracking-[-0.06em]">{{ stat.value }}</div>
            </div>
            <h3 class="mt-6 text-2xl font-black tracking-[-0.04em]">{{ stat.title }}</h3>
            <p class="mt-3 text-sm leading-7 text-stone-600 dark:text-stone-300">{{ stat.description }}</p>
            <div class="mt-5 h-2 rounded-full bg-stone-200/80 dark:bg-stone-800">
              <div class="h-full rounded-full" :class="stat.progressColor" :style="{ width: `${stat.progress}%` }"></div>
            </div>
            <div class="mt-5 flex flex-wrap gap-2">
              <span v-for="tag in stat.tags" :key="tag" class="rounded-full border border-stone-200/80 px-3 py-1 text-xs font-medium text-stone-500 dark:border-stone-700 dark:text-stone-400">{{ tag }}</span>
            </div>
          </article>
        </div>
      </section>

      <!-- CTA Section -->
      <section class="reveal-section mx-auto max-w-6xl px-6 pb-20 pt-10">
        <div class="reveal-item relative overflow-hidden rounded-[2.6rem] border border-primary-300/60 bg-gradient-to-br from-primary-300 to-lime-200 px-8 py-12 text-center text-primary-950 shadow-[0_30px_90px_rgba(52,211,153,0.22)]">
          <div class="absolute left-10 top-8 h-28 w-28 rounded-full bg-white/12 blur-2xl"></div>
          <div class="absolute bottom-0 right-8 h-32 w-32 rounded-full bg-stone-950/10 blur-2xl"></div>
          <div class="relative">
            <div class="text-xs font-semibold uppercase tracking-[0.32em] text-stone-900/70">{{ t('home.homeNew.cta.ready') }}</div>
            <h2 class="mt-4 text-3xl font-black tracking-[-0.06em] md:text-5xl">{{ t('home.homeNew.cta.title') }}</h2>
            <p class="mx-auto mt-4 max-w-2xl text-base leading-8 text-stone-900/80">
              {{ t('home.homeNew.cta.subtitle') }}
            </p>
            <router-link
              :to="loginPath"
              class="mt-8 inline-flex items-center gap-2 rounded-full bg-stone-950 px-6 py-3 text-sm font-semibold text-white transition hover:bg-stone-800 hover:translate-y-[-2px]"
            >
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.homeNew.cta.button') }}
              <svg class="h-4 w-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.9">
                <path stroke-linecap="round" stroke-linejoin="round" d="M4 10h12M12 6l4 4-4 4"></path>
              </svg>
            </router-link>
          </div>
        </div>
      </section>
    </main>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-stone-200/70 px-6 py-8 text-center text-sm text-stone-500 dark:border-stone-800 dark:text-stone-400">
      <div class="flex flex-col items-center justify-center gap-4 sm:flex-row sm:gap-6">
        <p>© {{ new Date().getFullYear() }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</p>
        <div class="flex items-center gap-4">
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="transition-colors hover:text-stone-700 dark:hover:text-white">
            {{ t('home.docs') }}
          </a>
          <a :href="githubUrl" target="_blank" rel="noopener noreferrer" class="transition-colors hover:text-stone-700 dark:hover:text-white">
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<style scoped>
/* Reveal Animations */
.reveal-section {
  opacity: 0;
}

.reveal-section.is-visible {
  opacity: 1;
}

.reveal-item {
  opacity: 0;
  transform: translateY(24px);
  transition: opacity 0.6s cubic-bezier(0.22, 1, 0.36, 1),
              transform 0.6s cubic-bezier(0.22, 1, 0.36, 1);
}

.reveal-item.is-visible {
  opacity: 1;
  transform: translateY(0);
}

/* Reduced Motion Support */
@media (prefers-reduced-motion: reduce) {
  .reveal-item,
  .reveal-section {
    animation: none;
    transition: none;
    opacity: 1;
    transform: none;
  }
}

/* Card Hover Effects */
article {
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

/* Hover lift effect */
.hover-lift:hover {
  transform: translateY(-2px);
}

.hover-lift-md:hover {
  transform: translateY(-4px);
}
</style>