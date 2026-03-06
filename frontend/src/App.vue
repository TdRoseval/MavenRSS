<script setup lang="ts">
import { useAuthStore } from './stores/auth';
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted, onBeforeUnmount, defineAsyncComponent } from 'vue';
import { saveLanguage } from './i18n';
import Sidebar from './components/sidebar/Sidebar.vue';
import ArticleList from '@/features/article/components/ArticleList.vue';
import ArticleDetail from '@/features/article/components/ArticleDetail.vue';
import ImageGalleryView from '@/features/article/components/imageGallery/index.vue';
import Toast from '@/shared/ui/Toast.vue';
import LoginPage from './components/auth/LoginPage.vue';
import AdminUserManagement from './components/auth/AdminUserManagement.vue';
import { useNotifications } from './composables/ui/useNotifications';
import { useKeyboardShortcuts } from './composables/ui/useKeyboardShortcuts';
import { useContextMenu } from './composables/ui/useContextMenu';
import { useResizablePanels } from './composables/ui/useResizablePanels';
import { useWindowState } from './composables/core/useWindowState';
import { useAppUpdates } from './composables/core/useAppUpdates';
import { apiClient } from '@/shared/lib/apiClient';
import { authFetchJson } from '@/shared/lib/authFetch';
import type { Feed } from './types/models';
import { useArticleStore } from '@/features/article/store';
import { useFeedStore } from '@/features/feed/store';
import { useAppStore } from '@/stores/app';


const AddFeedModal = defineAsyncComponent(
  () => import('@/features/feed/components/AddFeedModal.vue')
);
const EditFeedModal = defineAsyncComponent(
  () => import('@/features/feed/components/EditFeedModal.vue')
);
const SettingsModal = defineAsyncComponent(() => import('./components/modals/SettingsModal.vue'));
const DiscoverFeedsModal = defineAsyncComponent(
  () => import('@/features/discovery/components/DiscoverFeedsModal.vue')
);
const UpdateAvailableDialog = defineAsyncComponent(
  () => import('./components/modals/update/UpdateAvailableDialog.vue')
);
const ContextMenu = defineAsyncComponent(() => import('@/shared/ui/ContextMenu.vue'));
const ConfirmDialog = defineAsyncComponent(
  () => import('@/shared/ui/ConfirmDialog.vue')
);
const InputDialog = defineAsyncComponent(
  () => import('@/shared/ui/InputDialog.vue')
);
const MultiSelectDialog = defineAsyncComponent(
  () => import('@/shared/ui/MultiSelectDialog.vue')
);

const articleStore = useArticleStore();
const feedStore = useFeedStore();
const appStore = useAppStore();
const authStore = useAuthStore();
const { t, locale } = useI18n();

const isAdmin = computed(() => authStore.user?.role === 'admin');

const showAddFeed = ref(false);
const showEditFeed = ref(false);
const feedToEdit = ref<Feed | null>(null);
const showSettings = ref(false);
const showDiscoverBlogs = ref(false);
const showUserManagement = ref(false);
const feedToDiscover = ref<Feed | null>(null);
const isSidebarOpen = ref(true);

const isMobile = ref(false);
const mobileView = ref<'list' | 'detail'>('list');
const currentArticleIdOnMobile = ref<number | null>(null);

function checkIsMobile(): boolean {
  return window.innerWidth < 768;
}

function handleResize(): void {
  const wasMobile = isMobile.value;
  isMobile.value = checkIsMobile();

  if (wasMobile && !isMobile.value) {
    if (mobileView.value === 'detail') {
      mobileView.value = 'list';
    }
  }
}

function openArticleOnMobile(articleId: number): void {
  currentArticleIdOnMobile.value = articleId;
  mobileView.value = 'detail';
}

function closeArticleOnMobile(): void {
  articleStore.currentArticleId = null;
  currentArticleIdOnMobile.value = null;
  mobileView.value = 'list';
}

// Check if we're in image gallery mode
const isImageGalleryMode = computed(() => articleStore.currentFilter === 'imageGallery');

// Check if we're in card mode
const isCardMode = ref(false);

// Use composables
const {
  confirmDialog,
  inputDialog,
  multiSelectDialog,
  toasts,
  removeToast,
  installGlobalHandlers,
} = useNotifications();

const { contextMenu, openContextMenu, handleContextMenuAction } = useContextMenu();

const {
  sidebarWidth,
  articleListWidth,
  startResizeArticleList,
  setArticleListWidth,
  setCompactMode,
} = useResizablePanels();

// Use app updates composable
const {
  updateInfo,
  checkForUpdates,
  downloadAndInstallUpdate,
  downloadingUpdate,
  installingUpdate,
  downloadProgress,
} = useAppUpdates();

// Update dialog state
const showUpdateDialog = ref(false);

// Initialize window state management
const windowState = useWindowState();
windowState.init();

// Initialize keyboard shortcuts
const { shortcuts } = useKeyboardShortcuts({
  onOpenSettings: () => {
    showSettings.value = true;
  },
  onAddFeed: () => {
    showAddFeed.value = true;
  },
  onMarkAllRead: async () => {
    await articleStore.markAllAsRead();
    window.showToast(t('article.action.markedAllAsRead'), 'success');
  },
});

// Event handler functions
function handleShowAddFeed(): void {
  showAddFeed.value = true;
}

function handleShowEditFeed(e: Event): void {
  const customEvent = e as CustomEvent<any>;
  feedToEdit.value = customEvent.detail;
  showEditFeed.value = true;
}

function handleShowSettings(): void {
  showSettings.value = true;
}

function handleShowDiscoverBlogs(e: Event): void {
  const customEvent = e as CustomEvent<any>;
  feedToDiscover.value = customEvent.detail;
  showDiscoverBlogs.value = true;
}

function handleLayoutModeChanged(e: Event): void {
  const customEvent = e as CustomEvent<{ mode: string }>;
  const mode = customEvent.detail.mode;
  const isCompactModeLayout = mode === 'compact';
  isCardMode.value = mode === 'card';
  setCompactMode(isCompactModeLayout);
  if (!isCardMode.value) {
    setArticleListWidth(isCompactModeLayout ? 600 : 400);
  }
}

function handleOpenContextMenu(e: Event): void {
  openContextMenu(e as CustomEvent<any>);
}

function handleShowUserManagement(): void {
  if (isAdmin.value) {
    showUserManagement.value = true;
  }
}

onMounted(() => {
  // Load authentication state from storage
  authStore.loadFromStorage();

  // Install global notification handlers
  installGlobalHandlers();

  // Initialize theme system immediately (lightweight)
  appStore.initTheme();

  // Initialize mobile detection
  isMobile.value = checkIsMobile();
  window.addEventListener('resize', handleResize);

  // Add event listeners inside onMounted
  window.addEventListener('show-add-feed', handleShowAddFeed);
  window.addEventListener('show-edit-feed', handleShowEditFeed);
  window.addEventListener('show-settings', handleShowSettings);
  window.addEventListener('show-discover-blogs', handleShowDiscoverBlogs);
  window.addEventListener('layout-mode-changed', handleLayoutModeChanged);
  window.addEventListener('open-context-menu', handleOpenContextMenu);
  window.addEventListener('show-user-management', handleShowUserManagement);

  // If user is authenticated, load settings and data
  if (authStore.isAuthenticated) {
    loadInitialSettings();

    // Update check on startup has been disabled

    // Load feeds and articles in background
    setTimeout(() => {
      feedStore.fetchFeeds();
      articleStore.fetchArticles();

      setTimeout(async () => {
        try {
          const progressData = await authFetchJson('/api/progress');

          if (progressData.is_running) {
            feedStore.refreshProgress = {
              ...feedStore.refreshProgress,
              isRunning: true,
              pool_task_count: progressData.pool_task_count,
              article_click_count: progressData.article_click_count,
              queue_task_count: progressData.queue_task_count,
            };
            feedStore.pollProgress();
            return;
          }
        } catch (e) {
          console.error('Error checking initial refresh progress:', e);
        }
      }, 500);
    }, 100);
  }
});

onBeforeUnmount(() => {
  // Clean up all event listeners
  window.removeEventListener('resize', handleResize);
  window.removeEventListener('show-add-feed', handleShowAddFeed);
  window.removeEventListener('show-edit-feed', handleShowEditFeed);
  window.removeEventListener('show-settings', handleShowSettings);
  window.removeEventListener('show-discover-blogs', handleShowDiscoverBlogs);
  window.removeEventListener('layout-mode-changed', handleLayoutModeChanged);
  window.removeEventListener('open-context-menu', handleOpenContextMenu);
  window.removeEventListener('show-user-management', handleShowUserManagement);
  
  // Clean up store timers
  feedStore.stopPollProgress();
  feedStore.stopFreshRSSStatusPolling();
});

async function loadInitialSettings() {
  let updateInterval = 10;
  let lastGlobalRefresh = '';

  try {
    const data = await apiClient.get<any>('/settings');

    const layoutMode = data.layout_mode || 'normal';
    const isCompactModeLayout = layoutMode === 'compact';
    isCardMode.value = layoutMode === 'card';
    setCompactMode(isCompactModeLayout);
    setArticleListWidth(isCompactModeLayout ? 500 : 350);

    window.dispatchEvent(new CustomEvent('settings-loaded'));

    if (data.theme) {
      appStore.setTheme(data.theme);
    }

    if (data.language) {
      locale.value = data.language;
      saveLanguage(data.language);
    }

    if (data.last_global_refresh) {
      lastGlobalRefresh = data.last_global_refresh;
    }

    if (data.shortcuts) {
      try {
        const parsed = JSON.parse(data.shortcuts);
        shortcuts.value = { ...shortcuts.value, ...parsed };
      } catch (e) {
        console.error('Error parsing shortcuts:', e);
      }
    }

    let latestLastGlobalRefresh = lastGlobalRefresh;
    try {
      const settingsData = await apiClient.get<any>('/settings');
      if (settingsData.last_global_refresh) {
        latestLastGlobalRefresh = settingsData.last_global_refresh;
      }
    } catch (e) {
      console.error('Error fetching latest last_global_refresh:', e);
    }

    const shouldRefresh = shouldTriggerRefresh(latestLastGlobalRefresh, updateInterval);
    if (shouldRefresh) {
      feedStore.refreshFeeds();
    }
  } catch (e) {
    console.error('Error loading initial settings:', e);
  }
}

// Check if we should trigger refresh based on last update time and interval
// Note: This is now mainly for display purposes - actual auto-refresh is controlled by backend
function shouldTriggerRefresh(lastUpdate: string, intervalMinutes: number): boolean {
  // Always return false - auto-refresh is now controlled by backend only
  // This function is kept for potential future use
  return false;
}

function toggleSidebar(): void {
  isSidebarOpen.value = !isSidebarOpen.value;
}

function onFeedAdded(): void {
  feedStore.fetchFeeds();
  // Start polling for progress as the backend is now fetching articles for the new feed
  feedStore.pollProgress();
}

function onFeedUpdated(): void {
  feedStore.fetchFeeds();
  // Refresh articles to immediately apply hide_from_timeline changes
  articleStore.fetchArticles();
}

function onLogin(): void {
  // After login, load settings and data
  loadInitialSettings();
  feedStore.fetchFeeds();
  articleStore.fetchArticles();
}
</script>

<template>
  <div
    class="app-container flex h-screen w-full bg-bg-primary text-text-primary overflow-hidden"
    :class="{ 'mobile-mode': isMobile }"
    :style="{
      '--sidebar-width': sidebarWidth + 'px',
      '--article-list-width': articleListWidth + 'px',
    }"
  >
    <!-- Show Login Page if not authenticated -->
    <LoginPage v-if="!authStore.isAuthenticated" @login="onLogin" />

    <!-- Show Main Application if authenticated -->
    <template v-else>
      <!-- Mobile: Slide-out Sidebar -->
      <Transition name="sidebar-slide">
        <Sidebar
          v-if="isMobile ? isSidebarOpen : true"
          :is-open="isSidebarOpen"
          :is-mobile="isMobile"
          @toggle="toggleSidebar"
          @open-user-management="showUserManagement = true"
        />
      </Transition>

      <!-- Mobile overlay -->
      <Transition name="overlay-fade">
        <div
          v-if="isMobile && isSidebarOpen"
          class="fixed inset-0 bg-black/50 z-40 md:hidden"
          @click="toggleSidebar"
        ></div>
      </Transition>

      <!-- Mobile main content area -->
      <div v-if="isMobile" class="flex-1 flex flex-col h-full overflow-hidden relative">
        <!-- Mobile: Article List View (always rendered, but hidden when in detail view) -->
        <div
          :class="[
            'absolute inset-0 z-10 transition-opacity duration-200',
            mobileView === 'list'
              ? 'opacity-100 visible'
              : 'opacity-0 invisible pointer-events-none',
          ]"
        >
          <ArticleList
            ref="articleListRef"
            :is-mobile="isMobile"
            :is-sidebar-open="isSidebarOpen"
            @toggle-sidebar="toggleSidebar"
            @select-article="openArticleOnMobile"
          />
        </div>

        <!-- Mobile: Article Detail View (always rendered, but hidden when in list view) -->
        <div
          :class="[
            'absolute inset-0 z-20 transition-transform duration-300',
            mobileView === 'detail' ? 'translate-x-0' : 'translate-x-full',
          ]"
        >
          <ArticleDetail :is-mobile="isMobile" @close="closeArticleOnMobile" />
        </div>
      </div>

      <!-- Desktop: Original layout -->
      <template v-else>
        <!-- Show ImageGalleryView when in image gallery mode -->
        <template v-if="isImageGalleryMode">
          <ImageGalleryView :is-sidebar-open="isSidebarOpen" @toggle-sidebar="toggleSidebar" />
        </template>

        <!-- Show ArticleList and ArticleDetail when not in image gallery mode -->
        <template v-else>
          <ArticleList
            ref="articleListRef"
            :is-sidebar-open="isSidebarOpen"
            @toggle-sidebar="toggleSidebar"
          />

          <!-- Hide resizer and ArticleDetail when in card mode -->
          <template v-if="!isCardMode">
            <div class="resizer hidden md:block" @mousedown="startResizeArticleList"></div>

            <ArticleDetail />
          </template>
        </template>
      </template>

      <AddFeedModal v-if="showAddFeed" @close="showAddFeed = false" @added="onFeedAdded" />
      <EditFeedModal
        v-if="showEditFeed && feedToEdit"
        :feed="feedToEdit"
        @close="showEditFeed = false"
        @updated="onFeedUpdated"
      />
      <SettingsModal v-if="showSettings" @close="showSettings = false" />
      <DiscoverFeedsModal
        v-if="showDiscoverBlogs && feedToDiscover"
        :feed="feedToDiscover"
        :show="showDiscoverBlogs"
        @close="showDiscoverBlogs = false"
      />

      <UpdateAvailableDialog
        v-if="showUpdateDialog && updateInfo"
        :update-info="updateInfo"
        :downloading-update="downloadingUpdate"
        :installing-update="installingUpdate"
        :download-progress="downloadProgress"
        @close="showUpdateDialog = false"
        @update="downloadAndInstallUpdate"
      />

      <ContextMenu
        v-if="contextMenu.show"
        :x="contextMenu.x"
        :y="contextMenu.y"
        :items="contextMenu.items"
        @close="contextMenu.show = false"
        @action="handleContextMenuAction"
      />

      <!-- User Management Modal for Admin -->
      <div
        v-if="showUserManagement && isAdmin"
        class="user-management-overlay"
        @click.self="showUserManagement = false"
      >
        <div class="user-management-modal">
          <div class="modal-header">
            <h2>{{ t('admin.title') }}</h2>
            <button class="close-btn" @click="showUserManagement = false">&times;</button>
          </div>
          <AdminUserManagement />
        </div>
      </div>

      <!-- Global Notification System -->
      <ConfirmDialog
        v-if="confirmDialog"
        :title="confirmDialog.title"
        :message="confirmDialog.message"
        :confirm-text="confirmDialog.confirmText"
        :cancel-text="confirmDialog.cancelText"
        :is-danger="confirmDialog.isDanger"
        :use-html="confirmDialog.useHtml"
        @confirm="confirmDialog.onConfirm"
        @cancel="confirmDialog.onCancel"
        @close="confirmDialog = null"
      />

      <InputDialog
        v-if="inputDialog"
        :title="inputDialog.title"
        :message="inputDialog.message"
        :placeholder="inputDialog.placeholder"
        :default-value="inputDialog.defaultValue"
        :confirm-text="inputDialog.confirmText"
        :cancel-text="inputDialog.cancelText"
        :suggestions="inputDialog.suggestions"
        @confirm="inputDialog.onConfirm"
        @cancel="inputDialog.onCancel"
        @close="inputDialog = null"
      />

      <MultiSelectDialog
        v-if="multiSelectDialog"
        :title="multiSelectDialog.title"
        :message="multiSelectDialog.message"
        :options="multiSelectDialog.options"
        :confirm-text="multiSelectDialog.confirmText"
        :cancel-text="multiSelectDialog.cancelText"
        @confirm="multiSelectDialog.onConfirm"
        @cancel="multiSelectDialog.onCancel"
        @close="multiSelectDialog = null"
      />

      <div class="toast-container">
        <Toast
          v-for="toast in toasts"
          :key="toast.id"
          :message="toast.message"
          :type="toast.type"
          :duration="toast.duration"
          @close="removeToast(toast.id)"
        />
      </div>
    </template>
  </div>
</template>

<style>
.toast-container {
  position: fixed;
  top: 10px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 9999;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  pointer-events: none;
}

.toast-container > * {
  top: 42px; /* Account for MacOS top padding */
}

.toast-container > * {
  pointer-events: auto;
}
@media (min-width: 640px) {
  .toast-container {
    top: 20px;
    gap: 10px;
  }
  .app-container.macos-padding .toast-container {
    top: 52px; /* Account for MacOS top padding on larger screens */
  }
}
.resizer {
  width: 4px;
  cursor: col-resize;
  background-color: transparent;
  flex-shrink: 0;
  transition: background-color 0.2s;
  z-index: 10;
  margin-left: -2px;
  margin-right: -2px;
}
.resizer:hover,
.resizer:active {
  background-color: var(--color-accent, #3b82f6);
}

/* Mobile sidebar slide transition */
.sidebar-slide-enter-active,
.sidebar-slide-leave-active {
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.sidebar-slide-enter-from,
.sidebar-slide-leave-to {
  transform: translateX(-100%);
}

/* Mobile overlay fade transition */
.overlay-fade-enter-active,
.overlay-fade-leave-active {
  transition: opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.overlay-fade-enter-from,
.overlay-fade-leave-to {
  opacity: 0;
}

/* Mobile mode adjustments */
.mobile-mode .resizer {
  display: none;
}

/* User Management Modal Styles */
.user-management-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.user-management-modal {
  background: white;
  border-radius: 8px;
  width: 95%;
  max-width: 1400px;
  max-height: 90vh;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px;
  border-bottom: 1px solid #eee;
  position: sticky;
  top: 0;
  background: white;
  z-index: 10;
}

.modal-header h2 {
  margin: 0;
  font-size: 1.5rem;
  color: #333;
}

.close-btn {
  background: none;
  border: none;
  font-size: 2rem;
  color: #999;
  cursor: pointer;
  padding: 0;
  line-height: 1;
}

.close-btn:hover {
  color: #333;
}

/* Global styles if needed */
</style>
