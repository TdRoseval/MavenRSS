<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { PhShield, PhGlobe, PhPlug, PhLock, PhUser, PhKey, PhInfo } from '@phosphor-icons/vue';
import type { SettingsData } from '@/types/settings';

const { t } = useI18n();

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();
</script>

<template>
  <div class="setting-group">
    <label
      class="font-semibold mb-2 sm:mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2"
    >
      <PhShield :size="14" class="sm:w-4 sm:h-4" />
      {{ t('proxySettings') }}
    </label>
    <div class="tip-box">
      <PhInfo :size="16" class="text-accent shrink-0 sm:w-5 sm:h-5" />
      <span class="text-xs sm:text-sm">{{ t('systemProxyInfo') }}</span>
    </div>

    <!-- Enable Proxy Toggle -->
    <div class="setting-item mt-2 sm:mt-3">
      <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
        <PhGlobe :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
        <div class="flex-1 min-w-0">
          <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
            {{ t('enableProxy') }}
          </div>
          <div class="text-xs text-text-secondary hidden sm:block">
            {{ t('enableProxyDesc') }}
          </div>
        </div>
      </div>
      <input
        :checked="props.settings.proxy_enabled"
        type="checkbox"
        class="toggle"
        @change="
          (e) =>
            emit('update:settings', {
              ...props.settings,
              proxy_enabled: (e.target as HTMLInputElement).checked,
            })
        "
      />
    </div>

    <!-- Proxy Settings (shown when proxy is enabled) -->
    <div
      v-if="props.settings.proxy_enabled"
      class="mt-2 sm:mt-3 ml-4 sm:ml-6 space-y-2 sm:space-y-3 pl-3 sm:pl-4 border-l-2 border-border"
    >
      <!-- Proxy Type -->
      <div class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhPlug :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-xs sm:text-sm">
              {{ t('proxyType') }}
            </div>
            <div class="text-[10px] sm:text-xs text-text-secondary hidden sm:block">
              {{ t('proxyTypeDesc') }}
            </div>
          </div>
        </div>
        <select
          :value="props.settings.proxy_type"
          class="input-field w-28 sm:w-32 text-xs sm:text-sm"
          @change="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                proxy_type: (e.target as HTMLSelectElement).value,
              })
          "
        >
          <option value="http">{{ t('httpProxy') }}</option>
          <option value="https">{{ t('httpsProxy') }}</option>
          <option value="socks5">{{ t('socks5Proxy') }}</option>
        </select>
      </div>

      <!-- Proxy Host -->
      <div class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhGlobe :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-xs sm:text-sm">
              {{ t('proxyHost') }} <span class="text-red-500">*</span>
            </div>
            <div class="text-[10px] sm:text-xs text-text-secondary hidden sm:block">
              {{ t('proxyHostDesc') }}
            </div>
          </div>
        </div>
        <input
          :value="props.settings.proxy_host"
          type="text"
          :placeholder="t('proxyHostPlaceholder')"
          :class="[
            'input-field w-36 sm:w-48 text-xs sm:text-sm',
            props.settings.proxy_enabled && !props.settings.proxy_host?.trim()
              ? 'border-red-500'
              : '',
          ]"
          @input="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                proxy_host: (e.target as HTMLInputElement).value,
              })
          "
        />
      </div>

      <!-- Proxy Port -->
      <div class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhLock :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-xs sm:text-sm">
              {{ t('proxyPort') }} <span class="text-red-500">*</span>
            </div>
            <div class="text-[10px] sm:text-xs text-text-secondary hidden sm:block">
              {{ t('proxyPortDesc') }}
            </div>
          </div>
        </div>
        <input
          :value="props.settings.proxy_port"
          type="text"
          :placeholder="t('proxyPortPlaceholder')"
          :class="[
            'input-field w-20 sm:w-24 text-center text-xs sm:text-sm',
            props.settings.proxy_enabled && !props.settings.proxy_port?.trim()
              ? 'border-red-500'
              : '',
          ]"
          @input="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                proxy_port: (e.target as HTMLInputElement).value,
              })
          "
        />
      </div>

      <!-- Proxy Username -->
      <div class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhUser :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-xs sm:text-sm">
              {{ t('proxyUsername') }}
            </div>
            <div class="text-[10px] sm:text-xs text-text-secondary hidden sm:block">
              {{ t('proxyUsernameDesc') }}
            </div>
          </div>
        </div>
        <input
          :value="props.settings.proxy_username"
          type="text"
          :placeholder="t('proxyUsernamePlaceholder')"
          class="input-field w-28 sm:w-36 text-xs sm:text-sm"
          @input="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                proxy_username: (e.target as HTMLInputElement).value,
              })
          "
        />
      </div>

      <!-- Proxy Password -->
      <div class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhKey :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-xs sm:text-sm">
              {{ t('proxyPassword') }}
            </div>
            <div class="text-[10px] sm:text-xs text-text-secondary hidden sm:block">
              {{ t('proxyPasswordDesc') }}
            </div>
          </div>
        </div>
        <input
          :value="props.settings.proxy_password"
          type="password"
          :placeholder="t('proxyPasswordPlaceholder')"
          class="input-field w-28 sm:w-36 text-xs sm:text-sm"
          @input="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                proxy_password: (e.target as HTMLInputElement).value,
              })
          "
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../../style.css";

.input-field {
  @apply p-1.5 sm:p-2.5 border border-border rounded-md bg-bg-secondary text-text-primary focus:border-accent focus:outline-none transition-colors;
}

.toggle {
  @apply w-10 h-5 appearance-none bg-bg-tertiary rounded-full relative cursor-pointer border border-border transition-colors checked:bg-accent checked:border-accent shrink-0;
}

.toggle::after {
  content: '';
  @apply absolute top-0.5 left-0.5 w-3.5 h-3.5 bg-white rounded-full shadow-sm transition-transform;
}

.toggle:checked::after {
  transform: translateX(20px);
}

.setting-item {
  @apply flex items-center sm:items-start justify-between gap-2 sm:gap-4 p-2 sm:p-3 rounded-lg bg-bg-secondary border border-border;
}

.sub-setting-item {
  @apply flex items-center sm:items-start justify-between gap-2 sm:gap-4 p-2 sm:p-2.5 rounded-md bg-bg-tertiary;
}

.setting-group {
  @apply mb-4 sm:mb-6;
}

.tip-box {
  @apply flex items-center gap-2 sm:gap-3 py-2 sm:py-2.5 px-2.5 sm:px-3 rounded-lg;
  background-color: rgba(59, 130, 246, 0.05);
  border: 1px solid rgba(59, 130, 246, 0.3);
}
</style>
