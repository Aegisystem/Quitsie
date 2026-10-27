import { defineConfig } from 'vitest/config'

// Kept apart from vite.config.ts so tests load neither React nor the PWA plugin.
// The domain is pure TypeScript (ADR 0003), so it runs in Node without a DOM.
export default defineConfig({
  test: {
    include: ['src/**/*.test.ts'],
    environment: 'node',
  },
})
