<script setup lang="ts">
import type { SettingsData } from '@/types/settings';
import { useI18n } from 'vue-i18n';
import { TipBox } from '@/components/settings';
import ObsidianSettings from './ObsidianSettings.vue';
import NotionSettings from './NotionSettings.vue';
import FreshRSSSettings from './FreshRSSSettings.vue';
import RSSHubSettings from './RSSHubSettings.vue';

interface Props {
  settings: SettingsData;
}

defineProps<Props>();
const { t } = useI18n();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();

// Handler for settings updates from child components
function handleUpdateSettings(updatedSettings: SettingsData) {
  // Emit the updated settings to parent
  emit('update:settings', updatedSettings);
}
</script>

<template>
  <div class="space-y-4 sm:space-y-6">
    <TipBox type="info" :title="t('common.warning.isInDevelopment')" />

    <ObsidianSettings :settings="settings" @update:settings="handleUpdateSettings" />

    <NotionSettings :settings="settings" @update:settings="handleUpdateSettings" />

    <FreshRSSSettings :settings="settings" @update:settings="handleUpdateSettings" />

    <RSSHubSettings :settings="settings" @update:settings="handleUpdateSettings" />
  </div>
</template>

<style scoped>
@reference "../../../../style.css";
</style>
