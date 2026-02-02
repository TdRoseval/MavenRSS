<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { computed } from 'vue';
import type { SettingsData } from '@/types/settings';
import { useSettingsAutoSave } from '@/composables/core/useSettingsAutoSave';
import { TipBox } from '@/components/settings';
import AISettings from './AISettings.vue';
import AITestSettings from './AITestSettings.vue';
import AIUsageSettings from './AIUsageSettings.vue';
import AIFeatureSettings from './AIFeatureSettings.vue';

const { t } = useI18n();

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();

// Create a computed ref that returns the settings object
const settingsRef = computed(() => props.settings);

// Use composable for auto-save
useSettingsAutoSave(settingsRef);

// Handler for settings updates from child components
function handleUpdateSettings(updatedSettings: SettingsData) {
  emit('update:settings', updatedSettings);
}
</script>

<template>
  <div class="space-y-4 sm:space-y-6">
    <TipBox type="info" :title="t('setting.ai.isDanger')" />
    <AISettings :settings="settings" @update:settings="handleUpdateSettings" />
    <AITestSettings :settings="settings" @update:settings="handleUpdateSettings" />
    <AIUsageSettings :settings="settings" @update:settings="handleUpdateSettings" />
    <AIFeatureSettings :settings="settings" @update:settings="handleUpdateSettings" />
  </div>
</template>

<style scoped>
@reference "../../../../style.css";
</style>
