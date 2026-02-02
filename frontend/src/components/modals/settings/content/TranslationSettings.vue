<script setup lang="ts">
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  PhGlobe,
  PhTranslate,
  PhList,
  PhLink,
  PhPackage,
  PhSliders,
  PhCode,
  PhTrash,
  PhBroom,
  PhTimer,
  PhRobot,
  PhKey,
} from '@phosphor-icons/vue';
import {
  SettingGroup,
  SettingWithToggle,
  NestedSettingsContainer,
  SubSettingItem,
  TextAreaControl,
  TipBox,
  ToggleControl,
  KeyValueList,
} from '@/components/settings';
import '@/components/settings/styles.css';
import type { SettingsData } from '@/types/settings';

const { t } = useI18n();

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();

function updateSetting(key: keyof SettingsData, value: any) {
  emit('update:settings', {
    ...props.settings,
    [key]: value,
  });
}

const isClearingCache = ref(false);
const showCustomTemplates = ref(false);

// Preset templates for common translation services
const customTemplates = [
  {
    name: 'DeepLX',
    endpoint: 'http://localhost:8080/translate',
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    bodyTemplate: '{"text": "%text%", "source_lang": "auto", "target_lang": "%target_lang%"}',
    responsePath: 'data',
  },
  {
    name: 'LibreTranslate',
    endpoint: 'https://libretranslate.com/translate',
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    bodyTemplate: '{"q": "%text%", "source": "auto", "target": "%target_lang%", "format": "text"}',
    responsePath: 'translatedText',
  },
  {
    name: 'Argos Translate',
    endpoint: 'https://translate.argosopentech.com/translate',
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    bodyTemplate: '{"q": "%text%", "source": "auto", "target": "%target_lang%"}',
    responsePath: 'translatedText',
  },
];

function applyTemplate(template: (typeof customTemplates)[0]) {
  // Convert placeholders back to {{}} format
  const bodyTemplate = template.bodyTemplate
    .replace(/%text%/g, '{{text}}')
    .replace(/%target_lang%/g, '{{target_lang}}')
    .replace(/%source_lang%/g, '{{source_lang}}');

  emit('update:settings', {
    ...props.settings,
    custom_translation_endpoint: template.endpoint,
    custom_translation_method: template.method,
    custom_translation_headers: JSON.stringify(template.headers),
    custom_translation_body_template: bodyTemplate,
    custom_translation_response_path: template.responsePath,
  });
  showCustomTemplates.value = false;
}

async function clearTranslationCache() {
  const confirmed = await window.showConfirm({
    title: t('setting.content.clearTranslationCache'),
    message: t('setting.content.clearTranslationCacheConfirm'),
    isDanger: true,
  });
  if (!confirmed) return;

  isClearingCache.value = true;
  try {
    const response = await fetch('/api/articles/clear-translations', {
      method: 'POST',
    });

    if (response.ok) {
      window.showToast(t('setting.content.clearTranslationCacheSuccess'), 'success');
      // Refresh article list to show updated translations
      window.dispatchEvent(new CustomEvent('refresh-articles'));
    } else {
      console.error('Server error:', response.status);
      window.showToast(t('setting.content.clearTranslationCacheFailed'), 'error');
    }
  } catch (error) {
    console.error('Failed to clear translation cache:', error);
    window.showToast(t('setting.content.clearTranslationCacheFailed'), 'error');
  } finally {
    isClearingCache.value = false;
  }
}

// Helper for validation error styling
const getErrorClass = (condition: boolean) => (condition ? 'border-red-500' : '');
</script>

<template>
  <SettingGroup :icon="PhGlobe" :title="t('setting.content.translation')">
    <SettingWithToggle
      :icon="PhTranslate"
      :title="t('setting.content.enableTranslation')"
      :description="t('setting.content.enableTranslationDesc')"
      :model-value="settings.translation_enabled"
      @update:model-value="updateSetting('translation_enabled', $event)"
    />

    <NestedSettingsContainer v-if="settings.translation_enabled">
      <SubSettingItem
        :icon="PhTranslate"
        :title="t('setting.content.translationOnlyMode')"
        :description="t('setting.content.translationOnlyModeDesc')"
      >
        <ToggleControl
          :model-value="settings.translation_only_mode"
          @update:model-value="updateSetting('translation_only_mode', $event)"
        />
      </SubSettingItem>

      <SubSettingItem
        :icon="PhPackage"
        :title="t('setting.content.translationProvider')"
        :description="t('setting.content.translationProviderDesc')"
      >
        <select
          :value="settings.translation_provider"
          class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          @change="
            updateSetting('translation_provider', ($event.target as HTMLSelectElement).value)
          "
        >
          <option value="google">{{ t('setting.content.googleTranslate') }}</option>
          <option value="deepl">{{ t('setting.content.deeplApi') }}</option>
          <option value="baidu">{{ t('setting.content.baiduTranslate') }}</option>
          <option value="ai">{{ t('setting.content.aiTranslation') }}</option>
          <option value="custom">{{ t('setting.translation.custom.title') }}</option>
        </select>
      </SubSettingItem>

      <!-- Google Translate Endpoint -->
      <SubSettingItem
        v-if="settings.translation_provider === 'google'"
        :icon="PhLink"
        :title="t('setting.content.googleTranslateEndpoint')"
        :description="t('setting.content.googleTranslateEndpointDesc')"
      >
        <select
          :value="settings.google_translate_endpoint"
          class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          @change="
            updateSetting('google_translate_endpoint', ($event.target as HTMLSelectElement).value)
          "
        >
          <option value="translate.googleapis.com">
            {{ t('setting.content.googleTranslateEndpointDefault') }}
          </option>
          <option value="clients5.google.com">
            {{ t('setting.content.googleTranslateEndpointAlternate') }}
          </option>
        </select>
      </SubSettingItem>

      <!-- DeepL API Key -->
      <SubSettingItem
        v-if="settings.translation_provider === 'deepl'"
        :icon="PhKey"
        :title="t('setting.content.deeplApiKey')"
        :description="t('setting.content.deeplApiKeyDesc')"
        :required="!settings.deepl_endpoint?.trim()"
      >
        <input
          :value="settings.deepl_api_key"
          type="password"
          :placeholder="t('setting.content.deeplApiKeyPlaceholder')"
          :class="[
            'input-field w-32 sm:w-48 text-xs sm:text-sm',
            getErrorClass(
              settings.translation_provider === 'deepl' &&
                !settings.deepl_api_key?.trim() &&
                !settings.deepl_endpoint?.trim()
            ),
          ]"
          @input="updateSetting('deepl_api_key', ($event.target as HTMLInputElement).value)"
        />
      </SubSettingItem>

      <!-- DeepL Custom Endpoint (deeplx) -->
      <SubSettingItem
        v-if="settings.translation_provider === 'deepl'"
        :icon="PhLink"
        :title="t('setting.content.deeplEndpoint')"
        :description="t('setting.content.deeplEndpointDesc')"
      >
        <input
          :value="settings.deepl_endpoint"
          type="text"
          :placeholder="t('setting.content.deeplEndpointPlaceholder')"
          class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          @input="updateSetting('deepl_endpoint', ($event.target as HTMLInputElement).value)"
        />
      </SubSettingItem>

      <!-- Baidu Translate Settings -->
      <template v-if="settings.translation_provider === 'baidu'">
        <SubSettingItem
          :icon="PhKey"
          :title="t('setting.content.baiduAppId')"
          :description="t('setting.content.baiduAppIdDesc')"
          required
        >
          <input
            :value="settings.baidu_app_id"
            type="text"
            :placeholder="t('setting.content.baiduAppIdPlaceholder')"
            :class="[
              'input-field w-32 sm:w-48 text-xs sm:text-sm',
              getErrorClass(
                settings.translation_provider === 'baidu' && !settings.baidu_app_id?.trim()
              ),
            ]"
            @input="updateSetting('baidu_app_id', ($event.target as HTMLInputElement).value)"
          />
        </SubSettingItem>

        <SubSettingItem
          :icon="PhKey"
          :title="t('setting.content.baiduSecretKey')"
          :description="t('setting.content.baiduSecretKeyDesc')"
          required
        >
          <input
            :value="settings.baidu_secret_key"
            type="password"
            :placeholder="t('setting.content.baiduSecretKeyPlaceholder')"
            :class="[
              'input-field w-32 sm:w-48 text-xs sm:text-sm',
              getErrorClass(
                settings.translation_provider === 'baidu' && !settings.baidu_secret_key?.trim()
              ),
            ]"
            @input="updateSetting('baidu_secret_key', ($event.target as HTMLInputElement).value)"
          />
        </SubSettingItem>
      </template>

      <!-- AI Translation Prompt -->
      <template v-if="settings.translation_provider === 'ai'">
        <TipBox type="info" :title="t('common.aiSettingsConfiguredInAITab')" />

        <div class="sub-setting-item-col">
          <div class="flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhRobot :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-xs sm:text-sm">
                {{ t('setting.content.aiTranslationPrompt') }}
              </div>
              <div class="text-[10px] sm:text-xs text-text-secondary hidden sm:block">
                {{ t('setting.content.aiTranslationPromptDesc') }}
              </div>
            </div>
          </div>
          <TextAreaControl
            :model-value="settings.ai_translation_prompt"
            :placeholder="t('setting.content.aiTranslationPromptPlaceholder')"
            :rows="3"
            @update:model-value="updateSetting('ai_translation_prompt', $event)"
          />
        </div>
      </template>

      <!-- Custom Translation Provider -->
      <template v-if="settings.translation_provider === 'custom'">
        <!-- Template Selection -->
        <div class="sub-setting-item">
          <div class="flex items-center sm:items-start justify-between gap-2 sm:gap-4 w-full">
            <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
              <PhList :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
              <div class="flex-1 min-w-0">
                <div class="font-medium mb-0 sm:mb-1 text-xs sm:text-sm">
                  {{ t('setting.translation.custom.template') }}
                </div>
                <div class="text-[10px] sm:text-xs text-text-secondary hidden sm:block">
                  {{ t('setting.translation.custom.templateDesc') }}
                </div>
              </div>
            </div>
            <div class="relative shrink-0">
              <button
                type="button"
                class="btn-secondary"
                @click="showCustomTemplates = !showCustomTemplates"
              >
                {{ t('setting.content.custom.selectTemplate') || 'Select Template' }}
              </button>
              <div
                v-if="showCustomTemplates"
                class="absolute top-full right-0 mt-1 z-50 bg-bg-secondary border border-border rounded-lg shadow-lg overflow-hidden"
              >
                <button
                  v-for="tmpl in customTemplates"
                  :key="tmpl.name"
                  type="button"
                  class="w-full px-4 py-2 text-left hover:bg-bg-tertiary text-sm"
                  @click="applyTemplate(tmpl)"
                >
                  {{ tmpl.name }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Custom Translation Endpoint -->
        <SubSettingItem
          :icon="PhLink"
          :title="t('setting.translation.custom.endpoint')"
          :description="t('setting.translation.custom.endpointDesc')"
          required
        >
          <input
            :value="settings.custom_translation_endpoint"
            type="text"
            :placeholder="t('setting.translation.custom.endpointPlaceholder')"
            :class="[
              'input-field w-32 sm:w-48 text-xs sm:text-sm',
              getErrorClass(
                settings.translation_provider === 'custom' &&
                  !settings.custom_translation_endpoint?.trim()
              ),
            ]"
            @input="
              updateSetting(
                'custom_translation_endpoint',
                ($event.target as HTMLInputElement).value
              )
            "
          />
        </SubSettingItem>

        <!-- Custom Translation Method -->
        <SubSettingItem
          :icon="PhCode"
          :title="t('setting.translation.custom.method')"
          :description="t('setting.translation.custom.methodDesc')"
        >
          <select
            :value="settings.custom_translation_method || 'POST'"
            class="input-field w-24 sm:w-32 text-xs sm:text-sm"
            @change="
              updateSetting('custom_translation_method', ($event.target as HTMLSelectElement).value)
            "
          >
            <option value="GET">GET</option>
            <option value="POST">POST</option>
          </select>
        </SubSettingItem>

        <!-- Custom Translation Headers -->
        <div class="sub-setting-item-col">
          <div class="flex items-center gap-2 sm:gap-3">
            <PhSliders :size="20" class="text-text-secondary shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium text-sm">
                {{ t('setting.translation.custom.headers') }}
              </div>
              <div class="text-xs text-text-secondary">
                {{ t('setting.translation.custom.headersDesc') }}
              </div>
            </div>
          </div>

          <KeyValueList
            :model-value="settings.custom_translation_headers"
            :key-placeholder="t('setting.content.custom.headerName') || 'Header name'"
            :value-placeholder="t('setting.content.custom.headerValue') || 'Value'"
            :add-button-text="t('setting.content.addHeader')"
            :remove-button-title="t('common.action.remove') || 'Remove'"
            ascii-only
            @update:model-value="updateSetting('custom_translation_headers', $event)"
          />
        </div>

        <!-- Custom Translation Body Template -->
        <div
          v-if="(settings.custom_translation_method || 'POST') === 'POST'"
          class="sub-setting-item flex-col items-stretch gap-2"
        >
          <div class="flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhCode :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">
                {{ t('setting.translation.custom.bodyTemplate') }}
                <span class="text-red-500">*</span>
              </div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{ t('setting.translation.custom.bodyTemplateDesc') }}
              </div>
            </div>
          </div>
          <textarea
            :value="settings.custom_translation_body_template"
            class="input-field w-full text-xs sm:text-sm font-mono resize-none"
            rows="4"
            placeholder="Enter request body template"
            @input="
              updateSetting(
                'custom_translation_body_template',
                ($event.target as HTMLTextAreaElement).value
              )
            "
          />
        </div>

        <!-- Custom Translation Response Path -->
        <SubSettingItem
          :icon="PhCode"
          :title="t('setting.translation.custom.responsePath')"
          :description="t('setting.translation.custom.responsePathDesc')"
          required
        >
          <input
            :value="settings.custom_translation_response_path"
            type="text"
            :placeholder="t('setting.translation.custom.responsePathPlaceholder')"
            :class="[
              'input-field w-32 sm:w-48 text-xs sm:text-sm font-mono',
              getErrorClass(
                settings.translation_provider === 'custom' &&
                  !settings.custom_translation_response_path?.trim()
              ),
            ]"
            @input="
              updateSetting(
                'custom_translation_response_path',
                ($event.target as HTMLInputElement).value
              )
            "
          />
        </SubSettingItem>

        <!-- Custom Translation Language Mapping -->
        <div class="sub-setting-item-col">
          <div class="flex items-center gap-2 sm:gap-3">
            <PhGlobe :size="20" class="text-text-secondary shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium text-sm">
                {{ t('setting.translation.custom.langMapping') }}
              </div>
              <div class="text-xs text-text-secondary">
                {{ t('setting.translation.custom.langMappingDesc') }}
              </div>
            </div>
          </div>

          <KeyValueList
            :model-value="settings.custom_translation_lang_mapping"
            :key-placeholder="
              t('setting.content.custom.mrssLangCode') || 'MrRSS code (en, zh, ...)'
            "
            :value-placeholder="t('setting.content.apiLangCode') || 'API code'"
            :add-button-text="t('setting.content.addLangMapping')"
            :remove-button-title="t('common.action.remove') || 'Remove'"
            @update:model-value="updateSetting('custom_translation_lang_mapping', $event)"
          />
        </div>

        <!-- Custom Translation Timeout -->
        <SubSettingItem
          :icon="PhTimer"
          :title="t('setting.translation.custom.timeout')"
          :description="t('setting.translation.custom.timeoutDesc')"
        >
          <div class="flex items-center gap-1 sm:gap-2 shrink-0">
            <input
              :value="settings.custom_translation_timeout || 10"
              type="number"
              min="1"
              max="60"
              class="input-field w-14 sm:w-20 text-center text-xs sm:text-sm"
              @input="
                updateSetting(
                  'custom_translation_timeout',
                  parseInt(($event.target as HTMLInputElement).value) || 10
                )
              "
            />
            <span class="text-xs sm:text-sm text-text-secondary">{{
              t('common.time.seconds')
            }}</span>
          </div>
        </SubSettingItem>
      </template>

      <SubSettingItem
        :icon="PhGlobe"
        :title="t('setting.content.targetLanguage')"
        :description="t('setting.content.targetLanguageDesc')"
      >
        <select
          :value="settings.target_language"
          class="input-field w-24 sm:w-48 text-xs sm:text-sm"
          @change="updateSetting('target_language', ($event.target as HTMLSelectElement).value)"
        >
          <option value="en">{{ t('common.language.english') }}</option>
          <option value="es">{{ t('common.language.spanish') }}</option>
          <option value="fr">{{ t('common.language.french') }}</option>
          <option value="de">{{ t('common.language.german') }}</option>
          <option value="zh">{{ t('common.language.simplifiedChinese') }}</option>
          <option value="zh-TW">{{ t('common.language.traditionalChinese') }}</option>
          <option value="ja">{{ t('common.language.japanese') }}</option>
        </select>
      </SubSettingItem>

      <!-- Cache Management -->
      <SubSettingItem
        :icon="PhTrash"
        :title="t('setting.content.clearTranslationCache')"
        :description="t('setting.content.clearTranslationCacheDesc')"
      >
        <button
          type="button"
          :disabled="isClearingCache"
          class="btn-secondary"
          @click="clearTranslationCache"
        >
          <PhBroom :size="16" class="sm:w-5 sm:h-5" />
          {{
            isClearingCache
              ? t('setting.database.cleaning')
              : t('setting.content.clearTranslationCacheButton')
          }}
        </button>
      </SubSettingItem>
    </NestedSettingsContainer>
  </SettingGroup>
</template>

<style scoped>
@reference "../../../../style.css";

.input-field {
  @apply p-1.5 sm:p-2.5 border border-border rounded-md bg-bg-secondary text-text-primary focus:border-accent focus:outline-none transition-colors;
}
</style>
