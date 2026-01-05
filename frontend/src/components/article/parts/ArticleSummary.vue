<script setup lang="ts">
/* eslint-disable vue/no-v-html */
import { ref, computed, watch, onUnmounted } from 'vue';
import {
  PhTextAlignLeft,
  PhSpinnerGap,
  PhPlay,
  PhWarning,
  PhClock,
  PhBrain,
} from '@phosphor-icons/vue';
import { useI18n } from 'vue-i18n';

interface Props {
  summaryResult: {
    summary: string;
    html?: string;
    sentence_count: number;
    is_too_short: boolean;
    limit_reached?: boolean;
    used_fallback?: boolean;
    thinking?: string;
    error?: string;
  } | null;
  isLoadingSummary: boolean;
  translationEnabled: boolean;
  summaryProvider?: string;
  summaryTriggerMode?: string;
  isLoadingContent?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  summaryProvider: 'local',
  summaryTriggerMode: 'auto',
  isLoadingContent: false,
});

const emit = defineEmits<{
  'generate-summary': [];
}>();

const { t } = useI18n();

const showSummary = ref(true);
const showThinking = ref(false);

// Enhanced loading states
const loadingTime = ref(0);
const loadingStartTime = ref<number | null>(null);

// Track loading time for better UX
watch(
  () => props.isLoadingSummary,
  (isLoading: boolean) => {
    if (isLoading && !loadingStartTime.value) {
      loadingStartTime.value = Date.now();
      loadingTime.value = 0;
    } else if (!isLoading && loadingStartTime.value) {
      loadingStartTime.value = null;
      loadingTime.value = 0;
    }
  }
);

let intervalId: number | null = null;

// Update loading time display
intervalId = window.setInterval(() => {
  if (props.isLoadingSummary && loadingStartTime.value) {
    loadingTime.value = Math.floor((Date.now() - loadingStartTime.value) / 1000);
  }
}, 100);

// Cleanup interval on unmount
onUnmounted(() => {
  if (intervalId) {
    clearInterval(intervalId);
  }
});

// Check if should show manual trigger button
const shouldShowManualTrigger = computed(() => {
  return (
    props.summaryProvider === 'ai' &&
    props.summaryTriggerMode === 'manual' &&
    !props.summaryResult &&
    !props.isLoadingSummary &&
    !props.isLoadingContent
  );
});

// Generate summary on button click
function handleGenerateSummary() {
  emit('generate-summary');
}
</script>

<template>
  <!-- Summary Section -->
  <div
    v-if="summaryResult || isLoadingSummary || shouldShowManualTrigger"
    class="mb-6 p-4 rounded-lg border border-border bg-bg-secondary"
  >
    <!-- Summary Header -->
    <div
      class="flex items-center justify-between cursor-pointer"
      @click="showSummary = !showSummary"
    >
      <div class="flex items-center gap-2 text-accent font-medium">
        <PhTextAlignLeft :size="20" />
        <span>{{ t('articleSummary') }}</span>
      </div>
      <span class="text-xs text-text-secondary">
        {{ showSummary ? '▲' : '▼' }}
      </span>
    </div>

    <!-- Summary Content -->
    <div v-if="showSummary" class="mt-3">
      <!-- Enhanced Loading State -->
      <div v-if="isLoadingSummary" class="flex flex-col items-center gap-3 py-4">
        <!-- Loading Animation -->
        <div class="flex items-center gap-3">
          <PhSpinnerGap :size="24" class="animate-spin text-accent" />
          <div class="flex flex-col gap-1">
            <div class="text-sm font-medium text-text-primary">
              {{
                props.summaryProvider === 'ai' ? t('generatingAISummary') : t('generatingSummary')
              }}
            </div>
            <div class="flex items-center gap-2 text-xs text-text-secondary">
              <PhClock :size="12" />
              <span>{{ t('generatingSummaryTime', { seconds: loadingTime }) }}</span>
            </div>
          </div>
        </div>

        <!-- Progress Indicator -->
        <div class="w-full max-w-xs">
          <div class="text-xs text-text-secondary text-center mb-2">
            {{
              loadingTime < 3
                ? t('summaryInitiating')
                : loadingTime < 8
                  ? t('summaryProcessing')
                  : t('summaryAlmostDone')
            }}
          </div>
          <div class="w-full bg-bg-tertiary rounded-full h-1.5 overflow-hidden">
            <div
              class="bg-accent h-full transition-all duration-300 ease-out rounded-full"
              :style="{ width: `${Math.min(loadingTime * 12, 95)}%` }"
            />
          </div>
        </div>

        <!-- Provider-specific Tips -->
        <div
          v-if="props.summaryProvider === 'ai'"
          class="text-xs text-text-secondary text-center italic"
        >
          {{ t('summaryProcessingTip') }}
        </div>
      </div>

      <!-- Manual Trigger Button (only shown for AI manual mode when no summary) -->
      <div v-else-if="shouldShowManualTrigger" class="flex flex-col items-center gap-3 py-4">
        <div class="text-sm text-text-secondary text-center">
          {{ t('summaryManualTriggerDesc') }}
        </div>
        <button
          class="flex items-center gap-2 px-4 py-2 bg-accent text-white rounded-lg hover:bg-accent/90 transition-colors"
          @click="handleGenerateSummary"
        >
          <PhPlay :size="16" />
          <span class="text-sm font-medium">{{ t('generateSummary') }}</span>
        </button>
      </div>

      <!-- Too Short Warning -->
      <div v-else-if="summaryResult?.is_too_short" class="flex flex-col items-center gap-3 py-4">
        <div class="text-sm text-text-secondary text-center">
          {{ t('summaryTooShort') }}
        </div>
        <button
          class="flex items-center gap-2 px-4 py-2 bg-accent text-white rounded-lg hover:bg-accent/90 transition-colors"
          :disabled="isLoadingSummary"
          @click="handleGenerateSummary"
        >
          <PhSpinnerGap v-if="isLoadingSummary" :size="16" class="animate-spin" />
          <PhPlay v-else :size="16" />
          <span class="text-sm font-medium">{{ t('regenerateSummary') }}</span>
        </button>
      </div>

      <!-- Summary Display -->
      <div v-else-if="summaryResult?.summary">
        <!-- Regenerate Button -->
        <div class="flex justify-between items-center mb-2">
          <button
            v-if="summaryResult.thinking"
            class="flex items-center gap-1 px-2 py-1 text-xs bg-bg-secondary text-text-secondary rounded hover:bg-bg-tertiary transition-colors"
            @click="showThinking = !showThinking"
          >
            <PhBrain :size="12" />
            <span>{{ showThinking ? t('hideThinking') : t('showThinking') }}</span>
          </button>
          <div class="flex-1"></div>
          <button
            class="flex items-center gap-1 px-2 py-1 text-xs bg-bg-secondary text-text-secondary rounded hover:bg-bg-tertiary transition-colors"
            :disabled="isLoadingSummary"
            @click="handleGenerateSummary"
          >
            <PhSpinnerGap v-if="isLoadingSummary" :size="12" class="animate-spin" />
            <PhPlay v-else :size="12" />
            <span>{{ t('regenerateSummary') }}</span>
          </button>
        </div>

        <!-- Thinking section -->
        <div
          v-if="summaryResult.thinking && showThinking"
          class="mb-3 p-3 bg-bg-tertiary border-l-2 border-accent rounded text-xs text-text-secondary"
        >
          <div class="font-bold mb-2 flex items-center gap-1">
            <PhBrain :size="12" />
            {{ t('thinking') }}
          </div>
          <div class="whitespace-pre-wrap">{{ summaryResult.thinking }}</div>
        </div>

        <!-- Show original summary -->
        <div
          class="text-sm text-text-primary leading-relaxed select-text prose prose-sm max-w-none"
          v-html="summaryResult.html || summaryResult.summary"
        ></div>
      </div>

      <!-- Error State -->
      <div v-else-if="summaryResult?.error" class="flex flex-col items-center gap-3 py-4">
        <div class="flex items-center gap-2 text-accent-error">
          <PhWarning :size="20" />
          <span class="text-sm font-medium">{{ t('summaryGenerationFailed') }}</span>
        </div>
        <div class="text-xs text-text-secondary text-center max-w-xs">
          {{ summaryResult.error }}
        </div>
        <!-- Retry Button for AI Summary -->
        <button
          v-if="props.summaryProvider === 'ai'"
          class="flex items-center gap-2 px-3 py-1.5 bg-accent text-white rounded-md hover:bg-accent/90 transition-colors text-xs"
          @click="handleGenerateSummary"
        >
          <PhPlay :size="14" />
          <span>{{ t('retrySummary') }}</span>
        </button>
      </div>

      <!-- No Summary Available -->
      <div v-else class="text-sm text-text-secondary italic">{{ t('noSummaryAvailable') }}</div>
    </div>
  </div>
</template>

<style>
/* Enable text selection for summary content */
.prose.select-text,
.prose.select-text * {
  user-select: text !important;
  -webkit-user-select: text !important;
  -moz-user-select: text !important;
  -ms-user-select: text !important;
}

/* Markdown prose styles */
.prose {
  color: inherit;
}

.prose pre {
  margin: 0.5rem 0;
  white-space: pre-wrap;
  word-wrap: break-word;
}

.prose code {
  font-family: 'Courier New', Courier, monospace;
}

.prose ul {
  list-style-type: disc;
}

.prose ol {
  list-style-type: decimal;
}
</style>
