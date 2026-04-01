import { defineStore } from 'pinia';
import { ref } from 'vue';
import type { Article, UnreadCounts, Feed } from '@/types/models';
import type { FilterCondition } from '@/types/filter';
import { apiClient } from '@/shared/lib/apiClient';
import { useFeedStore } from '@/features/feed/store';

export type Filter =
  | 'all'
  | 'unread'
  | 'favorites'
  | 'readLater'
  | 'imageGallery'
  | 'clusters'
  | 'dailyRecommendations'
  | '';

export interface TempSelection {
  feedId: number | null;
  category: string | null;
}

export const useArticleStore = defineStore('article', () => {
  const articles = ref<Article[]>([]);
  const unreadCounts = ref<UnreadCounts>({
    total: 0,
    feedCounts: {},
  });
  const currentFilter = ref<Filter>('all');
  const currentFeedId = ref<number | null>(null);
  const currentCategory = ref<string | null>(null);
  const currentArticleId = ref<number | null>(null);
  const tempSelection = ref<TempSelection>({ feedId: null, category: null });
  const isLoading = ref<boolean>(false);
  const page = ref<number>(1);
  const hasMore = ref<boolean>(true);
  const searchQuery = ref<string>('');
  const showOnlyUnread = ref<boolean>(localStorage.getItem('showOnlyUnread') === 'true');
  const activeFilters = ref<FilterCondition[]>([]);
  const filteredArticlesFromServer = ref<Article[]>([]);
  const isFilterLoading = ref(false);
  const aiEnhancedMode = ref(false);

  const articleViewModePreferences = ref<Map<number, 'original' | 'rendered'>>(new Map());

  const aiSearchResults = ref<Article[]>([]);

  const filterCounts = ref<Record<string, Record<number | string, number>>>({
    unread: {},
    favorites: {},
    favorites_unread: {},
    read_later: {},
    read_later_unread: {},
    images: {},
    images_unread: {},
  });

  function shouldUseClusterList(filter: Filter = currentFilter.value): boolean {
    return aiEnhancedMode.value && filter !== 'imageGallery' && filter !== 'dailyRecommendations';
  }

  function resetArticleCollection(): void {
    articles.value = [];
    hasMore.value = false;
    isLoading.value = false;
    page.value = 1;
  }

  function setAIEnhancedMode(enabled: boolean): void {
    aiEnhancedMode.value = enabled;
    if (!enabled && currentFilter.value === 'dailyRecommendations') {
      currentFilter.value = 'all';
      currentFeedId.value = null;
      currentCategory.value = null;
      tempSelection.value = { feedId: null, category: null };
      void fetchFilterCounts();
      fetchArticles();
      return;
    }

    if (shouldUseClusterList()) {
      resetArticleCollection();
      return;
    }

    if (articles.value.length === 0 && currentFilter.value !== 'dailyRecommendations') {
      fetchArticles();
    }
  }

  async function setFilter(filter: Filter): Promise<void> {
    currentFilter.value = filter;
    currentFeedId.value = null;
    currentCategory.value = null;
    tempSelection.value = { feedId: null, category: null };
    await fetchFilterCounts();

    if (
      filter === 'dailyRecommendations' ||
      shouldUseClusterList(filter) ||
      filter === 'clusters'
    ) {
      resetArticleCollection();
      return;
    }

    fetchArticles();
  }

  function setFeed(feedId: number): void {
    const feedStore = useFeedStore();
    const feed = feedStore.feeds.find((f: Feed) => f.id === feedId);
    if (feed?.is_image_mode) {
      currentFilter.value = 'imageGallery';
      currentFeedId.value = feedId;
      currentCategory.value = null;
      tempSelection.value = { feedId, category: null };
      return;
    }

    currentFeedId.value = feedId;
    currentCategory.value = null;
    tempSelection.value = { feedId, category: null };

    if (shouldUseClusterList()) {
      resetArticleCollection();
      return;
    }

    fetchArticles();
  }

  function setCategory(category: string): void {
    const feedStore = useFeedStore();
    const categoryFeeds = feedStore.feeds.filter((f: Feed) => {
      if (category === '') {
        return !f.category || f.category === '';
      }

      const feedCategory = f.category || '';
      return feedCategory === category || feedCategory.startsWith(category + '/');
    });

    const allImageMode =
      categoryFeeds.length > 0 && categoryFeeds.every((f: Feed) => f.is_image_mode);

    if (allImageMode) {
      currentFilter.value = 'imageGallery';
      currentFeedId.value = null;
      currentCategory.value = category;
      tempSelection.value = { feedId: null, category };
      return;
    }

    currentFeedId.value = null;
    currentCategory.value = category;
    tempSelection.value = { feedId: null, category };

    if (shouldUseClusterList()) {
      resetArticleCollection();
      return;
    }

    fetchArticles();
  }

  async function fetchArticles(append: boolean = false): Promise<void> {
    if (
      currentFilter.value === 'dailyRecommendations' ||
      currentFilter.value === 'clusters' ||
      shouldUseClusterList()
    ) {
      resetArticleCollection();
      return;
    }

    if (isLoading.value) return;

    if (!append) {
      page.value = 1;
      articles.value = [];
      hasMore.value = true;
    }

    isLoading.value = true;
    const limit = 50;

    try {
      const params: Record<string, any> = {
        page: page.value,
        limit: limit,
      };
      if (currentFilter.value) params.filter = currentFilter.value;
      if (currentFeedId.value) params.feed_id = currentFeedId.value;
      if (currentCategory.value !== null) params.category = currentCategory.value;

      const data: Article[] = (await apiClient.get<Article[]>('/articles', params)) || [];

      if (data.length < limit) {
        hasMore.value = false;
      }

      if (append) {
        articles.value = [...articles.value, ...data];
      } else {
        articles.value = data;
      }
    } catch (e) {
      console.error('Error fetching articles:', e);
    } finally {
      isLoading.value = false;
    }
  }

  async function loadMore(): Promise<void> {
    if (hasMore.value && !isLoading.value) {
      page.value++;
      await fetchArticles(true);
    }
  }

  function getCountViewParams(): Record<string, string> {
    return shouldUseClusterList() ? { view: 'clusters' } : {};
  }

  async function fetchUnreadCounts(): Promise<void> {
    try {
      const data: any = await apiClient.get('/articles/unread-counts', getCountViewParams());
      unreadCounts.value = {
        total: data.total || 0,
        feedCounts: data.feed_counts || {},
      };
    } catch {
      unreadCounts.value = { total: 0, feedCounts: {} };
    }
  }

  async function fetchFilterCounts(): Promise<void> {
    try {
      const data: any = await apiClient.get('/articles/filter-counts', getCountViewParams());
      filterCounts.value = {
        unread: data.unread || {},
        favorites: data.favorites || {},
        favorites_unread: data.favorites_unread || {},
        read_later: data.read_later || {},
        read_later_unread: data.read_later_unread || {},
        images: data.images || {},
        images_unread: data.images_unread || {},
      };
    } catch (e) {
      console.error('[Article Store] Fetch filter counts error:', e);
      filterCounts.value = {
        unread: {},
        favorites: {},
        favorites_unread: {},
        read_later: {},
        read_later_unread: {},
        images: {},
        images_unread: {},
      };
    }
  }

  async function markAllAsRead(feedId?: number, category?: string): Promise<void> {
    try {
      const params: Record<string, any> = {};
      if (feedId) params.feed_id = feedId;
      if (category) params.category = category;

      await apiClient.post('/articles/mark-all-read', {}, params);
      await fetchArticles();
      await fetchUnreadCounts();
    } catch (e) {
      console.error('Error marking all as read:', e);
      throw e;
    }
  }

  function setCurrentArticle(id: number | null): void {
    currentArticleId.value = id;
  }

  function setSearchQuery(query: string): void {
    searchQuery.value = query;
  }

  function setShowOnlyUnread(value: boolean): void {
    showOnlyUnread.value = value;
    localStorage.setItem('showOnlyUnread', String(value));
  }

  function setViewModePreference(articleId: number, mode: 'original' | 'rendered'): void {
    articleViewModePreferences.value.set(articleId, mode);
  }

  function getViewModePreference(articleId: number): 'original' | 'rendered' | undefined {
    return articleViewModePreferences.value.get(articleId);
  }

  function setAISearchResults(results: Article[]): void {
    aiSearchResults.value = results;
  }

  function clearAISearchResults(): void {
    aiSearchResults.value = [];
  }

  function setActiveFilters(filters: FilterCondition[]): void {
    activeFilters.value = filters;
  }

  function setFilteredArticlesFromServer(articlesFromServer: Article[]): void {
    filteredArticlesFromServer.value = articlesFromServer;
  }

  function setFilterLoading(loading: boolean): void {
    isFilterLoading.value = loading;
  }

  function updateArticleSummary(articleId: number, summary: string): void {
    const collections = [articles.value, filteredArticlesFromServer.value, aiSearchResults.value];

    collections.forEach((collection) => {
      const article = collection.find((item) => item.id === articleId);
      if (article) {
        article.summary = summary;
      }
    });
  }

  return {
    articles,
    unreadCounts,
    currentFilter,
    currentFeedId,
    currentCategory,
    currentArticleId,
    tempSelection,
    isLoading,
    page,
    hasMore,
    searchQuery,
    showOnlyUnread,
    activeFilters,
    filteredArticlesFromServer,
    isFilterLoading,
    aiEnhancedMode,
    articleViewModePreferences,
    aiSearchResults,
    filterCounts,
    shouldUseClusterList,
    setAIEnhancedMode,
    setFilter,
    setFeed,
    setCategory,
    fetchArticles,
    loadMore,
    fetchUnreadCounts,
    fetchFilterCounts,
    markAllAsRead,
    setCurrentArticle,
    setSearchQuery,
    setShowOnlyUnread,
    setViewModePreference,
    getViewModePreference,
    setAISearchResults,
    clearAISearchResults,
    setActiveFilters,
    setFilteredArticlesFromServer,
    setFilterLoading,
    updateArticleSummary,
  };
});
