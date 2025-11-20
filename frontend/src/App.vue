<script setup>
import { store } from './store.js';
import Sidebar from './components/Sidebar.vue';
import ArticleList from './components/ArticleList.vue';
import ArticleDetail from './components/ArticleDetail.vue';
import AddFeedModal from './components/modals/AddFeedModal.vue';
import EditFeedModal from './components/modals/EditFeedModal.vue';
import SettingsModal from './components/modals/SettingsModal.vue';
import ContextMenu from './components/ContextMenu.vue';
import { onMounted, ref } from 'vue';

const showAddFeed = ref(false);
const showEditFeed = ref(false);
const feedToEdit = ref(null);
const showSettings = ref(false);
const isSidebarOpen = ref(false);

// Context Menu State
const contextMenu = ref({
    show: false,
    x: 0,
    y: 0,
    items: [],
    data: null
});

onMounted(async () => {
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
    <div class="app-container flex h-screen w-full bg-bg-primary text-text-primary overflow-hidden">
        <Sidebar :isOpen="isSidebarOpen" @toggle="toggleSidebar" />
        <ArticleList :isSidebarOpen="isSidebarOpen" @toggleSidebar="toggleSidebar" />
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
    </div>
</template>

<style>
/* Global styles if needed */
</style>
