/**
 * Composable for manual settings saving
 * Replaces auto-save with explicit save/cancel actions
 */
import { ref, type Ref, computed, isRef, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useAppStore } from '@/stores/app';
import type { SettingsData } from '@/types/settings';
import { settingsDefaults } from '@/config/defaults';
import { authPost } from '@/utils/authFetch';
import { saveLanguage } from '@/i18n';

export function useSettingsManualSave(settings: Ref<SettingsData> | (() => SettingsData)) {
  const { locale } = useI18n();
  const store = useAppStore();
  const isSaving = ref(false);
  const hasChanges = ref(false);

  // Convert to ref if it's a getter function
  const settingsRef = isRef(settings) ? settings : computed(settings);

  // Store original settings for comparison and cancel
  let originalSettings: Record<string, any> | null = null;

  /**
   * Save original settings snapshot
   */
  function saveOriginalSettings() {
    originalSettings = { ...settingsRef.value };
    hasChanges.value = false;
  }

  /**
   * Check if settings have changed
   */
  function checkForChanges() {
    if (!originalSettings) {
      hasChanges.value = true;
      return;
    }

    // Compare current settings with original
    for (const key in settingsRef.value) {
      if (key === '_has_inherited') continue; // Skip internal flags

      const currentValue = (settingsRef.value as any)[key];
      const originalValue = originalSettings[key];

      if (currentValue !== originalValue) {
        hasChanges.value = true;
        return;
      }
    }

    hasChanges.value = false;
  }

  /**
   * Build payload for manual save (only include changed values)
   */
  function buildManualSavePayload(): Record<string, string> {
    const payload: Record<string, string> = {};

    if (!originalSettings) {
      // If no original, send all settings
      for (const key in settingsRef.value) {
        if (key === '_has_inherited') continue;
        const value = (settingsRef.value as any)[key];
        payload[key] = typeof value === 'boolean' ? value.toString() : String(value ?? '');
      }
    } else {
      // Only include changed settings
      for (const key in settingsRef.value) {
        if (key === '_has_inherited') continue;

        const currentValue = (settingsRef.value as any)[key];
        const originalValue = originalSettings[key];

        if (currentValue !== originalValue) {
          payload[key] = typeof currentValue === 'boolean' ? currentValue.toString() : String(currentValue ?? '');
        }
      }
    }

    return payload;
  }

  /**
   * Save settings to backend
   */
  async function saveSettings(): Promise<boolean> {
    if (!hasChanges.value) {
      return true; // No changes to save
    }

    isSaving.value = true;
    try {
      const payload = buildManualSavePayload();

      // Apply UI changes immediately (theme, language)
      locale.value = settingsRef.value.language;
      saveLanguage(settingsRef.value.language as any);
      store.setTheme(settingsRef.value.theme as 'light' | 'dark' | 'auto');

      // Notify components about default view mode change
      window.dispatchEvent(
        new CustomEvent('default-view-mode-changed', {
          detail: {
            mode: settingsRef.value.default_view_mode,
          },
        })
      );

      // Save to backend
      await authPost('/api/settings', payload);

      // Handle translation settings change
      const translationChanged =
        originalSettings && (
          originalSettings.translation_enabled !== settingsRef.value.translation_enabled ||
          originalSettings.translation_provider !== settingsRef.value.translation_provider ||
          (settingsRef.value.translation_enabled &&
            originalSettings.target_language !== settingsRef.value.target_language)
        );

      if (translationChanged) {
        await authPost('/api/articles/clear-translations');
        // Notify ArticleList about translation settings change
        window.dispatchEvent(
          new CustomEvent('translation-settings-changed', {
            detail: {
              enabled: settingsRef.value.translation_enabled,
              targetLang: settingsRef.value.target_language,
            },
          })
        );
        // Refresh articles
        store.fetchArticles();
      }

      // Refresh articles if show_hidden_articles changed
      if (
        originalSettings &&
        originalSettings.show_hidden_articles !== settingsRef.value.show_hidden_articles
      ) {
        store.fetchArticles();
      }

      // Notify about other setting changes
      window.dispatchEvent(
        new CustomEvent('show-preview-images-changed', {
          detail: {
            value: settingsRef.value.show_article_preview_images,
          },
        })
      );

      window.dispatchEvent(
        new CustomEvent('image-gallery-setting-changed', {
          detail: {
            enabled: settingsRef.value.image_gallery_enabled,
          },
        })
      );

      window.dispatchEvent(
        new CustomEvent('auto-show-all-content-changed', {
          detail: {
            value: settingsRef.value.auto_show_all_content,
          },
        })
      );

      if (originalSettings && originalSettings.layout_mode !== settingsRef.value.layout_mode) {
        window.dispatchEvent(
          new CustomEvent('layout-mode-changed', {
            detail: {
              mode: settingsRef.value.layout_mode,
            },
          })
        );
      }

      // Check if summary settings changed
      if (
        originalSettings && (
          originalSettings.summary_enabled !== settingsRef.value.summary_enabled ||
          originalSettings.summary_provider !== settingsRef.value.summary_provider ||
          originalSettings.summary_trigger_mode !== settingsRef.value.summary_trigger_mode ||
          originalSettings.summary_length !== settingsRef.value.summary_length
        )
      ) {
        window.dispatchEvent(
          new CustomEvent('summary-settings-changed', {
            detail: {
              enabled: settingsRef.value.summary_enabled,
              provider: settingsRef.value.summary_provider,
              triggerMode: settingsRef.value.summary_trigger_mode,
              length: settingsRef.value.summary_length,
            },
          })
        );
      }

      // Notify all components that settings have been updated
      window.dispatchEvent(new CustomEvent('settings-updated', { detail: { autoSave: false } }));

      // Update original settings after successful save
      saveOriginalSettings();

      return true;
    } catch (e) {
      console.error('Error saving settings:', e);
      window.showToast?.(e instanceof Error ? e.message : 'Failed to save settings', 'error');
      return false;
    } finally {
      isSaving.value = false;
    }
  }

  /**
   * Cancel changes and revert to original settings
   */
  function cancelChanges() {
    if (!originalSettings) {
      return;
    }

    for (const key in originalSettings) {
      if (key === '_has_inherited') continue;
      (settingsRef.value as any)[key] = originalSettings[key];
    }

    locale.value = originalSettings.language as string;
    saveLanguage(originalSettings.language as any);
    store.setTheme(originalSettings.theme as 'light' | 'dark' | 'auto');

    hasChanges.value = false;
  }

  /**
   * Reset hasChanges flag (e.g., after external updates)
   */
  function resetChanges() {
    saveOriginalSettings();
  }

  // Watch for settings changes
  watch(() => settingsRef.value, checkForChanges, { deep: true });

  return {
    hasChanges: computed(() => hasChanges.value),
    isSaving: computed(() => isSaving.value),
    saveSettings,
    cancelChanges,
    resetChanges,
    saveOriginalSettings,
  };
}
