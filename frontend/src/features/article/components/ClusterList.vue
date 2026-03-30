<script setup lang="ts">
import { useAuthStore } from '@/stores/auth';
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue';
import {
  PhCheckCircle,
  PhTrash
} from '@phosphor-icons/vue';
import ClusterItem from './ClusterItem.vue';
import { useSettings } from '@/composables/core/useSettings';
import { authFetch } from '@/shared/lib/authFetch';
import type { Cluster } from '@/types/models';
import { useClusterStore } from '@/stores/cluster';
import { apiClient } from '@/shared/lib/apiClient';

const clusterStore = useClusterStore();
const authStore = useAuthStore();
const { t } = useI18n();
const { settings } = useSettings();

const listRef = ref<HTMLDivElement | null>(null);
const shouldRestoreScroll = ref(false);

interface Props {
  isSidebarOpen?: boolean;
  isMobile?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  isSidebarOpen: true,
  isMobile: false,
});

const emit = defineEmits<{
  toggleSidebar: [];
  selectCluster: [clusterId: number];
}>();

// Layout mode computed
const layoutMode = computed(() => settings.value.layout_mode || 'normal');
const isCardMode = computed(() => layoutMode.value === 'card');

// Virtual scroll state
const scrollTop = ref(0);
const containerHeight = ref(0);
const ITEM_HEIGHT = 96; // Estimated height per item
const BUFFER_SIZE = 10;

const visibleRange = computed(() => {
  const clusters = clusterStore.clusters;
  if (!clusters.length) return { start: 0, end: 0 };

  const start = Math.max(0, Math.floor(scrollTop.value / ITEM_HEIGHT) - BUFFER_SIZE);
  const end = Math.min(
    clusters.length,
    Math.ceil((scrollTop.value + containerHeight.value) / ITEM_HEIGHT) + BUFFER_SIZE
  );

  return { start, end };
});

const visibleClusters = computed(() => {
  const clusters = clusterStore.clusters;
  const { start, end } = visibleRange.value;
  return clusters.slice(start, end);
});

const listPaddingTop = computed(() => {
  return visibleRange.value.start * ITEM_HEIGHT;
});

const listPaddingBottom = computed(() => {
  return (clusterStore.clusters.length - visibleRange.value.end) * ITEM_HEIGHT;
});

onMounted(() => {
  if (authStore.isAuthenticated) {
    clusterStore.fetchClusters();
  }
});

// Watch for array length changes (list content changes)
watch(
  () => clusterStore.clusters.length,
  async () => {
    if (shouldRestoreScroll.value && listRef.value) {
      const currentScroll = listRef.value.scrollTop;
      await nextTick();
      listRef.value.scrollTop = currentScroll;
      shouldRestoreScroll.value = false;
    }
  }
);

let scrollThrottleTimer: ReturnType<typeof setTimeout> | null = null;
onBeforeUnmount(() => {
  if (scrollThrottleTimer) {
    clearTimeout(scrollThrottleTimer);
    scrollThrottleTimer = null;
  }
});

function selectCluster(cluster: Cluster): void {
  if (!authStore.isAuthenticated) {
    return;
  }

  clusterStore.currentClusterId = cluster.id;
  if (!cluster.is_read) {
    cluster.is_read = true;
    apiClient
      .post('/clusters/read', { id: cluster.id, read: true })
      .catch((e) => {
        console.error('Error marking as read:', e);
      });
  }

  if (props.isMobile) {
    emit('selectCluster', cluster.id);
  }
}

const SCROLL_THROTTLE_DELAY = 200;
const SCROLL_THRESHOLD = 400;

function handleScroll(e: Event): void {
  const target = e.target as HTMLElement;

  scrollTop.value = target.scrollTop;
  containerHeight.value = target.clientHeight;

  if (scrollThrottleTimer) return;

  scrollThrottleTimer = setTimeout(() => {
    scrollThrottleTimer = null;
    const { scrollTop, clientHeight, scrollHeight } = target;

    if (scrollTop + clientHeight >= scrollHeight - SCROLL_THRESHOLD) {
      clusterStore.loadMore();
    }
  }, SCROLL_THROTTLE_DELAY);
}

function handleContextmenu(e: MouseEvent, cluster: Cluster) {
   // Minimal context menu stub
   e.preventDefault();
}

async function markAllAsRead(): Promise<void> {
  // Not implemented for clusters yet - stub
  window.showToast(t('common.toast.clearedReadLater'), 'info');
}

function handleHoverMarkAsRead(clusterId: number): void {
  const cluster = clusterStore.clusters.find((c) => c.id === clusterId);
  if (cluster) {
    cluster.is_read = true;
  }
}
</script>

<template>
  <section
    :class="[
      'article-list flex flex-col w-full border-r border-border bg-bg-primary shrink-0 h-full',
      { 'card-mode': isCardMode },
    ]"
  >
    <div class="p-2 sm:p-4 border-b border-border bg-bg-primary">
      <div class="flex items-center justify-between">
        <h3
          class="m-0 text-base sm:text-lg font-semibold truncate flex-1"
          title="AI Fusion Clusters"
        >
          AI Fusion Clusters
        </h3>
        <div class="flex items-center gap-1 sm:gap-2">
          <button
            class="text-text-secondary hover:text-text-primary hover:bg-bg-tertiary p-1 sm:p-1.5 rounded transition-colors"
            title="Mark all clusters as read"
            @click="markAllAsRead"
          >
            <PhCheckCircle :size="18" class="sm:w-5 sm:h-5" />
          </button>
        </div>
      </div>
    </div>

    <!-- Loading State for full refresh -->
    <div
      v-if="clusterStore.isLoading && clusterStore.page === 1"
      class="flex-1 flex flex-col items-center justify-center p-8 text-text-secondary"
    >
      <div class="w-8 h-8 border-4 border-accent border-t-transparent rounded-full animate-spin mb-4" />
      <div>{{ t('article.content.loadingContent') }}</div>
    </div>

    <!-- Virtual Scrolling List -->
    <div
      v-else-if="clusterStore.clusters.length > 0"
      ref="listRef"
      class="flex-1 overflow-y-auto w-full custom-scrollbar"
      @scroll="handleScroll"
    >
      <div
        class="w-full relative"
        :style="{
          paddingTop: `${listPaddingTop}px`,
          paddingBottom: `${listPaddingBottom}px`,
        }"
      >
        <ClusterItem
          v-for="cluster in visibleClusters"
          :key="cluster.id"
          :cluster="cluster"
          :is-active="clusterStore.currentClusterId === cluster.id"
          @click="selectCluster(cluster)"
          @contextmenu="handleContextmenu($event, cluster)"
          @hover-mark-as-read="handleHoverMarkAsRead"
        />

        <!-- Loading More Indicator -->
        <div
          v-if="clusterStore.isLoading && clusterStore.page > 1"
          class="py-4 flex justify-center items-center text-text-secondary w-full"
        >
          <div class="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin mr-2" />
          <span class="text-sm">Loading more...</span>
        </div>
      </div>
    </div>
    
    <!-- Empty State -->
    <div v-else class="flex-1 flex flex-col items-center justify-center p-8 text-text-secondary text-center">
      <div class="w-16 h-16 bg-bg-tertiary rounded-full flex items-center justify-center mb-4">
        <span class="text-2xl">⚡️</span>
      </div>
      <h3 class="text-lg font-medium text-text-primary mb-2">No Clusters Found</h3>
      <p class="text-sm max-w-[250px]">
        Articles will automatically be grouped and merged here as they are processed by the AI integration engine.
      </p>
    </div>
  </section>
</template>

<style scoped>
@reference "../../style.css";
</style>
