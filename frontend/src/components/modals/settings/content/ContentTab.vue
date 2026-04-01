<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue';
import type { SettingsData } from '@/types/settings';
import { useSettingsValidation } from '@/composables/core/useSettingsValidation';
import { useI18n } from 'vue-i18n';
import { PhWarning } from '@phosphor-icons/vue';
import TranslationSettings from './TranslationSettings.vue';
import SummarySettings from './SummarySettings.vue';
import { useClusterStore } from '@/stores/cluster';

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();
const { t } = useI18n();
const clusterStore = useClusterStore();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();

// Create a computed ref that returns the settings object
// This ensures reactivity while allowing modifications
const settingsRef = computed(() => props.settings);

// Use validation composable
const { isValid, isTranslationValid, isSummaryValid } = useSettingsValidation(settingsRef);

// Handler for settings updates from child components
function handleUpdateSettings(updatedSettings: SettingsData) {
  // Emit the updated settings to parent
  emit('update:settings', updatedSettings);
}

onMounted(() => {
  clusterStore.startAIProcessingPolling().catch((error) => {
    console.error('Failed to start AI processing polling in content settings:', error);
  });
});

onBeforeUnmount(() => {
  clusterStore.stopAIProcessingPolling();
});
</script>

<template>
  <div class="relative space-y-4 sm:space-y-6">
    <!-- Validation Warning -->
    <div
      v-if="!isValid"
      class="p-3 sm:p-4 rounded-lg border-2 border-red-500 bg-red-500/10 flex items-start gap-3"
    >
      <PhWarning :size="20" class="text-red-500 shrink-0 mt-0.5" :weight="'fill'" />
      <div class="flex-1">
        <div class="font-semibold text-red-500 text-sm sm:text-base mb-1">
          {{ t('common.form.requiredField') }}
        </div>
        <div class="text-xs sm:text-sm text-text-secondary">
          <span v-if="!isTranslationValid">
            {{ t('setting.content.translationCredentialsRequired') }}
          </span>
          <span v-if="!isTranslationValid && !isSummaryValid"> • </span>
          <span v-if="!isSummaryValid">
            {{ t('setting.content.summaryCredentialsRequired') }}
          </span>
        </div>
      </div>
    </div>

    <div
      v-if="clusterStore.isAIProcessingLocked"
      class="absolute inset-0 z-20 rounded-2xl bg-bg-primary/80 backdrop-blur-sm"
    >
      <div
        class="flex h-full min-h-[320px] flex-col items-center justify-center rounded-2xl border border-border bg-bg-primary/70 p-6 text-center"
      >
        <div class="text-sm font-medium uppercase tracking-[0.18em] text-text-tertiary">
          {{ t('setting.ai.processingLockedLabel') }}
        </div>
        <h3 class="mt-3 text-xl font-semibold text-text-primary">
          {{ t('setting.ai.processingLockedTitle') }}
        </h3>
        <p class="mt-2 max-w-lg text-sm leading-6 text-text-secondary">
          {{ t('setting.ai.processingLockedDescription') }}
        </p>
        <div class="mt-6 w-full max-w-md">
          <div class="mb-2 flex items-center justify-between text-sm text-text-secondary">
            <span>{{ t('article.cluster.processingProgress') }}</span>
            <span>{{ clusterStore.aiProcessingProgressPercent }}%</span>
          </div>
          <div class="h-3 overflow-hidden rounded-full bg-bg-tertiary">
            <div
              class="h-full rounded-full bg-accent transition-[width] duration-500 ease-out"
              :style="{ width: `${clusterStore.aiProcessingProgressPercent}%` }"
            />
          </div>
        </div>
      </div>
    </div>

    <div
      :class="[
        'space-y-4 sm:space-y-6 transition-opacity',
        clusterStore.isAIProcessingLocked ? 'pointer-events-none opacity-40 select-none' : '',
      ]"
    >
      <TranslationSettings :settings="settings" @update:settings="handleUpdateSettings" />
      <SummarySettings :settings="settings" @update:settings="handleUpdateSettings" />
    </div>
  </div>
</template>

<style scoped>
@reference "../../../../style.css";
</style>
