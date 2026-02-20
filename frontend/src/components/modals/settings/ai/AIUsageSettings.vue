<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { PhChartLine, PhArrowCounterClockwise } from '@phosphor-icons/vue';
import { SettingGroup, SettingItem, StatusBoxGroup } from '@/components/settings';
import '@/components/settings/styles.css';
import type { SettingsData } from '@/types/settings';
import { authGet, authPost } from '@/utils/authFetch';

const { t } = useI18n();

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();

// AI usage tracking
const aiUsage = ref<{
  usage: number;
  limit: number;
  limit_reached: boolean;
}>({
  usage: 0,
  limit: 0,
  limit_reached: false,
});

async function fetchAIUsage() {
  try {
    aiUsage.value = await authGet('/api/ai-usage');
  } catch (e) {
    console.error('Failed to fetch AI usage:', e);
  }
}

async function resetAIUsage() {
  const confirmed = await window.showConfirm({
    title: t('common.confirm'),
    message: t('setting.ai.aiUsageResetConfirm'),
    isDanger: true,
  });
  if (!confirmed) return;

  try {
    await authPost('/api/ai-usage/reset');
    await fetchAIUsage();
    // Reset the local settings value as well
    emit('update:settings', {
      ...props.settings,
      ai_usage_tokens: '0',
    });
    window.showToast(t('setting.ai.aiUsageResetSuccess'), 'success');
  } catch (e) {
    console.error('Failed to reset AI usage:', e);
    window.showToast(t('setting.ai.aiUsageResetError'), 'error');
  }
}

// Helper to get the current limit (use settings value if available)
const currentLimit = computed((): number => {
  if (props.settings.ai_usage_limit !== undefined) {
    return parseInt(props.settings.ai_usage_limit, 10);
  }
  return aiUsage.value.limit;
});

// Calculate usage percentage
function getUsagePercentage(): number {
  if (currentLimit.value === 0) return 0;
  return Math.min(100, (aiUsage.value.usage / currentLimit.value) * 100);
}

// Status box type based on usage
const statusType = computed(() => {
  if (currentLimit.value === 0) return 'neutral';
  if (aiUsage.value.limit_reached) return 'error';
  const percentage = getUsagePercentage();
  if (percentage > 80) return 'warning';
  return 'success';
});

// Token display value - use settings value if available, otherwise fall back to api value
const tokenDisplay = computed(() => {
  if (currentLimit.value > 0) {
    return `${aiUsage.value.usage.toLocaleString()} / ${currentLimit.value.toLocaleString()}`;
  }
  return `${aiUsage.value.usage.toLocaleString()} / ∞`;
});

onMounted(() => {
  fetchAIUsage();
});
</script>

<template>
  <SettingGroup :icon="PhChartLine" :title="t('setting.ai.aiUsage')">
    <!-- AI Usage Display -->
    <StatusBoxGroup
      class="ai-usage-status-group"
      :statuses="[
        {
          label: t('setting.ai.aiUsageTokens'),
          value: tokenDisplay,
          unit: currentLimit > 0 ? t('setting.ai.tokens') : '',
          type: statusType,
        },
      ]"
      :action-button="{
        label: t('setting.ai.aiUsageReset'),
        icon: PhArrowCounterClockwise,
        onClick: resetAIUsage,
      }"
      :status-info="
        currentLimit > 0
          ? {
              label: t('common.text.progress'),
              time: getUsagePercentage().toFixed(2) + '%',
            }
          : undefined
      "
    />

    <!-- Set AI Usage Limit -->
    <SettingItem
      :icon="PhChartLine"
      :title="t('setting.ai.setUsageLimit')"
      :description="t('setting.ai.setUsageLimitDesc')"
    >
      <div class="flex items-center gap-2">
        <input
          :value="props.settings.ai_usage_limit"
          type="number"
          min="0"
          :placeholder="t('setting.ai.aiUsageLimitPlaceholder')"
          class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          @input="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                ai_usage_limit: (e.target as HTMLInputElement).value,
              })
          "
        />
        <span v-if="props.settings.ai_usage_limit === '0'" class="text-accent text-xs sm:text-sm font-medium">
          ({{ t('common.text.unlimited') }})
        </span>
      </div>
    </SettingItem>
  </SettingGroup>
</template>

<style scoped>
@reference "../../../../style.css";

.input-field {
  @apply p-1.5 sm:p-2.5 border border-border rounded-md bg-bg-secondary text-text-primary focus:border-accent focus:outline-none transition-colors;
}

.ai-usage-status-group :deep(.status-box) {
  @apply sm:min-w-[180px];
}
</style>
