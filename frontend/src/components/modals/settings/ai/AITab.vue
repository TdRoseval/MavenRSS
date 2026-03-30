<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import type { SettingsData } from '@/types/settings';
import { TipBox } from '@/components/settings';
import AIProfileList from './AIProfileList.vue';
import AIUsageSettings from './AIUsageSettings.vue';
import AIEmbeddingSettings from './AIEmbeddingSettings.vue';
import AIFeatureSettings from './AIFeatureSettings.vue';

const { t } = useI18n();

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
</script>

<template>
  <div class="space-y-4 sm:space-y-6">
    <TipBox type="info" :title="t('setting.ai.isDanger')" />
    <AIProfileList />
    <AIUsageSettings :settings="settings" @update:settings="handleUpdateSettings" />
    <AIEmbeddingSettings :settings="settings" @update:settings="handleUpdateSettings" />
    <AIFeatureSettings :settings="settings" @update:settings="handleUpdateSettings" />
  </div>
</template>

<style scoped>
@reference "../../../../style.css";
</style>
