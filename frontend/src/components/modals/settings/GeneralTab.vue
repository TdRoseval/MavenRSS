<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { toRef } from 'vue';
import {
  PhPalette,
  PhMoon,
  PhTranslate,
  PhArticle,
  PhArrowClockwise,
  PhClock,
  PhCalendarCheck,
  PhPower,
  PhDatabase,
  PhBroom,
  PhHardDrive,
  PhCalendarX,
  PhEyeSlash,
  PhGlobe,
  PhPackage,
  PhKey,
} from '@phosphor-icons/vue';
import type { SettingsData } from '@/types/settings';
import { useSettingsAutoSave } from '@/composables/core/useSettingsAutoSave';
import { formatRelativeTime } from '@/utils/date';

const { t } = useI18n();

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();

// Use composable for auto-save with reactivity
const settingsRef = toRef(props, 'settings');
useSettingsAutoSave(settingsRef);

// Format last update time using shared utility
function formatLastUpdate(timestamp: string): string {
  return formatRelativeTime(timestamp, props.settings.language, t);
}
</script>

<template>
  <div class="space-y-4 sm:space-y-6">
    <div class="setting-group">
      <label
        class="font-semibold mb-2 sm:mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2"
      >
        <PhPalette :size="14" class="sm:w-4 sm:h-4" />
        {{ t('appearance') }}
      </label>
      <div class="setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhMoon :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">{{ t('theme') }}</div>
            <div class="text-xs text-text-secondary hidden sm:block">{{ t('themeDesc') }}</div>
          </div>
        </div>
        <select v-model="settings.theme" class="input-field w-24 sm:w-48 text-xs sm:text-sm">
          <option value="light">{{ t('light') }}</option>
          <option value="dark">{{ t('dark') }}</option>
          <option value="auto">{{ t('auto') }}</option>
        </select>
      </div>
      <div class="setting-item mt-2 sm:mt-3">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhTranslate :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">{{ t('language') }}</div>
            <div class="text-xs text-text-secondary hidden sm:block">{{ t('languageDesc') }}</div>
          </div>
        </div>
        <select v-model="settings.language" class="input-field w-24 sm:w-48 text-xs sm:text-sm">
          <option value="en-US">{{ t('english') }}</option>
          <option value="zh-CN">{{ t('chinese') }}</option>
        </select>
      </div>
      <div class="setting-item mt-2 sm:mt-3">
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
          v-model="settings.default_view_mode"
          class="input-field w-24 sm:w-48 text-xs sm:text-sm"
        >
          <option value="original">{{ t('viewModeOriginal') }}</option>
          <option value="rendered">{{ t('viewModeRendered') }}</option>
        </select>
      </div>
    </div>

    <div class="setting-group">
      <label
        class="font-semibold mb-2 sm:mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2"
      >
        <PhArrowClockwise :size="14" class="sm:w-4 sm:h-4" />
        {{ t('updates') }}
      </label>
      <div class="setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhClock :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
              {{ t('autoUpdateInterval') }}
            </div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('autoUpdateIntervalDesc') }}
            </div>
          </div>
        </div>
        <input
          type="number"
          v-model="settings.update_interval"
          min="1"
          class="input-field w-16 sm:w-20 text-center text-xs sm:text-sm"
        />
      </div>

      <!-- Last update time - read-only info display -->
      <div class="info-display mt-2 sm:mt-3">
        <div class="flex items-center gap-2">
          <PhCalendarCheck :size="18" class="text-text-secondary shrink-0 sm:w-5 sm:h-5" />
          <div class="flex-1 min-w-0">
            <div class="text-xs sm:text-sm text-text-secondary truncate">
              {{ t('lastArticleUpdate') }}
            </div>
          </div>
          <div class="text-xs sm:text-sm font-medium text-accent shrink-0">
            {{ formatLastUpdate(settings.last_article_update) }}
          </div>
        </div>
      </div>

      <div class="setting-item mt-2 sm:mt-3">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhPower :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
              {{ t('startupOnBoot') }}
            </div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('startupOnBootDesc') }}
            </div>
          </div>
        </div>
        <input type="checkbox" v-model="settings.startup_on_boot" class="toggle" />
      </div>
    </div>

    <div class="setting-group">
      <label
        class="font-semibold mb-2 sm:mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2"
      >
        <PhDatabase :size="14" class="sm:w-4 sm:h-4" />
        {{ t('database') }}
      </label>
      <div class="setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhBroom :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">{{ t('autoCleanup') }}</div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('autoCleanupDesc') }}
            </div>
          </div>
        </div>
        <input type="checkbox" v-model="settings.auto_cleanup_enabled" class="toggle" />
      </div>

      <div
        v-if="settings.auto_cleanup_enabled"
        class="ml-2 sm:ml-4 mt-2 sm:mt-3 space-y-2 sm:space-y-3 border-l-2 border-border pl-2 sm:pl-4"
      >
        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhHardDrive :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('maxCacheSize') }}</div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{ t('maxCacheSizeDesc') }}
              </div>
            </div>
          </div>
          <div class="flex items-center gap-1 sm:gap-2 shrink-0">
            <input
              type="number"
              v-model="settings.max_cache_size_mb"
              min="1"
              max="1000"
              class="input-field w-14 sm:w-20 text-center text-xs sm:text-sm"
            />
            <span class="text-xs sm:text-sm text-text-secondary">MB</span>
          </div>
        </div>

        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhCalendarX :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('maxArticleAge') }}</div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{ t('maxArticleAgeDesc') }}
              </div>
            </div>
          </div>
          <div class="flex items-center gap-1 sm:gap-2 shrink-0">
            <input
              type="number"
              v-model="settings.max_article_age_days"
              min="1"
              max="365"
              class="input-field w-14 sm:w-20 text-center text-xs sm:text-sm"
            />
            <span class="text-xs sm:text-sm text-text-secondary">{{ t('days') }}</span>
          </div>
        </div>
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
        <input type="checkbox" v-model="settings.show_hidden_articles" class="toggle" />
      </div>
    </div>

    <div class="setting-group">
      <label
        class="font-semibold mb-2 sm:mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2"
      >
        <PhGlobe :size="14" class="sm:w-4 sm:h-4" />
        {{ t('translation') }}
      </label>
      <div class="setting-item mb-2 sm:mb-4">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhArticle :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
              {{ t('enableTranslation') }}
            </div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('enableTranslationDesc') }}
            </div>
          </div>
        </div>
        <input type="checkbox" v-model="settings.translation_enabled" class="toggle" />
      </div>

      <div
        v-if="settings.translation_enabled"
        class="ml-2 sm:ml-4 space-y-2 sm:space-y-3 border-l-2 border-border pl-2 sm:pl-4"
      >
        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhPackage :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('translationProvider') }}</div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{ t('translationProviderDesc') || 'Choose the translation service to use' }}
              </div>
            </div>
          </div>
          <select
            v-model="settings.translation_provider"
            class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          >
            <option value="google">Google Translate (Free)</option>
            <option value="deepl">DeepL API</option>
          </select>
        </div>

        <div v-if="settings.translation_provider === 'deepl'" class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhKey :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('deeplApiKey') }}</div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{ t('deeplApiKeyDesc') || 'Enter your DeepL API key' }}
              </div>
            </div>
          </div>
          <input
            type="password"
            v-model="settings.deepl_api_key"
            :placeholder="t('deeplApiKeyPlaceholder')"
            class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          />
        </div>

        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhGlobe :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('targetLanguage') }}</div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{ t('targetLanguageDesc') || 'Language to translate article titles to' }}
              </div>
            </div>
          </div>
          <select
            v-model="settings.target_language"
            class="input-field w-24 sm:w-48 text-xs sm:text-sm"
          >
            <option value="en">{{ t('english') }}</option>
            <option value="es">{{ t('spanish') }}</option>
            <option value="fr">{{ t('french') }}</option>
            <option value="de">{{ t('german') }}</option>
            <option value="zh">{{ t('chinese') }}</option>
            <option value="ja">{{ t('japanese') }}</option>
          </select>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
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
.info-display {
  @apply px-2 sm:px-3 py-1.5 sm:py-2 rounded-lg border border-border;
  background-color: rgba(233, 236, 239, 0.3);
}
.dark-mode .info-display {
  background-color: rgba(45, 45, 45, 0.3);
}
</style>
