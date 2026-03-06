import { defineStore } from 'pinia';
import { ref } from 'vue';
import { apiClient } from '@/shared/lib/apiClient';
import { useSettings } from '@/composables/core/useSettings';

export type ThemePreference = 'light' | 'dark' | 'auto';
export type Theme = 'light' | 'dark';

export const useAppStore = defineStore('app', () => {
  // State
  const themePreference = ref<ThemePreference>(
    (localStorage.getItem('themePreference') as ThemePreference) || 'auto'
  );
  const theme = ref<Theme>('light');

  // Theme Management
  function toggleTheme(): void {
    // Cycle through: light -> dark -> auto -> light
    if (themePreference.value === 'light') {
      themePreference.value = 'dark';
    } else if (themePreference.value === 'dark') {
      themePreference.value = 'auto';
    } else {
      themePreference.value = 'light';
    }
    localStorage.setItem('themePreference', themePreference.value);
    applyTheme();
  }

  function setTheme(preference: ThemePreference): void {
    themePreference.value = preference;
    localStorage.setItem('themePreference', preference);
    applyTheme();
  }

  function applyTheme(): void {
    let actualTheme: Theme = themePreference.value as Theme;

    // If auto, detect system preference
    if (themePreference.value === 'auto') {
      actualTheme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    }

    theme.value = actualTheme;

    // Apply to both html and body for consistency
    const htmlElement = document.documentElement;
    if (actualTheme === 'dark') {
      htmlElement.classList.add('dark-mode');
      document.body.classList.add('dark-mode');
    } else {
      htmlElement.classList.remove('dark-mode');
      document.body.classList.remove('dark-mode');
    }
  }

  function initTheme(): void {
    // Listen for system theme changes
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    mediaQuery.addEventListener('change', () => {
      if (themePreference.value === 'auto') {
        applyTheme();
      }
    });

    // Apply initial theme
    applyTheme();
  }

  async function checkForAppUpdates(): Promise<void> {
    try {
      const data: any = await apiClient.get('/check-updates');

      // Only proceed if there's an update available and a download URL
      if (data.has_update && data.download_url) {
        // Check if auto-update is enabled before downloading
        const { settings } = useSettings();

        if (settings.value.auto_update) {
          // Auto download and install in background
          autoDownloadAndInstall(data.download_url, data.asset_name);
        } else {
          // Just show notification that update is available
          if ((window as any).showToast) {
            (window as any).showToast(`Update available: v${data.latest_version}`, 'info');
          }
        }
      }
    } catch (e) {
      console.error('Auto-update check failed:', e);
      // Silently fail - don't disrupt user experience
    }
  }

  async function autoDownloadAndInstall(downloadUrl: string, assetName?: string): Promise<void> {
    try {
      // Download the update in background
      const downloadData: any = await apiClient.post('/download-update', {
        download_url: downloadUrl,
        asset_name: assetName,
      });

      if (!downloadData.success || !downloadData.file_path) {
        console.error('Auto-download failed: Invalid response');
        return;
      }

      // Wait a moment to ensure file is fully written
      await new Promise((resolve) => setTimeout(resolve, 500));

      // Install the update
      const installData: any = await apiClient.post('/install-update', {
        file_path: downloadData.file_path,
      });

      if (installData.success && (window as any).showToast) {
        (window as any).showToast('Update installed. Restart to apply.', 'success');
      }
    } catch (e) {
      console.error('Auto-update failed:', e);
      // Silently fail - don't disrupt user experience
    }
  }

  return {
    themePreference,
    theme,
    toggleTheme,
    setTheme,
    applyTheme,
    initTheme,
    checkForAppUpdates,
    autoDownloadAndInstall,
  };
});
