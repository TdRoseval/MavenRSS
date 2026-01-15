<script setup lang="ts">
import { ref, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { useSettings } from '@/composables/core/useSettings';
import {
  PhArticle,
  PhImage,
  PhImages,
  PhArticleNyTimes,
  PhCursorClick,
  PhEyeSlash,
  PhPlayCircle,
  PhPalette,
  PhUpload,
  PhTrash,
  PhCheck,
  PhBookOpen,
  PhListDashes,
} from '@phosphor-icons/vue';
import type { SettingsData } from '@/types/settings';
import { openInBrowser } from '@/utils/browser';

const { t, locale } = useI18n();
const { fetchSettings } = useSettings();

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();

const uploading = ref(false);
const deleteLoading = ref(false);

// Use props settings for real-time updates (passed from parent)
const hasCustomCSS = computed(() => !!props.settings.custom_css_file);

function openDocumentation() {
  const docUrl = locale.value.startsWith('zh')
    ? 'https://github.com/WCY-dt/MrRSS/blob/main/docs/CUSTOM_CSS.zh.md'
    : 'https://github.com/WCY-dt/MrRSS/blob/main/docs/CUSTOM_CSS.md';
  openInBrowser(docUrl);
}

const handleFileUpload = async () => {
  uploading.value = true;

  try {
    const response = await fetch('/api/custom-css/upload-dialog', {
      method: 'POST',
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Unknown error' }));
      throw new Error(error.error || 'Upload failed');
    }

    const result = await response.json();

    if (result.status === 'cancelled') {
      console.log('CSS upload cancelled by user');
      return;
    }

    if (result.status === 'success') {
      console.log('CSS upload successful:', result);
      window.showToast(t('cssFileUploaded'), 'success');

      // Reload settings from backend to update composable
      try {
        const updatedSettings = await fetchSettings();

        // Emit the updated settings to parent
        emit('update:settings', updatedSettings);
        console.log('Settings updated with custom_css_file:', updatedSettings.custom_css_file);
      } catch (settingsError) {
        console.error('Failed to reload settings:', settingsError);
        // Don't show error toast for this, since upload succeeded
      }

      // Notify ArticleBody components to reload CSS
      window.dispatchEvent(new CustomEvent('custom-css-changed'));
    } else {
      console.error('CSS upload failed:', result);
      window.showToast(result.message || t('cssFileUploadFailed'), 'error');
    }
  } catch (error) {
    console.error('CSS upload error:', error);
    window.showToast(t('cssFileUploadFailed'), 'error');
  } finally {
    uploading.value = false;
  }
};

const handleDeleteCSS = async () => {
  deleteLoading.value = true;

  try {
    console.log('Deleting custom CSS...');

    const response = await fetch('/api/custom-css/delete', {
      method: 'POST',
    });

    if (!response.ok) {
      console.error('Delete failed with status:', response.status);
      throw new Error('Delete failed');
    }

    const result = await response.json();
    console.log('Delete response:', result);

    window.showToast(t('cssFileDeleted'), 'success');

    // Reload settings from backend to update composable
    try {
      const updatedSettings = await fetchSettings();

      // Emit the updated settings to parent
      emit('update:settings', updatedSettings);
      console.log('Settings updated with custom_css_file:', updatedSettings.custom_css_file);
    } catch (settingsError) {
      console.error('Failed to reload settings:', settingsError);
    }

    // Notify ArticleBody components to reload CSS
    window.dispatchEvent(new CustomEvent('custom-css-changed'));
  } catch (error) {
    console.error('Failed to delete CSS file:', error);
    window.showToast(t('cssFileDeleteFailed'), 'error');
  } finally {
    deleteLoading.value = false;
  }
};
</script>

<template>
  <div class="setting-group">
    <label
      class="font-semibold mb-2 sm:mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2"
    >
      <PhArticle :size="14" class="sm:w-4 sm:h-4" />
      {{ t('readingAndDisplay') }}
    </label>

    <div class="setting-item">
      <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
        <PhArticle :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
        <div class="flex-1 min-w-0">
          <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
            {{ t('defaultViewMode') }}
          </div>
          <div class="text-xs text-text-secondary hidden sm:block">
            {{ t('defaultViewModeDesc') }}
          </div>
        </div>
      </div>
      <select
        :value="settings.default_view_mode"
        class="input-field w-24 sm:w-48 text-xs sm:text-sm"
        @change="
          (e) =>
            emit('update:settings', {
              ...settings,
              default_view_mode: (e.target as HTMLSelectElement).value,
            })
        "
      >
        <option value="original">{{ t('viewModeOriginal') }}</option>
        <option value="rendered">{{ t('viewModeRendered') }}</option>
      </select>
    </div>

    <div class="setting-item mt-2 sm:mt-3">
      <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
        <PhImage :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
        <div class="flex-1 min-w-0">
          <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
            {{ t('showArticlePreviewImages') }}
          </div>
          <div class="text-xs text-text-secondary hidden sm:block">
            {{ t('showArticlePreviewImagesDesc') }}
          </div>
        </div>
      </div>
      <input
        :checked="settings.show_article_preview_images"
        type="checkbox"
        class="toggle"
        @change="
          (e) =>
            emit('update:settings', {
              ...settings,
              show_article_preview_images: (e.target as HTMLInputElement).checked,
            })
        "
      />
    </div>

    <div class="setting-item mt-2 sm:mt-3">
      <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
        <PhListDashes :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
        <div class="flex-1 min-w-0">
          <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
            {{ t('compactMode') }}
          </div>
          <div class="text-xs text-text-secondary hidden sm:block">
            {{ t('compactModeDesc') }}
          </div>
        </div>
      </div>
      <input
        :checked="settings.compact_mode"
        type="checkbox"
        class="toggle"
        @change="
          (e) =>
            emit('update:settings', {
              ...settings,
              compact_mode: (e.target as HTMLInputElement).checked,
            })
        "
      />
    </div>

    <div class="setting-item mt-2 sm:mt-3">
      <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
        <PhArticleNyTimes :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
        <div class="flex-1 min-w-0">
          <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
            {{ t('enableFullTextFetch') }}
          </div>
          <div class="text-xs text-text-secondary hidden sm:block">
            {{ t('enableFullTextFetchDesc') }}
          </div>
        </div>
      </div>
      <input
        :checked="settings.full_text_fetch_enabled"
        type="checkbox"
        class="toggle"
        @change="
          (e) =>
            emit('update:settings', {
              ...settings,
              full_text_fetch_enabled: (e.target as HTMLInputElement).checked,
            })
        "
      />
    </div>

    <!-- Sub-setting: Auto Show All Content -->
    <div
      v-if="settings.full_text_fetch_enabled"
      class="mt-2 sm:mt-3 ml-4 sm:ml-6 pl-3 sm:pl-4 border-l-2 border-border"
    >
      <div class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhPlayCircle :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-5 sm:h-5" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-xs sm:text-sm">
              {{ t('autoShowAllContent') }}
            </div>
            <div class="text-[10px] sm:text-xs text-text-secondary hidden sm:block">
              {{ t('autoShowAllContentDesc') }}
            </div>
          </div>
        </div>
        <input
          :checked="settings.auto_show_all_content"
          type="checkbox"
          class="toggle"
          @change="
            (e) =>
              emit('update:settings', {
                ...settings,
                auto_show_all_content: (e.target as HTMLInputElement).checked,
              })
          "
        />
      </div>
    </div>

    <div class="setting-item mt-2 sm:mt-3">
      <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
        <PhImages :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
        <div class="flex-1 min-w-0">
          <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
            {{ t('imageGalleryEnabled') }}
          </div>
          <div class="text-xs text-text-secondary hidden sm:block">
            {{ t('imageGalleryEnabledDesc') }}
          </div>
        </div>
      </div>
      <input
        :checked="settings.image_gallery_enabled"
        type="checkbox"
        class="toggle"
        @change="
          (e) =>
            emit('update:settings', {
              ...settings,
              image_gallery_enabled: (e.target as HTMLInputElement).checked,
            })
        "
      />
    </div>

    <div class="setting-item mt-2 sm:mt-3">
      <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
        <PhCursorClick :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
        <div class="flex-1 min-w-0">
          <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
            {{ t('hoverMarkAsRead') }}
          </div>
          <div class="text-xs text-text-secondary hidden sm:block">
            {{ t('hoverMarkAsReadDesc') }}
          </div>
        </div>
      </div>
      <input
        :checked="settings.hover_mark_as_read"
        type="checkbox"
        class="toggle"
        @change="
          (e) =>
            emit('update:settings', {
              ...settings,
              hover_mark_as_read: (e.target as HTMLInputElement).checked,
            })
        "
      />
    </div>

    <div class="setting-item mt-2 sm:mt-3">
      <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
        <PhEyeSlash :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
        <div class="flex-1 min-w-0">
          <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
            {{ t('showHiddenArticles') }}
          </div>
          <div class="text-xs text-text-secondary hidden sm:block">
            {{ t('showHiddenArticlesDesc') }}
          </div>
        </div>
      </div>
      <input
        :checked="settings.show_hidden_articles"
        type="checkbox"
        class="toggle"
        @change="
          (e) =>
            emit('update:settings', {
              ...settings,
              show_hidden_articles: (e.target as HTMLInputElement).checked,
            })
        "
      />
    </div>

    <!-- Custom CSS Setting -->
    <div class="setting-item mt-2 sm:mt-3">
      <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
        <PhPalette :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
        <div class="flex-1 min-w-0">
          <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
            {{ t('customCSS') }}
          </div>
          <div class="text-xs text-text-secondary hidden sm:block">
            {{ t('customCSSDesc') }}
          </div>
          <div v-if="hasCustomCSS" class="flex items-center gap-1 mt-1">
            <PhCheck :size="14" class="text-green-500" />
            <span class="text-xs text-text-secondary">{{ t('customCSSApplied') }}</span>
          </div>
          <!-- Documentation Link -->
          <button
            type="button"
            class="text-xs text-accent hover:underline flex items-center gap-1 mt-1"
            @click="openDocumentation"
          >
            <PhBookOpen :size="12" />
            {{ t('customCSSGuide') }}
          </button>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <button
          v-if="!hasCustomCSS"
          class="btn-secondary"
          :disabled="uploading"
          @click="handleFileUpload"
        >
          <PhUpload v-if="!uploading" :size="16" class="sm:w-5 sm:h-5" />
          <span class="hidden sm:inline">{{ uploading ? t('uploading') : t('uploadCSS') }}</span>
        </button>
        <button
          v-if="hasCustomCSS"
          class="btn-danger"
          :disabled="deleteLoading"
          @click="handleDeleteCSS"
        >
          <PhTrash v-if="!deleteLoading" :size="16" class="sm:w-5 sm:h-5" />
          <span class="hidden sm:inline">{{ deleteLoading ? t('deleting') : t('deleteCSS') }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../../style.css";

.input-field {
  @apply p-1.5 sm:p-2.5 border border-border rounded-md bg-bg-secondary text-text-primary focus:border-accent focus:outline-none transition-colors;
}
.toggle {
  @apply w-10 h-5 appearance-none bg-bg-tertiary rounded-full relative cursor-pointer border border-border transition-colors checked:bg-accent checked:border-accent shrink-0;
}
.toggle::after {
  content: '';
  @apply absolute top-0.5 left-0.5 w-3.5 h-3.5 bg-white rounded-full shadow-sm transition-transform;
}
.toggle:checked::after {
  transform: translateX(20px);
}
.setting-item {
  @apply flex items-center sm:items-start justify-between gap-2 sm:gap-4 p-2 sm:p-3 rounded-lg bg-bg-secondary border border-border;
}
.sub-setting-item {
  @apply flex items-center sm:items-start justify-between gap-2 sm:gap-4 p-2 sm:p-2.5 rounded-md bg-bg-tertiary;
}
.btn-secondary {
  @apply bg-bg-tertiary border border-border text-text-primary px-3 sm:px-4 py-1.5 sm:py-2 rounded-md cursor-pointer flex items-center gap-1.5 sm:gap-2 font-medium hover:bg-bg-secondary transition-colors disabled:opacity-50 disabled:cursor-not-allowed;
}
.btn-danger {
  @apply bg-bg-tertiary border border-border text-red-500 px-3 sm:px-4 py-1.5 sm:py-2 rounded-md cursor-pointer flex items-center gap-1.5 sm:gap-2 font-medium hover:bg-bg-secondary transition-colors disabled:opacity-50 disabled:cursor-not-allowed;
}
</style>
