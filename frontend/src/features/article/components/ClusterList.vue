<script setup lang="ts">
import { useAuthStore } from '@/stores/auth';
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue';
import { PhCheckCircle, PhArrowClockwise } from '@phosphor-icons/vue';
import ClusterItem from './ClusterItem.vue';
import { useSettings } from '@/composables/core/useSettings';
import type { Cluster, DailyRecommendationItem } from '@/types/models';
import { useClusterStore } from '@/stores/cluster';
import { useArticleStore } from '@/features/article/store';

const clusterStore = useClusterStore();
const articleStore = useArticleStore();
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
const isDailyRecommendationMode = computed(
  () => articleStore.currentFilter === 'dailyRecommendations'
);

const temporarilyKeptClusterIds = ref<Set<number>>(new Set());

const displayedClusters = computed<Cluster[]>(() => {
  const source = isDailyRecommendationMode.value
    ? clusterStore.dailyRecommendations.map((item: DailyRecommendationItem) => item.cluster)
    : clusterStore.clusters;

  if (!articleStore.showOnlyUnread) {
    return source;
  }

  return source.filter((cluster) => !cluster.is_read || temporarilyKeptClusterIds.value.has(cluster.id));
});

const scrollTop = ref(0);
const containerHeight = ref(0);
const BUFFER_SIZE = 10;

const visibleRange = computed(() => {
  const clusters = displayedClusters.value;
  if (!clusters.length) return { start: 0, end: 0 };

  const start = Math.max(0, Math.floor(scrollTop.value / itemHeight.value) - BUFFER_SIZE);
  const end = Math.min(
    clusters.length,
    Math.ceil((scrollTop.value + containerHeight.value) / itemHeight.value) + BUFFER_SIZE
  );

  return { start, end };
});

const visibleClusters = computed(() => {
  const clusters = displayedClusters.value;
  const { start, end } = visibleRange.value;
  return clusters.slice(start, end);
});

const listPaddingTop = computed(() => {
  return visibleRange.value.start * itemHeight.value;
});

const listPaddingBottom = computed(() => {
  return (displayedClusters.value.length - visibleRange.value.end) * itemHeight.value;
});

const selectedRecommendationDate = computed({
  get: () => clusterStore.selectedRecommendationDate,
  set: (value: string) => {
    clusterStore.selectedRecommendationDate = value;
  },
});

const isRefreshingRecommendations = ref(false);

onMounted(async () => {
  if (authStore.isAuthenticated) {
    await nextTick();
    if (listRef.value) {
      containerHeight.value = listRef.value.clientHeight;
    }
  }
});

watch(
  () => displayedClusters.value.length,
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
  temporarilyKeptClusterIds.value.clear();
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
  clusterStore.reportClusterClick(cluster.id);

  if (!cluster.is_read) {
    if (articleStore.showOnlyUnread) {
      temporarilyKeptClusterIds.value = new Set(temporarilyKeptClusterIds.value).add(cluster.id);
    }

    clusterStore.markClusterRead(cluster.id, true).catch((e) => {
      if (articleStore.showOnlyUnread) {
        const next = new Set(temporarilyKeptClusterIds.value);
        next.delete(cluster.id);
        temporarilyKeptClusterIds.value = next;
      }
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

    if (
      !isDailyRecommendationMode.value &&
      scrollTop + clientHeight >= scrollHeight - SCROLL_THRESHOLD
    ) {
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
    title: isDailyRecommendationMode.value
      ? t('article.cluster.markAllRecommendationsReadTitle')
      : t('article.cluster.markAllReadTitle'),
    message: isDailyRecommendationMode.value
      ? t('article.cluster.markAllRecommendationsReadConfirm')
      : t('article.cluster.markAllReadConfirm'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    isDanger: false,
  });

  if (!confirmed) {
    return;
  }

  try {
    await clusterStore.markAllAsRead();
    window.showToast(
      isDailyRecommendationMode.value
        ? t('article.cluster.markedAllRecommendationsAsRead')
        : t('article.cluster.markedAllAsRead'),
      'success'
    );
  } catch (error) {
    console.error('Error marking all clusters as read:', error);
    window.showToast(t('common.errors.savingSettings'), 'error');
  }
}

function handleHoverMarkAsRead(clusterId: number): void {
  clusterStore.updateClusterState(clusterId, { is_read: true });
  articleStore.fetchUnreadCounts();
  articleStore.fetchFilterCounts();
}

async function handleRecommendationDateChange(): Promise<void> {
  if (!selectedRecommendationDate.value) {
    return;
  }

  try {
    await clusterStore.selectRecommendationDate(selectedRecommendationDate.value);
    clusterStore.currentClusterId = null;
  } catch (error) {
    console.error('Error changing recommendation date:', error);
    window.showToast(t('article.cluster.dailyRecommendationLoadFailed'), 'error');
  }
}

async function refreshRecommendations(): Promise<void> {
  isRefreshingRecommendations.value = true;
  try {
    await clusterStore.refreshDailyRecommendations();
    window.showToast(t('article.cluster.dailyRecommendationRefreshed'), 'success');
  } catch (error) {
    console.error('Error refreshing daily recommendations:', error);
    window.showToast(t('article.cluster.dailyRecommendationLoadFailed'), 'error');
  } finally {
    isRefreshingRecommendations.value = false;
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
    <div class="p-2 sm:p-4 border-b border-border bg-bg-primary space-y-3">
      <div class="flex items-center justify-between gap-2">
        <h3
          class="m-0 text-base sm:text-lg font-semibold truncate flex-1"
          :title="
            isDailyRecommendationMode
              ? t('article.cluster.dailyRecommendationTitle')
              : t('article.cluster.listTitle')
          "
        >
          {{
            isDailyRecommendationMode
              ? t('article.cluster.dailyRecommendationTitle')
              : t('article.cluster.listTitle')
          }}
        </h3>
        <div class="flex items-center gap-1 sm:gap-2">
          <button
            v-if="isDailyRecommendationMode"
            class="text-text-secondary hover:text-text-primary hover:bg-bg-tertiary p-1 sm:p-1.5 rounded transition-colors disabled:opacity-50"
            :disabled="isRefreshingRecommendations || clusterStore.isDailyRecommendationsLoading"
            :title="t('article.cluster.refreshRecommendations')"
            @click="refreshRecommendations"
          >
            <PhArrowClockwise :size="18" class="sm:w-5 sm:h-5" />
          </button>
          <button
            class="text-text-secondary hover:text-text-primary hover:bg-bg-tertiary p-1 sm:p-1.5 rounded transition-colors"
            :title="
              isDailyRecommendationMode
                ? t('article.cluster.markAllRecommendationsReadTitle')
                : t('article.cluster.markAllReadTitle')
            "
            @click="markAllAsRead"
          >
            <PhCheckCircle :size="18" class="sm:w-5 sm:h-5" />
          </button>
        </div>
      </div>

      <div v-if="isDailyRecommendationMode" class="flex flex-col sm:flex-row gap-2 sm:items-center">
        <select
          v-model="selectedRecommendationDate"
          class="min-w-0 flex-1 bg-bg-secondary border border-border rounded-lg px-3 py-2 text-sm text-text-primary"
          @change="handleRecommendationDateChange"
        >
          <option v-for="date in clusterStore.dailyRecommendationDates" :key="date" :value="date">
            {{ date }}
          </option>
        </select>
        <span class="text-xs text-text-secondary whitespace-nowrap">
          {{
            t('article.cluster.dailyRecommendationCount', {
              count: clusterStore.dailyRecommendations.length,
            })
          }}
        </span>
      </div>
    </div>

    <div
      v-if="
        clusterStore.isInitialLoading ||
        (isDailyRecommendationMode && clusterStore.isDailyRecommendationsLoading)
      "
      class="flex-1 flex flex-col items-center justify-center p-8 text-text-secondary"
    >
      <div
        class="w-8 h-8 border-4 border-accent border-t-transparent rounded-full animate-spin mb-4"
      />
      <div>{{ t('article.content.loadingContent') }}</div>
    </div>

    <div
      v-else-if="isDailyRecommendationMode && clusterStore.dailyRecommendationsError"
      class="flex-1 flex flex-col items-center justify-center p-8 text-text-secondary text-center"
    >
      <div class="w-16 h-16 bg-bg-tertiary rounded-full flex items-center justify-center mb-4">
        <span class="text-2xl">⚠️</span>
      </div>
      <h3 class="text-lg font-medium text-text-primary mb-2">
        {{ t('article.cluster.dailyRecommendationLoadFailedTitle') }}
      </h3>
      <p class="text-sm max-w-[280px] mb-4">
        {{ clusterStore.dailyRecommendationsError }}
      </p>
      <button class="btn-secondary" @click="refreshRecommendations">
        {{ t('article.cluster.refreshRecommendations') }}
      </button>
    </div>

    <div
      v-else-if="displayedClusters.length > 0"
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

        <div
          v-if="!isDailyRecommendationMode && clusterStore.isLoadingMore"
          class="py-4 flex justify-center items-center text-text-secondary w-full"
        >
          <div
            class="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin mr-2"
          />
          <span class="text-sm">{{ t('article.cluster.loadingMore') }}</span>
        </div>
      </div>
    </div>

    <div
      v-else
      class="flex-1 flex flex-col items-center justify-center p-8 text-text-secondary text-center"
    >
      <div class="w-16 h-16 bg-bg-tertiary rounded-full flex items-center justify-center mb-4">
        <span class="text-2xl">⚡️</span>
      </div>
      <h3 class="text-lg font-medium text-text-primary mb-2">
        {{
          isDailyRecommendationMode
            ? t('article.cluster.dailyRecommendationEmptyTitle')
            : t('article.cluster.emptyTitle')
        }}
      </h3>
      <p class="text-sm max-w-[250px]">
        {{
          isDailyRecommendationMode
            ? t('article.cluster.dailyRecommendationEmptyDescription')
            : t('article.cluster.emptyDescription')
        }}
      </p>
    </div>
  </section>
</template>

<style scoped>
@reference "../../style.css";
</style>
