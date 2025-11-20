<script setup>
import { ref, onMounted, onUnmounted } from 'vue';

const props = defineProps({
    items: { type: Array, required: true }, // [{ label: 'Edit', action: 'edit', icon: 'ph-pencil' }, { separator: true }]
    x: { type: Number, required: true },
    y: { type: Number, required: true }
});

const emit = defineEmits(['close', 'action']);
const menuRef = ref(null);

function handleClickOutside(event) {
    if (menuRef.value && !menuRef.value.contains(event.target)) {
        emit('close');
    }
}

onMounted(() => {
    // Use setTimeout to avoid catching the event that opened the menu
    setTimeout(() => {
        document.addEventListener('click', handleClickOutside);
        document.addEventListener('contextmenu', handleClickOutside);
    }, 0);
});

onUnmounted(() => {
    document.removeEventListener('click', handleClickOutside);
    document.removeEventListener('contextmenu', handleClickOutside);
});

function handleAction(item) {
    if (item.disabled) return;
    emit('action', item.action);
    emit('close');
}
</script>

<template>
    <div ref="menuRef" class="fixed z-50 bg-bg-primary border border-border rounded-lg shadow-xl py-1 min-w-[180px] animate-fade-in"
         :style="{ top: `${y}px`, left: `${x}px` }">
        <template v-for="(item, index) in items" :key="index">
            <div v-if="item.separator" class="h-px bg-border my-1"></div>
            <div v-else 
                 @click="handleAction(item)"
                 class="px-4 py-2 flex items-center gap-3 cursor-pointer hover:bg-bg-tertiary text-sm text-text-primary transition-colors"
                 :class="{ 'opacity-50 cursor-not-allowed': item.disabled }">
                <i v-if="item.icon" :class="['ph', item.icon, 'text-text-secondary text-lg']"></i>
                <span>{{ item.label }}</span>
            </div>
        </template>
    </div>
</template>

<style scoped>
.animate-fade-in {
    animation: fadeIn 0.1s ease-out;
}
@keyframes fadeIn {
    from { opacity: 0; transform: scale(0.95); }
    to { opacity: 1; transform: scale(1); }
}
</style>
