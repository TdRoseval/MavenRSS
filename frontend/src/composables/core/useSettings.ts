/**
 * Composable for settings management
 */
import { ref, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import type { SettingsData } from '@/types/settings';
import type { ThemePreference } from '@/stores/app';
import { generateInitialSettings, parseSettingsData } from './useSettings.generated';
import { apiClient } from '@/utils/apiClient';

export function useSettings() {
  const { locale } = useI18n();

  // Use generated helper for initial settings (alphabetically sorted)
  const settings = ref(generateInitialSettings()) as any;
  const isLoading = ref(false);

  /**
   * Fetch settings from backend
   */
  async function fetchSettings(): Promise<SettingsData> {
    isLoading.value = true;
    try {
      const data = await apiClient.get<SettingsData>('/settings');
      
      // Use generated helper to parse settings (alphabetically sorted)
      settings.value = parseSettingsData(data as Record<string, string>);

      return settings.value;
    } catch (e) {
      console.error('Error fetching settings:', e);
      throw e;
    } finally {
      isLoading.value = false;
    }
  }

  /**
   * Save settings to backend
   */
  async function saveSettings(data: Partial<SettingsData>): Promise<void> {
    isLoading.value = true;
    try {
      await apiClient.post('/settings', data);
      // Refresh settings to get the latest values
      await fetchSettings();
      // Notify other components that settings have been updated
      window.dispatchEvent(new CustomEvent('settings-updated', { detail: { autoSave: true } }));
    } catch (e) {
      console.error('Error saving settings:', e);
      throw e;
    } finally {
      isLoading.value = false;
    }
  }

  /**
   * Apply fetched settings to the app
   */

  function applySettings(data: SettingsData, setTheme: (preference: ThemePreference) => void) {
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

  /**
   * Handle settings-updated event
   * Re-fetches settings when backend updates them (e.g., after feed refresh)
   * Skips re-fetching if this is an auto-save event to prevent overwriting user input
   */
  function handleSettingsUpdated(event: Event) {
    const customEvent = event as CustomEvent<{ autoSave?: boolean }>;

    // Skip re-fetching if this is an auto-save event
    // The settings are already up-to-date since we just saved them
    if (customEvent.detail?.autoSave) {
      return;
    }

    fetchSettings().catch((e) => {
      console.error('Error re-fetching settings after update:', e);
    });
  }

  // Listen for settings-updated events
  onMounted(() => {
    window.addEventListener('settings-updated', handleSettingsUpdated);
  });

  // onUnmounted(() => {
  //   window.removeEventListener('settings-updated', handleSettingsUpdated);
  // });

  return {
    settings,
    isLoading,
    fetchSettings,
    saveSettings,
    applySettings,
  };
}
