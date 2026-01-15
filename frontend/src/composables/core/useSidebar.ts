import { computed, ref, watch, type Ref } from 'vue';
import { useAppStore } from '@/stores/app';
import { useI18n } from 'vue-i18n';
import { openInBrowser } from '@/utils/browser';
import type { Feed } from '@/types/models';

interface TreeNode {
  _feeds: Feed[];
  _children: Record<string, TreeNode>;
  isOpen: boolean;
}

interface TreeData {
  tree: Record<string, TreeNode>;
  uncategorized: Feed[];
  categories: Set<string>;
}

export function useSidebar() {
  const store = useAppStore();
  const { t } = useI18n();

  // Load saved category state from localStorage
  const savedCategories = localStorage.getItem('openCategories');
  const openCategories: Ref<Set<string>> = ref(
    savedCategories ? new Set(JSON.parse(savedCategories)) : new Set()
  );

  // Load saved pinned categories from localStorage
  const savedPinnedCategories = localStorage.getItem('pinnedCategories');
  const pinnedCategories: Ref<Set<string>> = ref(
    savedPinnedCategories ? new Set(JSON.parse(savedPinnedCategories)) : new Set()
  );

  const searchQuery: Ref<string> = ref('');

  // Build category tree with search filtering and filter-specific filtering
  const tree = computed<TreeData>(() => {
    const t: Record<string, TreeNode> = {};
    const uncategorized: Feed[] = [];
    const categories = new Set<string>();

    if (!store.feeds) return { tree: {}, uncategorized: [], categories };

    const query = searchQuery.value.toLowerCase().trim();

    // Determine which filter counts to use based on currentFilter
    const currentFilterType = store.currentFilter;
    const filterTypeMap: Record<string, string> = {
      unread: 'unread',
      favorites: 'favorites',
      readLater: 'read_later',
      imageGallery: 'images',
    };
    const filterKey = filterTypeMap[currentFilterType] || '';

    store.feeds.forEach((feed: Feed) => {
      const matchesSearch =
        query === '' ||
        feed.title.toLowerCase().includes(query) ||
        feed.url.toLowerCase().includes(query);

      if (!matchesSearch) return;

      // Filter by currentFilter if applicable
      if (currentFilterType && filterKey) {
        const feedCount = store.filterCounts[filterKey]?.[feed.id] || 0;
        if (feedCount === 0) return;
      }

      if (feed.category) {
        const parts = feed.category.split('/');
        let currentLevel = t;
        parts.forEach((part, index) => {
          if (!currentLevel[part]) {
            currentLevel[part] = { _feeds: [], _children: {}, isOpen: false };
          }
          if (index === parts.length - 1) {
            currentLevel[part]._feeds.push(feed);
            categories.add(feed.category);
          } else {
            currentLevel = currentLevel[part]._children;
          }
        });
      } else {
        uncategorized.push(feed);
      }
    });
    if (uncategorized.length > 0) {
      categories.add('uncategorized');
    }
    return { tree: t, uncategorized, categories };
  });

  // Compute unread counts for categories
  const categoryUnreadCounts = computed<Record<string, number>>(() => {
    const counts: Record<string, number> = {};
    if (!store.feeds || !store.unreadCounts.feedCounts) return counts;

    store.feeds.forEach((feed: Feed) => {
      if (feed.category) {
        const unreadCount = store.unreadCounts.feedCounts[feed.id] || 0;
        if (unreadCount > 0) {
          counts[feed.category] = (counts[feed.category] || 0) + unreadCount;
        }
      }
    });

    // Calculate uncategorized count
    const uncategorizedFeeds = store.feeds.filter((f) => !f.category);
    counts['uncategorized'] = uncategorizedFeeds.reduce((sum, feed) => {
      return sum + (store.unreadCounts.feedCounts[feed.id] || 0);
    }, 0);

    return counts;
  });

  // Auto-expand new categories only if no saved state exists
  watch(
    () => tree.value.categories,
    (newCategories) => {
      if (newCategories) {
        const hasSavedState = localStorage.getItem('openCategories') !== null;
        newCategories.forEach((cat) => {
          // Always auto-expand 'uncategorized' category
          if (cat === 'uncategorized' && !openCategories.value.has(cat)) {
            openCategories.value.add(cat);
            return;
          }
          // Only auto-expand if this is a new category and no saved state exists
          if (!openCategories.value.has(cat) && !hasSavedState) {
            openCategories.value.add(cat);
          }
        });

        // Also auto-expand parent categories for multi-level
        // For example, if "Tech/Blogs" exists, also expand "Tech"
        const parentCategories = new Set<string>();
        newCategories.forEach((cat) => {
          const parts = cat.split('/');
          for (let i = 1; i < parts.length; i++) {
            const parentPath = parts.slice(0, i).join('/');
            parentCategories.add(parentPath);
          }
        });

        parentCategories.forEach((parentCat) => {
          if (!openCategories.value.has(parentCat) && !hasSavedState) {
            openCategories.value.add(parentCat);
          }
        });
      }
    },
    { immediate: true }
  );

  function toggleCategory(path: string): void {
    if (openCategories.value.has(path)) {
      openCategories.value.delete(path);
    } else {
      openCategories.value.add(path);
    }
    // Save to localStorage
    localStorage.setItem('openCategories', JSON.stringify([...openCategories.value]));
  }

  function isCategoryOpen(path: string): boolean {
    return openCategories.value.has(path);
  }

  function togglePinCategory(path: string): void {
    if (pinnedCategories.value.has(path)) {
      pinnedCategories.value.delete(path);
    } else {
      pinnedCategories.value.add(path);
    }
    // Save to localStorage
    localStorage.setItem('pinnedCategories', JSON.stringify([...pinnedCategories.value]));
  }

  function isCategoryPinned(path: string): boolean {
    return pinnedCategories.value.has(path);
  }

  // Feed actions
  async function handleFeedAction(action: string, feed: Feed): Promise<void> {
    if (action === 'markAllRead') {
      await store.markAllAsRead(feed.id);
      window.showToast(t('markedAllAsRead'), 'success');
    } else if (action === 'refreshFeed') {
      await fetch(`/api/feeds/refresh?id=${feed.id}`, { method: 'POST' });
      window.showToast(t('feedRefreshStarted'), 'success');
      // Start polling for progress as the backend is now fetching articles for this feed
      store.pollProgress();
    } else if (action === 'syncFeed') {
      // Sync individual FreshRSS feed
      await fetch(`/api/freshrss/sync-feed?stream_id=${feed.freshrss_stream_id}`, {
        method: 'POST',
      });
      window.showToast(t('syncFeedStarted'), 'success');
      // Start polling for progress
      store.pollProgress();
    } else if (action === 'delete') {
      const confirmed = await window.showConfirm({
        title: t('unsubscribeTitle'),
        message: t('unsubscribeMessage', { name: feed.title }),
        confirmText: t('unsubscribe'),
        cancelText: t('cancel'),
        isDanger: true,
      });
      if (confirmed) {
        await fetch(`/api/feeds/delete?id=${feed.id}`, { method: 'POST' });
        store.fetchFeeds();
        window.showToast(t('unsubscribedSuccess'), 'success');
      }
    } else if (action === 'edit') {
      window.dispatchEvent(new CustomEvent('show-edit-feed', { detail: feed }));
    } else if (action === 'openWebsite') {
      // Handle RSSHub URLs - need to transform rsshub:// to full URL
      let urlToOpen = feed.website_url || feed.url;
      if (urlToOpen.startsWith('rsshub://')) {
        try {
          const response = await fetch('/api/rsshub/transform-url', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({ url: urlToOpen }),
          });
          if (response.ok) {
            const data = await response.json();
            if (data.url && (data.url.startsWith('http://') || data.url.startsWith('https://'))) {
              urlToOpen = data.url;
            } else {
              // Invalid transformed URL
              window.showToast(
                t('failedToTransformRSSHubURL') || 'Failed to transform RSSHub URL',
                'error'
              );
              return;
            }
          } else {
            // Transformation failed - try to get error message from response
            let errorMessage = t('failedToTransformRSSHubURL') || 'Failed to transform RSSHub URL';
            try {
              const errorText = await response.text();
              if (errorText) {
                errorMessage = errorText;
              }
            } catch (e) {
              // Ignore error reading response
            }
            window.showToast(errorMessage, 'error');
            return;
          }
        } catch (error) {
          window.showToast(
            t('failedToTransformRSSHubURL') || 'Failed to transform RSSHub URL',
            'error'
          );
          return;
        }
      }
      // Only open if URL is http/https (not rsshub://)
      if (urlToOpen.startsWith('http://') || urlToOpen.startsWith('https://')) {
        openInBrowser(urlToOpen);
      } else {
        window.showToast(t('invalidURLScheme') || 'Invalid URL scheme', 'error');
      }
    } else if (action === 'discover') {
      window.dispatchEvent(new CustomEvent('show-discover-blogs', { detail: feed }));
    }
  }

  function onFeedContextMenu(e: MouseEvent, feed: Feed): void {
    e.preventDefault();
    e.stopPropagation();

    // Build menu items dynamically based on whether this is a FreshRSS feed
    const items: Array<{
      label?: string;
      action?: string;
      icon?: string;
      separator?: boolean;
      danger?: boolean;
    }> = [];

    // For FreshRSS feeds, show "Sync Feed" instead of "Refresh Feed"
    if (feed.is_freshrss_source) {
      items.push({ label: t('syncFeed'), action: 'syncFeed', icon: 'PhArrowsClockwise' });
    } else {
      items.push({ label: t('refreshFeed'), action: 'refreshFeed', icon: 'PhArrowsClockwise' });
    }

    items.push({ label: t('markAllAsReadFeed'), action: 'markAllRead', icon: 'PhCheckCircle' });
    items.push({ separator: true });
    items.push({ label: t('openWebsite'), action: 'openWebsite', icon: 'PhGlobe' });

    // Only add discover for non-FreshRSS feeds
    if (!feed.is_freshrss_source) {
      items.push({ label: t('discoverFeeds'), action: 'discover', icon: 'PhBinoculars' });
    }

    // Only add edit and delete options for non-FreshRSS feeds
    if (!feed.is_freshrss_source) {
      items.push({ separator: true });
      items.push({ label: t('editSubscription'), action: 'edit', icon: 'PhPencil' });
      items.push({ label: t('unsubscribe'), action: 'delete', icon: 'PhTrash', danger: true });
    }

    window.dispatchEvent(
      new CustomEvent('open-context-menu', {
        detail: {
          x: e.clientX,
          y: e.clientY,
          items,
          data: feed,
          callback: handleFeedAction,
        },
      })
    );
  }

  // Category actions
  async function handleCategoryAction(action: string, categoryName: string): Promise<void> {
    if (action === 'markAllRead') {
      // Use the category parameter for the API call
      const category = categoryName === 'uncategorized' ? '' : categoryName;
      await fetch(`/api/articles/mark-all-read?category=${encodeURIComponent(category)}`, {
        method: 'POST',
      });
      store.fetchUnreadCounts();
      window.showToast(t('markedAllAsRead'), 'success');
    } else if (action === 'togglePin') {
      togglePinCategory(categoryName);
      window.showToast(
        isCategoryPinned(categoryName) ? t('categoryPinned') : t('categoryUnpinned'),
        'success'
      );
    } else if (action === 'rename') {
      const newName = await window.showInput({
        title: t('renameCategory'),
        message: t('enterCategoryName'),
        defaultValue: categoryName,
        confirmText: t('confirm'),
        cancelText: t('cancel'),
      });
      if (newName && newName !== categoryName) {
        const feedsToUpdate = store.feeds.filter(
          (f) => f.category === categoryName || f.category.startsWith(categoryName + '/')
        );

        const promises = feedsToUpdate.map((feed) => {
          let newCategory = feed.category;
          if (feed.category === categoryName) {
            newCategory = newName;
          } else if (feed.category.startsWith(categoryName + '/')) {
            newCategory = newName + feed.category.substring(categoryName.length);
          }

          return fetch('/api/feeds/update', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              id: feed.id,
              title: feed.title,
              url: feed.url,
              category: newCategory,
            }),
          });
        });

        await Promise.all(promises);
        store.fetchFeeds();
      }
    }
  }

  function onCategoryContextMenu(e: MouseEvent, categoryName: string): void {
    e.preventDefault();
    e.stopPropagation();

    const isPinned = isCategoryPinned(categoryName);

    const items: Array<{ label?: string; action?: string; icon?: string; separator?: boolean }> = [
      { label: t('markAllAsReadFeed'), action: 'markAllRead', icon: 'ph-check-circle' },
    ];

    if (categoryName !== 'uncategorized') {
      items.push({ separator: true });
      items.push({
        label: isPinned ? t('unpinCategory') : t('pinCategory'),
        action: 'togglePin',
        icon: isPinned ? 'ph-push-pin-slash' : 'ph-push-pin',
      });
      items.push({ separator: true });
      items.push({ label: t('renameCategory'), action: 'rename', icon: 'ph-pencil' });
    }

    window.dispatchEvent(
      new CustomEvent('open-context-menu', {
        detail: {
          x: e.clientX,
          y: e.clientY,
          items: items,
          data: categoryName,
          callback: handleCategoryAction,
        },
      })
    );
  }

  return {
    tree,
    categoryUnreadCounts,
    openCategories,
    pinnedCategories,
    searchQuery,
    toggleCategory,
    isCategoryOpen,
    togglePinCategory,
    isCategoryPinned,
    onFeedContextMenu,
    onCategoryContextMenu,
  };
}
