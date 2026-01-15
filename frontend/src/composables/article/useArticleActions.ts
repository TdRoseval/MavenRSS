import { openInBrowser } from '@/utils/browser';
import { copyArticleLink, copyArticleTitle } from '@/utils/clipboard';
import { useAppStore } from '@/stores/app';
import type { Article } from '@/types/models';
import type { Composer } from 'vue-i18n';

export function useArticleActions(
  t: Composer['t'],
  defaultViewMode: { value: 'original' | 'rendered' },
  onReadStatusChange?: () => void
) {
  const store = useAppStore();
  // Show context menu for article
  function showArticleContextMenu(e: MouseEvent, article: Article): void {
    e.preventDefault();
    e.stopPropagation();

    // Determine context menu text based on default view mode
    const contentActionLabel =
      defaultViewMode.value === 'rendered' ? t('showOriginal') : t('renderContent');
    const contentActionIcon = defaultViewMode.value === 'rendered' ? 'ph-globe' : 'ph-article';

    window.dispatchEvent(
      new CustomEvent('open-context-menu', {
        detail: {
          x: e.clientX,
          y: e.clientY,
          items: [
            {
              label: article.is_read ? t('markAsUnread') : t('markAsRead'),
              action: 'toggleRead',
              icon: article.is_read ? 'ph-envelope' : 'ph-envelope-open',
            },
            {
              label: article.is_favorite ? t('removeFromFavorites') : t('addToFavorites'),
              action: 'toggleFavorite',
              icon: 'ph-star',
              iconWeight: article.is_favorite ? 'fill' : 'regular',
              iconColor: article.is_favorite ? 'text-yellow-500' : '',
            },
            {
              label: article.is_read_later ? t('removeFromReadLater') : t('addToReadLater'),
              action: 'toggleReadLater',
              icon: 'ph-clock-countdown',
              iconWeight: article.is_read_later ? 'fill' : 'regular',
              iconColor: article.is_read_later ? 'text-blue-500' : '',
            },
            { separator: true },
            {
              label: contentActionLabel,
              action: 'renderContent',
              icon: contentActionIcon,
            },
            {
              label: article.is_hidden ? t('unhideArticle') : t('hideArticle'),
              action: 'toggleHide',
              icon: article.is_hidden ? 'ph-eye' : 'ph-eye-slash',
              danger: !article.is_hidden,
            },
            { separator: true },
            {
              label: t('copyLink'),
              action: 'copyLink',
              icon: 'ph-link',
            },
            {
              label: t('copyTitle'),
              action: 'copyTitle',
              icon: 'ph-text-t',
            },
            { separator: true },
            {
              label: t('openInBrowser'),
              action: 'openBrowser',
              icon: 'ph-arrow-square-out',
            },
          ],
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
        await fetch(`/api/articles/read?id=${article.id}&read=${newState}`, {
          method: 'POST',
        });
        // Update unread counts after toggling read status
        if (onReadStatusChange) {
          onReadStatusChange();
        }
      } catch (e) {
        console.error('Error toggling read status:', e);
        // Revert the state change on error
        article.is_read = !newState;
        window.showToast(t('errorSavingSettings'), 'error');
      }
    } else if (action === 'toggleFavorite') {
      const newState = !article.is_favorite;
      article.is_favorite = newState;
      try {
        await fetch(`/api/articles/favorite?id=${article.id}`, { method: 'POST' });
        // Update filter counts after toggling favorite status
        if (onReadStatusChange) {
          onReadStatusChange();
        }
      } catch (e) {
        console.error('Error toggling favorite:', e);
        // Revert the state change on error
        article.is_favorite = !newState;
        window.showToast(t('errorSavingSettings'), 'error');
      }
    } else if (action === 'toggleReadLater') {
      const newState = !article.is_read_later;
      article.is_read_later = newState;
      // When adding to read later, also mark as unread
      if (newState) {
        article.is_read = false;
      }
      try {
        await fetch(`/api/articles/toggle-read-later?id=${article.id}`, { method: 'POST' });
        // Update unread counts after toggling read later status
        if (onReadStatusChange) {
          onReadStatusChange();
        }
      } catch (e) {
        console.error('Error toggling read later:', e);
        // Revert the state change on error
        article.is_read_later = !newState;
        window.showToast(t('errorSavingSettings'), 'error');
      }
    } else if (action === 'toggleHide') {
      try {
        await fetch(`/api/articles/toggle-hide?id=${article.id}`, { method: 'POST' });
        // Dispatch event to refresh article list
        window.dispatchEvent(new CustomEvent('refresh-articles'));
      } catch (e) {
        console.error('Error toggling hide:', e);
        window.showToast(t('errorSavingSettings'), 'error');
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
          await fetch(`/api/articles/read?id=${article.id}&read=true`, {
            method: 'POST',
          });
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
    } else if (action === 'copyLink') {
      const success = await copyArticleLink(article.url);
      if (success) {
        window.showToast(t('copiedToClipboard'), 'success');
      } else {
        window.showToast(t('failedToCopy'), 'error');
      }
    } else if (action === 'copyTitle') {
      const success = await copyArticleTitle(article.title);
      if (success) {
        window.showToast(t('copiedToClipboard'), 'success');
      } else {
        window.showToast(t('failedToCopy'), 'error');
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
