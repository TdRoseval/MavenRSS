import { createI18n } from 'vue-i18n';
import type { SupportedLocale } from './types';

// Import translation messages
import en from './locales/en';
import zh from './locales/zh';

// LocalStorage key for language preference
const LANGUAGE_STORAGE_KEY = 'mrrss_language';

// Load saved language from localStorage
function getSavedLanguage(): SupportedLocale {
  try {
    const saved = localStorage.getItem(LANGUAGE_STORAGE_KEY);
    if (saved && (saved === 'en-US' || saved === 'zh-CN')) {
      return saved as SupportedLocale;
    }
  } catch (e) {
    console.error('Failed to load language from localStorage:', e);
  }
  return 'en-US';
}

// Save language to localStorage
export function saveLanguage(lang: SupportedLocale) {
  try {
    localStorage.setItem(LANGUAGE_STORAGE_KEY, lang);
  } catch (e) {
    console.error('Failed to save language to localStorage:', e);
  }
}

const i18n = createI18n({
  legacy: false, // Use Composition API mode
  locale: getSavedLanguage(), // Default to saved language or English
  fallbackLocale: 'en-US' as SupportedLocale,
  messages: {
    'en-US': en,
    'zh-CN': zh,
  },
});

export default i18n;

// Export the i18n instance for direct access
export const { locale } = i18n.global;
