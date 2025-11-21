<script setup>
import { ref } from 'vue';

const isOpen = ref(false);
const url = ref('');
const category = ref('');
const isSubmitting = ref(false);

const emit = defineEmits(['close', 'added']);

function close() {
    emit('close');
}

async function addFeed() {
    if (!url.value) return;
    isSubmitting.value = true;
    
    try {
        const res = await fetch('/api/feeds/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url: url.value, category: category.value })
        });
        
        if (res.ok) {
            emit('added');
            url.value = '';
            category.value = '';
            window.showToast('Feed added successfully', 'success');
            close();
        } else {
            window.showToast('Error adding feed', 'error');
        }
    } catch (e) {
        console.error(e);
        window.showToast('Error adding feed', 'error');
    } finally {
        isSubmitting.value = false;
    }
}
</script>

<template>
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
        <div class="bg-bg-primary w-full max-w-md rounded-2xl shadow-2xl border border-border overflow-hidden animate-fade-in">
            <div class="p-5 border-b border-border flex justify-between items-center">
                <h3 class="text-lg font-semibold m-0">Add New Feed</h3>
                <span @click="close" class="text-2xl cursor-pointer text-text-secondary hover:text-text-primary">&times;</span>
            </div>
            <div class="p-6">
                <div class="mb-4">
                    <label class="block mb-1.5 font-semibold text-sm text-text-secondary">RSS URL</label>
                    <input v-model="url" type="text" placeholder="https://example.com/rss" class="input-field">
                </div>
                <div class="mb-4">
                    <label class="block mb-1.5 font-semibold text-sm text-text-secondary">Category (Optional)</label>
                    <input v-model="category" type="text" placeholder="e.g. Tech/News" class="input-field">
                </div>
            </div>
            <div class="p-5 border-t border-border bg-bg-secondary text-right">
                <button @click="addFeed" :disabled="isSubmitting" class="btn-primary">
                    {{ isSubmitting ? 'Adding...' : 'Add Subscription' }}
                </button>
            </div>
        </div>
    </div>
</template>

<style scoped>
.input-field {
    @apply w-full p-2.5 border border-border rounded-md bg-bg-secondary text-text-primary text-sm focus:border-accent focus:outline-none transition-colors;
}
.btn-primary {
    @apply bg-accent text-white border-none px-5 py-2.5 rounded-lg cursor-pointer font-semibold hover:bg-accent-hover transition-colors disabled:opacity-70;
}
.animate-fade-in {
    animation: modalFadeIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes modalFadeIn {
    from { transform: translateY(-20px); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
}
</style>
