<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { PhTextAlignLeft, PhTextT, PhPackage, PhRobot, PhInfo } from '@phosphor-icons/vue';
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
      <PhTextAlignLeft :size="14" class="sm:w-4 sm:h-4" />
      {{ t('summary') }}
    </label>
    <div class="setting-item mb-2 sm:mb-4">
      <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
        <PhTextT :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
        <div class="flex-1 min-w-0">
          <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
            {{ t('enableSummary') }}
          </div>
          <div class="text-xs text-text-secondary hidden sm:block">
            {{ t('enableSummaryDesc') }}
          </div>
        </div>
      </div>
      <input
        :checked="props.settings.summary_enabled"
        type="checkbox"
        class="toggle"
        @change="
          (e) =>
            emit('update:settings', {
              ...props.settings,
              summary_enabled: (e.target as HTMLInputElement).checked,
            })
        "
      />
    </div>

    <div
      v-if="props.settings.summary_enabled"
      class="ml-2 sm:ml-4 space-y-2 sm:space-y-3 border-l-2 border-border pl-2 sm:pl-4"
    >
      <div class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhPackage :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('summaryProvider') }}</div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('summaryProviderDesc') }}
            </div>
          </div>
        </div>
        <select
          :value="props.settings.summary_provider"
          class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          @change="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                summary_provider: (e.target as HTMLSelectElement).value,
              })
          "
        >
          <option value="local">{{ t('localAlgorithm') }}</option>
          <option value="ai">{{ t('aiSummary') }}</option>
        </select>
      </div>

      <!-- AI Summary Prompt -->
      <div v-if="props.settings.summary_provider === 'ai'" class="tip-box">
        <PhInfo :size="16" class="text-accent shrink-0 sm:w-5 sm:h-5" />
        <span class="text-xs sm:text-sm">{{ t('aiSettingsConfiguredInAITab') }}</span>
      </div>
      <div
        v-if="props.settings.summary_provider === 'ai'"
        class="sub-setting-item flex-col items-stretch gap-2"
      >
        <div class="flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhRobot :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('aiSummaryPrompt') }}</div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('aiSummaryPromptDesc') }}
            </div>
          </div>
        </div>
        <textarea
          :value="props.settings.ai_summary_prompt"
          class="input-field w-full text-xs sm:text-sm resize-none"
          rows="3"
          :placeholder="t('aiSummaryPromptPlaceholder')"
          @input="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                ai_summary_prompt: (e.target as HTMLTextAreaElement).value,
              })
          "
        />
      </div>

      <div class="sub-setting-item">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhTextAlignLeft :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm">{{ t('summaryLength') }}</div>
            <div class="text-xs text-text-secondary hidden sm:block">
              {{ t('summaryLengthDesc') }}
            </div>
          </div>
        </div>
        <select
          :value="props.settings.summary_length"
          class="input-field w-32 sm:w-48 text-xs sm:text-sm"
          @change="
            (e) =>
              emit('update:settings', {
                ...props.settings,
                summary_length: (e.target as HTMLSelectElement).value,
              })
          "
        >
          <option value="short">{{ t('summaryLengthShort') }}</option>
          <option value="medium">{{ t('summaryLengthMedium') }}</option>
          <option value="long">{{ t('summaryLengthLong') }}</option>
        </select>
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
.tip-box {
  @apply flex items-center gap-2 sm:gap-3 py-2 sm:py-2.5 px-2.5 sm:px-3 rounded-lg w-full;
  background-color: rgba(59, 130, 246, 0.05);
  border: 1px solid rgba(59, 130, 246, 0.3);
}
</style>
