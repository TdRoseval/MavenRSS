<script setup lang="ts">
import { ref, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  PhDatabase,
  PhPlus,
  PhPencil,
  PhTrash,
  PhCheck,
  PhX,
  PhTestTube,
  PhArrowClockwise,
} from '@phosphor-icons/vue';
import { SettingGroup } from '@/components/settings';
import type { SettingsData } from '@/types/settings';
import type { AITestInfo } from '@/types/settings';
import { authPost } from '@/shared/lib/authFetch';

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();

interface EmbeddingModel {
  modelname: string;
  baseurl: string;
  apikey: string;
  rpm: number;
  tpm: number;
  use_global_proxy: boolean;
}

const { t } = useI18n();

const models = computed<EmbeddingModel[]>(() => {
  try {
    return JSON.parse(props.settings.ai_embedding_models || '[]');
  } catch (e) {
    return [];
  }
});

const isEditing = ref(false);
const editIndex = ref(-1);
const testResults = ref<Map<string, AITestInfo>>(new Map());
const testingModels = ref<Set<string>>(new Set());
const form = ref<EmbeddingModel>({
  modelname: '',
  baseurl: '',
  apikey: '',
  rpm: 0,
  tpm: 0,
  use_global_proxy: false,
});

function saveModels(newModels: EmbeddingModel[]) {
  testResults.value.clear();
  testingModels.value.clear();
  emit('update:settings', {
    ...props.settings,
    ai_embedding_models: JSON.stringify(newModels),
  });
}

function openAdd() {
  form.value = {
    modelname: '',
    baseurl: '',
    apikey: '',
    rpm: 0,
    tpm: 0,
    use_global_proxy: false,
  };
  editIndex.value = -1;
  isEditing.value = true;
}

function openEdit(index: number) {
  form.value = { ...models.value[index] };
  editIndex.value = index;
  isEditing.value = true;
}

function deleteModel(index: number) {
  const key = getModelKey(models.value[index], index);
  testResults.value.delete(key);
  testingModels.value.delete(key);
  const newModels = [...models.value];
  newModels.splice(index, 1);
  saveModels(newModels);
}

function getModelKey(model: EmbeddingModel, index: number): string {
  return `${index}:${model.modelname}:${model.baseurl}`;
}

function getTestStatus(model: EmbeddingModel, index: number): 'success' | 'error' | 'unknown' {
  const result = testResults.value.get(getModelKey(model, index));
  if (!result) return 'unknown';
  return result.config_valid && result.connection_success ? 'success' : 'error';
}

async function handleTestModel(model: EmbeddingModel, index: number) {
  const key = getModelKey(model, index);
  testingModels.value.add(key);
  testResults.value.delete(key);

  try {
    const result = await authPost<AITestInfo>('/api/ai/embeddings/test-config', model);
    testResults.value.set(key, result);
  } catch (error) {
    console.error('Error testing embedding model:', error);
    testResults.value.set(key, {
      config_valid: false,
      connection_success: false,
      model_available: false,
      response_time_ms: 0,
      test_time: '',
      error_message: error instanceof Error ? error.message : t('setting.ai.aiTestFailed'),
    });
  } finally {
    testingModels.value.delete(key);
  }
}

function saveForm() {
  if (!form.value.modelname || !form.value.baseurl) {
    return;
  }
  const newModels = [...models.value];
  const ModelToSave = {
    ...form.value,
    rpm: Number(form.value.rpm) || 0,
    tpm: Number(form.value.tpm) || 0,
  };

  if (editIndex.value === -1) {
    newModels.push(ModelToSave);
  } else {
    newModels[editIndex.value] = ModelToSave;
  }
  saveModels(newModels);
  isEditing.value = false;
}

function cancelEdit() {
  isEditing.value = false;
}
</script>

<template>
  <SettingGroup :icon="PhDatabase" :title="t('setting.ai.embeddingModels')">
    <div class="space-y-3">
      <div class="flex flex-wrap items-center gap-2">
        <button v-if="!isEditing" type="button" class="btn-secondary" @click="openAdd">
          <PhPlus :size="16" />
          {{ t('setting.ai.addEmbeddingModel') }}
        </button>
      </div>

      <div v-if="isEditing" class="edit-form">
        <div class="flex items-center gap-2 mb-3">
          <PhDatabase :size="20" class="text-text-tertiary" />
          <span class="font-medium text-sm">
            {{
              editIndex === -1
                ? t('setting.ai.addEmbeddingModel')
                : t('setting.ai.editEmbeddingModel')
            }}
          </span>
        </div>

        <div class="space-y-3">
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label class="field-label">{{ t('setting.ai.modelName') }}</label>
              <input
                v-model="form.modelname"
                type="text"
                class="inputbox w-full"
                :placeholder="t('setting.ai.modelNamePlaceholder')"
              />
            </div>
            <div>
              <label class="field-label">{{ t('setting.ai.apiBaseUrl') }}</label>
              <input
                v-model="form.baseurl"
                type="text"
                class="inputbox w-full"
                :placeholder="t('setting.ai.apiBaseUrlPlaceholder')"
              />
            </div>
          </div>

          <div>
            <label class="field-label">{{ t('setting.ai.apiKey') }}</label>
            <input
              v-model="form.apikey"
              type="password"
              class="inputbox w-full"
              :placeholder="t('setting.ai.apiKeyPlaceholder')"
            />
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label class="field-label">{{ t('setting.ai.rpm') }}</label>
              <input v-model.number="form.rpm" type="number" class="inputbox w-full" min="0" />
            </div>
            <div>
              <label class="field-label">{{ t('setting.ai.tpm') }}</label>
              <input v-model.number="form.tpm" type="number" class="inputbox w-full" min="0" />
            </div>
          </div>

          <div class="flex items-center">
            <input
              id="use-global-proxy"
              v-model="form.use_global_proxy"
              type="checkbox"
              class="toggle"
            />
            <label for="use-global-proxy" class="ml-2 text-sm cursor-pointer select-none">
              {{ t('setting.ai.useGlobalProxy') }}
            </label>
          </div>

          <div class="flex justify-end gap-2 pt-2">
            <button type="button" class="btn-secondary" @click="cancelEdit">
              <PhX :size="16" />
              {{ t('common.action.cancel') }}
            </button>
            <button
              type="button"
              class="btn-primary"
              :disabled="!form.modelname || !form.baseurl"
              @click="saveForm"
            >
              <PhCheck :size="16" />
              {{ t('common.action.save') }}
            </button>
          </div>
        </div>
      </div>

      <div v-else-if="models.length > 0" class="space-y-2">
        <div v-for="(model, index) in models" :key="index" class="model-item">
          <div class="flex items-start gap-2 sm:gap-3">
            <div class="shrink-0 w-8 h-8 flex items-center justify-center">
              <PhDatabase :size="28" class="text-text-tertiary" />
            </div>

            <div class="flex-1 min-w-0">
              <div class="font-medium text-sm sm:text-base truncate">
                {{ model.modelname }}
              </div>
              <div class="mt-1.5 flex flex-wrap gap-1.5">
                <div class="info-badge">
                  <span class="text-text-tertiary">URL</span>
                  <span
                    class="text-text-secondary font-mono truncate max-w-[150px] sm:max-w-[200px]"
                    >{{ model.baseurl }}</span
                  >
                </div>
                <div v-if="model.rpm > 0" class="info-badge">
                  <span class="text-text-tertiary">RPM</span>
                  <span class="text-text-secondary">{{ model.rpm }}</span>
                </div>
                <div v-if="model.tpm > 0" class="info-badge">
                  <span class="text-text-tertiary">TPM</span>
                  <span class="text-text-secondary">{{ model.tpm }}</span>
                </div>
                <div v-if="model.use_global_proxy" class="info-badge info-badge-accent">
                  <span class="text-text-tertiary">Proxy</span>
                </div>
              </div>
            </div>

            <div class="shrink-0">
              <div v-if="testingModels.has(getModelKey(model, index))" class="status-indicator">
                <PhArrowClockwise :size="16" class="animate-spin text-text-secondary" />
              </div>
              <div
                v-else-if="getTestStatus(model, index) === 'success'"
                class="status-indicator status-success"
                :title="t('setting.ai.connectionSuccess')"
              >
                <PhCheck :size="14" class="text-green-500" />
              </div>
              <div
                v-else-if="getTestStatus(model, index) === 'error'"
                class="status-indicator status-error"
                :title="testResults.get(getModelKey(model, index))?.error_message"
              >
                <PhX :size="14" class="text-red-500" />
              </div>
            </div>

            <div class="flex items-center gap-1 sm:gap-2 shrink-0">
              <button
                class="action-btn"
                :disabled="testingModels.has(getModelKey(model, index))"
                :title="t('setting.ai.testEmbeddingModel')"
                @click="handleTestModel(model, index)"
              >
                <PhTestTube :size="18" class="sm:w-5 sm:h-5" />
              </button>
              <button class="action-btn" :title="t('common.edit')" @click="openEdit(index)">
                <PhPencil :size="18" class="sm:w-5 sm:h-5" />
              </button>
              <button
                class="action-btn danger"
                :title="t('common.delete')"
                @click="deleteModel(index)"
              >
                <PhTrash :size="18" class="sm:w-5 sm:h-5" />
              </button>
            </div>
          </div>

          <div
            v-if="
              testResults.get(getModelKey(model, index))?.error_message &&
              getTestStatus(model, index) === 'error'
            "
            class="mt-2 text-xs text-red-500 bg-red-500/5 rounded p-2 break-words"
          >
            {{ testResults.get(getModelKey(model, index))?.error_message }}
          </div>
        </div>
      </div>

      <div v-else class="empty-state">
        <PhDatabase :size="40" class="mx-auto mb-2 text-text-tertiary" />
        <div class="text-text-secondary text-sm">{{ t('setting.ai.noEmbeddingModels') }}</div>
        <div class="text-xs text-text-tertiary mt-1">
          {{ t('setting.ai.noEmbeddingModelsHint') }}
        </div>
      </div>
    </div>
  </SettingGroup>
</template>

<style scoped>
@reference "../../../../style.css";

.edit-form {
  @apply p-3 rounded-lg bg-bg-secondary border border-border;
}

.field-label {
  @apply block text-xs text-text-secondary mb-1;
}

.model-item {
  @apply p-2 sm:p-3 rounded-lg bg-bg-secondary border border-border transition-all;
}

.model-item:hover {
  @apply bg-bg-tertiary;
}

.info-badge {
  @apply inline-flex items-center gap-1 px-2 py-0.5 rounded bg-bg-tertiary text-xs;
}

.info-badge-accent {
  background-color: rgb(var(--accent-color) / 0.1);
  @apply text-accent;
}

.info-badge-accent .text-text-tertiary {
  color: rgb(var(--accent-color) / 0.7);
}

.action-btn {
  @apply p-1.5 sm:p-2 rounded-lg bg-transparent border-none cursor-pointer text-text-secondary hover:bg-bg-tertiary hover:text-text-primary transition-all;
}

.action-btn.danger:hover {
  @apply text-red-500;
  background-color: rgb(239 68 68 / 0.1);
}

.action-btn:disabled {
  @apply opacity-50 cursor-not-allowed;
}

.status-indicator {
  @apply w-6 h-6 flex items-center justify-center rounded-full bg-bg-tertiary;
}

.status-indicator.status-success {
  @apply bg-green-500/10;
}

.status-indicator.status-error {
  @apply bg-red-500/10;
}

.animate-spin {
  animation: spin 1s linear infinite;
  display: inline-block;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
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

.empty-state {
  @apply py-6 text-center;
}
</style>
