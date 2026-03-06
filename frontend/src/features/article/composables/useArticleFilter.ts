import { ref, computed, type Ref } from 'vue';
import type { Article } from '@/types/models';
import type { FilterCondition } from '@/types/filter';
import { authFetchJson } from '@/shared/lib/authFetch';
import { useArticleStore } from '@/features/article/store';

export function useArticleFilter() {
  const articleStore = useArticleStore();
  // Use computed to get references to the store's filter state
  const activeFilters = computed({
    get: () => articleStore.activeFilters,
    set: (value) => articleStore.setActiveFilters(value),
  });
  const filteredArticlesFromServer = computed({
    get: () => articleStore.filteredArticlesFromServer,
    set: (value) => articleStore.setFilteredArticlesFromServer(value),
  });
  const isFilterLoading = computed({
    get: () => articleStore.isFilterLoading,
    set: (value) => articleStore.setIsFilterLoading(value),
  });
  const filterPage = ref(1);
  const filterHasMore = ref(true);
  const filterTotal = ref(0);

  // Reset filter state
  function resetFilterState(): void {
    articleStore.setFilteredArticlesFromServer([]);
    filterPage.value = 1;
    filterHasMore.value = true;
    filterTotal.value = 0;
  }

  // Fetch filtered articles from server with pagination
  async function fetchFilteredArticles(filters: FilterCondition[], append = false): Promise<void> {
    if (filters.length === 0) {
      resetFilterState();
      return;
    }

    articleStore.setIsFilterLoading(true);
    try {
      const page = append ? filterPage.value : 1;

      const data = await authFetchJson('/api/articles/filter', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          conditions: filters,
          page: page,
          limit: 50,
        }),
      });

      const articles = data.articles || [];

      if (append) {
        articleStore.setFilteredArticlesFromServer([...articleStore.filteredArticlesFromServer, ...articles]);
      } else {
        articleStore.setFilteredArticlesFromServer(articles);
        filterPage.value = 1;
      }

      // Ensure filtered articles are also in the store for article detail view
      articles.forEach((article) => {
        const existingIndex = articleStore.articles.findIndex((a) => a.id === article.id);
        if (existingIndex === -1) {
          // Article not in store, add it
          articleStore.articles.push(article);
        } else {
          // Article already in store, update it
          articleStore.articles[existingIndex] = article;
        }
      });

      filterHasMore.value = data.has_more;
      filterTotal.value = data.total;
    } catch (e) {
      console.error('Error fetching filtered articles:', e);
      if (!append) {
        articleStore.setFilteredArticlesFromServer([]);
      }
    } finally {
      articleStore.setIsFilterLoading(false);
    }
  }

  // Load more filtered articles
  async function loadMoreFilteredArticles(): Promise<void> {
    if (isFilterLoading.value || !filterHasMore.value) return;

    filterPage.value++;
    await fetchFilteredArticles(activeFilters.value, true);
  }

  // Clear all filters
  function clearAllFilters(): void {
    articleStore.setActiveFilters([]);
    articleStore.setFilteredArticlesFromServer([]);
    filterPage.value = 1;
    filterHasMore.value = true;
    filterTotal.value = 0;
  }

  return {
    activeFilters,
    filteredArticlesFromServer,
    isFilterLoading,
    filterPage,
    filterHasMore,
    filterTotal,
    resetFilterState,
    fetchFilteredArticles,
    loadMoreFilteredArticles,
    clearAllFilters,
  };
}
