<script setup lang="ts">
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { PhTestTube, PhArrowClockwise, PhBookOpen } from '@phosphor-icons/vue';
import { SettingGroup, StatusBoxGroup } from '@/components/settings';
import '@/components/settings/styles.css';
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
        errorMessage.value = data.error_message || t('setting.ai.aiTestFailed');
      } else {
        window.showToast(t('setting.ai.aiTestSuccess'), 'success');
      }
    } else {
      errorMessage.value = t('setting.ai.aiTestFailed');
    }
  } catch (error) {
    console.error('AI test error:', error);
    errorMessage.value = t('setting.ai.aiTestFailed');
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
    return t('common.time.justNow');
  }
}

function openDocumentation() {
  const docUrl = locale.value.startsWith('zh')
    ? 'https://github.com/WCY-dt/MrRSS/blob/main/docs/AI_CONFIGURATION.zh.md'
    : 'https://github.com/WCY-dt/MrRSS/blob/main/docs/AI_CONFIGURATION.md';
  openInBrowser(docUrl);
}

const statuses = computed(() => [
  {
    label: t('setting.ai.configValid'),
    value: testInfo.value.test_time
      ? testInfo.value.config_valid
        ? t('common.action.yes')
        : t('common.action.no')
      : '-',
    type: testInfo.value.test_time
      ? testInfo.value.config_valid
        ? 'success'
        : 'error'
      : 'neutral',
  },
  {
    label: t('setting.ai.connectionSuccess'),
    value: testInfo.value.test_time
      ? testInfo.value.connection_success
        ? t('common.action.yes')
        : t('common.action.no')
      : '-',
    type: testInfo.value.test_time
      ? testInfo.value.connection_success
        ? 'success'
        : 'error'
      : 'neutral',
  },
  {
    label: t('setting.ai.responseTime'),
    value: testInfo.value.response_time_ms > 0 ? testInfo.value.response_time_ms : '-',
    unit: testInfo.value.response_time_ms > 0 ? t('common.time.ms') : '',
  },
]);
</script>

<template>
  <SettingGroup :icon="PhTestTube" :title="t('setting.ai.aiConfigTest')">
    <!-- AI Test Status Display -->
    <StatusBoxGroup
      :statuses="statuses"
      :action-button="{
        label: isTesting ? t('setting.ai.testing') : t('setting.ai.testAIConfig'),
        icon: PhArrowClockwise,
        loading: isTesting,
        onClick: testAIConfig,
      }"
      :status-info="
        testInfo.test_time
          ? {
              label: t('setting.ai.lastTest'),
              time: formatTime(testInfo.test_time),
            }
          : undefined
      "
    />

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
      {{ t('setting.ai.aiConfigAllGood') }}
    </div>

    <!-- Documentation Link -->
    <div class="mt-3">
      <button
        type="button"
        class="text-xs sm:text-sm text-accent hover:underline flex items-center gap-1"
        @click="openDocumentation"
      >
        <PhBookOpen :size="14" />
        {{ t('setting.ai.aiConfigurationGuide') }}
      </button>
    </div>
  </SettingGroup>
</template>

<style scoped>
@reference "../../../../style.css";
</style>
