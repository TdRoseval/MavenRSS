<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, type Ref } from 'vue';
import { useAppStore } from '@/stores/app';
import { useI18n } from 'vue-i18n';
import { PhX, PhCircleNotch } from '@phosphor-icons/vue';
import type { Feed } from '@/types/models';
import DiscoveredFeedItem from './DiscoveredFeedItem.vue';
import DiscoveryProgress from './DiscoveryProgress.vue';
import { useModalClose } from '@/composables/ui/useModalClose';

const store = useAppStore();
const { t } = useI18n();

// Modal close handling
useModalClose(() => close());

interface Props {
  feed: Feed;
  show: boolean;
}

interface DiscoveredFeed {
  name: string;
  homepage: string;
  rss_feed: string;
  icon_url?: string;
  recent_articles?: Array<{
    title: string;
    date?: string;
  }>;
}

interface ProgressCounts {
  current: number;
  total: number;
  found: number;
}

interface ProgressState {
  is_complete: boolean;
  error?: string;
  feeds?: DiscoveredFeed[];
  progress?: {
    stage: string;
    message?: string;
    detail?: string;
    current?: number;
    total?: number;
    found_count?: number;
  };
}

const props = defineProps<Props>();

const emit = defineEmits<{
  close: [];
}>();

const isDiscovering = ref(false);
const discoveredFeeds: Ref<DiscoveredFeed[]> = ref([]);
const selectedFeeds: Ref<Set<number>> = ref(new Set());
const errorMessage = ref('');
const progressMessage = ref('');
const progressDetail = ref('');
const progressCounts: Ref<ProgressCounts> = ref({ current: 0, total: 0, found: 0 });
const isSubscribing = ref(false);
let pollInterval: ReturnType<typeof setInterval> | null = null;

function getHostname(url: string): string {
  try {
    return new URL(url).hostname;
  } catch {
    return url;
  }
}

async function startDiscovery() {
  isDiscovering.value = true;
  errorMessage.value = '';
  discoveredFeeds.value = [];
  selectedFeeds.value.clear();
  progressMessage.value = t('fetchingHomepage');
  progressDetail.value = '';
  progressCounts.value = { current: 0, total: 0, found: 0 };

  // Clear any existing poll interval
  if (pollInterval) {
    clearInterval(pollInterval);
    pollInterval = null;
  }

  try {
    // Validate feed ID
    if (!props.feed?.id) {
      throw new Error('Invalid feed ID');
    }

    // Clear any previous discovery state
    await fetch('/api/feeds/discover/clear', { method: 'POST' });

    // Start discovery in background
    const startResponse = await fetch('/api/feeds/discover/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ feed_id: props.feed.id }),
    });

    if (!startResponse.ok) {
      const errorText = await startResponse.text();
      throw new Error(errorText || 'Failed to start discovery');
    }

    // Start polling for progress
    pollInterval = setInterval(async () => {
      try {
        const progressResponse = await fetch('/api/feeds/discover/progress');
        if (!progressResponse.ok) {
          throw new Error('Failed to get progress');
        }

        const state = (await progressResponse.json()) as ProgressState;

        // Update progress display
        if (state.progress) {
          const progress = state.progress;
          switch (progress.stage) {
            case 'fetching_homepage':
              progressMessage.value = t('fetchingHomepage');
              progressDetail.value = progress.detail ? getHostname(progress.detail) : '';
              break;
            case 'finding_friend_links':
              progressMessage.value = t('searchingFriendLinks');
              progressDetail.value = progress.detail ? getHostname(progress.detail) : '';
              break;
            case 'fetching_friend_page':
              progressMessage.value = t('fetchingFriendPage');
              progressDetail.value = progress.detail ? getHostname(progress.detail) : '';
              break;
            case 'found_links':
              progressMessage.value = t('foundPotentialLinks', { count: progress.total });
              progressDetail.value = '';
              progressCounts.value.total = progress.total || 0;
              break;
            case 'checking_rss':
              progressMessage.value = t('checkingRssFeed');
              progressDetail.value = progress.detail ? getHostname(progress.detail) : '';
              progressCounts.value.current = progress.current || 0;
              progressCounts.value.total = progress.total || 0;
              progressCounts.value.found = progress.found_count || 0;
              break;
            default:
              progressMessage.value = progress.message || t('discovering');
              progressDetail.value = progress.detail ? getHostname(progress.detail) : '';
          }
        }

        // Check if complete
        if (state.is_complete) {
          if (pollInterval !== null) {
            clearInterval(pollInterval);
            pollInterval = null;
          }

          if (state.error) {
            errorMessage.value = t('discoveryFailed') + ': ' + state.error;
          } else {
            discoveredFeeds.value = state.feeds || [];
            if (discoveredFeeds.value.length === 0) {
              errorMessage.value = t('noFriendLinksFound');
            }
          }

          isDiscovering.value = false;
          progressMessage.value = '';
          progressDetail.value = '';

          // Clear the discovery state
          await fetch('/api/feeds/discover/clear', { method: 'POST' });
        }
      } catch (pollError) {
        console.error('Polling error:', pollError);
        // Don't stop polling on transient errors
      }
    }, 500); // Poll every 500ms
  } catch (error) {
    console.error('Discovery error:', error);
    errorMessage.value = t('discoveryFailed') + ': ' + (error as Error).message;
    isDiscovering.value = false;
    progressMessage.value = '';
    progressDetail.value = '';
    if (pollInterval) {
      clearInterval(pollInterval);
      pollInterval = null;
    }
  }
}

function toggleFeedSelection(index: number) {
  if (selectedFeeds.value.has(index)) {
    selectedFeeds.value.delete(index);
  } else {
    selectedFeeds.value.add(index);
  }
}

function selectAll() {
  if (selectedFeeds.value.size === discoveredFeeds.value.length) {
    selectedFeeds.value.clear();
  } else {
    discoveredFeeds.value.forEach((_, index) => selectedFeeds.value.add(index));
  }
}

const hasSelection = computed(() => selectedFeeds.value.size > 0);
const allSelected = computed(
  () =>
    discoveredFeeds.value.length > 0 && selectedFeeds.value.size === discoveredFeeds.value.length
);

async function subscribeSelected() {
  if (!hasSelection.value) return;

  isSubscribing.value = true;
  const subscribePromises = [];

  for (const index of selectedFeeds.value) {
    const feed = discoveredFeeds.value[index];
    const promise = fetch('/api/feeds/add', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        url: feed.rss_feed,
        category: props.feed.category || '',
        title: feed.name,
      }),
    });
    subscribePromises.push(promise);
  }

  try {
    const results = await Promise.allSettled(subscribePromises);
    const successful = results.filter((r) => r.status === 'fulfilled').length;
    const failed = results.filter((r) => r.status === 'rejected').length;

    await store.fetchFeeds();

    if (failed === 0) {
      window.showToast(t('feedsSubscribedSuccess', { count: successful }), 'success');
    } else {
      window.showToast(t('feedsSubscribedPartial', { successful, failed }), 'warning');
    }
    emit('close');
  } catch (error) {
    console.error('Subscription error:', error);
    window.showToast(t('errorSubscribingFeeds'), 'error');
  } finally {
    isSubscribing.value = false;
  }
}

function close() {
  // Clear polling interval if active
  if (pollInterval) {
    clearInterval(pollInterval);
    pollInterval = null;
  }
  // Clear discovery state on server
  fetch('/api/feeds/discover/clear', { method: 'POST' }).catch(() => {});
  emit('close');
}

// Auto-start discovery when component is mounted
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

// Cleanup on unmount
onUnmounted(() => {
  if (pollInterval !== null) {
    clearInterval(pollInterval);
    pollInterval = null;
  }
  // Clear discovery state on server
  fetch('/api/feeds/discover/clear', { method: 'POST' }).catch(() => {});
});
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
          <h2 class="text-xl font-bold text-text-primary">{{ t('discoverFeeds') }}</h2>
          <p class="text-sm text-text-secondary mt-1">{{ t('fromFeed') }}: {{ feed.title }}</p>
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
        <div v-else-if="discoveredFeeds.length > 0">
          <div class="mb-4 flex items-center justify-between bg-bg-secondary rounded-lg p-3">
            <p class="text-sm font-medium text-text-primary">
              {{ t('foundFeeds', { count: discoveredFeeds.length }) }}
            </p>
            <button
              @click="selectAll"
              class="text-sm text-accent hover:text-accent-hover font-medium px-3 py-1 rounded hover:bg-accent/10 transition-colors"
            >
              {{ allSelected ? t('deselectAll') : t('selectAll') }}
            </button>
          </div>

          <div class="space-y-3">
            <DiscoveredFeedItem
              v-for="(feed, index) in discoveredFeeds"
              :key="index"
              :feed="feed"
              :is-selected="selectedFeeds.has(index)"
              @toggle="toggleFeedSelection(index)"
            />
          </div>
        </div>

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
