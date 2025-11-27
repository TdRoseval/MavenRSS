import { computed, ref, watch, type Ref } from 'vue';
import { useAppStore } from '@/stores/app';
import { useI18n } from 'vue-i18n';
import { BrowserOpenURL } from '@/wailsjs/wailsjs/runtime/runtime';
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

  const openCategories: Ref<Set<string>> = ref(new Set());
  const searchQuery: Ref<string> = ref('');

  // Build category tree with search filtering
  const tree = computed<TreeData>(() => {
    const t: Record<string, TreeNode> = {};
    const uncategorized: Feed[] = [];
    const categories = new Set<string>();

    if (!store.feeds) return { tree: {}, uncategorized: [], categories };

    const query = searchQuery.value.toLowerCase().trim();

    store.feeds.forEach((feed: Feed) => {
      const matchesSearch =
        query === '' ||
        feed.title.toLowerCase().includes(query) ||
        feed.url.toLowerCase().includes(query);

      if (!matchesSearch) return;

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

  // Auto-expand all categories by default
  watch(
    () => tree.value.categories,
    (newCategories) => {
      if (newCategories) {
        newCategories.forEach((cat) => {
          if (!openCategories.value.has(cat)) {
            openCategories.value.add(cat);
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
  }

  function isCategoryOpen(path: string): boolean {
    return openCategories.value.has(path);
  }

  // Feed actions
  async function handleFeedAction(action: string, feed: Feed): Promise<void> {
    if (action === 'markAllRead') {
      await store.markAllAsRead(feed.id);
      window.showToast(t('markedAllAsRead'), 'success');
    } else if (action === 'refreshFeed') {
      await fetch(`/api/feeds/refresh?id=${feed.id}`, { method: 'POST' });
      window.showToast(t('feedRefreshStarted'), 'success');
      // Refresh articles after a short delay to show new content
      setTimeout(() => {
        store.fetchArticles();
        store.fetchUnreadCounts();
      }, 2000);
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
      const urlToOpen = feed.website_url || feed.url;
      BrowserOpenURL(urlToOpen);
    } else if (action === 'discover') {
      window.dispatchEvent(new CustomEvent('show-discover-blogs', { detail: feed }));
    }
  }

  function onFeedContextMenu(e: MouseEvent, feed: Feed): void {
    e.preventDefault();
    e.stopPropagation();
    window.dispatchEvent(
      new CustomEvent('open-context-menu', {
        detail: {
          x: e.clientX,
          y: e.clientY,
          items: [
            { label: t('refreshFeed'), action: 'refreshFeed', icon: 'PhArrowsClockwise' },
            { label: t('markAllAsReadFeed'), action: 'markAllRead', icon: 'ph-check-circle' },
            { separator: true },
            { label: t('openWebsite'), action: 'openWebsite', icon: 'ph-globe' },
            { label: t('discoverFeeds'), action: 'discover', icon: 'PhMagnifyingGlass' },
            { separator: true },
            { label: t('editSubscription'), action: 'edit', icon: 'ph-pencil' },
            { label: t('unsubscribe'), action: 'delete', icon: 'ph-trash', danger: true },
          ],
          data: feed,
          callback: handleFeedAction,
        },
      })
    );
  }

  // Category actions
  async function handleCategoryAction(action: string, categoryName: string): Promise<void> {
    if (action === 'markAllRead') {
      let feedsInCategory;
      if (categoryName === 'uncategorized') {
        feedsInCategory = store.feeds.filter((f) => !f.category);
      } else {
        feedsInCategory = store.feeds.filter(
          (f) => f.category === categoryName || f.category.startsWith(categoryName + '/')
        );
      }

      const promises = feedsInCategory.map((feed) => store.markAllAsRead(feed.id));
      await Promise.all(promises);
      window.showToast(t('markedAllAsRead'), 'success');
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

    const items: Array<{ label?: string; action?: string; icon?: string; separator?: boolean }> = [
      { label: t('markAllAsReadFeed'), action: 'markAllRead', icon: 'ph-check-circle' },
    ];

    if (categoryName !== 'uncategorized') {
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
    searchQuery,
    toggleCategory,
    isCategoryOpen,
    onFeedContextMenu,
    onCategoryContextMenu,
  };
}
