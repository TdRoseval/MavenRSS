import { ref, onMounted, onBeforeUnmount } from 'vue';
import { openInBrowser } from '@/shared/lib/browser';
import { authPost } from '@/shared/lib/authFetch';
import { useArticleStore } from '@/features/article/store';
import { useFeedStore } from '@/features/feed/store';
import { useClusterStore } from '@/stores/cluster';
import type { Article, Cluster, DailyRecommendationItem } from '@/types/models';

export interface KeyboardShortcuts {
  nextArticle: string;
  previousArticle: string;
  nextArticleArrow: string;
  previousArticleArrow: string;
  openArticle: string;
  closeArticle: string;
  toggleReadStatus: string;
  toggleFavoriteStatus: string;
  toggleReadLaterStatus: string;
  forceTranslate: string;
  openInBrowser: string;
  toggleContentView: string;
  refreshFeeds: string;
  markAllRead: string;
  openSettings: string;
  addFeed: string;
  focusSearch: string;
  toggleFilter: string;
  goToAllArticles: string;
  goToUnread: string;
  goToFavorites: string;
  goToReadLater: string;
}

export interface KeyboardShortcutCallbacks {
  onOpenSettings: () => void;
  onAddFeed: () => void;
  onMarkAllRead: () => Promise<void>;
}

export function useKeyboardShortcuts(callbacks: KeyboardShortcutCallbacks) {
  const articleStore = useArticleStore();
  const feedStore = useFeedStore();
  const clusterStore = useClusterStore();

  const shortcutsEnabled = ref(true);
  const shortcuts = ref<KeyboardShortcuts>({
    nextArticle: 'j',
    previousArticle: 'k',
    nextArticleArrow: 'ArrowRight',
    previousArticleArrow: 'ArrowLeft',
    openArticle: 'Enter',
    closeArticle: 'Escape',
    toggleReadStatus: 'r',
    toggleFavoriteStatus: 's',
    toggleReadLaterStatus: 'l',
    forceTranslate: 't',
    openInBrowser: 'o',
    toggleContentView: 'v',
    refreshFeeds: 'Shift+r',
    markAllRead: 'Shift+a',
    openSettings: ',',
    addFeed: 'a',
    focusSearch: '/',
    toggleFilter: 'f',
    goToAllArticles: '1',
    goToUnread: '2',
    goToFavorites: '3',
    goToReadLater: '4',
  });

  // Helper functions
  function buildKeyCombo(e: KeyboardEvent): string {
    let key = '';
    if (e.ctrlKey) key += 'Ctrl+';
    if (e.altKey) key += 'Alt+';
    if (e.shiftKey) key += 'Shift+';
    if (e.metaKey) key += 'Meta+';

    let actualKey = e.key;
    if (actualKey === ' ') actualKey = 'Space';
    else if (actualKey.length === 1) actualKey = actualKey.toLowerCase();

    key += actualKey;
    return key;
  }

  function isClusterMode(): boolean {
    return articleStore.shouldUseClusterList() || articleStore.currentFilter === 'dailyRecommendations';
  }

  // Effective cluster list matching ClusterList's displayedClusters order
  function getEffectiveClusters(): Cluster[] {
    const source =
      articleStore.currentFilter === 'dailyRecommendations'
        ? clusterStore.dailyRecommendations.map((item: DailyRecommendationItem) => item.cluster)
        : clusterStore.clusters;

    if (!articleStore.showOnlyUnread) {
      return source;
    }

    return source.filter(
      (item: Cluster) => !item.is_read || item.id === clusterStore.currentClusterId
    );
  }

  function navigateCluster(direction: number): void {
    const clusters = getEffectiveClusters();
    if (clusters.length === 0) return;

    const currentIndex = clusterStore.currentClusterId
      ? clusters.findIndex((c: Cluster) => c.id === clusterStore.currentClusterId)
      : -1;

    let newIndex: number;
    if (currentIndex === -1) {
      newIndex = direction > 0 ? 0 : clusters.length - 1;
    } else {
      newIndex = currentIndex + direction;
      if (newIndex < 0) newIndex = 0;
      if (newIndex >= clusters.length) newIndex = clusters.length - 1;
    }

    selectClusterByIndex(newIndex);
  }

  function selectClusterByIndex(index: number): void {
    const cluster = getEffectiveClusters()[index];
    if (!cluster) return;

    clusterStore.currentClusterId = cluster.id;
    clusterStore.reportClusterClick(cluster.id);

    if (!cluster.is_read) {
      clusterStore.markClusterRead(cluster.id, true).catch((e: unknown) => {
        console.error('Error marking cluster as read:', e);
      });
    }

    setTimeout(() => {
      const clusterEl = document.querySelector(`[data-cluster-id="${cluster.id}"]`);
      if (clusterEl) {
        clusterEl.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      }
    }, 50);
  }

  function toggleCurrentClusterFavorite(): void {
    const cluster = getEffectiveClusters().find(
      (c: Cluster) => c.id === clusterStore.currentClusterId
    );
    if (!cluster) return;

    cluster.is_favorite = !cluster.is_favorite;
    clusterStore
      .toggleClusterFavorite({ ...cluster, is_favorite: !cluster.is_favorite })
      .catch((e: unknown) => {
        console.error('Error toggling cluster favorite:', e);
        cluster.is_favorite = !cluster.is_favorite;
      });
  }

  function navigateArticle(direction: number): void {
    const articles = articleStore.articles;
    if (!articles || articles.length === 0) return;

    const currentIndex = articleStore.currentArticleId
      ? articles.findIndex((a: Article) => a.id === articleStore.currentArticleId)
      : -1;

    let newIndex: number;
    if (currentIndex === -1) {
      newIndex = direction > 0 ? 0 : articles.length - 1;
    } else {
      newIndex = currentIndex + direction;
      if (newIndex < 0) newIndex = 0;
      if (newIndex >= articles.length) newIndex = articles.length - 1;
    }

    selectArticleByIndex(newIndex);
  }

  function selectArticleByIndex(index: number): void {
    const article = articleStore.articles[index];
    if (!article) return;

    articleStore.currentArticleId = article.id;

    // Mark as read
    if (!article.is_read) {
      article.is_read = true;
      authPost(`/api/articles/read?id=${article.id}&read=true`)
        .then(() => articleStore.fetchUnreadCounts())
        .catch((e) => console.error('Error marking as read:', e));
    }

    // Scroll the article into view
    setTimeout(() => {
      const articleEl = document.querySelector(`[data-article-id="${article.id}"]`);
      if (articleEl) {
        articleEl.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      }
    }, 50);
  }

  function toggleCurrentArticleRead(): void {
    const article = articleStore.articles.find(
      (a: Article) => a.id === articleStore.currentArticleId
    );
    if (!article) return;

    const newState = !article.is_read;
    article.is_read = newState;
    authPost(`/api/articles/read?id=${article.id}&read=${newState}`)
      .then(() => articleStore.fetchUnreadCounts())
      .catch((e) => {
        console.error('Error toggling read:', e);
        article.is_read = !newState;
      });
  }

  function toggleCurrentArticleFavorite(): void {
    const article = articleStore.articles.find(
      (a: Article) => a.id === articleStore.currentArticleId
    );
    if (!article) return;

    const newState = !article.is_favorite;
    article.is_favorite = newState;
    authPost(`/api/articles/favorite?id=${article.id}`).catch((e) => {
      console.error('Error toggling favorite:', e);
      article.is_favorite = !newState;
    });
  }

  function toggleCurrentArticleReadLater(): void {
    const article = articleStore.articles.find(
      (a: Article) => a.id === articleStore.currentArticleId
    );
    if (!article) return;

    const newState = !article.is_read_later;
    article.is_read_later = newState;
    // When adding to read later, also mark as unread
    if (newState) {
      article.is_read = false;
    }
    authPost(`/api/articles/toggle-read-later?id=${article.id}`)
      .then(() => articleStore.fetchUnreadCounts())
      .catch((e) => {
        console.error('Error toggling read later:', e);
        article.is_read_later = !newState;
      });
  }

  function openCurrentArticleInBrowser(): void {
    const article = articleStore.articles.find(
      (a: Article) => a.id === articleStore.currentArticleId
    );
    if (article && article.url) {
      openInBrowser(article.url);
    }
  }

  function focusSearchInput(): void {
    const searchInput = document.querySelector('[data-search-input]') as HTMLInputElement;
    if (searchInput) {
      searchInput.focus();
    }
  }

  // Force re-translate the currently open article or cluster
  function forceTranslateCurrent(): void {
    if (isClusterMode()) {
      if (clusterStore.currentClusterId) {
        window.dispatchEvent(new CustomEvent('force-translate-cluster'));
      }
      return;
    }

    if (articleStore.currentArticleId) {
      window.dispatchEvent(new CustomEvent('force-translate-article'));
    }
  }

  // Check if an article detail panel is open and scrollable
  function isArticleDetailOpen(): boolean {
    // Check if there's a current article selected
    if (!articleStore.currentArticleId) return false;

    // Check if the article detail panel is visible
    const articleDetail = document.querySelector('main[class*="flex-1 bg-bg-primary"]');
    if (!articleDetail) return false;

    // Check if the article detail has scrollable content
    const scrollableContent = articleDetail.querySelector('.overflow-y-auto');
    if (!scrollableContent) return false;

    return true;
  }

  // Check if currently viewing original webpage (iframe mode)
  function isWebpageViewMode(): boolean {
    const iframe = document.querySelector('iframe[src*="/api/webpage/proxy"]');
    return iframe !== null;
  }

  // Find the currently open detail panel (article or cluster)
  function getActiveDetailMain(): HTMLElement | null {
    if (isClusterMode()) {
      if (!clusterStore.currentClusterId) return null;
      return document.querySelector<HTMLElement>('main[class*="min-w-0"]');
    }

    if (!articleStore.currentArticleId) return null;
    return document.querySelector<HTMLElement>('main[class*="flex-1 bg-bg-primary"]');
  }

  // Tab key: cycle focus through visible interactive elements inside the open
  // article/cluster detail, top to bottom. Native Enter then "clicks" the
  // focused element.
  function handleDetailTabCycle(e: KeyboardEvent): void {
    if (isWebpageViewMode()) return;

    const main = getActiveDetailMain();
    if (!main) return;

    const target = e.target as HTMLElement;
    const tagName = target.tagName.toLowerCase();
    // Let users tab out of form fields naturally
    if (tagName === 'input' || tagName === 'textarea' || tagName === 'select') return;
    if (target.isContentEditable) return;

    e.preventDefault();

    const toolbarSelector = '[data-article-toolbar], [data-cluster-toolbar]';
    const isVisible = (el: HTMLElement): boolean => {
      const rect = el.getBoundingClientRect();
      return rect.width > 0 && rect.height > 0;
    };

    const toolbarButtons = Array.from(
      main.querySelectorAll<HTMLElement>(`${toolbarSelector} button:not([disabled])`)
    ).filter(isVisible);

    const contentElements = Array.from(
      main.querySelectorAll<HTMLElement>('.prose-content a[href], .prose-content img')
    ).filter(isVisible);

    const items = [...toolbarButtons, ...contentElements];
    if (items.length === 0) return;

    // Images are not natively focusable
    items.forEach((el) => {
      if (el.tagName === 'IMG' && !el.hasAttribute('tabindex')) {
        el.setAttribute('tabindex', '-1');
      }
    });

    const active = document.activeElement as HTMLElement | null;
    const currentIndex = active ? items.indexOf(active) : -1;

    let nextIndex: number;
    if (e.shiftKey) {
      nextIndex = currentIndex <= 0 ? items.length - 1 : currentIndex - 1;
    } else {
      nextIndex = currentIndex === -1 || currentIndex >= items.length - 1 ? 0 : currentIndex + 1;
    }

    items[nextIndex].focus({ preventScroll: false });
  }

  // Scroll the article detail panel
  function scrollArticleDetail(direction: 'up' | 'down' | 'pageDown' | 'pageUp'): void {
    const articleDetail = document.querySelector('main[class*="flex-1 bg-bg-primary"]');
    if (!articleDetail) return;

    const scrollableContent = articleDetail.querySelector('.overflow-y-auto') as HTMLElement;
    if (!scrollableContent) return;

    const scrollAmount =
      direction === 'pageDown' || direction === 'pageUp'
        ? scrollableContent.clientHeight * 0.9
        : 100; // For arrow keys

    const newScrollTop =
      direction === 'down' || direction === 'pageDown'
        ? scrollableContent.scrollTop + scrollAmount
        : scrollableContent.scrollTop - scrollAmount;

    scrollableContent.scrollTo({
      top: newScrollTop,
      behavior: 'smooth',
    });
  }

  // Keyboard event handler
  function handleKeyboardShortcut(e: KeyboardEvent): void {
    // Skip if shortcuts are disabled
    if (!shortcutsEnabled.value) {
      return;
    }

    // Check if image viewer is open - if so, let it handle arrow keys
    const imageViewerOpen = document.querySelector('[data-image-viewer="true"]') !== null;
    if (imageViewerOpen) {
      // Image viewer handles its own keyboard events
      // Only ESC key should be handled here to close the viewer
      const key = buildKeyCombo(e);
      if (key === shortcuts.value.closeArticle) {
        // Let the image viewer's ESC handler close it
        return;
      }
      // Block all other shortcuts when image viewer is open
      return;
    }

    // Check if settings modal is open
    const settingsModalOpen = document.querySelector('[data-settings-modal="true"]') !== null;

    // If settings modal is open, only allow ESC key
    if (settingsModalOpen) {
      const key = buildKeyCombo(e);
      if (key === shortcuts.value.closeArticle) {
        // Let the modal's own ESC handler deal with it
        return;
      }
      // Block all other shortcuts when settings modal is open
      return;
    }

    // Skip if we're in an input field, textarea, or contenteditable
    const target = e.target as HTMLElement;
    const tagName = target.tagName.toLowerCase();
    const isEditable = target.isContentEditable;
    const isInput = tagName === 'input' || tagName === 'textarea' || tagName === 'select';

    const key = buildKeyCombo(e);

    // Handle article detail scrolling when article is open
    // Only in RSS content view mode, not in webpage (iframe) view mode
    if (isArticleDetailOpen() && !isWebpageViewMode()) {
      // Space key - scroll page down
      if (key === 'Space') {
        // Prevent default only if not in input field
        if (!isInput && !isEditable) {
          e.preventDefault();
          scrollArticleDetail('pageDown');
          return;
        }
      }

      // ArrowDown - scroll down
      if (key === 'ArrowDown') {
        if (!isInput && !isEditable) {
          e.preventDefault();
          scrollArticleDetail('down');
          return;
        }
      }

      // ArrowUp - scroll up
      if (key === 'ArrowUp') {
        if (!isInput && !isEditable) {
          e.preventDefault();
          scrollArticleDetail('up');
          return;
        }
      }

      // Shift+Space - scroll page up
      if (key === 'Shift+Space') {
        if (!isInput && !isEditable) {
          e.preventDefault();
          scrollArticleDetail('pageUp');
          return;
        }
      }
    }

    // Tab key: cycle focus through visible interactive elements in the open
    // article/cluster detail panel (top to bottom)
    if (key === 'Tab' || key === 'Shift+Tab') {
      handleDetailTabCycle(e);
      return;
    }

    // Check for escape key to close modals first (always allow, even when shortcuts disabled)
    if (key === shortcuts.value.closeArticle) {
      // Check if the find in page search input is focused
      const findInputFocused = document.activeElement?.classList.contains('find-input');

      // If find input is focused, don't handle ESC here - let FindInPage component handle it
      if (findInputFocused) {
        return;
      }

      // Check if there are any open modals
      const hasOpenModal = document.querySelector('[data-modal-open="true"]') !== null;

      if (!hasOpenModal) {
        // No modals open, handle article close
        if (articleStore.currentArticleId) {
          articleStore.currentArticleId = null;
          e.preventDefault();
        }
      }
      // If modals are open, let them handle ESC themselves
      return;
    }

    // Skip shortcuts if in input field (except escape)
    if (isInput || isEditable) {
      return;
    }

    // Match the key combination to a shortcut action
    const action = Object.entries(shortcuts.value).find(([, shortcut]) => shortcut === key)?.[0];

    if (!action) return;

    e.preventDefault();

    // Execute the action
    switch (action) {
      case 'nextArticle':
      case 'nextArticleArrow':
        if (isClusterMode()) {
          navigateCluster(1);
        } else {
          navigateArticle(1);
        }
        break;
      case 'previousArticle':
      case 'previousArticleArrow':
        if (isClusterMode()) {
          navigateCluster(-1);
        } else {
          navigateArticle(-1);
        }
        break;
      case 'openArticle':
        if (isClusterMode()) {
          if (getEffectiveClusters().length > 0 && !clusterStore.currentClusterId) {
            selectClusterByIndex(0);
          }
        } else if (articleStore.articles.length > 0 && !articleStore.currentArticleId) {
          selectArticleByIndex(0);
        }
        break;
      case 'toggleReadStatus':
        toggleCurrentArticleRead();
        break;
      case 'toggleFavoriteStatus':
        if (isClusterMode()) {
          toggleCurrentClusterFavorite();
        } else {
          toggleCurrentArticleFavorite();
        }
        break;
      case 'forceTranslate':
        forceTranslateCurrent();
        break;
      case 'toggleReadLaterStatus':
        toggleCurrentArticleReadLater();
        break;
      case 'openInBrowser':
        openCurrentArticleInBrowser();
        break;
      case 'toggleContentView':
        window.dispatchEvent(new CustomEvent('toggle-content-view'));
        break;
      case 'refreshFeeds':
        feedStore.refreshFeeds();
        break;
      case 'markAllRead':
        callbacks.onMarkAllRead();
        break;
      case 'openSettings':
        callbacks.onOpenSettings();
        break;
      case 'addFeed':
        callbacks.onAddFeed();
        break;
      case 'focusSearch':
        focusSearchInput();
        break;
      case 'toggleFilter':
        window.dispatchEvent(new CustomEvent('toggle-filter'));
        break;
      case 'goToAllArticles':
        articleStore.setFilter('all');
        break;
      case 'goToUnread':
        articleStore.setFilter('unread');
        break;
      case 'goToFavorites':
        articleStore.setFilter('favorites');
        break;
      case 'goToReadLater':
        articleStore.setFilter('readLater');
        break;
    }
  }

  // Handle shortcuts changed event
  function handleShortcutsChanged(e: Event): void {
    const customEvent = e as CustomEvent;
    if (customEvent.detail && customEvent.detail.shortcuts) {
      shortcuts.value = { ...shortcuts.value, ...customEvent.detail.shortcuts };
    }
  }

  // Handle shortcuts enabled changed event
  function handleShortcutsEnabledChanged(e: Event): void {
    const customEvent = e as CustomEvent;
    if (customEvent.detail && typeof customEvent.detail.enabled === 'boolean') {
      shortcutsEnabled.value = customEvent.detail.enabled;
    }
  }

  // Initialize shortcuts enabled state from settings
  function initializeShortcutsEnabled(): void {
    // Note: store.settings is not available in the store
    // The shortcuts_enabled state is initialized via the shortcuts-enabled-changed event
    // Default is true (enabled)
  }

  // Lifecycle
  onMounted(() => {
    initializeShortcutsEnabled();
    window.addEventListener('keydown', handleKeyboardShortcut);
    window.addEventListener('shortcuts-changed', handleShortcutsChanged);
    window.addEventListener('shortcuts-enabled-changed', handleShortcutsEnabledChanged);
  });

  onBeforeUnmount(() => {
    window.removeEventListener('keydown', handleKeyboardShortcut);
    window.removeEventListener('shortcuts-changed', handleShortcutsChanged);
    window.removeEventListener('shortcuts-enabled-changed', handleShortcutsEnabledChanged);
  });

  return {
    shortcuts,
  };
}
