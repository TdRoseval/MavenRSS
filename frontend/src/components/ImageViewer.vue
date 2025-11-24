<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { PhX, PhMagnifyingGlassMinus, PhMagnifyingGlassPlus } from "@phosphor-icons/vue";

const props = defineProps({
    src: { type: String, required: true },
    alt: { type: String, default: '' }
});

const emit = defineEmits(['close']);

const scale = ref(1);
const position = ref({ x: 0, y: 0 });
const isDragging = ref(false);
const dragStart = ref({ x: 0, y: 0 });
const imageRef = ref(null);

const MIN_SCALE = 0.5;
const MAX_SCALE = 5;
const SCALE_STEP = 0.25;

function close() {
    emit('close');
}

function zoomIn() {
    if (scale.value < MAX_SCALE) {
        scale.value = Math.min(scale.value + SCALE_STEP, MAX_SCALE);
    }
}

function zoomOut() {
    if (scale.value > MIN_SCALE) {
        scale.value = Math.max(scale.value - SCALE_STEP, MIN_SCALE);
        // Reset position if zooming out to 1 or less
        if (scale.value <= 1) {
            position.value = { x: 0, y: 0 };
        }
    }
}

function handleWheel(e) {
    e.preventDefault();
    if (e.deltaY < 0) {
        zoomIn();
    } else {
        zoomOut();
    }
}

function startDrag(e) {
    if (scale.value > 1) {
        isDragging.value = true;
        dragStart.value = {
            x: e.clientX - position.value.x,
            y: e.clientY - position.value.y
        };
    }
}

function onDrag(e) {
    if (isDragging.value && scale.value > 1) {
        position.value = {
            x: e.clientX - dragStart.value.x,
            y: e.clientY - dragStart.value.y
        };
    }
}

function stopDrag() {
    isDragging.value = false;
}

function handleKeyDown(e) {
    if (e.key === 'Escape') {
        close();
    } else if (e.key === '+' || e.key === '=') {
        zoomIn();
    } else if (e.key === '-' || e.key === '_') {
        zoomOut();
    }
}

const imageStyle = computed(() => ({
    transform: `translate(${position.value.x}px, ${position.value.y}px) scale(${scale.value})`,
    cursor: scale.value > 1 ? (isDragging.value ? 'grabbing' : 'grab') : 'default'
}));

onMounted(() => {
    document.addEventListener('keydown', handleKeyDown);
    document.addEventListener('mousemove', onDrag);
    document.addEventListener('mouseup', stopDrag);
});

onUnmounted(() => {
    document.removeEventListener('keydown', handleKeyDown);
    document.removeEventListener('mousemove', onDrag);
    document.removeEventListener('mouseup', stopDrag);
});
</script>

<template>
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/90 backdrop-blur-sm" @click="close">
        <!-- Controls -->
        <div class="absolute top-4 right-4 flex gap-2 z-10" @click.stop>
            <button @click="zoomOut" class="control-btn" :disabled="scale <= MIN_SCALE" :title="store.i18n?.t ? store.i18n.t('zoomOut') : 'Zoom Out'">
                <PhMagnifyingGlassMinus :size="20" />
            </button>
            <span class="control-btn pointer-events-none">{{ Math.round(scale * 100) }}%</span>
            <button @click="zoomIn" class="control-btn" :disabled="scale >= MAX_SCALE" :title="store.i18n?.t ? store.i18n.t('zoomIn') : 'Zoom In'">
                <PhMagnifyingGlassPlus :size="20" />
            </button>
            <button @click="close" class="control-btn" :title="store.i18n?.t ? store.i18n.t('close') : 'Close'">
                <PhX :size="20" />
            </button>
        </div>

        <!-- Image Container -->
        <div class="relative w-full h-full flex items-center justify-center overflow-hidden" @click.stop @wheel="handleWheel">
            <img
                ref="imageRef"
                :src="src"
                :alt="alt"
                :style="imageStyle"
                @mousedown="startDrag"
                @dragstart.prevent
                class="max-w-full max-h-full object-contain transition-transform select-none"
                :class="{ 'transition-none': isDragging }"
            />
        </div>

        <!-- Help text -->
        <div class="absolute bottom-4 left-1/2 -translate-x-1/2 text-white/70 text-sm text-center px-4">
            <p class="hidden sm:block">{{ store.i18n?.t ? store.i18n.t('imageViewerHelp') : 'Use mouse wheel or +/- keys to zoom • Drag to move • ESC to close' }}</p>
        </div>
    </div>
</template>

<style scoped>
.control-btn {
    @apply bg-bg-primary text-text-primary px-3 py-2 rounded-lg hover:bg-bg-primary transition-colors backdrop-blur-sm flex items-center justify-center min-w-[40px];
    background-color: rgba(var(--color-bg-primary-rgb, 255, 255, 255), 0.9);
}

.control-btn:disabled {
    @apply opacity-50 cursor-not-allowed;
}

.control-btn:not(:disabled):hover {
    @apply bg-bg-secondary;
}
</style>
