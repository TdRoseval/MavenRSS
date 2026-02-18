import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  base: '/',
  resolve: {
    alias: {
      '@': resolve(__dirname, './src')
    }
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:1234',
        changeOrigin: true
      },
      '/docs': {
        target: 'http://127.0.0.1:1234',
        changeOrigin: true
      },
      '/swagger': {
        target: 'http://127.0.0.1:1234',
        changeOrigin: true
      }
    }
  },
  build: {
    target: 'es2015',
    cssMinify: true,
    sourcemap: false,
    chunkSizeWarningLimit: 1000,
    reportCompressedSize: true,
    minify: 'esbuild',
    esbuild: {
      drop: ['console', 'debugger']
    },
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['vue', 'pinia', 'vue-i18n'],
          ui: ['@phosphor-icons/vue', 'tailwindcss'],
          utils: ['highlight.js', 'katex']
        },
        entryFileNames: 'assets/index-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash].[ext]'
      }
    },
    emptyOutDir: true
  },
  optimizeDeps: {
    include: ['vue', 'pinia', 'vue-i18n', '@phosphor-icons/vue', 'highlight.js', 'katex']
  },
  test: {
    globals: true,
    environment: 'jsdom'
  }
})
