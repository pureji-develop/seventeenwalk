import { defineConfig } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';

const proxyTarget = process.env.VITE_PROXY_TARGET ?? 'http://127.0.0.1:8080';
const wsTarget = proxyTarget.replace(/^http/, 'ws');
const port = Number(process.env.VITE_PORT ?? 5175);

export default defineConfig({
  appType: 'spa',
  server: {
    host: '0.0.0.0',
    port,
    strictPort: true,
    hmr: {
      clientPort: port,
    },
    watch: {
      usePolling: true,
      interval: 500,
    },
    proxy: {
      '/rooms': {
        target: proxyTarget,
        changeOrigin: true,
      },
      '/health': {
        target: proxyTarget,
        changeOrigin: true,
      },
      '/ws': {
        target: wsTarget,
        ws: true,
      },
    },
  },
  optimizeDeps: {
    include: ['phaser'],
  },
  plugins: [
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: [
        'favicon.ico',
        'favicon.png',
        'favicon-16x16.png',
        'favicon-32x32.png',
        'favicon-48x48.png',
        'apple-touch-icon.png',
        'tiles.webp',
        'tiles.json',
      ],
      manifest: {
        name: '十七歩 — SeventeenWalk',
        short_name: '十七歩',
        description: '2人用オンライン麻雀バリアント「十七歩」',
        lang: 'ja',
        start_url: '/',
        scope: '/',
        display: 'standalone',
        orientation: 'landscape',
        background_color: '#0c2e24',
        theme_color: '#0c2e24',
        categories: ['games'],
        icons: [
          {
            src: 'pwa-192x192.png',
            sizes: '192x192',
            type: 'image/png',
          },
          {
            src: 'pwa-512x512.png',
            sizes: '512x512',
            type: 'image/png',
          },
          {
            src: 'pwa-512x512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'maskable',
          },
        ],
      },
      workbox: {
        // Phaser + tiles can be large; keep shell + assets offline-friendly.
        globPatterns: ['**/*.{js,css,html,ico,png,svg,webp,json,woff2}'],
        navigateFallback: '/index.html',
        // API / WS must always hit the network (never cache).
        navigateFallbackDenylist: [/^\/rooms/, /^\/ws/, /^\/health/],
        runtimeCaching: [
          {
            urlPattern: ({ url }) =>
              url.pathname === '/rooms' ||
              url.pathname.startsWith('/rooms/') ||
              url.pathname === '/health' ||
              url.pathname === '/ws',
            handler: 'NetworkOnly',
          },
          {
            urlPattern: /^https:\/\/fonts\.googleapis\.com\/.*/i,
            handler: 'CacheFirst',
            options: {
              cacheName: 'google-fonts-stylesheets',
              expiration: { maxEntries: 8, maxAgeSeconds: 60 * 60 * 24 * 365 },
            },
          },
          {
            urlPattern: /^https:\/\/fonts\.gstatic\.com\/.*/i,
            handler: 'CacheFirst',
            options: {
              cacheName: 'google-fonts-webfonts',
              expiration: { maxEntries: 16, maxAgeSeconds: 60 * 60 * 24 * 365 },
            },
          },
        ],
      },
      devOptions: {
        // Dev では HMR 優先。本番ビルドで SW を検証する。
        enabled: false,
      },
    }),
  ],
});
