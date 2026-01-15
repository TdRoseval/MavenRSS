<script setup lang="ts">
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  PhTestTube,
  PhCheckCircle,
  PhArrowClockwise,
  PhWarningCircle,
  PhBookOpen,
} from '@phosphor-icons/vue';
import type { AITestInfo } from '@/types/settings';
import { openInBrowser } from '@/utils/browser';

const { t, locale } = useI18n();

const testInfo = ref<AITestInfo>({
  config_valid: false,
  connection_success: false,
  model_available: false,
  response_time_ms: 0,
  test_time: '',
});

const isTesting = ref(false);
const errorMessage = ref('');

async function testAIConfig() {
  isTesting.value = true;
  errorMessage.value = '';

  try {
    const response = await fetch('/api/ai/test', {
      method: 'POST',
    });

    if (response.ok) {
      const data = await response.json();
      testInfo.value = data;

      if (!data.config_valid || !data.connection_success) {
        errorMessage.value = data.error_message || t('aiTestFailed');
      } else {
        window.showToast(t('aiTestSuccess'), 'success');
      }
    } else {
      errorMessage.value = t('aiTestFailed');
    }
  } catch (error) {
    console.error('AI test error:', error);
    errorMessage.value = t('aiTestFailed');
  } finally {
    isTesting.value = false;
  }
}

function formatTime(timeStr: string): string {
  if (!timeStr) return '';
  const date = new Date(timeStr);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  const minutes = Math.floor(diff / 60000);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 0) {
    return t('daysAgo', { count: days });
  } else if (hours > 0) {
    return t('hoursAgo', { count: hours });
  } else if (minutes > 0) {
    return t('minutesAgo', { count: minutes });
  } else {
    return t('justNow');
  }
}

function openDocumentation() {
  const docUrl = locale.value.startsWith('zh')
    ? 'https://github.com/WCY-dt/MrRSS/blob/main/docs/AI_CONFIGURATION.zh.md'
    : 'https://github.com/WCY-dt/MrRSS/blob/main/docs/AI_CONFIGURATION.md';
  openInBrowser(docUrl);
}
</script>

<template>
  <div class="setting-group">
    <label
      class="font-semibold mb-2 sm:mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2"
    >
      <PhTestTube :size="14" class="sm:w-4 sm:h-4" />
      {{ t('aiConfigTest') }}
    </label>

    <!-- AI Test Status Display -->
    <div
      class="flex flex-col sm:flex-row sm:items-stretch sm:justify-between gap-3 sm:gap-4 p-2 sm:p-3 rounded-lg bg-bg-secondary border border-border"
    >
      <!-- Status Indicators -->
      <div class="flex flex-col sm:flex-row items-center gap-3 sm:gap-4">
        <!-- Config Valid Box -->
        <div
          class="flex flex-col gap-2 p-3 rounded-lg bg-bg-primary border border-border w-full sm:min-w-[120px]"
          :class="{
            'border-green-500/30': testInfo.config_valid,
            'border-red-500/30': testInfo.test_time && !testInfo.config_valid,
          }"
        >
          <span class="text-sm text-text-secondary text-left">{{ t('configValid') }}</span>
          <div class="flex items-center gap-2">
            <PhCheckCircle v-if="testInfo.config_valid" :size="20" class="text-green-500" />
            <PhWarningCircle v-else-if="testInfo.test_time" :size="20" class="text-red-500" />
            <span
              class="text-xl sm:text-2xl font-bold"
              :class="{
                'text-green-500': testInfo.config_valid,
                'text-red-500': testInfo.test_time && !testInfo.config_valid,
                'text-text-primary': !testInfo.test_time,
              }"
            >
              {{ testInfo.test_time ? (testInfo.config_valid ? t('yes') : t('no')) : '-' }}
            </span>
          </div>
        </div>

        <!-- Connection Success Box -->
        <div
          class="flex flex-col gap-2 p-3 rounded-lg bg-bg-primary border border-border w-full sm:min-w-[120px]"
          :class="{
            'border-green-500/30': testInfo.connection_success,
            'border-red-500/30': testInfo.test_time && !testInfo.connection_success,
          }"
        >
          <span class="text-sm text-text-secondary text-left">{{ t('connectionSuccess') }}</span>
          <div class="flex items-center gap-2">
            <PhCheckCircle v-if="testInfo.connection_success" :size="20" class="text-green-500" />
            <PhWarningCircle v-else-if="testInfo.test_time" :size="20" class="text-red-500" />
            <span
              class="text-xl sm:text-2xl font-bold"
              :class="{
                'text-green-500': testInfo.connection_success,
                'text-red-500': testInfo.test_time && !testInfo.connection_success,
                'text-text-primary': !testInfo.test_time,
              }"
            >
              {{ testInfo.test_time ? (testInfo.connection_success ? t('yes') : t('no')) : '-' }}
            </span>
          </div>
        </div>

        <!-- Response Time Box -->
        <div
          class="flex flex-col gap-2 p-3 rounded-lg bg-bg-primary border border-border w-full sm:min-w-[120px]"
        >
          <span class="text-sm text-text-secondary text-left">{{ t('responseTime') }}</span>
          <div class="flex items-baseline gap-1">
            <span class="text-xl sm:text-2xl font-bold text-text-primary">{{
              testInfo.response_time_ms > 0 ? testInfo.response_time_ms : '-'
            }}</span>
            <span class="text-sm text-text-secondary">{{ t('ms') }}</span>
          </div>
        </div>
      </div>

      <!-- Right: Test Button and Test Time -->
      <div class="flex flex-col sm:justify-between flex-1 gap-2 sm:gap-0">
        <div class="flex justify-center sm:justify-end">
          <button class="btn-secondary" :disabled="isTesting" @click="testAIConfig">
            <PhArrowClockwise
              :size="16"
              :class="{ 'animate-spin': isTesting, 'sm:w-5 sm:h-5': true }"
            />
            <span>{{ isTesting ? t('testing') : t('testAIConfig') }}</span>
          </button>
        </div>

        <div
          v-if="testInfo.test_time"
          class="flex items-center justify-center sm:justify-end gap-2"
        >
          <span class="text-xs text-text-secondary">{{ t('lastTest') }}:</span>
          <span class="text-xs text-accent font-medium">{{ formatTime(testInfo.test_time) }}</span>
        </div>
      </div>
    </div>

    <!-- Error Message -->
    <div
      v-if="errorMessage"
      class="bg-red-500/10 border border-red-500/30 rounded-lg p-2 sm:p-3 text-xs sm:text-sm text-red-500 mt-3"
    >
      {{ errorMessage }}
    </div>

    <!-- Success Message (when all checks pass) -->
    <div
      v-if="
        testInfo.config_valid &&
        testInfo.connection_success &&
        testInfo.model_available &&
        !errorMessage
      "
      class="bg-green-500/10 border border-green-500/30 rounded-lg p-2 sm:p-3 text-xs sm:text-sm text-green-500 mt-3"
    >
      {{ t('aiConfigAllGood') }}
    </div>

    <!-- Documentation Link -->
    <div class="mt-3">
      <button
        type="button"
        class="text-xs sm:text-sm text-accent hover:underline flex items-center gap-1"
        @click="openDocumentation"
      >
        <PhBookOpen :size="14" />
        {{ t('aiConfigurationGuide') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../../style.css";

.btn-secondary {
  @apply bg-bg-tertiary border border-border text-text-primary px-3 sm:px-4 py-1.5 sm:py-2 rounded-md cursor-pointer flex items-center gap-1.5 sm:gap-2 font-medium hover:bg-bg-secondary transition-colors;
}

.btn-secondary:disabled {
  @apply cursor-not-allowed opacity-50;
}

.setting-group {
  @apply mb-4 sm:mb-6;
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
