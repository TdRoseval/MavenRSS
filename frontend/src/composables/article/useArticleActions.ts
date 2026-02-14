import { openInBrowser } from '@/utils/browser';
import { copyArticleLink, copyArticleTitle } from '@/utils/clipboard';
import { useAppStore } from '@/stores/app';
import { apiClient } from '@/utils/apiClient';
import type { Article } from '@/types/models';

type ViewMode = 'original' | 'rendered' | 'external';

interface MenuItem {
  label?: string;
  action?: string;
  icon?: string;
  iconWeight?: string;
  iconColor?: string;
  separator?: boolean;
  danger?: boolean;
}

export function useArticleActions(
  t: (key: string, params?: Record<string, any>) => string,
  defaultViewMode: { value: ViewMode },
  onReadStatusChange?: () => void
) {
  const store = useAppStore();

  // Get effective view mode for an article based on feed settings and global settings
  function getEffectiveViewMode(article: Article): ViewMode {
    const feed = store.feeds.find((f: any) => f.id === article.feed_id);
    if (!feed) return defaultViewMode.value;

    if (feed.article_view_mode === 'webpage') {
      return 'original';
    } else if (feed.article_view_mode === 'rendered') {
      return 'rendered';
    } else if (feed.article_view_mode === 'external') {
      return 'external';
    } else {
      // 'global' or undefined - use global setting
      return defaultViewMode.value;
    }
  }
  // Show context menu for article
  function showArticleContextMenu(e: MouseEvent, article: Article): void {
    e.preventDefault();
    e.stopPropagation();

    // Get effective view mode for this article
    const effectiveMode = getEffectiveViewMode(article);

    // Build menu items array
    const menuItems: MenuItem[] = [
      {
        label: article.is_read ? t('article.action.markAsUnread') : t('article.action.markAsRead'),
        action: 'toggleRead',
        icon: article.is_read ? 'ph-envelope' : 'ph-envelope-open',
      },
      {
        label: t('article.action.markAboveAsRead'),
        action: 'markAboveAsRead',
        icon: 'ph-arrow-bend-right-up',
      },
      {
        label: t('article.action.markBelowAsRead'),
        action: 'markBelowAsRead',
        icon: 'ph-arrow-bend-left-down',
      },
      {
        label: article.is_favorite
          ? t('article.action.removeFromFavorites')
          : t('article.action.addToFavorite'),
        action: 'toggleFavorite',
        icon: 'ph-star',
        iconWeight: article.is_favorite ? 'fill' : 'regular',
        iconColor: article.is_favorite ? 'text-yellow-500' : '',
      },
      {
        label: article.is_read_later
          ? t('article.action.removeFromReadLater')
          : t('article.action.addToReadLater'),
        action: 'toggleReadLater',
        icon: 'ph-clock-countdown',
        iconWeight: article.is_read_later ? 'fill' : 'regular',
        iconColor: article.is_read_later ? 'text-blue-500' : '',
      },
      { separator: true },
    ];

    // Add view mode specific menu items
    if (effectiveMode === 'external') {
      // When mode is external, show both "View Original" and "Render Content" options
      menuItems.push({
        label: t('article.action.viewModeOriginal'),
        action: 'viewInAppOriginal',
        icon: 'ph-globe',
      });
      menuItems.push({
        label: t('article.action.viewModeRendered'),
        action: 'viewInAppRendered',
        icon: 'ph-article',
      });
    } else if (effectiveMode === 'rendered') {
      // When mode is rendered, show "View Original" option
      menuItems.push({
        label: t('setting.reading.showOriginal'),
        action: 'renderContent',
        icon: 'ph-globe',
      });
    } else {
      // When mode is original, show "Render Content" option
      menuItems.push({
        label: t('article.content.renderContent'),
        action: 'renderContent',
        icon: 'ph-article',
      });
    }

    // Add remaining menu items
    menuItems.push(
      { separator: true },
      {
        label: article.is_hidden
          ? t('article.action.unhideArticle')
          : t('article.action.hideArticle'),
        action: 'toggleHide',
        icon: article.is_hidden ? 'ph-eye' : 'ph-eye-slash',
        danger: !article.is_hidden,
      },
      { separator: true },
      {
        label: t('common.contextMenu.copyLink'),
        action: 'copyLink',
        icon: 'ph-link',
      },
      {
        label: t('common.contextMenu.copyTitle'),
        action: 'copyTitle',
        icon: 'ph-text-t',
      }
    );

    // Only add "Open in Browser" option if not in external mode
    if (effectiveMode !== 'external') {
      menuItems.push(
        { separator: true },
        {
          label: t('article.action.openInBrowser'),
          action: 'openBrowser',
          icon: 'ph-arrow-square-out',
        }
      );
    }

    window.dispatchEvent(
      new CustomEvent('open-context-menu', {
        detail: {
          x: e.clientX,
          y: e.clientY,
          items: menuItems,
          data: article,
          callback: (action: string, article: Article) =>
            handleArticleAction(action, article, onReadStatusChange),
        },
      })
    );
  }

  // Handle article actions
  async function handleArticleAction(
    action: string,
    article: Article,
    onReadStatusChange?: () => void
  ): Promise<void> {
    if (action === 'toggleRead') {
      const newState = !article.is_read;
      article.is_read = newState;
      try {
        await apiClient.post('/articles/read', { id: article.id, read: newState });
        // Update unread counts after toggling read status
        if (onReadStatusChange) {
          onReadStatusChange();
        }
      } catch (e) {
        console.error('Error toggling read status:', e);
        // Revert the state change on error
        article.is_read = !newState;
        window.showToast(t('common.errors.savingSettings'), 'error');
      }
    } else if (action === 'markAboveAsRead' || action === 'markBelowAsRead') {
      try {
        const direction = action === 'markAboveAsRead' ? 'above' : 'below';

        // Show confirmation dialog
        const confirmTitle =
          action === 'markAboveAsRead'
            ? t('article.action.markAboveReadConfirmTitle')
            : t('article.action.markBelowReadConfirmTitle');
        const confirmMessage =
          action === 'markAboveAsRead'
            ? t('article.action.markAboveReadConfirmMessage')
            : t('article.action.markBelowReadConfirmMessage');

        const confirmed = await window.showConfirm({
          title: confirmTitle,
          message: confirmMessage,
          confirmText: t('common.confirm'),
          cancelText: t('common.cancel'),
          isDanger: false,
        });

        if (!confirmed) {
          return;
        }

        // Build query parameters
        const params: Record<string, any> = {
          id: article.id,
          direction: direction,
        };

        // Add feed_id or category if we're in a filtered view
        if (store.currentFeedId) {
          params.feed_id = store.currentFeedId;
        } else if (store.currentCategory) {
          params.category = store.currentCategory;
        }

        const data: any = await apiClient.post('/articles/mark-relative', {}, params);

        // Refresh the article list to show updated read status
        if (onReadStatusChange) {
          onReadStatusChange();
        }

        // Refresh articles from server
        await store.fetchArticles();

        window.showToast(
          t('article.action.markedNArticlesAsRead', { count: data.count || 0 }),
          'success'
        );
      } catch (e) {
        console.error('Error marking articles as read:', e);
        window.showToast(t('common.errors.savingSettings'), 'error');
      }
    } else if (action === 'toggleFavorite') {
      const newState = !article.is_favorite;
      article.is_favorite = newState;
      try {
        await apiClient.post('/articles/favorite', { id: article.id });
        // Update filter counts after toggling favorite status
        if (onReadStatusChange) {
          onReadStatusChange();
        }
      } catch (e) {
        console.error('Error toggling favorite:', e);
        // Revert the state change on error
        article.is_favorite = !newState;
        window.showToast(t('common.errors.savingSettings'), 'error');
      }
    } else if (action === 'toggleReadLater') {
      const newState = !article.is_read_later;
      article.is_read_later = newState;
      // When adding to read later, also mark as unread
      if (newState) {
        article.is_read = false;
      }
      try {
        await apiClient.post('/articles/toggle-read-later', { id: article.id });
        // Update unread counts after toggling read later status
        if (onReadStatusChange) {
          onReadStatusChange();
        }
      } catch (e) {
        console.error('Error toggling read later:', e);
        // Revert the state change on error
        article.is_read_later = !newState;
        window.showToast(t('common.errors.savingSettings'), 'error');
      }
    } else if (action === 'toggleHide') {
      try {
        await apiClient.post('/articles/toggle-hide', { id: article.id });
        // Dispatch event to refresh article list
        window.dispatchEvent(new CustomEvent('refresh-articles'));
      } catch (e) {
        console.error('Error toggling hide:', e);
        window.showToast(t('common.errors.savingSettings'), 'error');
      }
    } else if (action === 'renderContent') {
      // Determine the action based on default view mode
      const renderAction = defaultViewMode.value === 'rendered' ? 'showOriginal' : 'showContent';

      // Select the article first
      store.currentArticleId = article.id;

      // Dispatch explicit action event
      window.dispatchEvent(
        new CustomEvent('explicit-render-action', {
          detail: { action: renderAction },
        })
      );

      // Mark as read
      if (!article.is_read) {
        article.is_read = true;
        try {
          await apiClient.post('/articles/read', { id: article.id, read: true });
          if (onReadStatusChange) {
            onReadStatusChange();
          }
        } catch (e) {
          console.error('Error marking as read:', e);
        }
      }

      // Trigger the render action
      window.dispatchEvent(
        new CustomEvent('render-article-content', {
          detail: { action: renderAction },
        })
      );
    } else if (action === 'viewInAppOriginal') {
      // View article in app as original (webpage) - override external mode
      store.currentArticleId = article.id;

      // Dispatch explicit action to show original
      window.dispatchEvent(
        new CustomEvent('explicit-render-action', {
          detail: { action: 'showOriginal' },
        })
      );

      // Mark as read
      if (!article.is_read) {
        article.is_read = true;
        try {
          await apiClient.post('/articles/read', { id: article.id, read: true });
          if (onReadStatusChange) {
            onReadStatusChange();
          }
        } catch (e) {
          console.error('Error marking as read:', e);
        }
      }

      // Trigger the render action
      window.dispatchEvent(
        new CustomEvent('render-article-content', {
          detail: { action: 'showOriginal' },
        })
      );
    } else if (action === 'viewInAppRendered') {
      // View article in app as rendered content - override external mode
      store.currentArticleId = article.id;

      // Dispatch explicit action to show rendered content
      window.dispatchEvent(
        new CustomEvent('explicit-render-action', {
          detail: { action: 'showContent' },
        })
      );

      // Mark as read
      if (!article.is_read) {
        article.is_read = true;
        try {
          await apiClient.post('/articles/read', { id: article.id, read: true });
          if (onReadStatusChange) {
            onReadStatusChange();
          }
        } catch (e) {
          console.error('Error marking as read:', e);
        }
      }

      // Trigger the render action
      window.dispatchEvent(
        new CustomEvent('render-article-content', {
          detail: { action: 'showContent' },
        })
      );
    } else if (action === 'copyLink') {
      const success = await copyArticleLink(article.url);
      if (success) {
        window.showToast(t('common.toast.copiedToClipboard'), 'success');
      } else {
        window.showToast(t('common.errors.failedToCopy'), 'error');
      }
    } else if (action === 'copyTitle') {
      const success = await copyArticleTitle(article.title);
      if (success) {
        window.showToast(t('common.toast.copiedToClipboard'), 'success');
      } else {
        window.showToast(t('common.errors.failedToCopy'), 'error');
      }
    } else if (action === 'openBrowser') {
      openInBrowser(article.url);
    }
  }

  return {
    showArticleContextMenu,
    handleArticleAction,
  };
}
