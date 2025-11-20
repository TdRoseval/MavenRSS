<script setup>
import { ref, onMounted } from 'vue';

const props = defineProps({
    feed: { type: Object, required: true }
});

const emit = defineEmits(['close', 'updated']);

const title = ref('');
const url = ref('');
const category = ref('');
const isSubmitting = ref(false);

onMounted(() => {
    title.value = props.feed.title;
    url.value = props.feed.url;
    category.value = props.feed.category;
});

function close() {
    emit('close');
}

async function save() {
    if (!url.value) return;
    isSubmitting.value = true;
    
    try {
        const res = await fetch('/api/feeds/update', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ 
                id: props.feed.id, 
                title: title.value, 
                url: url.value, 
                category: category.value 
            })
        });
        
        if (res.ok) {
            emit('updated');
            close();
        } else {
            alert('Error updating feed');
        }
    } catch (e) {
        console.error(e);
        alert('Error updating feed');
    } finally {
        isSubmitting.value = false;
    }
}
</script>

<template>
    <div class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 backdrop-blur-sm">
        <div class="bg-bg-primary w-[450px] rounded-2xl shadow-2xl border border-border overflow-hidden animate-fade-in">
            <div class="p-5 border-b border-border flex justify-between items-center">
                <h3 class="text-lg font-semibold m-0">Edit Feed</h3>
                <span @click="close" class="text-2xl cursor-pointer text-text-secondary hover:text-text-primary">&times;</span>
            </div>
            <div class="p-6">
                <div class="mb-4">
                    <label class="block mb-1.5 font-semibold text-sm text-text-secondary">Title</label>
                    <input v-model="title" type="text" class="input-field">
                </div>
                <div class="mb-4">
                    <label class="block mb-1.5 font-semibold text-sm text-text-secondary">RSS URL</label>
                    <input v-model="url" type="text" class="input-field">
                </div>
                <div class="mb-4">
                    <label class="block mb-1.5 font-semibold text-sm text-text-secondary">Category</label>
                    <input v-model="category" type="text" placeholder="e.g. Tech/News" class="input-field">
                </div>
            </div>
            <div class="p-5 border-t border-border bg-bg-secondary text-right">
                <button @click="save" :disabled="isSubmitting" class="btn-primary">
                    {{ isSubmitting ? 'Saving...' : 'Save Changes' }}
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
