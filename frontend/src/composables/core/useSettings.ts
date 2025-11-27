/**
 * Composable for settings management
 */
import { ref, type Ref } from 'vue';
import { useI18n } from 'vue-i18n';
import type { SettingsData } from '@/types/settings';
import type { ThemePreference } from '@/stores/app';

export function useSettings() {
  const { locale } = useI18n();

  const settings: Ref<SettingsData> = ref({
    update_interval: 10,
    translation_enabled: false,
    target_language: 'zh',
    translation_provider: 'google',
    deepl_api_key: '',
    auto_cleanup_enabled: false,
    max_cache_size_mb: 20,
    max_article_age_days: 30,
    language: locale.value,
    theme: 'auto',
    last_article_update: '',
    show_hidden_articles: false,
    default_view_mode: 'original',
    startup_on_boot: false,
    shortcuts: '',
    rules: '',
  });

  /**
   * Fetch settings from backend
   */
  async function fetchSettings() {
    try {
      const res = await fetch('/api/settings');
      const data = await res.json();

      settings.value = {
        update_interval: data.update_interval || 10,
        translation_enabled: data.translation_enabled === 'true',
        target_language: data.target_language || 'zh',
        translation_provider: data.translation_provider || 'google',
        deepl_api_key: data.deepl_api_key || '',
        auto_cleanup_enabled: data.auto_cleanup_enabled === 'true',
        max_cache_size_mb: parseInt(data.max_cache_size_mb) || 20,
        max_article_age_days: parseInt(data.max_article_age_days) || 30,
        language: data.language || locale.value,
        theme: data.theme || 'auto',
        last_article_update: data.last_article_update || '',
        show_hidden_articles: data.show_hidden_articles === 'true',
        default_view_mode: data.default_view_mode || 'original',
        startup_on_boot: data.startup_on_boot === 'true',
        shortcuts: data.shortcuts || '',
        rules: data.rules || '',
      };

      return settings.value;
    } catch (e) {
      console.error('Error fetching settings:', e);
      throw e;
    }
  }

  /**
   * Apply fetched settings to the app
   */
  function applySettings(data: SettingsData, setTheme: (theme: ThemePreference) => void) {
    // Apply the saved language
    if (data.language) {
      locale.value = data.language;
    }

    // Apply the saved theme
    if (data.theme) {
      setTheme(data.theme as ThemePreference);
    }

    // Initialize shortcuts in store
    if (data.shortcuts) {
      try {
        const parsed = JSON.parse(data.shortcuts);
        window.dispatchEvent(
          new CustomEvent('shortcuts-changed', {
            detail: { shortcuts: parsed },
          })
        );
      } catch (e) {
        console.error('Error parsing shortcuts:', e);
      }
    }
  }

  return {
    settings,
    fetchSettings,
    applySettings,
  };
}
