import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import { apiClient } from '@/shared/lib/apiClient';
import type { Cluster, DailyRecommendationItem, DailyRecommendationResponse } from '@/types/models';
import type { FilterCondition } from '@/types/filter';
import { useArticleStore } from '@/features/article/store';

interface ClusterListResponse {
  clusters: Cluster[];
  total?: number;
}

export const useClusterStore = defineStore('cluster', () => {
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
  const pageSize = 20;
  const activeFilters = ref<FilterCondition[]>([]);
  const filteredClusterIds = ref<number[] | null>(null);
  const isDailyRecommendationsLoading = ref(false);
  const dailyRecommendationsError = ref('');

  const currentCluster = computed(
    () =>
      clusters.value.find((cluster) => cluster.id === currentClusterId.value) ||
      dailyRecommendations.value.find((item) => item.cluster.id === currentClusterId.value)
        ?.cluster ||
      null
  );

  function normalizeClusterListResponse(response: Cluster[] | ClusterListResponse): Cluster[] {
    if (Array.isArray(response)) {
      return response;
    }

    return response.clusters || [];
  }

  async function fetchClusters(page = 1) {
    if (isLoading.value) return;

    const articleStore = useArticleStore();
    const isFirstPage = page === 1;

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
      const params: Record<string, any> = {
        page,
        limit: pageSize,
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

      hasMore.value = clusterData.length === pageSize;
      currentPage.value = page;

      if (isFirstPage) {
        currentClusterId.value = clusters.value[0]?.id ?? null;
      }
    } catch (error) {
      console.error('Failed to fetch clusters:', error);
      throw error;
    } finally {
      isLoading.value = false;
      isInitialLoading.value = false;
      isLoadingMore.value = false;
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
      const response = await apiClient.get<{ dates: string[] }>('/recommendations/dates');
      dailyRecommendationDates.value = response.dates || [];

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
      const response = await apiClient.get<DailyRecommendationResponse>('/recommendations/daily', {
        date: targetDate,
      });
      dailyRecommendations.value = response.recommendations || [];
      selectedRecommendationDate.value = response.date || targetDate;

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
    const dates = await fetchDailyRecommendationDates();
    if (dates.length === 0) {
      dailyRecommendations.value = [];
      currentClusterId.value = null;
      return;
    }

    const targetDate = selectedRecommendationDate.value || dates[0];
    await fetchDailyRecommendations(targetDate);
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
      await apiClient.post(`/recommendations/clusters/${clusterId}/click`, {});
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
    setFilteredClusterIds,
    clearData,
  };
});
