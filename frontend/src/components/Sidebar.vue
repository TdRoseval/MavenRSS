<script setup>
import { store } from '../store.js';
import { computed, ref, watch } from 'vue';
import { BrowserOpenURL } from '../wailsjs/wailsjs/runtime/runtime.js';

const props = defineProps(['isOpen']);
const emit = defineEmits(['toggle']);

const tree = computed(() => {
    const t = {};
    const uncategorized = [];
    const categories = new Set();
    
    if (!store.feeds) return { tree: {}, uncategorized: [], categories };

    store.feeds.forEach(feed => {
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

const openCategories = ref(new Set());

// Auto-expand all categories by default when they are first loaded
watch(() => tree.value.categories, (newCategories) => {
    if (newCategories) {
        newCategories.forEach(cat => {
            if (!openCategories.value.has(cat)) {
                openCategories.value.add(cat);
            }
        });
    }
}, { immediate: true });

function toggleCategory(path) {
    if (openCategories.value.has(path)) {
        openCategories.value.delete(path);
    } else {
        openCategories.value.add(path);
    }
}

function isCategoryOpen(path) {
    return openCategories.value.has(path);
}

function getFavicon(url) {
    try {
        return `https://www.google.com/s2/favicons?domain=${new URL(url).hostname}`;
    } catch {
        return '';
    }
}

const emitShowAddFeed = () => window.dispatchEvent(new CustomEvent('show-add-feed'));
const emitShowSettings = () => window.dispatchEvent(new CustomEvent('show-settings'));

function onFeedContextMenu(e, feed) {
    e.preventDefault();
    e.stopPropagation();
    window.dispatchEvent(new CustomEvent('open-context-menu', {
        detail: {
            x: e.clientX,
            y: e.clientY,
            items: [
                { label: store.i18n.t('openWebsite'), action: 'openWebsite', icon: 'ph-globe' },
                { separator: true },
                { label: store.i18n.t('editSubscription'), action: 'edit', icon: 'ph-pencil' },
                { label: store.i18n.t('unsubscribe'), action: 'delete', icon: 'ph-trash', danger: true }
            ],
            data: feed,
            callback: handleFeedAction
        }
    }));
}

async function handleFeedAction(action, feed) {
    if (action === 'delete') {
        const confirmed = await window.showConfirm({
            title: 'Unsubscribe',
            message: `Are you sure you want to unsubscribe from ${feed.title}?`,
            confirmText: 'Unsubscribe',
            cancelText: 'Cancel',
            isDanger: true
        });
        if (confirmed) {
            await fetch(`/api/feeds/delete?id=${feed.id}`, { method: 'POST' });
            store.fetchFeeds();
            window.showToast('Successfully unsubscribed', 'success');
        }
    } else if (action === 'edit') {
        window.dispatchEvent(new CustomEvent('show-edit-feed', { detail: feed }));
    } else if (action === 'openWebsite') {
        BrowserOpenURL(feed.url);
    }
}

function onCategoryContextMenu(e, categoryName) {
    e.preventDefault();
    e.stopPropagation();
    window.dispatchEvent(new CustomEvent('open-context-menu', {
        detail: {
            x: e.clientX,
            y: e.clientY,
            items: [
                { label: store.i18n.t('renameCategory'), action: 'rename', icon: 'ph-pencil' }
            ],
            data: categoryName,
            callback: handleCategoryAction
        }
    }));
}

async function handleCategoryAction(action, categoryName) {
    if (action === 'rename') {
        const newName = prompt('Enter new category name:', categoryName);
        if (newName && newName !== categoryName) {
            const feedsToUpdate = store.feeds.filter(f => f.category === categoryName || f.category.startsWith(categoryName + '/'));
            
            // Simple rename for exact match, but handling nested categories properly would be more complex.
            // For now, let's assume flat or simple hierarchy where we just replace the prefix or exact match.
            // Actually, the tree view splits by '/', so 'Tech/News' is under 'Tech'.
            // If I rename 'Tech', I should update 'Tech/News' to 'NewName/News'.
            
            const promises = feedsToUpdate.map(feed => {
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
                        category: newCategory 
                    })
                });
            });

            await Promise.all(promises);
            store.fetchFeeds();
        }
    }
}

</script>

<template>
    <aside :class="['sidebar flex flex-col bg-bg-secondary border-r border-border h-full transition-transform duration-300 absolute z-20 md:relative md:translate-x-0', isOpen ? 'translate-x-0' : '-translate-x-full']">
        <div class="p-5 border-b border-border flex justify-between items-center">
            <h2 class="m-0 text-lg font-bold flex items-center gap-2 text-accent">
                <img src="/assets/logo.svg" alt="Logo" class="h-7 w-auto" /> {{ store.i18n.t('appName') }}
            </h2>
        </div>

        <nav class="p-3 space-y-1">
            <button @click="store.setFilter('all')" :class="['nav-item', store.currentFilter === 'all' ? 'active' : '']">
                <i class="ph ph-list-dashes"></i> {{ store.i18n.t('allArticles') }}
            </button>
            <button @click="store.setFilter('unread')" :class="['nav-item', store.currentFilter === 'unread' ? 'active' : '']">
                <i class="ph ph-circle"></i> {{ store.i18n.t('unread') }}
            </button>
            <button @click="store.setFilter('favorites')" :class="['nav-item', store.currentFilter === 'favorites' ? 'active' : '']">
                <i class="ph ph-star"></i> {{ store.i18n.t('favorites') }}
            </button>
        </nav>

        <div class="flex-1 overflow-y-auto p-2 border-t border-border">
            <!-- Recursive Tree Component would be better, but flattening for simplicity or inline -->
            <div v-for="(data, name) in tree.tree" :key="name" class="mb-1">
                <div 
                    :class="['category-header', store.currentCategory === name ? 'active' : '']"
                    @contextmenu="onCategoryContextMenu($event, name)"
                >
                    <span class="flex-1 flex items-center gap-2" @click="store.setCategory(name)">
                        <i class="ph ph-folder"></i> {{ name }}
                    </span>
                    <i class="ph ph-caret-down p-1 cursor-pointer transition-transform" 
                       :class="{ 'rotate-180': isCategoryOpen(name) }"
                       @click.stop="toggleCategory(name)"></i>
                </div>
                <div v-show="isCategoryOpen(name)" class="pl-2">
                    <div v-for="feed in data._feeds" :key="feed.id" 
                         @click="store.setFeed(feed.id)"
                         @contextmenu="onFeedContextMenu($event, feed)"
                         :class="['feed-item', store.currentFeedId === feed.id ? 'active' : '']">
                        <div class="w-4 h-4 flex items-center justify-center shrink-0">
                            <img :src="feed.image_url || getFavicon(feed.url)" class="w-full h-full object-contain" @error="$event.target.style.display='none'">
                        </div>
                        <span class="truncate">{{ feed.title }}</span>
                    </div>
                </div>
            </div>

            <!-- Uncategorized -->
             <div v-if="tree.uncategorized.length > 0" class="mb-1">
                <div class="category-header" @click="toggleCategory('uncategorized')">
                     <span class="flex-1 flex items-center gap-2">
                        <i class="ph ph-folder-dashed"></i> {{ store.i18n.t('uncategorized') }}
                    </span>
                    <i class="ph ph-caret-down p-1 cursor-pointer transition-transform" 
                       :class="{ 'rotate-180': isCategoryOpen('uncategorized') }"></i>
                </div>
                <div v-show="isCategoryOpen('uncategorized')" class="pl-2">
                    <div v-for="feed in tree.uncategorized" :key="feed.id" 
                         @click="store.setFeed(feed.id)"
                         @contextmenu="onFeedContextMenu($event, feed)"
                         :class="['feed-item', store.currentFeedId === feed.id ? 'active' : '']">
                        <div class="w-4 h-4 flex items-center justify-center shrink-0">
                            <img :src="feed.image_url || getFavicon(feed.url)" class="w-full h-full object-contain" @error="$event.target.style.display='none'">
                        </div>
                        <span class="truncate">{{ feed.title }}</span>
                    </div>
                </div>
             </div>
        </div>

        <div class="p-4 border-t border-border flex gap-2">
            <button @click="emitShowAddFeed" class="footer-btn" :title="store.i18n.t('addFeed')"><i class="ph ph-plus"></i></button>
            <button @click="emitShowSettings" class="footer-btn" :title="store.i18n.t('settings')"><i class="ph ph-gear"></i></button>
        </div>
    </aside>
    <!-- Overlay for mobile -->
    <div v-if="isOpen" @click="emit('toggle')" class="fixed inset-0 bg-black/50 z-10 md:hidden"></div>
</template>

<style scoped>
.sidebar {
    width: 16rem;
}
@media (min-width: 768px) {
    .sidebar {
        width: var(--sidebar-width, 16rem);
    }
}
.nav-item {
    @apply flex items-center gap-3 w-full px-3 py-2.5 text-text-secondary rounded-lg font-medium transition-colors hover:bg-bg-tertiary hover:text-text-primary text-left;
}
.nav-item.active {
    @apply bg-bg-tertiary text-accent font-semibold;
}
.category-header {
    @apply px-3 py-2 cursor-pointer font-semibold text-sm text-text-secondary flex items-center justify-between rounded-md hover:bg-bg-tertiary hover:text-text-primary transition-colors;
}
.category-header.active {
    @apply bg-bg-tertiary text-accent;
}
.feed-item {
    @apply px-3 py-2 cursor-pointer rounded-md text-sm text-text-primary flex items-center gap-2.5 hover:bg-bg-tertiary transition-colors;
}
.feed-item.active {
    @apply bg-bg-tertiary text-accent font-medium;
}
.footer-btn {
    @apply flex-1 flex items-center justify-center gap-2 p-2.5 text-text-secondary rounded-lg text-xl hover:bg-bg-tertiary hover:text-text-primary transition-colors;
}
</style>
