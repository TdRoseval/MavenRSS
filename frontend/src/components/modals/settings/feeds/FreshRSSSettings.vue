<script setup lang="ts">
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  PhRss,
  PhCloudCheck,
  PhGlobe,
  PhUser,
  PhKey,
  PhWifiHigh,
  PhArrowClockwise,
} from '@phosphor-icons/vue';
import type { SettingsData } from '@/types/settings';

const { t } = useI18n();

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();

const isSyncing = ref(false);

// Sync with FreshRSS server
async function syncNow() {
  isSyncing.value = true;

  try {
    const response = await fetch('/api/freshrss/sync', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
    });

    if (response.ok) {
      window.showToast(t('syncCompleted'), 'success');
    } else {
      throw new Error(t('syncFailed'));
    }
  } catch (error) {
    window.showToast(error instanceof Error ? error.message : t('syncFailed'), 'error');
  } finally {
    isSyncing.value = false;
  }
}
</script>

<template>
  <div class="setting-group">
    <label
      class="font-semibold mb-2 sm:mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2"
    >
      <PhRss :size="14" class="sm:w-4 sm:h-4" />
      {{ t('freshrss') }}
    </label>
    <div class="tip-box">
      <PhInfo :size="16" class="text-accent shrink-0 sm:w-5 sm:h-5" />
      <span class="text-xs sm:text-sm">{{ t('isInDevelopment') }}</span>
    </div>
    <!-- Enable FreshRSS Sync -->
    <div class="setting-item">
      <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
        <PhCloudCheck :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
        <div class="flex-1 min-w-0">
          <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
            {{ t('freshrssEnabled') }}
          </div>
          <div class="text-xs text-text-secondary hidden sm:block">
            {{ t('freshrssEnabledDesc') }}
          </div>
        </div>
      </div>
      <input
        type="checkbox"
        :checked="props.settings.freshrss_enabled"
        class="toggle"
        @change="
          (e) =>
            emit('update:settings', {
              ...props.settings,
              freshrss_enabled: (e.target as HTMLInputElement).checked,
            })
        "
      />
    </div>

    <div
      v-if="props.settings.freshrss_enabled"
      class="ml-2 sm:ml-4 space-y-2 sm:space-y-3 border-l-2 border-border pl-2 sm:pl-4"
    >
      <!-- Server URL -->
      <div class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhGlobe :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
              {{ t('freshrssServerUrl') }}
            </div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('freshrssServerUrlDesc') }}
            </div>
          </div>
        </div>
        <input
          type="url"
          :value="props.settings.freshrss_server_url"
          :placeholder="t('freshrssServerUrlPlaceholder')"
          class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          @input="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                freshrss_server_url: (e.target as HTMLInputElement).value,
              })
          "
        />
      </div>

      <!-- Username -->
      <div class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhUser :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
              {{ t('freshrssUsername') }}
            </div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('freshrssUsernameDesc') }}
            </div>
          </div>
        </div>
        <input
          type="text"
          :value="props.settings.freshrss_username"
          :placeholder="t('freshrssUsernamePlaceholder')"
          class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          @input="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                freshrss_username: (e.target as HTMLInputElement).value,
              })
          "
        />
      </div>

      <!-- API Password -->
      <div class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhKey :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
              {{ t('freshrssApiPassword') }}
            </div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('freshrssApiPasswordDesc') }}
            </div>
          </div>
        </div>
        <input
          type="password"
          :value="props.settings.freshrss_api_password"
          :placeholder="t('freshrssApiPasswordPlaceholder')"
          class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          @input="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                freshrss_api_password: (e.target as HTMLInputElement).value,
              })
          "
        />
      </div>

      <!-- Connection Test and Sync Buttons -->
      <div class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhWifiHigh :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
              {{ t('testConnection') }}
            </div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('testConnectionDesc') }}
            </div>
          </div>
        </div>
        <div class="flex items-center gap-2 shrink-0">
          <button
            :disabled="isSyncing"
            class="btn-primary text-xs sm:text-sm px-2 sm:px-3 py-1 sm:py-1.5"
            @click="syncNow"
          >
            <PhArrowClockwise
              :size="16"
              :class="{ 'animate-spin': isSyncing, 'sm:w-5 sm:h-5': true }"
            />
            {{ isSyncing ? t('syncing') : t('syncNow') }}
          </button>
        </div>
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
.btn-primary {
  @apply bg-accent text-white border-none px-2 py-1 sm:px-3 sm:py-1.5 rounded-lg cursor-pointer flex items-center gap-1 sm:gap-2 font-medium hover:bg-accent-hover transition-colors text-sm sm:text-base;
}
.btn-secondary {
  @apply bg-bg-secondary text-text-primary border border-border px-2 py-1 sm:px-3 sm:py-1.5 rounded-lg cursor-pointer flex items-center gap-1 sm:gap-2 font-medium hover:bg-bg-primary transition-colors text-sm sm:text-base;
}
.setting-group {
  @apply space-y-2 sm:space-y-3;
}
.tip-box {
  @apply flex items-center gap-2 sm:gap-3 py-2 sm:py-2.5 px-2.5 sm:px-3 rounded-lg;
  background-color: rgba(59, 130, 246, 0.05);
  border: 1px solid rgba(59, 130, 246, 0.3);
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }

  to {
    transform: rotate(360deg);
  }
}

.animate-spin {
  animation: spin 1s linear infinite;
}
</style>
