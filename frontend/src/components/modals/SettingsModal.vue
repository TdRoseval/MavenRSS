<script setup>
import { store } from '../../store.js';
import { ref, onMounted, computed } from 'vue';

const emit = defineEmits(['close']);
const activeTab = ref('general');
const selectedFeeds = ref([]);

const settings = ref({
    update_interval: 10,
    translation_enabled: false,
    target_language: 'en',
    translation_provider: 'google',
    deepl_api_key: ''
});

onMounted(async () => {
    try {
        const res = await fetch('/api/settings');
        const data = await res.json();
        settings.value = {
            update_interval: data.update_interval || 10,
            translation_enabled: data.translation_enabled === 'true',
            target_language: data.target_language || 'en',
            translation_provider: data.translation_provider || 'google',
            deepl_api_key: data.deepl_api_key || ''
        };
    } catch (e) {
        console.error(e);
    }
});

async function saveSettings() {
    try {
        await fetch('/api/settings', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                update_interval: settings.value.update_interval.toString(),
                translation_enabled: settings.value.translation_enabled.toString(),
                target_language: settings.value.target_language,
                translation_provider: settings.value.translation_provider,
                deepl_api_key: settings.value.deepl_api_key
            })
        });
        store.startAutoRefresh(settings.value.update_interval);
        emit('close');
    } catch (e) {
        alert('Error saving settings');
    }
}

function importOPML(event) {
    const file = event.target.files[0];
    if (!file) return;
    
    const formData = new FormData();
    formData.append('file', file);

    fetch('/api/opml/import', {
        method: 'POST',
        body: formData
    }).then(res => {
        if (res.ok) {
            alert('OPML Imported. Feeds will appear shortly.');
            store.fetchFeeds();
        } else {
            alert('Import failed');
        }
    });
}

function exportOPML() {
    window.location.href = '/api/opml/export';
}

async function deleteFeed(id) {
    if (!confirm('Delete this feed?')) return;
    await fetch(`/api/feeds/delete?id=${id}`, { method: 'POST' });
    store.fetchFeeds();
}

const isAllSelected = computed(() => {
    return store.feeds.length > 0 && selectedFeeds.value.length === store.feeds.length;
});

function toggleSelectAll(e) {
    if (e.target.checked) {
        selectedFeeds.value = store.feeds.map(f => f.id);
    } else {
        selectedFeeds.value = [];
    }
}

async function batchDelete() {
    if (selectedFeeds.value.length === 0) return;
    if (!confirm(`Delete ${selectedFeeds.value.length} feeds?`)) return;

    const promises = selectedFeeds.value.map(id => fetch(`/api/feeds/delete?id=${id}`, { method: 'POST' }));
    await Promise.all(promises);
    selectedFeeds.value = [];
    store.fetchFeeds();
}

async function batchMove() {
    if (selectedFeeds.value.length === 0) return;
    const newCategory = prompt('Enter new category name:');
    if (newCategory === null) return;

    const promises = selectedFeeds.value.map(id => {
        const feed = store.feeds.find(f => f.id === id);
        if (!feed) return Promise.resolve();
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
    selectedFeeds.value = [];
    store.fetchFeeds();
}

function editFeed(feed) {
    window.dispatchEvent(new CustomEvent('show-edit-feed', { detail: feed }));
}

</script>

<template>
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
        <div class="bg-bg-primary w-[800px] h-[600px] flex flex-col rounded-2xl shadow-2xl border border-border overflow-hidden animate-fade-in">
            <div class="p-5 border-b border-border flex justify-between items-center shrink-0">
                <h3 class="text-lg font-semibold m-0">Settings</h3>
                <span @click="emit('close')" class="text-2xl cursor-pointer text-text-secondary hover:text-text-primary">&times;</span>
            </div>
            
            <div class="flex border-b border-border bg-bg-secondary shrink-0">
                <button @click="activeTab = 'general'" :class="['tab-btn', activeTab === 'general' ? 'active' : '']">General</button>
                <button @click="activeTab = 'feeds'" :class="['tab-btn', activeTab === 'feeds' ? 'active' : '']">Feeds</button>
                <button @click="activeTab = 'about'" :class="['tab-btn', activeTab === 'about' ? 'active' : '']">About</button>
            </div>

            <div class="flex-1 overflow-y-auto p-6">
                <div v-if="activeTab === 'general'" class="space-y-6">
                    <div class="setting-group">
                        <label class="block font-semibold mb-3 text-text-secondary uppercase text-xs tracking-wider">Appearance</label>
                        <div class="flex justify-between items-center mb-3">
                            <span>Dark Mode</span>
                            <input type="checkbox" :checked="store.theme === 'dark'" @change="store.toggleTheme()" class="toggle">
                        </div>
                        <div class="flex justify-between items-center">
                            <span>Show Summaries</span>
                            <input type="checkbox" v-model="store.showSummaries" @change="localStorage.setItem('showSummaries', store.showSummaries)" class="toggle">
                        </div>
                    </div>

                    <div class="setting-group">
                        <label class="block font-semibold mb-3 text-text-secondary uppercase text-xs tracking-wider">Translation</label>
                        <div class="flex justify-between items-center mb-3">
                            <span>Enable Automatic Translation</span>
                            <input type="checkbox" v-model="settings.translation_enabled" class="toggle">
                        </div>
                        <div class="mb-3">
                            <label class="block text-sm mb-1">Provider</label>
                            <select v-model="settings.translation_provider" class="input-field">
                                <option value="google">Google Translate (Free)</option>
                                <option value="deepl">DeepL API</option>
                            </select>
                        </div>
                        <div v-if="settings.translation_provider === 'deepl'" class="mb-3">
                            <label class="block text-sm mb-1">DeepL API Key</label>
                            <input type="password" v-model="settings.deepl_api_key" class="input-field">
                        </div>
                        <div>
                            <label class="block text-sm mb-1">Target Language</label>
                            <select v-model="settings.target_language" class="input-field">
                                <option value="en">English</option>
                                <option value="es">Spanish</option>
                                <option value="fr">French</option>
                                <option value="de">German</option>
                                <option value="zh">Chinese</option>
                                <option value="ja">Japanese</option>
                            </select>
                        </div>
                    </div>

                    <div class="setting-group">
                        <label class="block font-semibold mb-3 text-text-secondary uppercase text-xs tracking-wider">Updates</label>
                        <div class="mb-3">
                            <label class="block text-sm mb-1">Auto-update Interval (minutes)</label>
                            <input type="number" v-model="settings.update_interval" min="1" class="input-field">
                        </div>
                    </div>
                </div>

                <div v-if="activeTab === 'feeds'" class="space-y-6">
                    <div class="setting-group">
                        <label class="block font-semibold mb-3 text-text-secondary uppercase text-xs tracking-wider">Data Management</label>
                        <div class="flex gap-3">
                            <button @click="$refs.opmlInput.click()" class="btn-secondary flex-1 justify-center">
                                <i class="ph ph-upload"></i> Import OPML
                            </button>
                            <input type="file" ref="opmlInput" class="hidden" @change="importOPML">
                            <button @click="exportOPML" class="btn-secondary flex-1 justify-center">
                                <i class="ph ph-download"></i> Export OPML
                            </button>
                        </div>
                    </div>
                    
                    <div class="setting-group">
                        <label class="block font-semibold mb-3 text-text-secondary uppercase text-xs tracking-wider">Manage Feeds</label>
                        
                        <div class="flex gap-2 mb-2">
                            <button @click="batchDelete" class="btn-secondary text-sm py-1.5" :disabled="selectedFeeds.length === 0">
                                <i class="ph ph-trash"></i> Delete Selected
                            </button>
                            <button @click="batchMove" class="btn-secondary text-sm py-1.5" :disabled="selectedFeeds.length === 0">
                                <i class="ph ph-folder"></i> Move Selected
                            </button>
                            <div class="flex-1"></div>
                            <label class="flex items-center gap-2 text-sm cursor-pointer select-none">
                                <input type="checkbox" :checked="isAllSelected" @change="toggleSelectAll" class="rounded border-border">
                                Select All
                            </label>
                        </div>

                        <div class="border border-border rounded-lg bg-bg-secondary flex-1 overflow-y-auto min-h-[300px]">
                            <div v-for="feed in store.feeds" :key="feed.id" class="flex items-center p-3 border-b border-border last:border-0 bg-bg-primary hover:bg-bg-secondary">
                                <input type="checkbox" :value="feed.id" v-model="selectedFeeds" class="mr-3 rounded border-border">
                                <div class="truncate flex-1 mr-2">
                                    <div class="font-medium truncate">{{ feed.title }}</div>
                                    <div class="text-xs text-text-secondary truncate">{{ feed.url }}</div>
                                </div>
                                <button @click="editFeed(feed)" class="text-accent hover:bg-bg-tertiary p-1.5 rounded mr-1" title="Edit"><i class="ph ph-pencil"></i></button>
                                <button @click="deleteFeed(feed.id)" class="text-red-500 hover:bg-red-50 p-1.5 rounded" title="Delete"><i class="ph ph-trash"></i></button>
                            </div>
                        </div>
                    </div>
                </div>

                <div v-if="activeTab === 'about'" class="text-center py-10">
                    <img src="/assets/logo.svg" alt="Logo" class="h-16 w-auto mb-4 mx-auto">
                    <h3 class="text-xl font-bold mb-2">MrRSS</h3>
                    <p class="text-text-secondary">A simple, modern RSS reader.</p>
                    <p class="text-text-secondary text-sm mt-2">Version 1.0.0</p>
                </div>
            </div>

            <div class="p-5 border-t border-border bg-bg-secondary text-right shrink-0">
                <button @click="saveSettings" class="btn-primary">Save Settings</button>
            </div>
        </div>
    </div>
</template>

<style scoped>
.tab-btn {
    @apply px-5 py-3 bg-transparent border-b-2 border-transparent text-text-secondary font-semibold cursor-pointer hover:text-text-primary transition-colors;
}
.tab-btn.active {
    @apply text-accent border-accent;
}
.input-field {
    @apply w-full p-2.5 border border-border rounded-md bg-bg-secondary text-text-primary text-sm focus:border-accent focus:outline-none transition-colors;
}
.btn-primary {
    @apply bg-accent text-white border-none px-5 py-2.5 rounded-lg cursor-pointer font-semibold hover:bg-accent-hover transition-colors;
}
.btn-secondary {
    @apply bg-transparent border border-border text-text-primary px-4 py-2 rounded-md cursor-pointer flex items-center gap-2 font-medium hover:bg-bg-tertiary transition-colors;
}
.toggle {
    @apply w-10 h-5 appearance-none bg-bg-tertiary rounded-full relative cursor-pointer border border-border transition-colors checked:bg-accent checked:border-accent;
}
.toggle::after {
    content: '';
    @apply absolute top-0.5 left-0.5 w-3.5 h-3.5 bg-white rounded-full shadow-sm transition-transform;
}
.toggle:checked::after {
    transform: translateX(20px);
}
.animate-fade-in {
    animation: modalFadeIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes modalFadeIn {
    from { transform: translateY(-20px); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
}
</style>
