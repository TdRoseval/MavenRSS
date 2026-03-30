<script setup lang="ts">
import { ref, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { PhDatabase, PhPlus, PhPencilSimple, PhTrash, PhCheck, PhX } from '@phosphor-icons/vue';
import { SettingGroup, TipBox } from '@/components/settings';
import type { SettingsData } from '@/types/settings';
import '@/components/settings/styles.css';

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

const models = computed<EmbeddingModel[]>(() => {
  try {
    return JSON.parse(props.settings.ai_embedding_models || '[]');
  } catch (e) {
    return [];
  }
});

const isEditing = ref(false);
const editIndex = ref(-1);
const form = ref<EmbeddingModel>({
  modelname: '',
  baseurl: '',
  apikey: '',
  rpm: 0,
  tpm: 0,
  use_global_proxy: false,
});

function saveModels(newModels: EmbeddingModel[]) {
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
  const newModels = [...models.value];
  newModels.splice(index, 1);
  saveModels(newModels);
}

function saveForm() {
  if (!form.value.modelname || !form.value.baseurl) {
    // Basic validation
    return;
  }
  const newModels = [...models.value];
  // Ensure numeric types
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
  <SettingGroup :icon="PhDatabase" title="嵌入模型配置 (Embedding Models)">
    <div class="space-y-4">
      <div class="flex justify-between items-center">
        <div class="text-sm text-content-lighter">
          配置用于 AI 增强模式的嵌入模型（如 SiliconFlow BGE 等）。系统将按顺序尝试使用。
        </div>
        <button
          v-if="!isEditing"
          type="button"
          class="btn-primary py-1 px-3 text-sm"
          @click="openAdd"
        >
          <PhPlus class="w-4 h-4 mr-1" />
          添加配置
        </button>
      </div>

      <div v-if="isEditing" class="p-4 bg-surface rounded-lg border border-border">
        <div class="space-y-4">
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium mb-1">模型名称 (Model Name)</label>
              <input
                v-model="form.modelname"
                type="text"
                class="inputbox w-full"
                placeholder="例如: BAAI/bge-m3"
              />
            </div>
            <div>
              <label class="block text-sm font-medium mb-1">API Base URL</label>
              <input
                v-model="form.baseurl"
                type="text"
                class="inputbox w-full"
                placeholder="例如: https://api.siliconflow.cn/v1"
              />
            </div>
          </div>

          <div>
            <label class="block text-sm font-medium mb-1">API Key</label>
            <input
              v-model="form.apikey"
              type="password"
              class="inputbox w-full"
              placeholder="sk-..."
            />
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium mb-1">RPM (每分钟请求数，0为不限制)</label>
              <input v-model.number="form.rpm" type="number" class="inputbox w-full" min="0" />
            </div>
            <div>
              <label class="block text-sm font-medium mb-1">TPM (每分钟Token数，0为不限制)</label>
              <input v-model.number="form.tpm" type="number" class="inputbox w-full" min="0" />
            </div>
          </div>

          <div class="flex items-center mt-2">
            <input
              id="use-global-proxy"
              v-model="form.use_global_proxy"
              type="checkbox"
              class="w-4 h-4 rounded border-border text-primary focus:ring-primary/20 transition-colors cursor-pointer"
            />
            <label for="use-global-proxy" class="ml-2 text-sm cursor-pointer">使用全局代理</label>
          </div>

          <div class="flex justify-end space-x-2 pt-2 border-t border-border mt-4">
            <button type="button" class="btn-secondary" @click="cancelEdit">
              <PhX class="w-4 h-4 mr-1" /> 取消
            </button>
            <button
              type="button"
              class="btn-primary"
              :disabled="!form.modelname || !form.baseurl"
              @click="saveForm"
            >
              <PhCheck class="w-4 h-4 mr-1" /> 保存
            </button>
          </div>
        </div>
      </div>

      <div v-else-if="models.length > 0" class="space-y-2">
        <div
          v-for="(model, index) in models"
          :key="index"
          class="flex items-center justify-between p-3 bg-surface rounded-lg border border-border group"
        >
          <div class="flex flex-col">
            <span class="font-medium text-sm">{{ model.modelname }}</span>
            <span
              class="text-xs text-content-lighter mt-0.5 truncate max-w-[200px] sm:max-w-[400px]"
              >{{ model.baseurl }}</span
            >
            <span class="text-xs text-content-lighter mt-0.5 flex space-x-2">
              <span v-if="model.rpm > 0">RPM: {{ model.rpm }}</span>
              <span v-else>RPM: 不限</span>
              <span v-if="model.tpm > 0">TPM: {{ model.tpm }}</span>
              <span v-else>TPM: 不限</span>
              <span v-if="model.use_global_proxy">| 代理: 开</span>
            </span>
          </div>
          <div class="flex space-x-1 opacity-0 group-hover:opacity-100 transition-opacity">
            <button
              class="p-1.5 text-content-lighter hover:text-primary hover:bg-black/5 dark:hover:bg-white/10 rounded"
              @click="openEdit(index)"
            >
              <PhPencilSimple class="w-4 h-4" />
            </button>
            <button
              class="p-1.5 text-content-lighter hover:text-red-500 hover:bg-red-500/10 rounded"
              @click="deleteModel(index)"
            >
              <PhTrash class="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>

      <div
        v-else
        class="text-center py-6 border-2 border-dashed border-border rounded-lg text-content-light text-sm"
      >
        尚未配置嵌入模型。必须至少配一个才能完整启用 AI 增强模式。
      </div>
    </div>
  </SettingGroup>
</template>

<style scoped>
@reference "../../../../style.css";
</style>
