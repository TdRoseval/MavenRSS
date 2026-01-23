<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { PhRobot, PhKey, PhLink, PhBrain, PhSliders } from '@phosphor-icons/vue';
import { SettingGroup, SettingItem, KeyValueList } from '@/components/settings';
import '@/components/settings/styles.css';
import type { SettingsData } from '@/types/settings';

const { t } = useI18n();

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();

function updateSetting(key: keyof SettingsData, value: any) {
  emit('update:settings', {
    ...props.settings,
    [key]: value,
  });
}
</script>

<template>
  <SettingGroup :icon="PhRobot" :title="t('setting.ai.aiSettings')">
    <div class="text-xs text-text-secondary mb-3 sm:mb-4">{{ t('setting.ai.settingsDesc') }}</div>

    <!-- API Key -->
    <SettingItem
      :icon="PhKey"
      :title="t('setting.ai.aiApiKey')"
      :description="t('setting.ai.aiApiKeyDesc')"
    >
      <input
        :value="settings.ai_api_key"
        type="password"
        :placeholder="t('setting.ai.aiApiKeyPlaceholder')"
        class="input-field w-32 sm:w-48 text-xs sm:text-sm"
        @input="updateSetting('ai_api_key', ($event.target as HTMLInputElement).value)"
      />
    </SettingItem>

    <!-- Endpoint -->
    <SettingItem
      :icon="PhLink"
      :title="t('setting.ai.aiEndpoint')"
      :description="t('setting.ai.aiEndpointDesc')"
      required
    >
      <input
        :value="settings.ai_endpoint"
        type="text"
        :placeholder="t('setting.ai.aiEndpointPlaceholder')"
        class="input-field w-32 sm:w-48 text-xs sm:text-sm"
        @input="updateSetting('ai_endpoint', ($event.target as HTMLInputElement).value)"
      />
    </SettingItem>

    <!-- Model -->
    <SettingItem
      :icon="PhBrain"
      :title="t('setting.ai.aiModel')"
      :description="t('setting.ai.aiModelDesc')"
      required
    >
      <input
        :value="settings.ai_model"
        type="text"
        :placeholder="t('setting.ai.aiModelPlaceholder')"
        class="input-field w-32 sm:w-48 text-xs sm:text-sm"
        @input="updateSetting('ai_model', ($event.target as HTMLInputElement).value)"
      />
    </SettingItem>

    <!-- Custom Headers -->
    <div class="setting-item-col">
      <div class="flex items-center gap-2 sm:gap-3">
        <PhSliders :size="20" class="text-text-secondary shrink-0 sm:w-6 sm:h-6" />
        <div class="flex-1 min-w-0">
          <div class="font-medium text-sm">{{ t('setting.ai.aiCustomHeaders') }}</div>
          <div class="text-xs text-text-secondary hidden sm:block">
            {{ t('setting.ai.aiCustomHeadersDesc') }}
          </div>
        </div>
      </div>

      <KeyValueList
        :model-value="settings.ai_custom_headers"
        :key-placeholder="t('setting.ai.aiCustomHeadersName')"
        :value-placeholder="t('setting.ai.aiCustomHeadersValue')"
        :add-button-text="t('setting.ai.aiCustomHeadersAdd')"
        :remove-button-title="t('setting.ai.aiCustomHeadersRemove')"
        @update:model-value="updateSetting('ai_custom_headers', $event)"
      />
    </div>
  </SettingGroup>
</template>

<style scoped>
@reference "../../../../style.css";

.input-field {
  @apply p-1.5 sm:p-2.5 border border-border rounded-md bg-bg-secondary text-text-primary focus:border-accent focus:outline-none transition-colors;
}
</style>
