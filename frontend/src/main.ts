import { createApp } from 'vue';
import { createPinia } from 'pinia';
import PhosphorIcons from '@phosphor-icons/vue';
import i18n, { locale } from './i18n';
import './style.css';
import App from './App.vue';
import { setCachedServerMode } from './utils/serverMode';

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

// Initialize settings and language in the background (non-blocking)
async function initializeSettings() {
  try {
    const res = await fetch('/api/settings');
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}: ${res.statusText}`);
    }

    const text = await res.text();
    let data;

    try {
      data = JSON.parse(text);
    } catch (jsonError) {
      console.error('JSON parse error:', jsonError);
      console.error('Response text (first 500 chars):', text.substring(0, 500));
      data = {};
    }

    if (data.language) {
      locale.value = data.language;
    }

    try {
      const versionRes = await fetch('/api/version');
      if (versionRes.ok) {
        const versionData = await versionRes.json();
        setCachedServerMode(versionData.server_mode === 'true');
      }
    } catch (e) {
      console.error('Failed to cache server mode:', e);
    }
  } catch (e) {
    console.error('Error loading language setting:', e);
  }
}

initializeSettings();
