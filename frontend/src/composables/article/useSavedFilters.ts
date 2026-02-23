import { ref, type Ref } from 'vue';
import type { SavedFilter } from '@/types/filter';
import type { FilterCondition } from '@/types/filter';
import { authFetchJson, authPost, authFetch } from '@/utils/authFetch';
import { useAuthStore } from '@/stores/auth';

export function useSavedFilters() {
  const authStore = useAuthStore();
  const savedFilters: Ref<SavedFilter[]> = ref([]);
  const isLoading = ref(false);
  const error: Ref<string | null> = ref(null);

  // Fetch all saved filters
  async function fetchSavedFilters(): Promise<void> {
    if (!authStore.isAuthenticated) {
      return;
    }
    isLoading.value = true;
    error.value = null;
    try {
      const data = await authFetchJson<any[]>('/api/saved-filters');
      savedFilters.value = Array.isArray(data) ? data : [];
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Unknown error';
      console.error('Error fetching saved filters:', e);
      savedFilters.value = [];
    } finally {
      isLoading.value = false;
    }
  }

  // Create new saved filter
  async function createSavedFilter(
    name: string,
    conditions: FilterCondition[]
  ): Promise<SavedFilter | null> {
    if (!authStore.isAuthenticated) {
      return null;
    }
    const conditionsJson = JSON.stringify(conditions);

    const requestBody = {
      name,
      conditions: conditionsJson,
    };

    const newFilter = await authPost('/api/saved-filters', requestBody);
    // Ensure savedFilters is an array before pushing
    if (Array.isArray(savedFilters.value)) {
      savedFilters.value.push(newFilter);
    } else {
      savedFilters.value = [newFilter];
    }
    return newFilter;
  }

  // Update existing saved filter
  async function updateSavedFilter(
    id: number,
    name: string,
    conditions: FilterCondition[]
  ): Promise<boolean> {
    if (!authStore.isAuthenticated) {
      return false;
    }
    try {
      await authFetchJson(`/api/saved-filters/filter?id=${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name,
          conditions: JSON.stringify(conditions),
        }),
      });

      await fetchSavedFilters(); // Refresh list
      return true;
    } catch (e) {
      console.error('Error updating saved filter:', e);
      return false;
    }
  }

  // Delete saved filter
  async function deleteSavedFilter(id: number): Promise<boolean> {
    if (!authStore.isAuthenticated) {
      return false;
    }
    try {
      await authFetchJson(`/api/saved-filters/filter?id=${id}`, {
        method: 'DELETE',
      });

      // Ensure savedFilters is an array before filtering
      if (Array.isArray(savedFilters.value)) {
        savedFilters.value = savedFilters.value.filter((f) => f.id !== id);
      } else {
        savedFilters.value = [];
      }
      return true;
    } catch (e) {
      console.error('Error deleting saved filter:', e);
      return false;
    }
  }

  // Reorder saved filters
  async function reorderSavedFilters(filters: SavedFilter[]): Promise<boolean> {
    if (!authStore.isAuthenticated) {
      return false;
    }
    try {
      await authPost('/api/saved-filters/reorder', filters);
      // Ensure filters is an array before assigning
      savedFilters.value = Array.isArray(filters) ? filters : [];
      return true;
    } catch (e) {
      console.error('Error reordering saved filters:', e);
      return false;
    }
  }

  // Parse conditions from JSON string
  function parseConditions(conditionsJson: string): FilterCondition[] {
    try {
      return JSON.parse(conditionsJson);
    } catch {
      return [];
    }
  }

  return {
    savedFilters,
    isLoading,
    error,
    fetchSavedFilters,
    createSavedFilter,
    updateSavedFilter,
    deleteSavedFilter,
    reorderSavedFilters,
    parseConditions,
  };
}
