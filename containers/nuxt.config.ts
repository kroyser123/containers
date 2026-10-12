const siteUrl = process.env.NUXT_PUBLIC_SITE_URL?.replace(/\/$/, '') || '';
if (siteUrl && !/^https:\/\/[^/]+$/.test(siteUrl)) {
  throw new Error('NUXT_PUBLIC_SITE_URL must be an HTTPS origin without a path');
}

export default defineNuxtConfig({
  compatibilityDate: '2026-01-01',
  devtools: { enabled: false },
  modules: ['@nuxt/eslint'],
  css: ['~/assets/styles/main.scss'],
  typescript: { strict: true },
  runtimeConfig: {
    telegramBotToken: '',
    telegramChatId: '',
    public: { siteUrl, productionReady: false },
  },
  app: {
    head: {
      htmlAttrs: { lang: 'ru' },
      link: [{ rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' }],
      meta: [{ name: 'theme-color', content: '#26343b' }],
    },
  },
  routeRules: {
    '/': { prerender: true },
    '/privacy': { prerender: true },
    '/consent': { prerender: true },
    '/api/**': { headers: { 'Cache-Control': 'no-store', 'X-Robots-Tag': 'noindex' } },
  },
  nitro: { preset: 'node-server' },
  eslint: { config: { stylistic: false } },
});
