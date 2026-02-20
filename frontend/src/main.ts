import { createApp } from 'vue';
import { createPinia } from 'pinia';
import PhosphorIcons from '@phosphor-icons/vue';
import i18n, { locale } from './i18n';
import './style.css';
import App from './App.vue';
import { setCachedServerMode } from './utils/serverMode';
import { register as registerServiceWorker } from './utils/serviceWorker';

const app = createApp(App);
const pinia = createPinia();

// Add global error handler for Vue errors
app.config.errorHandler = (err: unknown, instance: unknown, info: string) => {
  console.error('[Vue Error Handler] Error:', err);
  console.error('[Vue Error Handler] Component:', (instance as any)?.$?.type?.name || 'Unknown');
  console.error('[Vue Error Handler] Info:', info);
  // Log the full stack trace
  if (err instanceof Error) {
    console.error('[Vue Error Handler] Stack:', err.stack);
  }
};

app.use(pinia);
app.use(i18n);
app.use(PhosphorIcons);

// Mount app immediately for fast initial render
app.mount('#app');

// Register Service Worker for offline caching (only in production-like environments)
if (import.meta.env.PROD || window.location.hostname !== 'localhost') {
  registerServiceWorker({
    onSuccess: () => {
      console.log('[ServiceWorker] Service Worker registered successfully');
    },
    onUpdate: () => {
      console.log('[ServiceWorker] New content available, please refresh');
    },
  });
}

// Initialize server mode in the background (non-blocking)
async function initializeServerMode() {
  try {
    const versionRes = await fetch('/api/version');
    if (versionRes.ok) {
      const versionData = await versionRes.json();
      setCachedServerMode(versionData.server_mode === 'true');
    }
  } catch (e) {
    console.error('Failed to cache server mode:', e);
  }
}

initializeServerMode();
