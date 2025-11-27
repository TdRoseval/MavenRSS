<script setup lang="ts">
import { watch, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { PhX, PhCircleNotch } from '@phosphor-icons/vue';
import { useDiscoverAllFeeds } from '@/composables/discovery/useDiscoverAllFeeds';
import DiscoveryProgress from './DiscoveryProgress.vue';
import DiscoveryResults from './DiscoveryResults.vue';
import { useModalClose } from '@/composables/ui/useModalClose';

const { t } = useI18n();

interface Props {
  show: boolean;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  close: [];
}>();

const {
  isDiscovering,
  discoveredFeeds,
  selectedFeeds,
  errorMessage,
  progressMessage,
  progressDetail,
  progressCounts,
  isSubscribing,
  hasSelection,
  allSelected,
  startDiscovery,
  toggleFeedSelection,
  selectAll,
  subscribeSelected,
  cleanup,
} = useDiscoverAllFeeds();

// Modal close handling
useModalClose(() => close());

function close() {
  cleanup();
  emit('close');
}

// Auto-start discovery when component is mounted and shown
onMounted(() => {
  if (props.show) {
    startDiscovery();
  }
});

// Watch for modal opening and trigger discovery (for when modal is reused)
watch(
  () => props.show,
  (newShow, oldShow) => {
    if (newShow && !oldShow) {
      startDiscovery();
    }
  }
);
</script>

<template>
  <div
    v-if="show"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4"
    @click.self="close"
    data-modal-open="true"
  >
    <div
      class="bg-bg-primary w-full max-w-4xl max-h-[90vh] rounded-2xl shadow-2xl border border-border flex flex-col"
    >
      <!-- Header -->
      <div
        class="flex justify-between items-center p-6 border-b border-border bg-gradient-to-r from-accent/5 to-transparent"
      >
        <div>
          <h2 class="text-xl font-bold text-text-primary">{{ t('discoverAllFeeds') }}</h2>
          <p class="text-sm text-text-secondary mt-1">{{ t('discoverAllFeedsDesc') }}</p>
        </div>
        <button @click="close" class="p-2 hover:bg-bg-tertiary rounded-lg transition-colors">
          <PhX :size="24" class="text-text-secondary" />
        </button>
      </div>

      <!-- Content -->
      <div class="flex-1 overflow-y-auto p-6">
        <!-- Loading State -->
        <DiscoveryProgress
          v-if="isDiscovering"
          :progress-message="progressMessage"
          :progress-detail="progressDetail"
          :progress-counts="progressCounts"
        />

        <!-- Error State -->
        <div
          v-else-if="errorMessage"
          class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4 text-red-600 dark:text-red-400"
        >
          {{ errorMessage }}
        </div>

        <!-- Results -->
        <DiscoveryResults
          v-if="discoveredFeeds.length > 0"
          :discovered-feeds="discoveredFeeds"
          :selected-feeds="selectedFeeds"
          :all-selected="allSelected"
          @toggle-feed-selection="toggleFeedSelection"
          @select-all="selectAll"
        />

        <!-- Initial State (should not be visible as discovery auto-starts) -->
        <div v-else class="text-center py-16">
          <PhCircleNotch :size="64" class="text-accent mx-auto mb-4 animate-spin" />
          <p class="text-text-secondary text-lg">{{ t('preparing') }}...</p>
        </div>
      </div>

      <!-- Footer -->
      <div class="flex justify-between items-center p-6 border-t border-border bg-bg-secondary/50">
        <button @click="close" class="btn-secondary" :disabled="isSubscribing">
          {{ t('cancel') }}
        </button>
        <button
          @click="subscribeSelected"
          :disabled="!hasSelection || isSubscribing"
          :class="[
            'btn-primary flex items-center gap-2',
            (!hasSelection || isSubscribing) && 'opacity-50 cursor-not-allowed',
          ]"
        >
          <PhCircleNotch v-if="isSubscribing" :size="16" class="animate-spin" />
          {{ isSubscribing ? t('subscribing') : t('subscribeSelected') }}
          <span
            v-if="hasSelection && !isSubscribing"
            class="bg-white/20 px-2 py-0.5 rounded-full text-sm"
            >({{ selectedFeeds.size }})</span
          >
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.btn-primary {
  @apply px-6 py-2.5 bg-accent text-white rounded-lg hover:bg-accent-hover transition-all font-medium shadow-sm hover:shadow-md;
}

.btn-secondary {
  @apply px-6 py-2.5 bg-bg-tertiary text-text-primary rounded-lg hover:opacity-80 transition-all font-medium;
}
</style>
