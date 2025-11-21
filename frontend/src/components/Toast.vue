<script setup>
import { ref, onMounted } from 'vue';

const props = defineProps({
    message: { type: String, required: true },
    type: { type: String, default: 'info' }, // 'info', 'success', 'error', 'warning'
    duration: { type: Number, default: 3000 }
});

const emit = defineEmits(['close']);

const show = ref(true);

onMounted(() => {
    if (props.duration > 0) {
        setTimeout(() => {
            show.value = false;
            setTimeout(() => emit('close'), 300);
        }, props.duration);
    }
});

function handleClose() {
    show.value = false;
    setTimeout(() => emit('close'), 300);
}
</script>

<template>
    <div v-if="show" :class="['toast', `toast-${type}`, show ? 'toast-show' : 'toast-hide']">
        <div class="flex items-center gap-3">
            <i v-if="type === 'success'" class="ph ph-check-circle text-xl"></i>
            <i v-else-if="type === 'error'" class="ph ph-x-circle text-xl"></i>
            <i v-else-if="type === 'warning'" class="ph ph-warning text-xl"></i>
            <i v-else class="ph ph-info text-xl"></i>
            <span class="flex-1">{{ message }}</span>
            <button @click="handleClose" class="text-xl opacity-70 hover:opacity-100 transition-opacity">
                <i class="ph ph-x"></i>
            </button>
        </div>
    </div>
</template>

<style scoped>
.toast {
    @apply fixed top-5 right-5 z-[60] px-5 py-3 rounded-lg shadow-lg border min-w-[300px] max-w-md;
}
.toast-show {
    animation: slideIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}
.toast-hide {
    animation: slideOut 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}
.toast-info {
    @apply bg-blue-50 border-blue-200 text-blue-900;
}
.dark-mode .toast-info {
    @apply bg-blue-900/20 border-blue-700 text-blue-100;
}
.toast-success {
    @apply bg-green-50 border-green-200 text-green-900;
}
.dark-mode .toast-success {
    @apply bg-green-900/20 border-green-700 text-green-100;
}
.toast-error {
    @apply bg-red-50 border-red-200 text-red-900;
}
.dark-mode .toast-error {
    @apply bg-red-900/20 border-red-700 text-red-100;
}
.toast-warning {
    @apply bg-orange-50 border-orange-200 text-orange-900;
}
.dark-mode .toast-warning {
    @apply bg-orange-900/20 border-orange-700 text-orange-100;
}
@keyframes slideIn {
    from { transform: translateX(400px); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
}
@keyframes slideOut {
    from { transform: translateX(0); opacity: 1; }
    to { transform: translateX(400px); opacity: 0; }
}
</style>
