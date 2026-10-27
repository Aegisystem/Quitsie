import { writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import react from '@vitejs/plugin-react'
import { defineConfig, type Plugin } from 'vite'
import { VitePWA } from 'vite-plugin-pwa'

// The Go binary embeds the compiled PWA from this folder (api/cmd/server/main.go).
const outDir = fileURLToPath(new URL('../api/cmd/server/dist', import.meta.url))

// Vite empties outDir before building, but Go's embed needs the folder to exist
// even before the first build, so the committed .gitkeep is put back.
function keepGitkeep(): Plugin {
  return {
    name: 'quitsie:keep-gitkeep',
    apply: 'build',
    closeBundle() {
      writeFileSync(`${outDir}/.gitkeep`, '')
    },
  }
}

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.svg', 'apple-touch-icon-180x180.png'],
      manifest: {
        name: 'Quitsie',
        short_name: 'Quitsie',
        description: 'Gastos compartidos: quién le debe cuánto a quién.',
        lang: 'es',
        start_url: '/',
        display: 'standalone',
        theme_color: '#0f766e',
        background_color: '#f7f7f5',
        icons: [
          { src: 'pwa-192x192.png', sizes: '192x192', type: 'image/png' },
          { src: 'pwa-512x512.png', sizes: '512x512', type: 'image/png' },
          { src: 'maskable-512x512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        // The service worker only precaches the interface. Data lives in IndexedDB,
        // and /api must always reach the server instead of getting index.html.
        navigateFallbackDenylist: [/^\/api\//],
      },
    }),
    keepGitkeep(),
  ],
  build: {
    outDir,
    emptyOutDir: true,
  },
  server: {
    // During `npm run dev`, API calls go to the Go server.
    proxy: { '/api': 'http://127.0.0.1:8080' },
  },
})
