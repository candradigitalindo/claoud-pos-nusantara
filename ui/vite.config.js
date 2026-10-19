/**
 * vite.config.js
 *
 * Vite 6 configuration for Cloud POS UI.
 *
 * Plugins:
 * - @vitejs/plugin-vue  : enables Single File Component (.vue) support
 * - @tailwindcss/vite   : Tailwind CSS v4 first-class Vite integration
 *
 * Proxy:
 *   /api/v1  →  http://localhost:3000  (forwards API calls to Go backend
 *               so we avoid CORS during local development)
 *
 * AI NOTE: To add a new environment, duplicate the proxy entry or use
 * import.meta.env.VITE_API_BASE_URL for dynamic base URL.
 */

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { resolve } from 'path'

export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(), // Tailwind v4: no config file needed, uses CSS @import
  ],

  resolve: {
    alias: {
      // '@' maps to /src for clean imports: import X from '@/components/X.vue'
      '@': resolve(__dirname, 'src'),
    },
  },

  server: {
    port: 5174,
    proxy: {
      // Proxy API requests to local Go server
      '/api/v1': {
        target: 'http://localhost:4000',
        changeOrigin: true,
      },
      '/uploads': {
        target: 'http://localhost:4000',
        changeOrigin: true,
      },
      '/cctv': {
        target: 'http://localhost:4000',
        changeOrigin: true,
      },
    },
  },

  build: {
    outDir: 'dist',
    sourcemap: false,
    rollupOptions: {
      output: {
        // Manual chunk splitting for better caching
        manualChunks: {
          vendor: ['vue', 'vue-router', 'pinia'],
          http: ['axios'],
        },
      },
    },
  },
})
