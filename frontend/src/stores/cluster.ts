import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import { apiClient } from '@/shared/lib/apiClient';
import { authGet, authPost } from '@/shared/lib/authFetch';
import type {
  AIProcessingStatus,
  Cluster,
  ClusterRenormalizeResponse,
  DailyRecommendationRefreshResponse,
  DailyRecommendationItem,
  DailyRecommendationResponse,
  DailyRecommendationTaskStatus,
} from '@/types/models';
import type { FilterCondition } from '@/types/filter';
import { useArticleStore } from '@/features/article/store';

interface ClusterListResponse {
  clusters: Cluster[];
  total?: number;
  has_more?: boolean;
  cache_hit?: boolean;
}

export const useClusterStore = defineStore('cluster', () => {
  const AI_PROCESSING_POLL_INTERVAL_MS = 10000;
  const AI_PROCESSING_IDLE_POLL_INTERVAL_MS = 30000;
  const AI_PROCESSING_MAX_IDLE_POLL_INTERVAL_MS = 60000;
  const DEFAULT_PAGE_SIZE = 20;
  const REALTIME_BATCH_SIZE = 30;
  const clusters = ref<Cluster[]>([]);
  const dailyRecommendations = ref<DailyRecommendationItem[]>([]);
  const dailyRecommendationDates = ref<string[]>([]);
  const selectedRecommendationDate = ref('');
  const currentClusterId = ref<number | null>(null);
  const isLoading = ref(false);
  const isInitialLoading = ref(false);
  const isLoadingMore = ref(false);
  const hasMore = ref(true);
  const currentPage = ref(1);
  const activeFilters = ref<FilterCondition[]>([]);
  const filteredClusterIds = ref<number[] | null>(null);
  const isDailyRecommendationsLoading = ref(false);
  const dailyRecommendationsError = ref('');
  const dailyRecommendationTaskStatus = ref<DailyRecommendationTaskStatus | null>(null);
  const isDailyRecommendationTaskStatusLoading = ref(false);
  const aiProcessingStatus = ref<AIProcessingStatus | null>(null);
  const isAIProcessingStatusLoading = ref(false);
  const hasLoadedAIProcessingStatus = ref(false);

  let aiProcessingPollingTimer: ReturnType<typeof setTimeout> | null = null;
  let aiProcessingPollingConsumers = 0;
  let aiProcessingIdlePollCount = 0;
  let queuedFetchPage: number | null = null;
  let activeFetchContextKey = '';
  let fetchSequence = 0;

  const currentCluster = computed(
    () =>
      clusters.value.find((cluster) => cluster.id === currentClusterId.value) ||
      dailyRecommendations.value.find((item) => item.cluster.id === currentClusterId.value)
        ?.cluster ||
      null
  );

  const isAIProcessingLocked = computed(() => aiProcessingStatus.value?.is_config_frozen === true);
  const shouldBlockDailyRecommendationView = computed(
    () => dailyRecommendationTaskStatus.value?.has_task === true
  );
  const hasRealtimeInterestStream = computed(() => {
    const articleStore = useArticleStore();
    return (
      articleStore.shouldUseClusterList() && aiProcessingStatus.value?.has_interest_vector === true
    );
  });
  const shouldBlockClusterView = computed(
    () =>
      aiProcessingStatus.value?.is_enabled === true &&
      (isAIProcessingLocked.value ||
        (!hasLoadedAIProcessingStatus.value && isAIProcessingStatusLoading.value))
  );
  const aiProcessingProgressPercent = computed(() => {
    const rawValue = aiProcessingStatus.value?.progress_percent ?? 0;
    return Math.max(0, Math.min(100, Math.round(rawValue)));
  });
  const dailyRecommendationTaskProgressPercent = computed(() => {
    const rawValue = dailyRecommendationTaskStatus.value?.progress_percent ?? 0;
    return Math.max(0, Math.min(100, Math.round(rawValue)));
  });

  function normalizeClusterListResponse(
    response: Cluster[] | ClusterListResponse | null | undefined
  ): Cluster[] {
    if (!response) {
      return [];
    }

    if (Array.isArray(response)) {
      return response;
    }

    return response.clusters || [];
  }

  function buildFetchContextKey(): string {
    const articleStore = useArticleStore();

    return JSON.stringify({
      filter: articleStore.currentFilter || 'all',
      feedId: articleStore.currentFeedId ?? null,
      category: articleStore.currentCategory ?? null,
      activeFilters: activeFilters.value.map((filter) => ({
        field: filter.field,
        type: filter.type,
        operator: filter.operator,
        value: filter.value,
        logic: filter.logic || 'and',
      })),
      filteredClusterIds: filteredClusterIds.value,
      hasRealtimeInterestStream: hasRealtimeInterestStream.value,
    });
  }

  async function fetchClusters(page = 1) {
    const articleStore = useArticleStore();
    const isFirstPage = page === 1;
    const requestContextKey = buildFetchContextKey();

    if (isLoading.value) {
      queuedFetchPage = isFirstPage ? 1 : Math.max(queuedFetchPage ?? 0, page);

      if (isFirstPage) {
        isInitialLoading.value = true;
        clusters.value = [];
        currentClusterId.value = null;
        currentPage.value = 1;
        hasMore.value = true;
      }
      return;
    }

    activeFetchContextKey = requestContextKey;
    const requestSequence = ++fetchSequence;

    if (isFirstPage) {
      isInitialLoading.value = true;
      clusters.value = [];
      currentPage.value = 1;
      hasMore.value = true;
    } else {
      isLoadingMore.value = true;
    }

    isLoading.value = true;

    try {
      if (hasRealtimeInterestStream.value) {
        const payload: Record<string, any> = {
          exclude_ids: isFirstPage ? [] : clusters.value.map((cluster) => cluster.id),
          limit: REALTIME_BATCH_SIZE,
        };

        if (articleStore.currentFilter && articleStore.currentFilter !== 'all') {
          payload.filter = articleStore.currentFilter;
        }
        if (articleStore.currentFeedId) {
          payload.feed_id = articleStore.currentFeedId;
        }
        if (articleStore.currentCategory !== null) {
          payload.category = articleStore.currentCategory;
        }

        const response = await apiClient.post<Cluster[] | ClusterListResponse>('/clusters/feed', payload);
        const clusterData = normalizeClusterListResponse(response);
        const realtimeHasMore = Array.isArray(response)
          ? clusterData.length === REALTIME_BATCH_SIZE
          : response?.has_more === true;

        if (
          requestSequence !== fetchSequence ||
          activeFetchContextKey !== requestContextKey ||
          buildFetchContextKey() !== requestContextKey
        ) {
          queuedFetchPage = 1;
          return;
        }

        if (isFirstPage) {
          clusters.value = clusterData;
        } else {
          const existingIds = new Set(clusters.value.map((cluster) => cluster.id));
          const newClusters = clusterData.filter((cluster) => !existingIds.has(cluster.id));
          clusters.value = [...clusters.value, ...newClusters];
        }

        if (filteredClusterIds.value) {
          clusters.value = clusters.value.filter((cluster) =>
            filteredClusterIds.value?.includes(cluster.id)
          );
        }

        hasMore.value = realtimeHasMore;
        currentPage.value = page;

        return;
      }

      const params: Record<string, any> = {
        page,
        limit: DEFAULT_PAGE_SIZE,
      };

      if (articleStore.currentFilter && articleStore.currentFilter !== 'all') {
        params.filter = articleStore.currentFilter;
      }
      if (articleStore.currentFeedId) {
        params.feed_id = articleStore.currentFeedId;
      }
      if (articleStore.currentCategory !== null) {
        params.category = articleStore.currentCategory;
      }

      if (activeFilters.value.length > 0) {
        const filterParams = new URLSearchParams();
        activeFilters.value.forEach((filter, index) => {
          filterParams.append(`f${index}_type`, filter.type);
          filterParams.append(`f${index}_op`, filter.operator);
          filterParams.append(`f${index}_value`, filter.value);
          filterParams.append(`f${index}_logic`, filter.logic || 'and');
        });
        params.filters = filterParams.toString();
      }

      const response = await apiClient.get<Cluster[] | ClusterListResponse>('/clusters', params);
      const clusterData = normalizeClusterListResponse(response);

      if (
        requestSequence !== fetchSequence ||
        activeFetchContextKey !== requestContextKey ||
        buildFetchContextKey() !== requestContextKey
      ) {
        queuedFetchPage = 1;
        return;
      }

      if (isFirstPage) {
        clusters.value = clusterData;
      } else {
        const existingIds = new Set(clusters.value.map((cluster) => cluster.id));
        const newClusters = clusterData.filter((cluster) => !existingIds.has(cluster.id));
        clusters.value = [...clusters.value, ...newClusters];
      }

      if (filteredClusterIds.value) {
        clusters.value = clusters.value.filter((cluster) =>
          filteredClusterIds.value?.includes(cluster.id)
        );
      }

      hasMore.value = clusterData.length === DEFAULT_PAGE_SIZE;
      currentPage.value = page;

    } catch (error) {
      console.error('Failed to fetch clusters:', error);
      throw error;
    } finally {
      isLoading.value = false;
      isInitialLoading.value = false;
      isLoadingMore.value = false;

      const nextQueuedPage = queuedFetchPage;
      queuedFetchPage = null;
      if (nextQueuedPage !== null) {
        void fetchClusters(nextQueuedPage);
      }
    }
  }

  async function loadMore() {
    if (!hasMore.value || isLoading.value || isLoadingMore.value) {
      return;
    }

    await fetchClusters(currentPage.value + 1);
  }

  async function fetchClusterDetail(clusterId: number) {
    return apiClient.get('/clusters/detail', { id: clusterId });
  }

  async function fetchDailyRecommendationDates(): Promise<string[]> {
    try {
      const response = await apiClient.get<string[]>('/clusters/daily-recommendations/dates');
      dailyRecommendationDates.value = response || [];

      if (!selectedRecommendationDate.value && dailyRecommendationDates.value.length > 0) {
        selectedRecommendationDate.value = dailyRecommendationDates.value[0];
      }

      return dailyRecommendationDates.value;
    } catch (error) {
      console.error('Failed to fetch daily recommendation dates:', error);
      dailyRecommendationDates.value = [];
      throw error;
    }
  }

  async function fetchDailyRecommendations(date?: string): Promise<DailyRecommendationItem[]> {
    const targetDate = date || selectedRecommendationDate.value;
    if (!targetDate) {
      dailyRecommendations.value = [];
      dailyRecommendationsError.value = '';
      return [];
    }

    isDailyRecommendationsLoading.value = true;
    dailyRecommendationsError.value = '';

    try {
      const response = await apiClient.get<DailyRecommendationResponse>(
        '/clusters/daily-recommendations',
        {
          date: targetDate,
        }
      );
      dailyRecommendations.value = response.recommendations || [];
      selectedRecommendationDate.value = response.selected_date || targetDate;

      if (dailyRecommendations.value.length > 0) {
        currentClusterId.value = dailyRecommendations.value[0].cluster.id;
      }

      return dailyRecommendations.value;
    } catch (error: any) {
      console.error('Failed to fetch daily recommendations:', error);
      dailyRecommendations.value = [];
      dailyRecommendationsError.value = error?.message || 'Failed to fetch daily recommendations';
      throw error;
    } finally {
      isDailyRecommendationsLoading.value = false;
    }
  }

  async function selectRecommendationDate(date: string): Promise<DailyRecommendationItem[]> {
    selectedRecommendationDate.value = date;
    return fetchDailyRecommendations(date);
  }

  async function refreshDailyRecommendations(): Promise<void> {
    dailyRecommendationsError.value = '';
    const response = await apiClient.post<DailyRecommendationRefreshResponse>(
      '/clusters/daily-recommendations/refresh',
      {
      date: selectedRecommendationDate.value || undefined,
      wait_for_idle: true,
      }
    );

    if (response.date) {
      selectedRecommendationDate.value = response.date;
    }
    dailyRecommendationTaskStatus.value = response.status;
  }

  async function markClusterRead(clusterId: number, isRead: boolean) {
    const articleStore = useArticleStore();
    await apiClient.put('/clusters/read', { id: clusterId, read: isRead });
    updateClusterState(clusterId, { is_read: isRead });
    await articleStore.fetchUnreadCounts();
    await articleStore.fetchFilterCounts();
  }

  async function toggleClusterFavorite(cluster: Cluster) {
    const articleStore = useArticleStore();
    await apiClient.put('/clusters/favorite', { id: cluster.id });
    updateClusterState(cluster.id, { is_favorite: !cluster.is_favorite });
    await articleStore.fetchFilterCounts();
  }

  async function toggleClusterReadLater(cluster: Cluster) {
    const articleStore = useArticleStore();
    const nextReadLater = !cluster.is_read_later;
    await apiClient.put('/clusters/read-later', { id: cluster.id });
    updateClusterState(cluster.id, {
      is_read_later: nextReadLater,
      is_read: nextReadLater ? cluster.is_read : false,
    });
    await articleStore.fetchUnreadCounts();
    await articleStore.fetchFilterCounts();
  }

  async function reportClusterClick(clusterId: number) {
    try {
      await apiClient.post('/clusters/click', { cluster_id: clusterId });
    } catch (error) {
      console.error('Failed to report cluster click:', error);
    }
  }

  function updateClusterState(clusterId: number, updates: Partial<Cluster>) {
    const cluster = clusters.value.find((item) => item.id === clusterId);
    if (cluster) {
      Object.assign(cluster, updates);
    }

    const recommendationCluster = dailyRecommendations.value.find(
      (item) => item.cluster.id === clusterId
    )?.cluster;
    if (recommendationCluster) {
      Object.assign(recommendationCluster, updates);
    }
  }

  async function markAllAsRead() {
    const articleStore = useArticleStore();

    if (dailyRecommendations.value.length > 0) {
      const unreadRecommendationClusters = dailyRecommendations.value
        .map((item) => item.cluster)
        .filter((cluster) => !cluster.is_read);

      await Promise.all(
        unreadRecommendationClusters.map((cluster) => markClusterRead(cluster.id, true))
      );
      return;
    }

    const params: Record<string, any> = {};
    if (articleStore.currentFilter && articleStore.currentFilter !== 'all') {
      params.filter = articleStore.currentFilter;
    }
    if (articleStore.currentFeedId) {
      params.feed_id = articleStore.currentFeedId;
    }
    if (articleStore.currentCategory !== null) {
      params.category = articleStore.currentCategory;
    }

    await apiClient.post('/clusters/mark-all-read', {}, params);

    clusters.value.forEach((cluster) => {
      cluster.is_read = true;
    });
    dailyRecommendations.value.forEach((item) => {
      item.cluster.is_read = true;
    });
    await articleStore.fetchUnreadCounts();
    await articleStore.fetchFilterCounts();
  }

  async function refreshCurrentCluster() {
    if (!currentClusterId.value) return;

    try {
      const updatedCluster = await apiClient.get<Cluster>('/clusters/detail', {
        id: currentClusterId.value,
      });
      const index = clusters.value.findIndex((cluster) => cluster.id === currentClusterId.value);
      if (index !== -1) {
        clusters.value[index] = updatedCluster;
      }

      const recommendationIndex = dailyRecommendations.value.findIndex(
        (item) => item.cluster.id === currentClusterId.value
      );
      if (recommendationIndex !== -1) {
        dailyRecommendations.value[recommendationIndex].cluster = updatedCluster;
      }
    } catch (error) {
      console.error('Failed to refresh current cluster:', error);
      throw error;
    }
  }

  function setActiveFilters(filters: FilterCondition[]) {
    activeFilters.value = filters;
  }

  async function fetchAIProcessingStatus(): Promise<AIProcessingStatus | null> {
    isAIProcessingStatusLoading.value = true;

    try {
      const response = await authGet<AIProcessingStatus>('/api/clusters/ai-processing-status');
      aiProcessingStatus.value = response;
      hasLoadedAIProcessingStatus.value = true;
      return response;
    } catch (error) {
      console.error('Failed to fetch AI processing status:', error);
      if (!hasLoadedAIProcessingStatus.value) {
        aiProcessingStatus.value = null;
      }
      throw error;
    } finally {
      isAIProcessingStatusLoading.value = false;
    }
  }

  async function fetchDailyRecommendationTaskStatus(): Promise<DailyRecommendationTaskStatus | null> {
    isDailyRecommendationTaskStatusLoading.value = true;

    try {
      const response = await apiClient.get<DailyRecommendationTaskStatus>(
        '/clusters/daily-recommendations/task-status'
      );
      dailyRecommendationTaskStatus.value = response;
      if (response?.recommendation_date) {
        selectedRecommendationDate.value = response.recommendation_date;
      }
      return response;
    } catch (error) {
      console.error('Failed to fetch daily recommendation task status:', error);
      throw error;
    } finally {
      isDailyRecommendationTaskStatusLoading.value = false;
    }
  }

  async function fetchProcessingStatuses(): Promise<void> {
    const results = await Promise.allSettled([
      fetchAIProcessingStatus(),
      fetchDailyRecommendationTaskStatus(),
    ]);
    const firstRejected = results.find(
      (result): result is PromiseRejectedResult => result.status === 'rejected'
    );
    if (firstRejected && results.every((result) => result.status === 'rejected')) {
      throw firstRejected.reason;
    }
  }

  async function forceStartClusterRenormalization(): Promise<ClusterRenormalizeResponse> {
    const response = await authPost<ClusterRenormalizeResponse>('/api/clusters/recluster-normalize', {
      force: true,
    });
    await fetchProcessingStatuses();
    return response;
  }

  function hasActiveProcessingWork(): boolean {
    return (
      aiProcessingStatus.value?.is_config_frozen === true ||
      dailyRecommendationTaskStatus.value?.has_task === true
    );
  }

  function getNextAIProcessingPollDelay(): number {
    if (hasActiveProcessingWork()) {
      aiProcessingIdlePollCount = 0;
      return AI_PROCESSING_POLL_INTERVAL_MS;
    }

    aiProcessingIdlePollCount += 1;
    if (aiProcessingIdlePollCount === 1) {
      return AI_PROCESSING_IDLE_POLL_INTERVAL_MS;
    }

    return AI_PROCESSING_MAX_IDLE_POLL_INTERVAL_MS;
  }

  function scheduleNextAIProcessingPoll(delay?: number) {
    if (aiProcessingPollingConsumers === 0) {
      return;
    }

    if (aiProcessingPollingTimer) {
      clearTimeout(aiProcessingPollingTimer);
    }

    aiProcessingPollingTimer = setTimeout(() => {
      fetchProcessingStatuses()
        .catch((error) => {
          console.error('Failed to poll processing status:', error);
        })
        .finally(() => {
          scheduleNextAIProcessingPoll(getNextAIProcessingPollDelay());
        });
    }, delay ?? getNextAIProcessingPollDelay());
  }

  async function startAIProcessingPolling() {
    aiProcessingPollingConsumers += 1;

    if (aiProcessingPollingConsumers === 1) {
      await fetchProcessingStatuses();
      aiProcessingIdlePollCount = 0;
      scheduleNextAIProcessingPoll(AI_PROCESSING_POLL_INTERVAL_MS);
      return;
    }

    if (!hasLoadedAIProcessingStatus.value && !isAIProcessingStatusLoading.value) {
      await fetchProcessingStatuses();
    }
  }

  function stopAIProcessingPolling() {
    aiProcessingPollingConsumers = Math.max(0, aiProcessingPollingConsumers - 1);

    if (aiProcessingPollingConsumers === 0 && aiProcessingPollingTimer) {
      clearTimeout(aiProcessingPollingTimer);
      aiProcessingPollingTimer = null;
      aiProcessingIdlePollCount = 0;
    }
  }

  function setFilteredClusterIds(ids: number[] | null) {
    filteredClusterIds.value = ids;

    if (!ids) return;

    clusters.value = clusters.value.filter((cluster) => ids.includes(cluster.id));
    dailyRecommendations.value = dailyRecommendations.value.filter((item) =>
      ids.includes(item.cluster.id)
    );

    if (currentClusterId.value && !ids.includes(currentClusterId.value)) {
      const firstRecommendation = dailyRecommendations.value[0]?.cluster.id;
      const firstCluster = clusters.value[0]?.id;
      currentClusterId.value = firstRecommendation ?? firstCluster ?? null;
    }
  }

  function clearData() {
    clusters.value = [];
    dailyRecommendations.value = [];
    dailyRecommendationDates.value = [];
    selectedRecommendationDate.value = '';
    currentClusterId.value = null;
    currentPage.value = 1;
    hasMore.value = true;
    dailyRecommendationsError.value = '';
    dailyRecommendationTaskStatus.value = null;
    queuedFetchPage = null;
    activeFetchContextKey = '';
  }

  return {
    clusters,
    dailyRecommendations,
    dailyRecommendationDates,
    selectedRecommendationDate,
    currentClusterId,
    currentCluster,
    isLoading,
    isInitialLoading,
    isLoadingMore,
    hasMore,
    currentPage,
    activeFilters,
    filteredClusterIds,
    isDailyRecommendationsLoading,
    dailyRecommendationsError,
    dailyRecommendationTaskStatus,
    isDailyRecommendationTaskStatusLoading,
    aiProcessingStatus,
    isAIProcessingStatusLoading,
    hasLoadedAIProcessingStatus,
    isAIProcessingLocked,
    shouldBlockDailyRecommendationView,
    shouldBlockClusterView,
    hasRealtimeInterestStream,
    aiProcessingProgressPercent,
    dailyRecommendationTaskProgressPercent,
    fetchClusters,
    loadMore,
    fetchClusterDetail,
    fetchDailyRecommendationDates,
    fetchDailyRecommendations,
    selectRecommendationDate,
    refreshDailyRecommendations,
    markClusterRead,
    toggleClusterFavorite,
    toggleClusterReadLater,
    reportClusterClick,
    updateClusterState,
    markAllAsRead,
    refreshCurrentCluster,
    setActiveFilters,
    fetchAIProcessingStatus,
    fetchDailyRecommendationTaskStatus,
    forceStartClusterRenormalization,
    startAIProcessingPolling,
    stopAIProcessingPolling,
    setFilteredClusterIds,
    clearData,
  };
});
