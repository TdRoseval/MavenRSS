import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, './src')
    }
  },
  build: {
    target: 'es2015',
    cssMinify: true,
    sourcemap: false,
    chunkSizeWarningLimit: 2000,
    reportCompressedSize: false,
    minify: 'esbuild',
    esbuild: {
      drop: ['console', 'debugger']
    },
    rollupOptions: {
      output: {
        manualChunks: undefined
      }
    }
  },
  test: {
    globals: true,
    environment: 'jsdom'
  }
})
