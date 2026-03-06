import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import type { Feed, Tag, RefreshProgress } from '@/types/models';
import { apiClient, stopRefreshFeeds } from '@/shared/lib/apiClient';
import { useSettings } from '@/composables/core/useSettings';
import { useArticleStore } from '@/features/article/store';

export const useFeedStore = defineStore('feed', () => {
  const { settings: settingsRef } = useSettings();
  
  // State
  const feeds = ref<Feed[]>([]);
  const tags = ref<Tag[]>([]);
  const refreshProgress = ref<RefreshProgress>({ isRunning: false });

  // Getters
  const feedMap = computed(() => {
    const map = new Map<number, Feed>();
    feeds.value.forEach((feed) => map.set(feed.id, feed));
    return map;
  });

  const tagMap = computed(() => {
    const map = new Map<number, Tag>();
    tags.value.forEach((tag) => map.set(tag.id, tag));
    return map;
  });

  // Actions
  async function fetchFeeds(): Promise<void> {
    try {
      const data = await apiClient.get<Feed[]>('/feeds');
      feeds.value = data || [];

      // Fetch tags after fetching feeds
      await fetchTags();
      
      // Update article store counts
      const articleStore = useArticleStore();
      await articleStore.fetchUnreadCounts();
      await articleStore.fetchFilterCounts();
    } catch (e) {
      console.error('[Feed Store] Fetch feeds error:', e);
      feeds.value = [];
    }
  }

  async function fetchTags(): Promise<void> {
    try {
      const data = await apiClient.get<Tag[]>('/tags');
      tags.value = data || [];
    } catch (e) {
      console.error('[Feed Store] Fetch tags error:', e);
      tags.value = [];
    }
  }

  // Auto Refresh
  let pollProgressInterval: ReturnType<typeof setInterval> | null = null;

  async function refreshFeeds(): Promise<void> {
    refreshProgress.value.isRunning = true;
    try {
      // First, trigger standard refresh
      await apiClient.post('/refresh');

      // Also trigger FreshRSS sync if enabled
      if (settingsRef.value.freshrss_enabled === true) {
        try {
          await apiClient.post('/freshrss/sync');
        } catch (e) {
          // If FreshRSS sync fails, it's okay - just log it
        }
      }

      // Wait a moment to check if refresh is actually running
      await new Promise((resolve) => setTimeout(resolve, 200));

      // Check progress to see if there are actually any tasks
      const progressData: any = await apiClient.get('/progress');

      // If no tasks are running, mark as completed immediately
      if (!progressData.is_running) {
        refreshProgress.value.isRunning = false;

        // Still refresh feeds and articles to get any updates from FreshRSS sync
        await fetchFeeds();
        
        const articleStore = useArticleStore();
        articleStore.fetchArticles();
        articleStore.fetchUnreadCounts();
        return;
      }

      // If tasks are running, proceed with normal progress polling
      await fetchProgressOnce();
      pollProgress();
    } catch (e) {
      console.error('Error refreshing feeds:', e);
      refreshProgress.value.isRunning = false;
    }
  }

  async function fetchProgressOnce(): Promise<void> {
    try {
      // Wait a bit for the backend to start processing
      await new Promise((resolve) => setTimeout(resolve, 100));

      const data: any = await apiClient.get('/progress');
      refreshProgress.value = {
        ...refreshProgress.value,
        isRunning: data.is_running,
        errors: data.errors,
        pool_task_count: data.pool_task_count,
        article_click_count: data.article_click_count,
        queue_task_count: data.queue_task_count,
      };
    } catch (e) {
      console.error('Error fetching initial progress:', e);
    }
  }

  function pollProgress(): void {
    // Clear any existing interval first
    stopPollProgress();

    // Track previous pool/queue counts to detect task completion
    let previousPoolCount = 0;
    let previousQueueCount = 0;

    pollProgressInterval = setInterval(async () => {
      try {
        const data: any = await apiClient.get('/progress');
        refreshProgress.value = {
          ...refreshProgress.value, // Preserve existing pool_tasks and queue_tasks
          isRunning: data.is_running,
          errors: data.errors,
          pool_task_count: data.pool_task_count ?? 0,
          article_click_count: data.article_click_count ?? 0,
          queue_task_count: data.queue_task_count ?? 0,
        };

        // Fetch task details if refresh is running
        if (data.is_running && (data.pool_task_count > 0 || data.queue_task_count > 0)) {
          await fetchTaskDetails();
        }

        // Detect task completion and update unread counts immediately
        const currentPoolCount = data.pool_task_count ?? 0;
        const currentQueueCount = data.queue_task_count ?? 0;
        const totalTasks = currentPoolCount + currentQueueCount;
        const previousTotal = previousPoolCount + previousQueueCount;

        // If task count decreased, tasks completed - update unread counts
        if (totalTasks < previousTotal && previousTotal > 0) {
          const articleStore = useArticleStore();
          articleStore.fetchUnreadCounts();
          fetchFeeds(); // Also update feeds to refresh error marks
        }

        // Update previous counts
        previousPoolCount = currentPoolCount;
        previousQueueCount = currentQueueCount;

        if (!data.is_running) {
          stopPollProgress();
          await fetchFeeds();
          
          const articleStore = useArticleStore();
          articleStore.fetchArticles();
          articleStore.fetchUnreadCounts();

          // Check for app updates after initial refresh completes
          // Note: checkForAppUpdates is in AppStore (or main.ts), we might need to move it or call it via event
          // For now, let's leave it out or import useAppStore if needed, but let's avoid circular if possible.
          // Ideally, AppStore should watch refresh status or we move update checking here.
          // Let's assume the component handling this will check for updates or we can add it later.
          // Actually, let's invoke it via useAppStore if we can.
          import('@/stores/app').then(({ useAppStore }) => {
             useAppStore().checkForAppUpdates();
          });
        }
      } catch (e) {
        console.error('Error polling progress:', e);
        stopPollProgress();
        refreshProgress.value.isRunning = false;
      }
    }, 500);
  }

  function stopPollProgress(): void {
    if (pollProgressInterval) {
      clearInterval(pollProgressInterval);
      pollProgressInterval = null;
    }
  }

  async function fetchTaskDetails(): Promise<void> {
    try {
      const data: any = await apiClient.get('/progress/task-details');
      refreshProgress.value = {
        ...refreshProgress.value,
        pool_tasks: data.pool_tasks,
        queue_tasks: data.queue_tasks,
      };
    } catch (e) {
      console.error('Error fetching task details:', e);
    }
  }

  async function stopRefresh(): Promise<void> {
    try {
      await stopRefreshFeeds();
      refreshProgress.value.isRunning = false;
    } catch (e) {
      console.error('Error stopping refresh:', e);
    }
  }

  // FreshRSS sync status monitoring
  let freshrssPollInterval: ReturnType<typeof setInterval> | null = null;
  let lastKnownFreshRSSSyncTime: string | null = null;

  async function startFreshRSSStatusPolling(): Promise<void> {
    // Stop any existing polling
    if (freshrssPollInterval) {
      clearInterval(freshrssPollInterval);
    }

    // Check if FreshRSS is enabled
    try {
      const settings: any = await apiClient.get('/settings');
      if (settings.freshrss_enabled !== 'true') {
        return; // FreshRSS not enabled, don't start polling
      }

      // Initialize last known sync time
      const statusData: any = await apiClient.get('/freshrss/status');
      lastKnownFreshRSSSyncTime = statusData.last_sync_time;
    } catch (e) {
      console.error('[FreshRSS] Error checking status:', e);
      return;
    }

    // Start polling every 5 seconds
    freshrssPollInterval = setInterval(async () => {
      try {
        const data: any = await apiClient.get('/freshrss/status');

        // Check if sync time has updated (sync completed)
        if (
          lastKnownFreshRSSSyncTime !== null &&
          data.last_sync_time !== lastKnownFreshRSSSyncTime
        ) {
          // Refresh all data
          await fetchFeeds();
          
          const articleStore = useArticleStore();
          await articleStore.fetchArticles();
          await articleStore.fetchUnreadCounts();
        }

        // Update known sync time
        lastKnownFreshRSSSyncTime = data.last_sync_time;
      } catch (e) {
        console.error('[FreshRSS] Error polling status:', e);
      }
    }, 5000); // Poll every 5 seconds
  }

  function stopFreshRSSStatusPolling(): void {
    if (freshrssPollInterval) {
      clearInterval(freshrssPollInterval);
      freshrssPollInterval = null;
    }
  }

  return {
    feeds,
    tags,
    refreshProgress,
    feedMap,
    tagMap,
    fetchFeeds,
    fetchTags,
    refreshFeeds,
    pollProgress,
    stopPollProgress,
    fetchTaskDetails,
    stopRefresh,
    startFreshRSSStatusPolling,
    stopFreshRSSStatusPolling,
  };
});
