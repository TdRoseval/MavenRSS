<script setup lang="ts">
import type { SettingsData } from '@/types/settings';
import NetworkSettings from './NetworkSettings.vue';
import ProxySettings from './ProxySettings.vue';

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
    <NetworkSettings :settings="settings" @update:settings="handleUpdateSettings" />

    <ProxySettings :settings="settings" @update:settings="handleUpdateSettings" />
  </div>
</template>

<style scoped>
@reference "../../../../style.css";
</style>
