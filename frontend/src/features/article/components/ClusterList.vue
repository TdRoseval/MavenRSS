<script setup lang="ts">
import { useAuthStore } from '@/stores/auth';
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue';
import { PhCheckCircle } from '@phosphor-icons/vue';
import ClusterItem from './ClusterItem.vue';
import { useSettings } from '@/composables/core/useSettings';
import type { Cluster } from '@/types/models';
import { useClusterStore } from '@/stores/cluster';

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

const layoutMode = computed(() => settings.value.layout_mode || 'normal');
const isCardMode = computed(() => layoutMode.value === 'card');
const itemHeight = computed(() => (layoutMode.value === 'compact' ? 104 : 128));

const scrollTop = ref(0);
const containerHeight = ref(0);
const BUFFER_SIZE = 10;

const visibleRange = computed(() => {
  const clusters = clusterStore.clusters;
  if (!clusters.length) return { start: 0, end: 0 };

  const start = Math.max(0, Math.floor(scrollTop.value / itemHeight.value) - BUFFER_SIZE);
  const end = Math.min(
    clusters.length,
    Math.ceil((scrollTop.value + containerHeight.value) / itemHeight.value) + BUFFER_SIZE
  );

  return { start, end };
});

const visibleClusters = computed(() => {
  const clusters = clusterStore.clusters;
  const { start, end } = visibleRange.value;
  return clusters.slice(start, end);
});

const listPaddingTop = computed(() => {
  return visibleRange.value.start * itemHeight.value;
});

const listPaddingBottom = computed(() => {
  return (clusterStore.clusters.length - visibleRange.value.end) * itemHeight.value;
});

onMounted(async () => {
  if (authStore.isAuthenticated) {
    await clusterStore.fetchClusters();
    await nextTick();
    if (listRef.value) {
      containerHeight.value = listRef.value.clientHeight;
    }
  }
});

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

  // Fire-and-forget Level 1 click feedback for interest vector update
  clusterStore.reportClusterClick(cluster.id);

  if (!cluster.is_read) {
    clusterStore.markClusterRead(cluster.id, true).catch((e) => {
      console.error('Error marking as read:', e);
      window.showToast(t('common.errors.savingSettings'), 'error');
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
  e.preventDefault();
  e.stopPropagation();

  window.dispatchEvent(
    new CustomEvent('open-context-menu', {
      detail: {
        x: e.clientX,
        y: e.clientY,
        items: [
          {
            label: cluster.is_read
              ? t('article.action.markAsUnread')
              : t('article.action.markAsRead'),
            action: 'toggleRead',
            icon: cluster.is_read ? 'ph-envelope' : 'ph-envelope-open',
          },
          {
            label: cluster.is_favorite
              ? t('article.action.removeFromFavorite')
              : t('article.action.addToFavorite'),
            action: 'toggleFavorite',
            icon: 'ph-star',
            iconWeight: cluster.is_favorite ? 'fill' : 'regular',
            iconColor: cluster.is_favorite ? 'text-yellow-500' : '',
          },
          {
            label: cluster.is_read_later
              ? t('article.action.removeFromReadLater')
              : t('article.action.addToReadLater'),
            action: 'toggleReadLater',
            icon: 'ph-clock-countdown',
            iconWeight: cluster.is_read_later ? 'fill' : 'regular',
            iconColor: cluster.is_read_later ? 'text-blue-500' : '',
          },
        ],
        callback: async (action: string, targetCluster: Cluster) => {
          try {
            if (action === 'toggleRead') {
              await clusterStore.markClusterRead(targetCluster.id, !targetCluster.is_read);
            } else if (action === 'toggleFavorite') {
              await clusterStore.toggleClusterFavorite(targetCluster);
            } else if (action === 'toggleReadLater') {
              await clusterStore.toggleClusterReadLater(targetCluster);
            }
          } catch (error) {
            console.error('Error handling cluster action:', error);
            window.showToast(t('common.errors.savingSettings'), 'error');
          }
        },
        data: cluster,
      },
    })
  );
}

async function markAllAsRead(): Promise<void> {
  const confirmed = await window.showConfirm({
    title: t('article.cluster.markAllReadTitle'),
    message: t('article.cluster.markAllReadConfirm'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    isDanger: false,
  });

  if (!confirmed) {
    return;
  }

  try {
    await clusterStore.markAllAsRead();
    window.showToast(t('article.cluster.markedAllAsRead'), 'success');
  } catch (error) {
    console.error('Error marking all clusters as read:', error);
    window.showToast(t('common.errors.savingSettings'), 'error');
  }
}

function handleHoverMarkAsRead(clusterId: number): void {
  clusterStore.updateClusterState(clusterId, { is_read: true });
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
          :title="t('article.cluster.listTitle')"
        >
          {{ t('article.cluster.listTitle') }}
        </h3>
        <div class="flex items-center gap-1 sm:gap-2">
          <button
            class="text-text-secondary hover:text-text-primary hover:bg-bg-tertiary p-1 sm:p-1.5 rounded transition-colors"
            :title="t('article.cluster.markAllReadTitle')"
            @click="markAllAsRead"
          >
            <PhCheckCircle :size="18" class="sm:w-5 sm:h-5" />
          </button>
        </div>
      </div>
    </div>

    <!-- Loading State for full refresh -->
    <div
      v-if="clusterStore.isInitialLoading"
      class="flex-1 flex flex-col items-center justify-center p-8 text-text-secondary"
    >
      <div
        class="w-8 h-8 border-4 border-accent border-t-transparent rounded-full animate-spin mb-4"
      />
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
          v-if="clusterStore.isLoadingMore"
          class="py-4 flex justify-center items-center text-text-secondary w-full"
        >
          <div
            class="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin mr-2"
          />
          <span class="text-sm">{{ t('article.cluster.loadingMore') }}</span>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div
      v-else
      class="flex-1 flex flex-col items-center justify-center p-8 text-text-secondary text-center"
    >
      <div class="w-16 h-16 bg-bg-tertiary rounded-full flex items-center justify-center mb-4">
        <span class="text-2xl">⚡️</span>
      </div>
      <h3 class="text-lg font-medium text-text-primary mb-2">
        {{ t('article.cluster.emptyTitle') }}
      </h3>
      <p class="text-sm max-w-[250px]">
        {{ t('article.cluster.emptyDescription') }}
      </p>
    </div>
  </section>
</template>

<style scoped>
@reference "../../style.css";
</style>
