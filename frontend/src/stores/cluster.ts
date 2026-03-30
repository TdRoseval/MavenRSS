import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import type { Cluster } from '@/types/models';
import { apiClient } from '@/shared/lib/apiClient';

export const useClusterStore = defineStore('cluster', () => {
  const clusters = ref<Cluster[]>([]);
  const currentClusterId = ref<number | null>(null);
  const isLoading = ref<boolean>(false);
  const hasMore = ref<boolean>(true);
  const filterCounts = ref<Record<string, number>>({
    unread: 0,
    favorites: 0,
    read_later: 0,
  });

  /** Set of cluster IDs already displayed — used for stateless exclusion pagination */
  const displayedIds = computed<number[]>(() => clusters.value.map((c) => c.id));
  const isInitialLoading = computed<boolean>(() => isLoading.value && clusters.value.length === 0);
  const isLoadingMore = computed<boolean>(() => isLoading.value && clusters.value.length > 0);

  function updateClusterState(id: number, patch: Partial<Cluster>): void {
    const index = clusters.value.findIndex((cluster) => cluster.id === id);
    if (index === -1) {
      return;
    }

    clusters.value[index] = {
      ...clusters.value[index],
      ...patch,
    };
  }

  /**
   * Fetches clusters using the AI-enhanced feed endpoint with exclude_ids pagination.
   * @param append If true, appends to existing list; if false, resets the list.
   */
  async function fetchClusters(append: boolean = false): Promise<void> {
    if (isLoading.value) return;

    if (!append) {
      clusters.value = [];
      hasMore.value = true;
    }

    isLoading.value = true;

    try {
      const excludeIds = append ? displayedIds.value : [];
      const data: Cluster[] =
        (await apiClient.post<Cluster[]>('/clusters/feed', {
          exclude_ids: excludeIds,
        })) || [];

      if (data.length < 30) {
        hasMore.value = false;
      }

      if (append) {
        const existingIds = new Set(clusters.value.map((cluster) => cluster.id));
        const nextClusters = data.filter((cluster) => !existingIds.has(cluster.id));
        clusters.value = [...clusters.value, ...nextClusters];
      } else {
        clusters.value = data;
      }
    } catch (e) {
      console.error('Error fetching clusters:', e);
    } finally {
      isLoading.value = false;
    }
  }

  async function loadMore(): Promise<void> {
    if (hasMore.value && !isLoading.value) {
      await fetchClusters(true);
    }
  }

  async function fetchClusterDetail(id: number): Promise<Cluster | null> {
    try {
      return await apiClient.get<Cluster>('/clusters/detail', { id });
    } catch (e) {
      console.error('Error fetching cluster detail:', e);
      return null;
    }
  }

  async function markClusterRead(id: number, read: boolean): Promise<void> {
    updateClusterState(id, { is_read: read });

    try {
      await apiClient.put('/clusters/read', { id, read });
    } catch (e) {
      updateClusterState(id, { is_read: !read });
      throw e;
    }
  }

  async function toggleClusterFavorite(cluster: Cluster): Promise<boolean> {
    const nextValue = !cluster.is_favorite;
    updateClusterState(cluster.id, { is_favorite: nextValue });

    try {
      await apiClient.put('/clusters/favorite', { id: cluster.id });
      return nextValue;
    } catch (e) {
      updateClusterState(cluster.id, { is_favorite: cluster.is_favorite });
      throw e;
    }
  }

  async function toggleClusterReadLater(
    cluster: Cluster
  ): Promise<{ isReadLater: boolean; isRead: boolean }> {
    const nextReadLater = !cluster.is_read_later;
    const nextRead = nextReadLater ? false : cluster.is_read;
    updateClusterState(cluster.id, {
      is_read_later: nextReadLater,
      is_read: nextRead,
    });

    try {
      await apiClient.put('/clusters/read-later', { id: cluster.id });
      return {
        isReadLater: nextReadLater,
        isRead: nextRead,
      };
    } catch (e) {
      updateClusterState(cluster.id, {
        is_read_later: cluster.is_read_later,
        is_read: cluster.is_read,
      });
      throw e;
    }
  }

  async function markAllAsRead(): Promise<void> {
    await apiClient.post('/clusters/mark-all-read');
    clusters.value = clusters.value.map((cluster) => ({
      ...cluster,
      is_read: true,
    }));
  }

  /** Fire-and-forget click feedback for Level 1 interest vector update */
  function reportClusterClick(clusterId: number): void {
    apiClient
      .post('/clusters/click', { cluster_id: clusterId })
      .catch((e: unknown) => {
        console.error('Failed to report cluster click:', e);
      });
  }

  return {
    clusters,
    currentClusterId,
    isLoading,
    isInitialLoading,
    isLoadingMore,
    hasMore,
    filterCounts,
    displayedIds,
    fetchClusters,
    loadMore,
    fetchClusterDetail,
    updateClusterState,
    markClusterRead,
    toggleClusterFavorite,
    toggleClusterReadLater,
    markAllAsRead,
    reportClusterClick,
  };
});
