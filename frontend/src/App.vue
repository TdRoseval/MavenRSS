<script setup lang="ts">
import { useAppStore } from './stores/app';
import { useI18n } from 'vue-i18n';
import Sidebar from './components/sidebar/Sidebar.vue';
import ArticleList from './components/article/ArticleList.vue';
import ArticleDetail from './components/article/ArticleDetail.vue';
import AddFeedModal from './components/modals/feed/AddFeedModal.vue';
import EditFeedModal from './components/modals/feed/EditFeedModal.vue';
import SettingsModal from './components/modals/SettingsModal.vue';
import DiscoverFeedsModal from './components/modals/discovery/DiscoverFeedsModal.vue';
import ContextMenu from './components/common/ContextMenu.vue';
import ConfirmDialog from './components/modals/common/ConfirmDialog.vue';
import InputDialog from './components/modals/common/InputDialog.vue';
import Toast from './components/common/Toast.vue';
import { onMounted, ref } from 'vue';
import { useNotifications } from './composables/ui/useNotifications';
import { useKeyboardShortcuts } from './composables/ui/useKeyboardShortcuts';
import { useContextMenu } from './composables/ui/useContextMenu';
import { useResizablePanels } from './composables/ui/useResizablePanels';
import type { Feed } from './types/models';

const store = useAppStore();
const { t } = useI18n();

const showAddFeed = ref(false);
const showEditFeed = ref(false);
const feedToEdit = ref<Feed | null>(null);
const showSettings = ref(false);
const showDiscoverBlogs = ref(false);
const feedToDiscover = ref<Feed | null>(null);
const isSidebarOpen = ref(false);

// Use composables
const { confirmDialog, inputDialog, toasts, removeToast, installGlobalHandlers } =
  useNotifications();

const { contextMenu, openContextMenu, handleContextMenuAction } = useContextMenu();

const { sidebarWidth, articleListWidth, startResizeSidebar, startResizeArticleList } =
  useResizablePanels();

// Initialize keyboard shortcuts
const { shortcuts } = useKeyboardShortcuts({
  onOpenSettings: () => {
    showSettings.value = true;
  },
  onAddFeed: () => {
    showAddFeed.value = true;
  },
  onMarkAllRead: async () => {
    await store.markAllAsRead();
    window.showToast(t('markedAllAsRead'), 'success');
  },
});

onMounted(async () => {
  // Install global notification handlers
  installGlobalHandlers();

  // Initialize theme system immediately (lightweight)
  store.initTheme();

  // Load remaining settings (theme and other settings are already loaded in main.ts)
  try {
    const res = await fetch('/api/settings');
    const data = await res.json();

    // Apply saved theme preference (already applied in main.ts, but ensure it's set)
    if (data.theme) {
      store.setTheme(data.theme);
    }

    // Apply other settings
    if (data.update_interval) {
      store.startAutoRefresh(parseInt(data.update_interval));
    }

    // Load saved shortcuts
    if (data.shortcuts) {
      try {
        const parsed = JSON.parse(data.shortcuts);
        shortcuts.value = { ...shortcuts.value, ...parsed };
      } catch (e) {
        console.error('Error parsing shortcuts:', e);
      }
    }
  } catch (e) {
    console.error('Error loading initial settings:', e);
  }

  // Defer heavy operations to allow UI to render first
  setTimeout(() => {
    // Load feeds and articles in background
    store.fetchFeeds();
    store.fetchArticles();

    // Trigger feed refresh after initial load
    setTimeout(() => {
      store.refreshFeeds();
    }, 1000);
  }, 100);

  // Listen for events from Sidebar
  window.addEventListener('show-add-feed', () => (showAddFeed.value = true));
  window.addEventListener('show-edit-feed', (e: Event) => {
    const customEvent = e as CustomEvent;
    feedToEdit.value = customEvent.detail;
    showEditFeed.value = true;
  });
  window.addEventListener('show-settings', () => (showSettings.value = true));
  window.addEventListener('show-discover-blogs', (e: Event) => {
    const customEvent = e as CustomEvent;
    feedToDiscover.value = customEvent.detail;
    showDiscoverBlogs.value = true;
  });

  // Global Context Menu Event Listener
  window.addEventListener('open-context-menu', (e: Event) => {
    openContextMenu(e as CustomEvent);
  });
});

function toggleSidebar(): void {
  isSidebarOpen.value = !isSidebarOpen.value;
}

function onFeedAdded(): void {
  store.fetchFeeds();
  store.fetchArticles(); // Refresh articles too
}

function onFeedUpdated(): void {
  store.fetchFeeds();
}
</script>

<template>
  <div
    class="app-container flex h-screen w-full bg-bg-primary text-text-primary overflow-hidden"
    :style="{
      '--sidebar-width': sidebarWidth + 'px',
      '--article-list-width': articleListWidth + 'px',
    }"
  >
    <Sidebar :isOpen="isSidebarOpen" @toggle="toggleSidebar" />

    <div class="resizer hidden md:block" @mousedown="startResizeSidebar"></div>

    <ArticleList :isSidebarOpen="isSidebarOpen" @toggleSidebar="toggleSidebar" />

    <div class="resizer hidden md:block" @mousedown="startResizeArticleList"></div>

    <ArticleDetail />

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

    <ContextMenu
      v-if="contextMenu.show"
      :x="contextMenu.x"
      :y="contextMenu.y"
      :items="contextMenu.items"
      @close="contextMenu.show = false"
      @action="handleContextMenuAction"
    />

    <!-- Global Notification System -->
    <ConfirmDialog
      v-if="confirmDialog"
      :title="confirmDialog.title"
      :message="confirmDialog.message"
      :confirmText="confirmDialog.confirmText"
      :cancelText="confirmDialog.cancelText"
      :isDanger="confirmDialog.isDanger"
      @confirm="confirmDialog.onConfirm"
      @cancel="confirmDialog.onCancel"
      @close="confirmDialog = null"
    />

    <InputDialog
      v-if="inputDialog"
      :title="inputDialog.title"
      :message="inputDialog.message"
      :placeholder="inputDialog.placeholder"
      :defaultValue="inputDialog.defaultValue"
      :confirmText="inputDialog.confirmText"
      :cancelText="inputDialog.cancelText"
      @confirm="inputDialog.onConfirm"
      @cancel="inputDialog.onCancel"
      @close="inputDialog = null"
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
  </div>
</template>

<style>
.toast-container {
  position: fixed;
  top: 10px;
  right: 10px;
  left: 10px;
  z-index: 60;
  display: flex;
  flex-direction: column;
  gap: 8px;
  pointer-events: none;
}
.toast-container > * {
  pointer-events: auto;
}
@media (min-width: 640px) {
  .toast-container {
    top: 20px;
    right: 20px;
    left: auto;
    gap: 10px;
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
/* Global styles if needed */
</style>
