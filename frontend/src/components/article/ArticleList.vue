<script setup lang="ts">
import { useAppStore } from '@/stores/app';
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick, type Ref } from 'vue';
import {
  PhArrowClockwise,
  PhList,
  PhSpinner,
  PhFunnel,
  PhTrash,
  PhCheckCircle,
  PhEye,
  PhEyeSlash,
  PhCircle,
  PhClock,
  PhLightning,
} from '@phosphor-icons/vue';
import ArticleFilterModal from '../modals/filter/ArticleFilterModal.vue';
import ArticleItem from './ArticleItem.vue';
import { useArticleTranslation } from '@/composables/article/useArticleTranslation';
import { useArticleFilter } from '@/composables/article/useArticleFilter';
import { useArticleActions } from '@/composables/article/useArticleActions';
import { useShowPreviewImages } from '@/composables/ui/useShowPreviewImages';
import type { Article } from '@/types/models';

const store = useAppStore();
const { t } = useI18n();

const listRef: Ref<HTMLDivElement | null> = ref(null);
const defaultViewMode = ref<'original' | 'rendered'>('original');
const showFilterModal = ref(false);
const isRefreshing = ref(false);
const savedScrollTop = ref(0);
const showRefreshTooltip = ref(false);
// Track articles that should be temporarily kept in list even if read
const temporarilyKeepArticles = ref<Set<number>>(new Set());

interface Props {
  isSidebarOpen?: boolean;
}

defineProps<Props>();

const emit = defineEmits<{
  toggleSidebar: [];
}>();

// Use composables
const {
  translationSettings,
  loadTranslationSettings,
  setupIntersectionObserver,
  observeArticle,
  handleTranslationSettingsChange,
  cleanup: cleanupTranslation,
} = useArticleTranslation();

const {
  activeFilters,
  filteredArticlesFromServer,
  isFilterLoading,
  resetFilterState,
  fetchFilteredArticles,
  loadMoreFilteredArticles,
} = useArticleFilter();

const { showArticleContextMenu } = useArticleActions(t, defaultViewMode, () =>
  store.fetchUnreadCounts()
);

// Computed filtered articles
const filteredArticles = computed(() => {
  let articles = activeFilters.value.length > 0 ? filteredArticlesFromServer.value : store.articles;

  // If show only unread is enabled, filter out read articles
  // but keep articles that are temporarily marked (currently being viewed)
  if (store.showOnlyUnread) {
    articles = articles.filter(
      (article) => !article.is_read || temporarilyKeepArticles.value.has(article.id)
    );
  }

  return articles;
});

// Initialize show preview images setting
const { initialize: initializeShowPreviewImages } = useShowPreviewImages();

// Load settings and setup
onMounted(async () => {
  await loadTranslationSettings();
  await initializeShowPreviewImages();

  try {
    const res = await fetch('/api/settings');
    const data = await res.json();
    defaultViewMode.value = data.default_view_mode || 'original';

    // Set up intersection observer for auto-translation
    if (translationSettings.value.enabled && listRef.value) {
      setupIntersectionObserver(listRef.value, store.articles);
    }
  } catch (e) {
    console.error('Error loading settings:', e);
  }

  // Listen for translation settings changes
  window.addEventListener(
    'translation-settings-changed',
    onTranslationSettingsChanged as EventListener
  );
  // Listen for default view mode changes
  window.addEventListener('default-view-mode-changed', onDefaultViewModeChanged as EventListener);
  // Listen for show preview images changes
  window.addEventListener(
    'show-preview-images-changed',
    onShowPreviewImagesChanged as EventListener
  );
  // Listen for refresh articles events
  window.addEventListener('refresh-articles', onRefreshArticles);
  // Listen for toggle filter events (from keyboard shortcut)
  window.addEventListener('toggle-filter', onToggleFilter);
});

// Watch for articles changes to maintain scroll position and re-observe new articles
watch(
  () => store.articles,
  async () => {
    if (isRefreshing.value && listRef.value) {
      // Restore scroll position during refresh
      listRef.value.scrollTop = savedScrollTop.value;
    } else if (!isRefreshing.value && listRef.value) {
      // For non-refresh changes (like clicking article to load content),
      // save and restore scroll position to prevent jumping to top
      const currentScroll = listRef.value.scrollTop;
      await nextTick();
      listRef.value.scrollTop = currentScroll;
    }

    // Re-setup observer to observe newly added articles
    if (translationSettings.value.enabled && listRef.value) {
      await nextTick();
      setupIntersectionObserver(listRef.value, store.articles);
    }
  },
  { deep: true }
);

// Watch for refresh completion to scroll to top
watch(
  () => store.refreshProgress.isRunning,
  (isRunning) => {
    if (!isRunning && isRefreshing.value) {
      // Refresh completed, scroll to top and reset state
      isRefreshing.value = false;
      if (listRef.value) {
        listRef.value.scrollTop = 0;
      }
    }
  }
);

// Watch for filtered articles changes to re-observe new articles
watch(
  () => filteredArticlesFromServer.value,
  async () => {
    // Re-setup observer to observe newly added filtered articles
    if (translationSettings.value.enabled && listRef.value) {
      await nextTick();
      setupIntersectionObserver(listRef.value, filteredArticlesFromServer.value);
    }
  },
  { deep: true }
);

onBeforeUnmount(() => {
  cleanupTranslation();
  window.removeEventListener(
    'translation-settings-changed',
    onTranslationSettingsChanged as EventListener
  );
  window.removeEventListener(
    'default-view-mode-changed',
    onDefaultViewModeChanged as EventListener
  );
  window.removeEventListener(
    'show-preview-images-changed',
    onShowPreviewImagesChanged as EventListener
  );
  window.removeEventListener('refresh-articles', onRefreshArticles);
  window.removeEventListener('toggle-filter', onToggleFilter);
});

interface CustomEventDetail {
  mode?: string;
  enabled?: boolean;
  targetLang?: string;
}

// Event handlers
function onDefaultViewModeChanged(e: Event): void {
  const customEvent = e as CustomEvent<CustomEventDetail>;
  if (customEvent.detail.mode) {
    defaultViewMode.value = customEvent.detail.mode as 'original' | 'rendered';
  }
}

function onTranslationSettingsChanged(e: Event): void {
  const customEvent = e as CustomEvent<CustomEventDetail>;
  const { enabled, targetLang } = customEvent.detail;
  if (enabled !== undefined && targetLang) {
    handleTranslationSettingsChange(enabled, targetLang);

    // Re-setup observer if needed
    if (enabled && listRef.value) {
      setupIntersectionObserver(listRef.value, store.articles);
    }
  }
}

function onShowPreviewImagesChanged(e: Event): void {
  const customEvent = e as CustomEvent<{ value: boolean }>;
  const { updateValue } = useShowPreviewImages();
  updateValue(customEvent.detail.value);
}

function onRefreshArticles(): void {
  store.fetchArticles();
}

function onToggleFilter(): void {
  showFilterModal.value = !showFilterModal.value;
}

// Show tooltip when hovering over refresh button
function onRefreshTooltipShow(): void {
  showRefreshTooltip.value = true;
  // Task details are automatically updated via pollProgress()
}

function onRefreshTooltipHide(): void {
  showRefreshTooltip.value = false;
}

// Article selection and interaction
function selectArticle(article: Article): void {
  // If clicking the same article, close the detail view
  if (store.currentArticleId === article.id) {
    store.currentArticleId = null;
    // Remove from temporarily keep list when closing
    temporarilyKeepArticles.value.delete(article.id);
    return;
  }

  // Reset user preference when selecting article via normal click
  window.dispatchEvent(new CustomEvent('reset-user-view-preference'));

  // If switching from one article to another, remove the previous one from temp list
  if (store.currentArticleId) {
    temporarilyKeepArticles.value.delete(store.currentArticleId);
  }

  store.currentArticleId = article.id;
  if (!article.is_read) {
    article.is_read = true;
    // Add to temporarily keep list so it doesn't disappear immediately
    temporarilyKeepArticles.value.add(article.id);
    fetch(`/api/articles/read?id=${article.id}&read=true`, { method: 'POST' })
      .then(() => {
        store.fetchUnreadCounts();
      })
      .catch((e) => {
        console.error('Error marking as read:', e);
      });
  }
}

// Scrolling handler
function handleScroll(e: Event): void {
  const target = e.target as HTMLElement;
  const { scrollTop, clientHeight, scrollHeight } = target;
  if (scrollTop + clientHeight >= scrollHeight - 200) {
    if (activeFilters.value.length > 0) {
      loadMoreFilteredArticles();
    } else {
      store.loadMore();
    }
  }
}

// Filter handlers
async function handleApplyFilters(filters: typeof activeFilters.value): Promise<void> {
  activeFilters.value = filters;
  if (filters.length === 0) {
    resetFilterState();
    store.page = 1;
    await store.fetchArticles(false);
  } else {
    await fetchFilteredArticles(filters, false);
  }
}

// Actions
async function refreshArticles(): Promise<void> {
  // Save current scroll position and set refreshing state
  if (listRef.value) {
    savedScrollTop.value = listRef.value.scrollTop;
  }
  isRefreshing.value = true;

  await store.refreshFeeds();
  // Note: Scrolling to top is now handled by the watch on refreshProgress.isRunning
}

async function markAllAsRead(): Promise<void> {
  // If filters are active, mark only filtered articles as read
  if (activeFilters.value.length > 0) {
    try {
      // Get IDs of filtered articles
      const articleIds = filteredArticlesFromServer.value.map((a) => a.id);
      if (articleIds.length === 0) {
        window.showToast(t('noArticlesToMark'), 'info');
        return;
      }

      // Mark all filtered articles as read
      await Promise.all(
        articleIds.map((id) => fetch(`/api/articles/read?id=${id}&read=true`, { method: 'POST' }))
      );

      // Refresh articles and counts
      await store.fetchArticles();
      await store.fetchUnreadCounts();
      window.showToast(t('markedAllAsRead'), 'success');
    } catch (e) {
      console.error('Error marking filtered articles as read:', e);
    }
  } else {
    // Use store's markAllAsRead which handles feed and category
    const params: { feed_id?: number; category?: string } = {};

    if (store.currentFeedId) {
      params.feed_id = store.currentFeedId;
    } else if (store.currentCategory) {
      params.category = store.currentCategory;
    }

    await store.markAllAsRead(params.feed_id, params.category);
    window.showToast(t('markedAllAsRead'), 'success');
  }
}

async function clearReadLater(): Promise<void> {
  try {
    const res = await fetch('/api/articles/clear-read-later', { method: 'POST' });
    if (res.ok) {
      await store.fetchArticles();
      window.showToast(t('clearedReadLater'), 'success');
    }
  } catch (e) {
    console.error('Error clearing read later:', e);
  }
}

// Handle hover mark as read event from ArticleItem
function handleHoverMarkAsRead(articleId: number): void {
  // Find and update the article in the store
  const article = store.articles.find((a) => a.id === articleId);
  if (article) {
    article.is_read = true;
  }
  // Also update in filtered articles if applicable
  const filteredArticle = filteredArticlesFromServer.value.find((a) => a.id === articleId);
  if (filteredArticle) {
    filteredArticle.is_read = true;
  }
}
</script>

<template>
  <section
    class="article-list flex flex-col w-full border-r border-border bg-bg-primary shrink-0 h-full"
  >
    <div class="p-2 sm:p-4 border-b border-border bg-bg-primary">
      <div class="flex items-center justify-between sm:mb-2">
        <h3 class="m-0 text-base sm:text-lg font-semibold">{{ t('articles') }}</h3>
        <div class="flex items-center gap-1 sm:gap-2">
          <!-- Clear Read Later button - only shown when viewing Read Later list -->
          <button
            v-if="store.currentFilter === 'readLater'"
            class="text-text-secondary hover:text-red-500 hover:bg-bg-tertiary p-1 sm:p-1.5 rounded transition-colors"
            :title="t('clearReadLater')"
            @click="clearReadLater"
          >
            <PhTrash :size="20" class="sm:w-6 sm:h-6" />
          </button>
          <button
            class="text-text-secondary hover:text-text-primary hover:bg-bg-tertiary p-1 sm:p-1.5 rounded transition-colors"
            :title="t('markAllRead')"
            @click="markAllAsRead"
          >
            <PhCheckCircle :size="20" class="sm:w-6 sm:h-6" />
          </button>
          <button
            class="text-text-secondary hover:text-text-primary hover:bg-bg-tertiary p-1 sm:p-1.5 rounded transition-colors"
            :class="store.showOnlyUnread ? 'text-accent' : ''"
            :title="t('showOnlyUnread')"
            @click="store.toggleShowOnlyUnread()"
          >
            <component
              :is="store.showOnlyUnread ? PhEye : PhEyeSlash"
              :size="20"
              class="sm:w-6 sm:h-6"
            />
          </button>
          <div class="relative">
            <button
              class="text-text-secondary hover:text-text-primary hover:bg-bg-tertiary p-1 sm:p-1.5 rounded transition-colors"
              :class="activeFilters.length > 0 ? 'filter-active' : ''"
              :title="t('filter')"
              @click="showFilterModal = true"
            >
              <PhFunnel :size="18" class="sm:w-5 sm:h-5" />
            </button>
            <div
              v-if="activeFilters.length > 0"
              class="absolute -top-1 -right-1 bg-accent text-white text-[9px] sm:text-[10px] font-bold rounded-full min-w-[14px] sm:min-w-[16px] h-3.5 sm:h-4 px-0.5 sm:px-1 flex items-center justify-center"
            >
              {{ activeFilters.length }}
            </div>
          </div>
          <div
            class="relative"
            @mouseenter="onRefreshTooltipShow"
            @mouseleave="onRefreshTooltipHide"
          >
            <button
              class="text-text-secondary hover:text-text-primary hover:bg-bg-tertiary p-1 sm:p-1.5 rounded transition-colors"
              :title="t('refresh')"
              @click="refreshArticles"
            >
              <PhArrowClockwise
                :size="20"
                class="sm:w-6 sm:h-6"
                :class="store.refreshProgress.isRunning ? 'animate-spin' : ''"
              />
            </button>
            <div
              v-if="
                store.refreshProgress.isRunning &&
                (store.refreshProgress.queue_task_count || 0) +
                  (store.refreshProgress.pool_task_count || 0) >
                  0
              "
              class="absolute -top-1 -right-1 bg-accent text-white text-[9px] sm:text-[10px] font-bold rounded-full min-w-[14px] sm:min-w-[16px] h-3.5 sm:h-4 px-0.5 sm:px-1 flex items-center justify-center"
            >
              {{
                (store.refreshProgress.queue_task_count || 0) +
                (store.refreshProgress.pool_task_count || 0)
              }}
            </div>

            <!-- Task Pool Tooltip -->
            <Transition
              enter-active-class="transition ease-out duration-200"
              enter-from-class="opacity-0 scale-95"
              enter-to-class="opacity-100 scale-100"
              leave-active-class="transition ease-in duration-150"
              leave-from-class="opacity-100 scale-100"
              leave-to-class="opacity-0 scale-95"
            >
              <div
                v-if="
                  showRefreshTooltip &&
                  ((store.refreshProgress.pool_task_count || 0) > 0 ||
                    (store.refreshProgress.queue_task_count || 0) > 0 ||
                    (store.refreshProgress.article_click_count || 0) > 0)
                "
                class="absolute right-0 top-full mt-2 z-50 w-72 bg-bg-secondary rounded-lg shadow-xl overflow-hidden"
              >
                <div class="px-3 py-2">
                  <div class="text-xs font-semibold text-text-primary mb-2 flex items-center gap-2">
                    <PhArrowClockwise :size="12" class="animate-spin-slow" />
                    {{ t('refreshing') }}
                  </div>

                  <!-- Pool Tasks - Show all tasks sorted alphabetically -->
                  <div v-if="(store.refreshProgress.pool_task_count || 0) > 0" class="mb-2">
                    <div
                      class="text-[10px] text-text-secondary mb-1.5 font-medium flex items-center gap-1"
                    >
                      <PhCircle :size="10" class="text-accent" />
                      {{ t('activeTasks') }} ({{ store.refreshProgress.pool_task_count || 0 }})
                    </div>
                    <div class="space-y-0.5">
                      <div
                        v-for="(task, index) in store.refreshProgress.pool_tasks || []"
                        :key="'pool-' + index"
                        class="text-xs text-text-primary bg-accent/10 px-2.5 py-1.5 rounded truncate"
                        :title="task.feed_title"
                      >
                        <div class="flex items-center gap-2">
                          <PhCircle :size="10" class="text-accent animate-pulse flex-shrink-0" />
                          <span class="truncate flex-1">{{ task.feed_title }}</span>
                        </div>
                      </div>
                    </div>
                  </div>

                  <!-- Queue Tasks - Show first 3 -->
                  <div v-if="(store.refreshProgress.queue_task_count || 0) > 0">
                    <div
                      class="text-[10px] text-text-secondary mb-1.5 font-medium flex items-center gap-1"
                    >
                      <PhClock :size="10" />
                      {{ t('queuedTasks') }} ({{ store.refreshProgress.queue_task_count || 0 }})
                    </div>
                    <div class="space-y-0.5">
                      <div
                        v-for="(task, index) in store.refreshProgress.queue_tasks || []"
                        :key="'queue-' + index"
                        class="text-xs text-text-secondary bg-bg-tertiary/50 px-2.5 py-1.5 rounded truncate"
                        :title="task.feed_title"
                      >
                        <div class="flex items-center gap-2">
                          <PhClock :size="10" class="flex-shrink-0" />
                          <span class="truncate flex-1">{{ task.feed_title }}</span>
                        </div>
                      </div>
                    </div>
                  </div>

                  <!-- Article Click Tasks -->
                  <div
                    v-if="(store.refreshProgress.article_click_count || 0) > 0"
                    class="mt-2 pt-2 border-t border-border/50"
                  >
                    <div
                      class="text-[10px] text-text-secondary mb-1.5 font-medium flex items-center gap-1"
                    >
                      <PhLightning :size="10" class="text-accent" />
                      {{ t('immediateTasks') }} ({{
                        store.refreshProgress.article_click_count || 0
                      }})
                    </div>
                    <div class="text-xs text-accent bg-accent/10 px-2.5 py-1.5 rounded truncate">
                      <div class="flex items-center gap-2">
                        <PhLightning :size="10" class="flex-shrink-0" />
                        <span class="truncate">{{ t('fetchingArticleContent') }}</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </Transition>
          </div>
          <button class="md:hidden text-xl sm:text-2xl p-1" @click="emit('toggleSidebar')">
            <PhList :size="20" class="sm:w-6 sm:h-6" />
          </button>
        </div>
      </div>
    </div>

    <div ref="listRef" class="flex-1 overflow-y-scroll" @scroll="handleScroll">
      <div
        v-if="filteredArticles.length === 0 && !store.isLoading && !isFilterLoading"
        class="p-4 sm:p-5 text-center text-text-secondary text-sm sm:text-base"
      >
        {{ t('noArticles') }}
      </div>

      <ArticleItem
        v-for="article in filteredArticles"
        :key="article.id"
        :article="article"
        :is-active="store.currentArticleId === article.id"
        @click="selectArticle(article)"
        @contextmenu="(e) => showArticleContextMenu(e, article)"
        @observe-element="observeArticle"
        @hover-mark-as-read="handleHoverMarkAsRead"
      />

      <div
        v-if="store.isLoading || isFilterLoading"
        class="p-3 sm:p-4 text-center text-text-secondary"
      >
        <PhSpinner :size="20" class="animate-spin sm:w-6 sm:h-6" />
      </div>
    </div>

    <!-- Filter Modal -->
    <ArticleFilterModal
      :show="showFilterModal"
      :current-filters="activeFilters"
      @close="showFilterModal = false"
      @apply="handleApplyFilters"
    />
  </section>
</template>

<style scoped>
@reference "../../style.css";

@media (min-width: 768px) {
  .article-list {
    width: var(--article-list-width, 400px);
  }
}

.filter-active {
  @apply text-accent border-accent;
  background-color: rgba(59, 130, 246, 0.1);
}

.animate-spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}
</style>
