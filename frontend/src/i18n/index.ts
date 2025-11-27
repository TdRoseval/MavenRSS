import { createI18n } from 'vue-i18n';
import type { SupportedLocale } from './types';

// Import translation messages
import en from './locales/en';
import zh from './locales/zh';

const i18n = createI18n({
  legacy: false, // Use Composition API mode
  locale: 'en-US', // Default to English, will be overridden by settings
  fallbackLocale: 'en-US' as SupportedLocale,
  messages: {
    'en-US': en,
    'zh-CN': zh,
  },
});

export default i18n;

// Export the i18n instance for direct access
export const { locale } = i18n.global;
