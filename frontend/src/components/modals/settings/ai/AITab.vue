<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import type { SettingsData } from '@/types/settings';
import { TipBox } from '@/components/settings';
import AIProfileList from './AIProfileList.vue';
import AIUsageSettings from './AIUsageSettings.vue';
import AIEmbeddingSettings from './AIEmbeddingSettings.vue';
import AIFeatureSettings from './AIFeatureSettings.vue';
import { useClusterStore } from '@/stores/cluster';

const { t } = useI18n();
const clusterStore = useClusterStore();

interface Props {
  settings: SettingsData;
}

defineProps<Props>();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();

// Handler for settings updates from child components
function handleUpdateSettings(updatedSettings: SettingsData) {
  emit('update:settings', updatedSettings);
}

onMounted(() => {
  clusterStore.startAIProcessingPolling().catch((error) => {
    console.error('Failed to start AI processing polling in settings:', error);
  });
});

onBeforeUnmount(() => {
  clusterStore.stopAIProcessingPolling();
});
</script>

<template>
  <div class="relative">
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
      <TipBox type="info" :title="t('setting.ai.isDanger')" />
      <AIProfileList />
      <AIUsageSettings :settings="settings" @update:settings="handleUpdateSettings" />
      <AIEmbeddingSettings :settings="settings" @update:settings="handleUpdateSettings" />
      <AIFeatureSettings :settings="settings" @update:settings="handleUpdateSettings" />
    </div>
  </div>
</template>

<style scoped>
@reference "../../../../style.css";
</style>
