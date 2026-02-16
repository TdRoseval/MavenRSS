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
    chunkSizeWarningLimit: 2000,
    reportCompressedSize: false,
    minify: 'esbuild',
    esbuild: {
      drop: ['console', 'debugger']
    },
    rollupOptions: {
      output: {
        manualChunks: undefined,
        entryFileNames: 'assets/index-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash].[ext]'
      }
    },
    emptyOutDir: true
  },
  test: {
    globals: true,
    environment: 'jsdom'
  }
})
