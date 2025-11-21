<script setup>
import { store } from './store.js';
import Sidebar from './components/Sidebar.vue';
import ArticleList from './components/ArticleList.vue';
import ArticleDetail from './components/ArticleDetail.vue';
import AddFeedModal from './components/modals/AddFeedModal.vue';
import EditFeedModal from './components/modals/EditFeedModal.vue';
import SettingsModal from './components/modals/SettingsModal.vue';
import ContextMenu from './components/ContextMenu.vue';
import ConfirmDialog from './components/modals/ConfirmDialog.vue';
import Toast from './components/Toast.vue';
import { onMounted, ref } from 'vue';

const showAddFeed = ref(false);
const showEditFeed = ref(false);
const feedToEdit = ref(null);
const showSettings = ref(false);
const isSidebarOpen = ref(false);

// Global notification system
const confirmDialog = ref(null);
const toasts = ref([]);

function showConfirm(options) {
    return new Promise((resolve) => {
        confirmDialog.value = {
            ...options,
            onConfirm: () => {
                confirmDialog.value = null;
                resolve(true);
            },
            onCancel: () => {
                confirmDialog.value = null;
                resolve(false);
            }
        };
    });
}

function showToast(message, type = 'info', duration = 3000) {
    const id = Date.now();
    toasts.value.push({ id, message, type, duration });
}

// Make these available globally
window.showConfirm = showConfirm;
window.showToast = showToast;

// Resizable columns state
const sidebarWidth = ref(256);
const articleListWidth = ref(400);
const isResizingSidebar = ref(false);
const isResizingArticleList = ref(false);

function startResizeSidebar(e) {
    isResizingSidebar.value = true;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    window.addEventListener('mousemove', handleResizeSidebar);
    window.addEventListener('mouseup', stopResizeSidebar);
}

function handleResizeSidebar(e) {
    if (!isResizingSidebar.value) return;
    const newWidth = e.clientX;
    if (newWidth >= 180 && newWidth <= 450) {
        sidebarWidth.value = newWidth;
    }
}

function stopResizeSidebar() {
    isResizingSidebar.value = false;
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
    window.removeEventListener('mousemove', handleResizeSidebar);
    window.removeEventListener('mouseup', stopResizeSidebar);
}

function startResizeArticleList(e) {
    isResizingArticleList.value = true;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    window.addEventListener('mousemove', handleResizeArticleList);
    window.addEventListener('mouseup', stopResizeArticleList);
}

function handleResizeArticleList(e) {
    if (!isResizingArticleList.value) return;
    // Assuming sidebar is visible and at the left
    const newWidth = e.clientX - sidebarWidth.value;
    if (newWidth >= 250 && newWidth <= 600) {
        articleListWidth.value = newWidth;
    }
}

function stopResizeArticleList() {
    isResizingArticleList.value = false;
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
    window.removeEventListener('mousemove', handleResizeArticleList);
    window.removeEventListener('mouseup', stopResizeArticleList);
}

// Context Menu State
const contextMenu = ref({
    show: false,
    x: 0,
    y: 0,
    items: [],
    data: null
});

onMounted(async () => {
    // Trigger immediate article refresh on app launch
    store.refreshFeeds();
    
    store.fetchFeeds();
    store.fetchArticles();
    
    // Initialize settings for auto-refresh
    try {
        const res = await fetch('/api/settings');
        const data = await res.json();
        if (data.update_interval) {
            store.startAutoRefresh(parseInt(data.update_interval));
        }
    } catch (e) {
        console.error(e);
    }
    
    // Listen for events from Sidebar
    window.addEventListener('show-add-feed', () => showAddFeed.value = true);
    window.addEventListener('show-edit-feed', (e) => {
        feedToEdit.value = e.detail;
        showEditFeed.value = true;
    });
    window.addEventListener('show-settings', () => showSettings.value = true);
    
    // Global Context Menu Event Listener
    window.addEventListener('open-context-menu', (e) => {
        contextMenu.value = {
            show: true,
            x: e.detail.x,
            y: e.detail.y,
            items: e.detail.items,
            data: e.detail.data,
            callback: e.detail.callback
        };
    });
    
    // Check theme
    if (store.theme === 'dark') {
        document.body.classList.add('dark-mode');
    }
});

function toggleSidebar() {
    isSidebarOpen.value = !isSidebarOpen.value;
}

function onFeedAdded() {
    store.fetchFeeds();
    store.fetchArticles(); // Refresh articles too
}

function onFeedUpdated() {
    store.fetchFeeds();
}

function handleContextMenuAction(action) {
    if (contextMenu.value.callback) {
        contextMenu.value.callback(action, contextMenu.value.data);
    }
}
</script>

<template>
    <div class="app-container flex h-screen w-full bg-bg-primary text-text-primary overflow-hidden"
         :style="{ '--sidebar-width': sidebarWidth + 'px', '--article-list-width': articleListWidth + 'px' }">
        <Sidebar :isOpen="isSidebarOpen" @toggle="toggleSidebar" />
        
        <div class="resizer hidden md:block" @mousedown="startResizeSidebar"></div>
        
        <ArticleList :isSidebarOpen="isSidebarOpen" @toggleSidebar="toggleSidebar" />
        
        <div class="resizer hidden md:block" @mousedown="startResizeArticleList"></div>
        
        <ArticleDetail />
        
        <AddFeedModal v-if="showAddFeed" @close="showAddFeed = false" @added="onFeedAdded" />
        <EditFeedModal v-if="showEditFeed" :feed="feedToEdit" @close="showEditFeed = false" @updated="onFeedUpdated" />
        <SettingsModal v-if="showSettings" @close="showSettings = false" />
        
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
        
        <div class="toast-container">
            <Toast 
                v-for="toast in toasts"
                :key="toast.id"
                :message="toast.message"
                :type="toast.type"
                :duration="toast.duration"
                @close="toasts = toasts.filter(t => t.id !== toast.id)"
            />
        </div>
    </div>
</template>

<style>
.toast-container {
    position: fixed;
    top: 20px;
    right: 20px;
    z-index: 60;
    display: flex;
    flex-direction: column;
    gap: 10px;
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
.resizer:hover, .resizer:active {
    background-color: var(--color-accent, #3b82f6);
}
/* Global styles if needed */
</style>
