import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import type { Article, UnreadCounts } from '@/types/models';
import type { FilterCondition } from '@/types/filter';
import { apiClient } from '@/shared/lib/apiClient';
import { useFeedStore } from '@/features/feed/store';

export type Filter = 'all' | 'unread' | 'favorites' | 'readLater' | 'imageGallery' | '';

export interface TempSelection {
  feedId: number | null;
  category: string | null;
}

export const useArticleStore = defineStore('article', () => {
  // State
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

  // Article view mode preferences (persisted across component mounts)
  const articleViewModePreferences = ref<Map<number, 'original' | 'rendered'>>(new Map());

  // AI Search results
  const aiSearchResults = ref<Article[]>([]);

  // Filter-specific counts for sidebar filtering
  const filterCounts = ref<Record<string, Record<number | string, number>>>({
    unread: {},
    favorites: {},
    favorites_unread: {},
    read_later: {},
    read_later_unread: {},
    images: {},
    images_unread: {},
  });

  // Actions
  async function setFilter(filter: Filter): Promise<void> {
    currentFilter.value = filter;
    currentFeedId.value = null;
    currentCategory.value = null;
    tempSelection.value = { feedId: null, category: null };
    // Refresh filter counts to ensure sidebar shows correct feeds
    await fetchFilterCounts();
    // Clear and reset will be handled by fetchArticles
    fetchArticles();
  }

  function setFeed(feedId: number): void {
    const feedStore = useFeedStore();
    // Check if this feed is an image mode feed
    const feed = feedStore.feeds.find((f) => f.id === feedId);
    if (feed?.is_image_mode) {
      // For image mode feeds, switch filter to image gallery
      currentFilter.value = 'imageGallery';
      currentFeedId.value = feedId;
      currentCategory.value = null;
      tempSelection.value = { feedId, category: null };
      // Clear and reset will be handled by fetchArticles
    } else {
      // For regular feeds, keep currentFilter and set tempSelection
      currentFeedId.value = feedId;
      currentCategory.value = null;
      tempSelection.value = { feedId, category: null };
      fetchArticles();
    }
  }

  function setCategory(category: string): void {
    const feedStore = useFeedStore();
    // Check if this category contains only image mode feeds
    const categoryFeeds = feedStore.feeds.filter((f) => {
      // Handle uncategorized category (empty string)
      if (category === '') {
        return !f.category || f.category === '';
      }

      // Handle nested categories by checking if the feed's category starts with the selected path
      const feedCategory = f.category || '';
      return feedCategory === category || feedCategory.startsWith(category + '/');
    });

    const allImageMode = categoryFeeds.length > 0 && categoryFeeds.every((f) => f.is_image_mode);

    // If all feeds in this category are image mode, switch to image gallery filter
    if (allImageMode) {
      currentFilter.value = 'imageGallery';
      currentFeedId.value = null;
      currentCategory.value = category;
      tempSelection.value = { feedId: null, category };
      // Don't call fetchArticles here - ImageGalleryView will handle fetching
    } else {
      // For regular categories, keep currentFilter and set tempSelection
      currentFeedId.value = null;
      currentCategory.value = category;
      tempSelection.value = { feedId: null, category };
      fetchArticles();
    }
  }

  async function fetchArticles(append: boolean = false): Promise<void> {
    if (isLoading.value) return;

    // If not appending, reset to page 1 and clear articles
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

  async function fetchUnreadCounts(): Promise<void> {
    try {
      const data: any = await apiClient.get('/articles/unread-counts');
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
      const data: any = await apiClient.get('/articles/filter-counts');
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
      // Refresh articles and unread counts
      await fetchArticles();
      await fetchUnreadCounts();
    } catch (e) {
      console.error('[Article Store] Mark all as read error:', e);
    }
  }

  function updateArticleSummary(articleId: number, summary: string): void {
    const articleIndex = articles.value.findIndex((a) => a.id === articleId);
    if (articleIndex !== -1) {
      articles.value[articleIndex] = {
        ...articles.value[articleIndex],
        summary,
      };
    }
  }

  function toggleShowOnlyUnread(): void {
    showOnlyUnread.value = !showOnlyUnread.value;
    localStorage.setItem('showOnlyUnread', String(showOnlyUnread.value));
  }

  function setActiveFilters(filters: FilterCondition[]): void {
    activeFilters.value = filters;
  }

  function setFilteredArticlesFromServer(articles: Article[]): void {
    filteredArticlesFromServer.value = articles;
  }

  function setIsFilterLoading(loading: boolean): void {
    isFilterLoading.value = loading;
  }

  function setAISearchResults(results: Article[]): void {
    aiSearchResults.value = results;
  }

  function clearAISearchResults(): void {
    aiSearchResults.value = [];
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
    articleViewModePreferences,
    aiSearchResults,
    filterCounts,
    setFilter,
    setFeed,
    setCategory,
    fetchArticles,
    loadMore,
    fetchUnreadCounts,
    fetchFilterCounts,
    markAllAsRead,
    updateArticleSummary,
    toggleShowOnlyUnread,
    setActiveFilters,
    setFilteredArticlesFromServer,
    setIsFilterLoading,
    setAISearchResults,
    clearAISearchResults,
  };
});
