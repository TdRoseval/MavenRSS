<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { PhGlobe, PhArticle, PhPackage, PhKey, PhLink, PhRobot, PhInfo } from '@phosphor-icons/vue';
import type { SettingsData } from '@/types/settings';

const { t } = useI18n();

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();
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
      <input
        :checked="props.settings.translation_enabled"
        type="checkbox"
        class="toggle"
        @change="
          (e) =>
            emit('update:settings', {
              ...props.settings,
              translation_enabled: (e.target as HTMLInputElement).checked,
            })
        "
      />
    </div>

    <div
      v-if="props.settings.translation_enabled"
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
          :value="props.settings.translation_provider"
          class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          @change="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                translation_provider: (e.target as HTMLSelectElement).value,
              })
          "
        >
          <option value="google">{{ t('googleTranslate') }}</option>
          <option value="deepl">{{ t('deeplApi') }}</option>
          <option value="baidu">{{ t('baiduTranslate') }}</option>
          <option value="ai">{{ t('aiTranslation') }}</option>
        </select>
      </div>

      <!-- Google Translate Endpoint -->
      <div v-if="props.settings.translation_provider === 'google'" class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhLink :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('googleTranslateEndpoint') }}</div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('googleTranslateEndpointDesc') }}
            </div>
          </div>
        </div>
        <select
          :value="props.settings.google_translate_endpoint"
          class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          @change="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                google_translate_endpoint: (e.target as HTMLSelectElement).value,
              })
          "
        >
          <option value="translate.googleapis.com">
            {{ t('googleTranslateEndpointDefault') }}
          </option>
          <option value="clients5.google.com">{{ t('googleTranslateEndpointAlternate') }}</option>
        </select>
      </div>

      <!-- DeepL API Key -->
      <div v-if="props.settings.translation_provider === 'deepl'" class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhKey :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm">
              {{ t('deeplApiKey') }}
              <span v-if="!props.settings.deepl_endpoint?.trim()" class="text-red-500">*</span>
            </div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('deeplApiKeyDesc') || 'Enter your DeepL API key' }}
            </div>
          </div>
        </div>
        <input
          :value="props.settings.deepl_api_key"
          type="password"
          :placeholder="t('deeplApiKeyPlaceholder')"
          :class="[
            'input-field w-32 sm:w-48 text-xs sm:text-sm',
            props.settings.translation_enabled &&
            props.settings.translation_provider === 'deepl' &&
            !props.settings.deepl_api_key?.trim() &&
            !props.settings.deepl_endpoint?.trim()
              ? 'border-red-500'
              : '',
          ]"
          @input="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                deepl_api_key: (e.target as HTMLInputElement).value,
              })
          "
        />
      </div>

      <!-- DeepL Custom Endpoint (deeplx) -->
      <div v-if="props.settings.translation_provider === 'deepl'" class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhLink :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('deeplEndpoint') }}</div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('deeplEndpointDesc') }}
            </div>
          </div>
        </div>
        <input
          :value="props.settings.deepl_endpoint"
          type="text"
          :placeholder="t('deeplEndpointPlaceholder')"
          class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          @input="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                deepl_endpoint: (e.target as HTMLInputElement).value,
              })
          "
        />
      </div>

      <!-- Baidu Translate Settings -->
      <template v-if="props.settings.translation_provider === 'baidu'">
        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhKey :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">
                {{ t('baiduAppId') }} <span class="text-red-500">*</span>
              </div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{ t('baiduAppIdDesc') }}
              </div>
            </div>
          </div>
          <input
            :value="props.settings.baidu_app_id"
            type="text"
            :placeholder="t('baiduAppIdPlaceholder')"
            :class="[
              'input-field w-32 sm:w-48 text-xs sm:text-sm',
              props.settings.translation_enabled &&
              props.settings.translation_provider === 'baidu' &&
              !props.settings.baidu_app_id?.trim()
                ? 'border-red-500'
                : '',
            ]"
            @input="
              (e) =>
                emit('update:settings', {
                  ...props.settings,
                  baidu_app_id: (e.target as HTMLInputElement).value,
                })
            "
          />
        </div>
        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhKey :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">
                {{ t('baiduSecretKey') }} <span class="text-red-500">*</span>
              </div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{ t('baiduSecretKeyDesc') }}
              </div>
            </div>
          </div>
          <input
            :value="props.settings.baidu_secret_key"
            type="password"
            :placeholder="t('baiduSecretKeyPlaceholder')"
            :class="[
              'input-field w-32 sm:w-48 text-xs sm:text-sm',
              props.settings.translation_enabled &&
              props.settings.translation_provider === 'baidu' &&
              !props.settings.baidu_secret_key?.trim()
                ? 'border-red-500'
                : '',
            ]"
            @input="
              (e) =>
                emit('update:settings', {
                  ...props.settings,
                  baidu_secret_key: (e.target as HTMLInputElement).value,
                })
            "
          />
        </div>
      </template>

      <!-- AI Translation Prompt -->
      <div v-if="props.settings.translation_provider === 'ai'" class="tip-box">
        <PhInfo :size="16" class="text-accent shrink-0 sm:w-5 sm:h-5" />
        <span class="text-xs sm:text-sm">{{ t('aiSettingsConfiguredInAITab') }}</span>
      </div>
      <div
        v-if="props.settings.translation_provider === 'ai'"
        class="sub-setting-item flex-col items-stretch gap-2"
      >
        <div class="flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhRobot :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('aiTranslationPrompt') }}</div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('aiTranslationPromptDesc') }}
            </div>
          </div>
        </div>
        <textarea
          :value="props.settings.ai_translation_prompt"
          class="input-field w-full text-xs sm:text-sm resize-none"
          rows="3"
          :placeholder="t('aiTranslationPromptPlaceholder')"
          @input="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                ai_translation_prompt: (e.target as HTMLTextAreaElement).value,
              })
          "
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
          :value="props.settings.target_language"
          class="input-field w-24 sm:w-48 text-xs sm:text-sm"
          @change="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                target_language: (e.target as HTMLSelectElement).value,
              })
          "
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
.tip-box {
  @apply flex items-center gap-2 sm:gap-3 py-2 sm:py-2.5 px-2.5 sm:px-3 rounded-lg w-full;
  background-color: rgba(59, 130, 246, 0.05);
  border: 1px solid rgba(59, 130, 246, 0.3);
}
</style>
