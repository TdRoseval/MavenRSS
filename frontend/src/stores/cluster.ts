import { defineStore } from 'pinia';
import { ref } from 'vue';
import type { Cluster } from '@/types/models';
import { apiClient } from '@/shared/lib/apiClient';

export const useClusterStore = defineStore('cluster', () => {
  const clusters = ref<Cluster[]>([]);
  const currentClusterId = ref<number | null>(null);
  const isLoading = ref<boolean>(false);
  const page = ref<number>(1);
  const hasMore = ref<boolean>(true);
  const filterCounts = ref<Record<string, number>>({
    unread: 0,
    favorites: 0,
    read_later: 0,
  });

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

  async function fetchClusters(append: boolean = false): Promise<void> {
    if (isLoading.value) return;

    if (!append) {
      page.value = 1;
      clusters.value = [];
      hasMore.value = true;
    }

    isLoading.value = true;
    const limit = 50;

    try {
      const offset = (page.value - 1) * limit;
      const params: Record<string, any> = {
        page: page.value,
        limit: limit,
        offset,
      };

      const data: Cluster[] = (await apiClient.get<Cluster[]>('/clusters', params)) || [];

      if (data.length < limit) {
        hasMore.value = false;
      }

      if (append) {
        clusters.value = [...clusters.value, ...data];
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
      page.value++;
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

  return {
    clusters,
    currentClusterId,
    isLoading,
    page,
    hasMore,
    filterCounts,
    fetchClusters,
    loadMore,
    fetchClusterDetail,
    updateClusterState,
    markClusterRead,
    toggleClusterFavorite,
    toggleClusterReadLater,
    markAllAsRead,
  };
});
