<script setup lang="ts">
import { ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  PhGlobe,
  PhTranslate,
  PhList,
  PhLink,
  PhSliders,
  PhCode,
  PhInfo,
  PhTrash,
  PhBroom,
  PhPlus,
  PhTimer,
  PhRobot,
  PhKey,
} from '@phosphor-icons/vue';
import type { SettingsData } from '@/types/settings';

const { t } = useI18n();

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();

const isClearingCache = ref(false);
const showCustomTemplates = ref(false);

// Custom headers state (similar to AI settings)
interface Header {
  id: string;
  name: string;
  value: string;
}

const customHeaders = ref<Header[]>([]);
let saveTimeout: ReturnType<typeof setTimeout> | null = null;

function loadCustomHeaders() {
  const jsonString = props.settings.custom_translation_headers || '';
  customHeaders.value = parseCustomHeaders(jsonString);
}

function parseCustomHeaders(jsonString: string): Header[] {
  if (!jsonString || jsonString.trim() === '') return [];
  try {
    const parsed = JSON.parse(jsonString) as Record<string, string>;
    return Object.entries(parsed).map(([name, value], index) => ({
      id: `${Date.now()}-${index}`,
      name,
      value,
    }));
  } catch {
    return [];
  }
}

function addCustomHeader() {
  customHeaders.value.push({
    id: `${Date.now()}`,
    name: '',
    value: '',
  });
  saveCustomHeaders();
}

function removeCustomHeader(id: string) {
  customHeaders.value = customHeaders.value.filter((h) => h.id !== id);
  saveCustomHeaders();
}

function saveCustomHeaders() {
  if (saveTimeout) clearTimeout(saveTimeout);
  saveTimeout = setTimeout(() => {
    const obj: Record<string, string> = {};
    customHeaders.value.forEach(({ name, value }) => {
      if (name && value) {
        obj[name] = value;
      }
    });
    emit('update:settings', {
      ...props.settings,
      custom_translation_headers: JSON.stringify(obj),
    });
    saveTimeout = null;
  }, 500);
}

// Language mapping state (similar pattern)
interface LangMapping {
  id: string;
  key: string;
  value: string;
}

const customLangMapping = ref<LangMapping[]>([]);
let saveLangTimeout: ReturnType<typeof setTimeout> | null = null;

function loadCustomLangMapping() {
  const jsonString = props.settings.custom_translation_lang_mapping || '';
  customLangMapping.value = parseCustomLangMapping(jsonString);
}

function parseCustomLangMapping(jsonString: string): LangMapping[] {
  if (!jsonString || jsonString.trim() === '') return [];
  try {
    const parsed = JSON.parse(jsonString) as Record<string, string>;
    return Object.entries(parsed).map(([key, value], index) => ({
      id: `${Date.now()}-${index}`,
      key,
      value,
    }));
  } catch {
    return [];
  }
}

function addCustomLangMapping() {
  customLangMapping.value.push({
    id: `${Date.now()}`,
    key: '',
    value: '',
  });
  saveCustomLangMapping();
}

function removeCustomLangMapping(id: string) {
  customLangMapping.value = customLangMapping.value.filter((m) => m.id !== id);
  saveCustomLangMapping();
}

function saveCustomLangMapping() {
  if (saveLangTimeout) clearTimeout(saveLangTimeout);
  saveLangTimeout = setTimeout(() => {
    const obj: Record<string, string> = {};
    customLangMapping.value.forEach(({ key, value }) => {
      if (key && value) {
        obj[key] = value;
      }
    });
    emit('update:settings', {
      ...props.settings,
      custom_translation_lang_mapping: JSON.stringify(obj),
    });
    saveLangTimeout = null;
  }, 500);
}

// Watch for external changes
watch(
  () => props.settings.custom_translation_headers,
  (newValue, oldValue) => {
    if (newValue !== oldValue) {
      const parsed = parseCustomHeaders(newValue || '');
      const currentIds = new Set(customHeaders.value.map((h) => h.id));
      const hasNewEntries = parsed.some((p) => !currentIds.has(p.id));
      if (hasNewEntries || parsed.length !== customHeaders.value.length) {
        customHeaders.value = parsed;
      }
    }
  }
);

watch(
  () => props.settings.custom_translation_lang_mapping,
  (newValue, oldValue) => {
    if (newValue !== oldValue) {
      const parsed = parseCustomLangMapping(newValue || '');
      const currentIds = new Set(customLangMapping.value.map((m) => m.id));
      const hasNewEntries = parsed.some((p) => !currentIds.has(p.id));
      if (hasNewEntries || parsed.length !== customLangMapping.value.length) {
        customLangMapping.value = parsed;
      }
    }
  }
);

// Load initial data
loadCustomHeaders();
loadCustomLangMapping();

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
    title: t('clearTranslationCache'),
    message: t('clearTranslationCacheConfirm'),
    isDanger: true,
  });
  if (!confirmed) return;

  isClearingCache.value = true;
  try {
    const response = await fetch('/api/articles/clear-translations', {
      method: 'POST',
    });

    if (response.ok) {
      window.showToast(t('clearTranslationCacheSuccess'), 'success');
      // Refresh article list to show updated translations
      window.dispatchEvent(new CustomEvent('refresh-articles'));
    } else {
      console.error('Server error:', response.status);
      window.showToast(t('clearTranslationCacheFailed'), 'error');
    }
  } catch (error) {
    console.error('Failed to clear translation cache:', error);
    window.showToast(t('clearTranslationCacheFailed'), 'error');
  } finally {
    isClearingCache.value = false;
  }
}
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
        <PhTranslate :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
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
          <PhTranslate :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('translationOnlyMode') }}</div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('translationOnlyModeDesc') }}
            </div>
          </div>
        </div>
        <input
          :checked="props.settings.translation_only_mode"
          type="checkbox"
          class="toggle"
          @change="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                translation_only_mode: (e.target as HTMLInputElement).checked,
              })
          "
        />
      </div>

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
            (e) => {
              emit('update:settings', {
                ...props.settings,
                translation_provider: (e.target as HTMLSelectElement).value,
              });
            }
          "
        >
          <option value="google">{{ t('googleTranslate') }}</option>
          <option value="deepl">{{ t('deeplApi') }}</option>
          <option value="baidu">{{ t('baiduTranslate') }}</option>
          <option value="ai">{{ t('aiTranslation') }}</option>
          <option value="custom">{{ t('customTranslation') }}</option>
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

      <!-- Custom Translation Provider -->
      <template v-if="props.settings.translation_provider === 'custom'">
        <!-- Template Selection -->
        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhList :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">
                {{ t('customTranslationTemplate') || 'Preset Templates' }}
              </div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{
                  t('customTranslationTemplateDesc') ||
                  'Quick start with pre-configured templates for popular services'
                }}
              </div>
            </div>
          </div>
          <div class="relative">
            <button
              type="button"
              class="btn-secondary"
              @click="showCustomTemplates = !showCustomTemplates"
            >
              {{ t('selectTemplate') || 'Select Template' }}
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

        <!-- Custom Translation Endpoint -->
        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhLink :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">
                {{ t('customTranslationEndpoint') || 'API Endpoint' }}
                <span class="text-red-500">*</span>
              </div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{
                  t('customTranslationEndpointDesc') ||
                  'API endpoint URL for the translation service'
                }}
              </div>
            </div>
          </div>
          <input
            :value="props.settings.custom_translation_endpoint"
            type="text"
            :placeholder="
              t('customTranslationEndpointPlaceholder') || 'https://api.example.com/translate'
            "
            :class="[
              'input-field w-32 sm:w-48 text-xs sm:text-sm',
              props.settings.translation_enabled &&
              props.settings.translation_provider === 'custom' &&
              !props.settings.custom_translation_endpoint?.trim()
                ? 'border-red-500'
                : '',
            ]"
            @input="
              (e) =>
                emit('update:settings', {
                  ...props.settings,
                  custom_translation_endpoint: (e.target as HTMLInputElement).value,
                })
            "
          />
        </div>

        <!-- Custom Translation Method -->
        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhCode :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">
                {{ t('customTranslationMethod') || 'HTTP Method' }}
              </div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{ t('customTranslationMethodDesc') || 'HTTP method for the API request' }}
              </div>
            </div>
          </div>
          <select
            :value="props.settings.custom_translation_method || 'POST'"
            class="input-field w-24 sm:w-32 text-xs sm:text-sm"
            @change="
              (e) =>
                emit('update:settings', {
                  ...props.settings,
                  custom_translation_method: (e.target as HTMLSelectElement).value,
                })
            "
          >
            <option value="GET">GET</option>
            <option value="POST">POST</option>
          </select>
        </div>

        <!-- Custom Translation Headers -->
        <div class="sub-setting-item flex-col items-stretch gap-2">
          <div class="flex items-center gap-2 sm:gap-3">
            <PhSliders :size="20" class="text-text-secondary shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium text-sm">
                {{ t('customTranslationHeaders') || 'HTTP Headers' }}
              </div>
              <div class="text-xs text-text-secondary">
                {{
                  t('customTranslationHeadersDesc') || 'Custom HTTP headers (e.g., Authorization)'
                }}
              </div>
            </div>
          </div>

          <!-- Headers List -->
          <div class="mt-2 sm:mt-3 space-y-1.5 sm:space-y-2 w-full">
            <div
              v-for="header in customHeaders"
              :key="header.id"
              class="flex items-center gap-1.5 sm:gap-2"
            >
              <input
                v-model="header.name"
                type="text"
                :placeholder="t('headerName') || 'Header name'"
                class="input-field text-xs sm:text-sm flex-1"
                @input="saveCustomHeaders()"
              />
              <input
                v-model="header.value"
                type="text"
                :placeholder="t('headerValue') || 'Value'"
                class="input-field text-xs sm:text-sm flex-1"
                @input="saveCustomHeaders()"
              />
              <button
                type="button"
                class="p-1.5 sm:p-2 rounded hover:bg-red-50 dark:hover:bg-red-900/20 text-text-secondary hover:text-red-500 transition-all shrink-0"
                :title="t('remove') || 'Remove'"
                @click="removeCustomHeader(header.id)"
              >
                <PhTrash :size="14" class="sm:w-4 sm:h-4" />
              </button>
            </div>

            <!-- Add Header Button -->
            <button
              type="button"
              class="w-full p-1.5 sm:p-2 rounded border border-dashed border-border text-text-secondary hover:border-accent hover:text-accent hover:bg-accent/5 transition-all text-xs font-medium flex items-center justify-center gap-1.5 sm:gap-2"
              @click="addCustomHeader"
            >
              <PhPlus :size="14" class="sm:w-4 sm:h-4" />
              <span>{{ t('addHeader') || 'Add Header' }}</span>
            </button>
          </div>
        </div>

        <!-- Custom Translation Body Template -->
        <div
          v-if="(props.settings.custom_translation_method || 'POST') === 'POST'"
          class="sub-setting-item flex-col items-stretch gap-2"
        >
          <div class="flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhCode :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">
                {{ t('customTranslationBodyTemplate') || 'Request Body Template' }}
                <span class="text-red-500">*</span>
              </div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{
                  t('customTranslationBodyTemplateDesc') || 'Use placeholders in your request body'
                }}
              </div>
            </div>
          </div>
          <textarea
            :value="props.settings.custom_translation_body_template"
            class="input-field w-full text-xs sm:text-sm font-mono resize-none"
            rows="4"
            placeholder="Enter request body template"
            @input="
              (e) =>
                emit('update:settings', {
                  ...props.settings,
                  custom_translation_body_template: (e.target as HTMLTextAreaElement).value,
                })
            "
          />
        </div>

        <!-- Custom Translation Response Path -->
        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhCode :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">
                {{ t('customTranslationResponsePath') || 'Response Path' }}
                <span class="text-red-500">*</span>
              </div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{
                  t('customTranslationResponsePathDesc') ||
                  'JSONPath to extract translation (e.g., data.translatedText)'
                }}
              </div>
            </div>
          </div>
          <input
            :value="props.settings.custom_translation_response_path"
            type="text"
            :placeholder="t('customTranslationResponsePathPlaceholder') || 'data'"
            :class="[
              'input-field w-32 sm:w-48 text-xs sm:text-sm font-mono',
              props.settings.translation_enabled &&
              props.settings.translation_provider === 'custom' &&
              !props.settings.custom_translation_response_path?.trim()
                ? 'border-red-500'
                : '',
            ]"
            @input="
              (e) =>
                emit('update:settings', {
                  ...props.settings,
                  custom_translation_response_path: (e.target as HTMLInputElement).value,
                })
            "
          />
        </div>

        <!-- Custom Translation Language Mapping -->
        <div class="sub-setting-item flex-col items-stretch gap-2">
          <div class="flex items-center gap-2 sm:gap-3">
            <PhGlobe :size="20" class="text-text-secondary shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium text-sm">
                {{ t('customTranslationLangMapping') || 'Language Code Mapping' }}
              </div>
              <div class="text-xs text-text-secondary">
                {{
                  t('customTranslationLangMappingDesc') ||
                  'Map MrRSS language codes to API-specific codes (optional)'
                }}
              </div>
            </div>
          </div>

          <!-- Language Mapping List -->
          <div class="mt-2 sm:mt-3 space-y-1.5 sm:space-y-2 w-full">
            <div
              v-for="mapping in customLangMapping"
              :key="mapping.id"
              class="flex items-center gap-1.5 sm:gap-2"
            >
              <input
                v-model="mapping.key"
                type="text"
                :placeholder="t('mrssLangCode') || 'MrRSS code (en, zh, ...)'"
                class="input-field text-xs sm:text-sm flex-1"
                @input="saveCustomLangMapping()"
              />
              <input
                v-model="mapping.value"
                type="text"
                :placeholder="t('apiLangCode') || 'API code'"
                class="input-field text-xs sm:text-sm flex-1"
                @input="saveCustomLangMapping()"
              />
              <button
                type="button"
                class="p-1.5 sm:p-2 rounded hover:bg-red-50 dark:hover:bg-red-900/20 text-text-secondary hover:text-red-500 transition-all shrink-0"
                :title="t('remove') || 'Remove'"
                @click="removeCustomLangMapping(mapping.id)"
              >
                <PhTrash :size="14" class="sm:w-4 sm:h-4" />
              </button>
            </div>

            <!-- Add Mapping Button -->
            <button
              type="button"
              class="w-full p-1.5 sm:p-2 rounded border border-dashed border-border text-text-secondary hover:border-accent hover:text-accent hover:bg-accent/5 transition-all text-xs font-medium flex items-center justify-center gap-1.5 sm:gap-2"
              @click="addCustomLangMapping"
            >
              <PhPlus :size="14" class="sm:w-4 sm:h-4" />
              <span>{{ t('addLangMapping') || 'Add Mapping' }}</span>
            </button>
          </div>
        </div>

        <!-- Custom Translation Timeout -->
        <div class="sub-setting-item">
          <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
            <PhTimer :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
            <div class="flex-1 min-w-0">
              <div class="font-medium mb-0 sm:mb-1 text-sm">
                {{ t('customTranslationTimeout') || 'Timeout' }}
              </div>
              <div class="text-xs text-text-secondary hidden sm:block">
                {{ t('customTranslationTimeoutDesc') || 'Maximum time to wait for API response' }}
              </div>
            </div>
          </div>
          <div class="flex items-center gap-1 sm:gap-2 shrink-0">
            <input
              :value="props.settings.custom_translation_timeout || 10"
              type="number"
              min="1"
              max="60"
              class="input-field w-14 sm:w-20 text-center text-xs sm:text-sm"
              @input="
                (e) =>
                  emit('update:settings', {
                    ...props.settings,
                    custom_translation_timeout:
                      parseInt((e.target as HTMLInputElement).value) || 10,
                  })
              "
            />
            <span class="text-xs sm:text-sm text-text-secondary">{{ t('seconds') }}</span>
          </div>
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
          <option value="zh">{{ t('simplifiedChinese') }}</option>
          <option value="zh-TW">{{ t('traditionalChinese') }}</option>
          <option value="ja">{{ t('japanese') }}</option>
        </select>
      </div>

      <!-- Cache Management -->
      <div class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhTrash :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('clearTranslationCache') }}</div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('clearTranslationCacheDesc') }}
            </div>
          </div>
        </div>
        <button
          type="button"
          :disabled="isClearingCache"
          class="btn-secondary"
          @click="clearTranslationCache"
        >
          <PhBroom :size="16" class="sm:w-5 sm:h-5" />
          {{ isClearingCache ? t('cleaning') : t('clearTranslationCacheButton') }}
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
.tip-box {
  @apply flex items-center gap-2 sm:gap-3 py-2 sm:py-2.5 px-2.5 sm:px-3 rounded-lg w-full;
  background-color: rgba(59, 130, 246, 0.05);
  border: 1px solid rgba(59, 130, 246, 0.3);
}
.btn-secondary {
  @apply bg-bg-tertiary border border-border text-text-primary px-3 sm:px-4 py-1.5 sm:py-2 rounded-md cursor-pointer flex items-center gap-1.5 sm:gap-2 font-medium hover:bg-bg-secondary transition-colors disabled:opacity-50 disabled:cursor-not-allowed;
}
</style>
