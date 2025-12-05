<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { PhGlobe, PhArticle, PhPackage, PhKey, PhLink, PhRobot } from '@phosphor-icons/vue';
import type { SettingsData } from '@/types/settings';

const { t } = useI18n();

interface Props {
  settings: SettingsData;
}

defineProps<Props>();
</script>

<template>
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
          <option value="baidu">{{ t('baiduTranslate') }}</option>
          <option value="ai">{{ t('aiTranslation') }}</option>
        </select>
      </div>

      <!-- DeepL API Key -->
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

      <!-- Baidu Translate Settings -->
      <template v-if="settings.translation_provider === 'baidu'">
        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhKey :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('baiduAppId') }}</div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{ t('baiduAppIdDesc') }}
              </div>
            </div>
          </div>
          <input
            type="text"
            v-model="settings.baidu_app_id"
            :placeholder="t('baiduAppIdPlaceholder')"
            class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          />
        </div>
        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhKey :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('baiduSecretKey') }}</div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{ t('baiduSecretKeyDesc') }}
              </div>
            </div>
          </div>
          <input
            type="password"
            v-model="settings.baidu_secret_key"
            :placeholder="t('baiduSecretKeyPlaceholder')"
            class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          />
        </div>
      </template>

      <!-- AI Translation Settings -->
      <template v-if="settings.translation_provider === 'ai'">
        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhKey :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('aiApiKey') }}</div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{ t('aiApiKeyDesc') }}
              </div>
            </div>
          </div>
          <input
            type="password"
            v-model="settings.ai_api_key"
            :placeholder="t('aiApiKeyPlaceholder')"
            class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          />
        </div>
        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhLink :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('aiEndpoint') }}</div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{ t('aiEndpointDesc') }}
              </div>
            </div>
          </div>
          <input
            type="text"
            v-model="settings.ai_endpoint"
            :placeholder="t('aiEndpointPlaceholder')"
            class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          />
        </div>
        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhRobot :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('aiModel') }}</div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{ t('aiModelDesc') }}
              </div>
            </div>
          </div>
          <input
            type="text"
            v-model="settings.ai_model"
            :placeholder="t('aiModelPlaceholder')"
            class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          />
        </div>
      </template>

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
</style>
