<script setup>
import { store } from '../store.js';
import { computed } from 'vue';

const article = computed(() => store.articles.find(a => a.id === store.currentArticleId));

function close() {
    store.currentArticleId = null;
}

function toggleRead() {
    if (!article.value) return;
    const newState = !article.value.is_read;
    article.value.is_read = newState;
    fetch(`/api/articles/read?id=${article.value.id}&read=${newState}`, { method: 'POST' });
}

function toggleFavorite() {
    if (!article.value) return;
    const newState = !article.value.is_favorite;
    article.value.is_favorite = newState;
    fetch(`/api/articles/favorite?id=${article.value.id}`, { method: 'POST' });
}

function openOriginal() {
    if (article.value) window.open(article.value.url, '_blank');
}
</script>

<template>
    <main :class="['flex-1 bg-bg-primary flex flex-col h-full absolute w-full md:static md:w-auto z-30 transition-transform duration-300', article ? 'translate-x-0' : 'translate-x-full md:translate-x-0']">
        <div v-if="!article" class="hidden md:flex flex-col items-center justify-center h-full text-text-secondary text-center">
            <i class="ph ph-newspaper text-5xl mb-5 opacity-50"></i>
            <p>Select an article to start reading</p>
        </div>

        <div v-else class="flex flex-col h-full bg-bg-primary">
            <div class="h-[50px] px-5 border-b border-border flex justify-between items-center bg-bg-primary shrink-0">
                <button @click="close" class="md:hidden flex items-center gap-2 text-text-secondary hover:text-text-primary">
                    <i class="ph ph-arrow-left"></i> Back
                </button>
                <div class="flex gap-2 ml-auto">
                    <button @click="toggleRead" class="action-btn" :title="article.is_read ? 'Mark as Unread' : 'Mark as Read'">
                        <i :class="['ph', article.is_read ? 'ph-envelope-open' : 'ph-envelope']"></i>
                    </button>
                    <button @click="toggleFavorite" :class="['action-btn', article.is_favorite ? 'text-yellow-400' : '']" title="Toggle Favorite">
                        <i :class="['ph', article.is_favorite ? 'ph-star-fill' : 'ph-star']"></i>
                    </button>
                    <button @click="openOriginal" class="action-btn" title="Open in Browser">
                        <i class="ph ph-arrow-square-out"></i>
                    </button>
                </div>
            </div>
            <div class="flex-1 bg-white w-full">
                <iframe :src="article.url" class="w-full h-full border-none" sandbox="allow-scripts allow-same-origin allow-popups"></iframe>
            </div>
        </div>
    </main>
</template>

<style scoped>
.action-btn {
    @apply text-xl cursor-pointer text-text-secondary p-1.5 rounded-md transition-colors hover:bg-bg-tertiary hover:text-text-primary;
}
</style>
