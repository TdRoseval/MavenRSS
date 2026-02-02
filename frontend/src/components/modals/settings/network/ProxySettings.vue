<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import {
  PhWall,
  PhGlobe,
  PhPlug,
  PhUsb,
  PhUser,
  PhKey,
  PhLink,
  PhArrowClockwise,
  PhTimer,
} from '@phosphor-icons/vue';
import {
  SettingGroup,
  SettingWithToggle,
  SettingItem,
  SubSettingItem,
  NestedSettingsContainer,
  TipBox,
  InputControl,
  NumberControl,
} from '@/components/settings';
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
  <!-- Proxy Settings -->
  <SettingGroup :icon="PhWall" :title="t('setting.network.proxySettings')">
    <TipBox type="tip" :title="t('setting.network.systemProxyInfo')" />

    <!-- Enable Proxy Toggle -->
    <SettingWithToggle
      :icon="PhGlobe"
      :title="t('setting.network.enableProxy')"
      :description="t('setting.network.enableProxyDesc')"
      :model-value="props.settings.proxy_enabled"
      @update:model-value="updateSetting('proxy_enabled', $event)"
    />

    <!-- Proxy Settings (shown when proxy is enabled) -->
    <NestedSettingsContainer v-if="props.settings.proxy_enabled">
      <!-- Proxy Type -->
      <SubSettingItem
        :icon="PhPlug"
        :title="t('setting.network.proxyType')"
        :description="t('setting.network.proxyTypeDesc')"
      >
        <select
          :value="props.settings.proxy_type"
          class="input-field w-28 sm:w-32 text-xs sm:text-sm"
          @change="updateSetting('proxy_type', ($event.target as HTMLSelectElement).value)"
        >
          <option value="http">{{ t('setting.network.httpProxy') }}</option>
          <option value="https">{{ t('setting.network.httpsProxy') }}</option>
          <option value="socks5">{{ t('setting.network.socks5Proxy') }}</option>
        </select>
      </SubSettingItem>

      <!-- Proxy Host -->
      <SubSettingItem
        :icon="PhLink"
        :title="t('setting.network.proxyHost')"
        :description="t('setting.network.proxyHostDesc')"
        required
      >
        <InputControl
          :model-value="props.settings.proxy_host"
          :placeholder="t('setting.network.proxyHostPlaceholder')"
          :error="props.settings.proxy_enabled && !props.settings.proxy_host?.trim()"
          width="lg"
          @update:model-value="updateSetting('proxy_host', $event)"
        />
      </SubSettingItem>

      <!-- Proxy Port -->
      <SubSettingItem
        :icon="PhUsb"
        :title="t('setting.network.proxyPort')"
        :description="t('setting.network.proxyPortDesc')"
        required
      >
        <InputControl
          :model-value="props.settings.proxy_port"
          :placeholder="t('setting.network.proxyPortPlaceholder')"
          :error="props.settings.proxy_enabled && !props.settings.proxy_port?.trim()"
          width="sm"
          class="text-center"
          @update:model-value="updateSetting('proxy_port', $event)"
        />
      </SubSettingItem>

      <!-- Proxy Username -->
      <SubSettingItem
        :icon="PhUser"
        :title="t('setting.network.proxyUsername')"
        :description="t('setting.network.proxyUsernameDesc')"
      >
        <InputControl
          :model-value="props.settings.proxy_username"
          :placeholder="t('setting.network.proxyUsernamePlaceholder')"
          width="md"
          @update:model-value="updateSetting('proxy_username', $event)"
        />
      </SubSettingItem>

      <!-- Proxy Password -->
      <SubSettingItem
        :icon="PhKey"
        :title="t('setting.network.proxyPassword')"
        :description="t('setting.network.proxyPasswordDesc')"
      >
        <InputControl
          :model-value="props.settings.proxy_password"
          type="password"
          :placeholder="t('setting.network.proxyPasswordPlaceholder')"
          width="md"
          @update:model-value="updateSetting('proxy_password', $event)"
        />
      </SubSettingItem>
    </NestedSettingsContainer>
  </SettingGroup>

  <!-- Retry Timeout Setting -->
  <SettingGroup :icon="PhArrowClockwise" :title="t('modal.feed.refreshSettings')">
    <SettingItem
      :icon="PhTimer"
      :title="t('setting.feed.retryTimeout')"
      :description="t('setting.feed.retryTimeoutDesc')"
    >
      <NumberControl
        :model-value="props.settings.retry_timeout_seconds"
        :min="10"
        :max="600"
        :step="10"
        :suffix="t('common.time.seconds')"
        width="xs"
        class="text-center"
        @update:model-value="updateSetting('retry_timeout_seconds', $event)"
      />
    </SettingItem>
  </SettingGroup>
</template>

<style scoped>
@reference "../../../../style.css";

.input-field {
  @apply p-1.5 sm:p-2.5 border border-border rounded-md bg-bg-secondary text-text-primary focus:border-accent focus:outline-none transition-colors;
}
</style>
