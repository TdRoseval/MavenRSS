/**
 * Composable for feed management operations in settings
 */
import { useI18n } from 'vue-i18n';
import { useAppStore } from '@/stores/app';
import type { Feed } from '@/types/models';

export function useFeedManagement() {
  const { t } = useI18n();
  const store = useAppStore();

  /**
   * Import OPML file using dialog
   */
  async function handleImportOPML() {
    try {
      console.log('Starting OPML import dialog...');
      const response = await fetch('/api/opml/import-dialog', {
        method: 'POST',
      });

      if (!response.ok) {
        // Handle HTTP error responses
        const errorData = await response.json().catch(() => ({ error: 'Unknown error' }));
        throw new Error(errorData.error || 'Import failed');
      }

      const result = await response.json();

      if (result.status === 'cancelled') {
        console.log('OPML import cancelled by user');
        return;
      }

      if (result.status === 'success') {
        console.log('OPML import successful:', result);
        window.showToast(t('opmlImportedSuccess', { count: result.feedCount }), 'success');
        store.fetchFeeds();
        // Start polling for progress as the backend is now fetching articles for imported feeds
        store.pollProgress();
      } else {
        console.error('OPML import failed:', result);
        window.showToast(t('importFailed', { error: 'Unknown error' }), 'error');
      }
    } catch (error) {
      console.error('OPML import network error:', error);
      window.showToast(t('importFailed', { error: (error as Error).message }), 'error');
    }
  }

  /**
   * Export OPML file using dialog
   */
  async function handleExportOPML() {
    try {
      console.log('Starting OPML export dialog...');
      const response = await fetch('/api/opml/export-dialog', {
        method: 'POST',
      });

      if (!response.ok) {
        // Handle HTTP error responses
        const errorData = await response.json().catch(() => ({ error: 'Unknown error' }));
        throw new Error(errorData.error || 'Export failed');
      }

      const result = await response.json();

      if (result.status === 'cancelled') {
        console.log('OPML export cancelled by user');
        return;
      }

      if (result.status === 'success') {
        console.log('OPML export successful:', result);
        window.showToast(t('opmlExportedSuccess'), 'success');
      } else {
        console.error('OPML export failed:', result);
        window.showToast(t('exportFailed', { error: 'Unknown error' }), 'error');
      }
    } catch (error) {
      console.error('OPML export network error:', error);
      window.showToast(t('exportFailed', { error: (error as Error).message }), 'error');
    }
  }

  /**
   * Clean up old articles from database
   */
  async function handleCleanupDatabase() {
    const confirmed = await window.showConfirm({
      title: t('cleanDatabaseTitle'),
      message: t('cleanDatabaseMessage'),
      confirmText: t('clean'),
      cancelText: t('cancel'),
      isDanger: true,
    });
    if (!confirmed) return;

    try {
      const res = await fetch('/api/articles/cleanup', { method: 'POST' });
      if (res.ok) {
        const result = await res.json();
        window.showToast(t('databaseCleanedSuccess', { count: result.deleted }), 'success');
        store.fetchArticles();
      } else {
        window.showToast(t('errorCleaningDatabase'), 'error');
      }
    } catch (e) {
      console.error('Error cleaning database:', e);
      window.showToast(t('errorCleaningDatabase'), 'error');
    }
  }

  /**
   * Add new feed
   */
  function handleAddFeed() {
    window.dispatchEvent(new CustomEvent('show-add-feed'));
  }

  /**
   * Edit existing feed
   */
  function handleEditFeed(feed: Feed) {
    window.dispatchEvent(new CustomEvent('show-edit-feed', { detail: feed }));
  }

  /**
   * Delete a single feed
   */
  async function handleDeleteFeed(id: number) {
    const confirmed = await window.showConfirm({
      title: t('deleteFeedTitle'),
      message: t('deleteFeedMessage'),
      confirmText: t('delete'),
      cancelText: t('cancel'),
      isDanger: true,
    });
    if (!confirmed) return;

    await fetch(`/api/feeds/delete?id=${id}`, { method: 'POST' });
    store.fetchFeeds();
    window.showToast(t('feedDeletedSuccess'), 'success');
  }

  /**
   * Delete multiple feeds
   */
  async function handleBatchDelete(selectedIds: number[]) {
    const confirmed = await window.showConfirm({
      title: t('deleteMultipleFeedsTitle'),
      message: t('deleteMultipleFeedsMessage', { count: selectedIds.length }),
      confirmText: t('delete'),
      cancelText: t('cancel'),
      isDanger: true,
    });
    if (!confirmed) return;

    const promises = selectedIds.map((id: number) =>
      fetch(`/api/feeds/delete?id=${id}`, { method: 'POST' })
    );
    await Promise.all(promises);
    store.fetchFeeds();
    window.showToast(t('feedsDeletedSuccess'), 'success');
  }

  /**
   * Move multiple feeds to a new category
   */
  async function handleBatchMove(selectedIds: number[]) {
    if (!store.feeds) return;

    const newCategory = await window.showInput({
      title: t('moveFeeds'),
      message: t('enterCategoryName'),
      placeholder: t('categoryPlaceholder'),
      confirmText: t('move'),
      cancelText: t('cancel'),
    });
    if (newCategory === null) return;

    const promises = selectedIds.map((id: number) => {
      const feed = store.feeds.find((f) => f.id === id);
      if (!feed) return Promise.resolve();
      return fetch('/api/feeds/update', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: feed.id,
          title: feed.title,
          url: feed.url,
          category: newCategory,
          is_image_mode: feed.is_image_mode,
          website_url: feed.website_url,
          image_url: feed.image_url,
          script_path: feed.script_path,
          hide_from_timeline: feed.hide_from_timeline,
          proxy_url: feed.proxy_url,
          proxy_enabled: feed.proxy_enabled,
          refresh_interval: feed.refresh_interval,
          type: feed.type,
          xpath_item: feed.xpath_item,
          xpath_item_title: feed.xpath_item_title,
          xpath_item_content: feed.xpath_item_content,
          xpath_item_uri: feed.xpath_item_uri,
          xpath_item_author: feed.xpath_item_author,
          xpath_item_timestamp: feed.xpath_item_timestamp,
          xpath_item_time_format: feed.xpath_item_time_format,
          xpath_item_thumbnail: feed.xpath_item_thumbnail,
          xpath_item_categories: feed.xpath_item_categories,
          xpath_item_uid: feed.xpath_item_uid,
          article_view_mode: feed.article_view_mode,
          auto_expand_content: feed.auto_expand_content,
        }),
      });
    });

    await Promise.all(promises);
    store.fetchFeeds();
    window.showToast(t('feedsMovedSuccess'), 'success');
  }

  return {
    handleImportOPML,
    handleExportOPML,
    handleCleanupDatabase,
    handleAddFeed,
    handleEditFeed,
    handleDeleteFeed,
    handleBatchDelete,
    handleBatchMove,
  };
}
