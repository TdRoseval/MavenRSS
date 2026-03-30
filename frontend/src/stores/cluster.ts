import { defineStore } from 'pinia';
import { ref } from 'vue';
import type { Cluster } from '@/types/models';
import { apiClient } from '@/shared/lib/apiClient';

export const useClusterStore = defineStore('cluster', () => {
  // State
  const clusters = ref<Cluster[]>([]);
  const currentClusterId = ref<number | null>(null);
  const isLoading = ref<boolean>(false);
  const page = ref<number>(1);
  const hasMore = ref<boolean>(true);
  
  // Counts
  const filterCounts = ref<Record<string, number>>({
    unread: 0,
    favorites: 0,
    read_later: 0,
  });

  // Actions
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
      const params: Record<string, any> = {
        page: page.value,
        limit: limit,
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
  };
});
