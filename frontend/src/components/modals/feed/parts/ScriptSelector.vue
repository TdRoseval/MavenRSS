<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { PhCode, PhBookOpen } from '@phosphor-icons/vue';
import { openInBrowser } from '@/utils/browser';

interface Props {
  modelValue: string;
  mode: 'add' | 'edit';
  isInvalid?: boolean;
  availableScripts: Array<{ path: string; name: string; type: string }>;
  scriptsDir: string;
}

const props = withDefaults(defineProps<Props>(), {
  isInvalid: false,
});

const emit = defineEmits<{
  'update:modelValue': [value: string];
  'open-scripts-folder': [];
}>();

const { t, locale } = useI18n();

function openScriptsFolder() {
  emit('open-scripts-folder');
}

function openDocumentation() {
  const docUrl = locale.value.startsWith('zh')
    ? 'https://github.com/WCY-dt/MrRSS/blob/main/docs/CUSTOM_SCRIPT_MODE.zh.md'
    : 'https://github.com/WCY-dt/MrRSS/blob/main/docs/CUSTOM_SCRIPT_MODE.md';
  openInBrowser(docUrl);
}
</script>

<template>
  <div class="mb-3 sm:mb-4">
    <label class="block mb-1 sm:mb-1.5 font-semibold text-xs sm:text-sm text-text-secondary"
      >{{ t('selectScript') }}
      <span v-if="props.mode === 'add'" class="text-red-500">*</span></label
    >
    <div v-if="props.availableScripts.length > 0" class="mb-2">
      <select
        :value="props.modelValue"
        :class="['input-field', props.mode === 'add' && props.isInvalid ? 'border-red-500' : '']"
        @change="emit('update:modelValue', ($event.target as HTMLSelectElement).value)"
      >
        <option value="">{{ t('selectScriptPlaceholder') }}</option>
        <option v-for="script in props.availableScripts" :key="script.path" :value="script.path">
          {{ script.name }} ({{ script.type }})
        </option>
      </select>
    </div>
    <div
      v-else
      class="text-xs sm:text-sm text-text-secondary bg-bg-secondary rounded-md p-2 sm:p-3 border border-border"
    >
      <p class="mb-2">{{ t('noScriptsFound') }}</p>
    </div>
    <div class="flex flex-col sm:flex-row gap-2 sm:gap-3 mt-3">
      <button
        type="button"
        class="text-xs sm:text-sm text-accent hover:underline flex items-center gap-1"
        @click="openDocumentation"
      >
        <PhBookOpen :size="14" />
        {{ t('scriptDocumentation') }}
      </button>
      <button
        type="button"
        class="text-xs sm:text-sm text-accent hover:underline flex items-center gap-1"
        @click="openScriptsFolder"
      >
        <PhCode :size="14" />
        {{ t('openScriptsFolder') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../style.css";

.input-field {
  @apply w-full p-2 sm:p-2.5 border border-border rounded-md bg-bg-tertiary text-text-primary text-xs sm:text-sm focus:border-accent focus:outline-none transition-colors;
}
</style>
