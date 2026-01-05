<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, type Ref, type CSSProperties } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  PhX,
  PhMagnifyingGlassMinus,
  PhMagnifyingGlassPlus,
  PhDownloadSimple,
} from '@phosphor-icons/vue';

const { t } = useI18n();

interface Props {
  src: string;
  alt?: string;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  close: [];
}>();

interface Position {
  x: number;
  y: number;
}

const scale = ref(1);
const position = ref<Position>({ x: 0, y: 0 });
const isDragging = ref(false);
const dragStart = ref<Position>({ x: 0, y: 0 });
const imageRef: Ref<HTMLImageElement | null> = ref(null);

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

function handleWheel(e: WheelEvent) {
  e.preventDefault();
  if (e.deltaY < 0) {
    zoomIn();
  } else {
    zoomOut();
  }
}

function startDrag(e: MouseEvent) {
  isDragging.value = true;
  dragStart.value = {
    x: e.clientX - position.value.x,
    y: e.clientY - position.value.y,
  };
}

function onDrag(e: MouseEvent) {
  if (isDragging.value) {
    position.value = {
      x: e.clientX - dragStart.value.x,
      y: e.clientY - dragStart.value.y,
    };
  }
}

function stopDrag() {
  isDragging.value = false;
}

function handleKeyDown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    close();
  } else if (e.key === '+' || e.key === '=') {
    zoomIn();
  } else if (e.key === '-' || e.key === '_') {
    zoomOut();
  } else if (e.key === 's' && (e.ctrlKey || e.metaKey)) {
    e.preventDefault();
    downloadImage();
  }
}

// Download image to local storage
async function downloadImage() {
  try {
    const response = await fetch(props.src);

    // Check if the response is successful
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const blob = await response.blob();

    // Extract and sanitize filename from URL
    let filename = 'image';
    try {
      const url = new URL(props.src);
      const pathname = url.pathname;
      const pathSegments = pathname.split('/').filter((segment) => segment.length > 0);
      if (pathSegments.length > 0) {
        const lastSegment = pathSegments[pathSegments.length - 1];
        // Remove query params and sanitize filename
        filename = lastSegment.split('?')[0].replace(/[^a-zA-Z0-9._-]/g, '_') || 'image';
      }
    } catch {
      // If URL parsing fails, use default filename
      filename = 'image';
    }

    // Ensure it has a valid extension based on MIME type
    if (!filename.match(/\.(jpg|jpeg|png|gif|webp|svg|bmp)$/i)) {
      const mimeType = blob.type;
      const ext = mimeType.split('/')[1]?.replace('jpeg', 'jpg') || 'png';
      filename = `${filename}.${ext}`;
    }

    // Create download link
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  } catch (error) {
    console.error('Failed to download image:', error);
    // Fallback: open image in new tab for manual saving
    window.open(props.src, '_blank');
  }
}

const imageStyle = computed<CSSProperties>(() => ({
  transform: `translate(${position.value.x}px, ${position.value.y}px) scale(${scale.value})`,
}));

onMounted(() => {
  document.addEventListener('keydown', handleKeyDown);
});

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeyDown);
});
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/90 backdrop-blur-sm">
    <!-- Controls -->
    <div class="absolute top-4 right-4 flex gap-2 z-10" @click.stop>
      <button
        class="control-btn"
        :disabled="scale <= MIN_SCALE"
        :title="t('zoomOut')"
        @click="zoomOut"
      >
        <PhMagnifyingGlassMinus :size="20" />
      </button>
      <span class="control-btn pointer-events-none">{{ Math.round(scale * 100) }}%</span>
      <button
        class="control-btn"
        :disabled="scale >= MAX_SCALE"
        :title="t('zoomIn')"
        @click="zoomIn"
      >
        <PhMagnifyingGlassPlus :size="20" />
      </button>
      <button class="control-btn" :title="t('downloadImage')" @click="downloadImage">
        <PhDownloadSimple :size="20" />
      </button>
      <button class="control-btn" :title="t('close')" @click="close">
        <PhX :size="20" />
      </button>
    </div>

    <!-- Image Container -->
    <div
      class="relative w-full h-full flex items-center justify-center overflow-hidden image-container"
      :class="{ 'cursor-grab': !isDragging, 'cursor-grabbing': isDragging }"
      @click.stop
      @wheel="handleWheel"
      @mousedown="startDrag"
      @mousemove="onDrag"
      @mouseup="stopDrag"
      @mouseleave="stopDrag"
    >
      <img
        ref="imageRef"
        :src="src"
        :alt="alt"
        :style="imageStyle"
        class="max-w-full max-h-full object-contain select-none"
        :class="[isDragging ? '' : 'transition-transform duration-150']"
        @dragstart.prevent
      />
    </div>

    <!-- Help text -->
    <div class="absolute bottom-4 left-1/2 -translate-x-1/2 text-white/70 text-sm text-center px-4">
      <p class="hidden sm:block">{{ t('imageViewerHelpExtended') }}</p>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.control-btn {
  @apply px-3 py-2 rounded-lg transition-colors backdrop-blur-sm flex items-center justify-center min-w-[40px];
  background-color: rgba(255, 255, 255, 0.9);
  color: #212529;
}

/* Dark mode support */
:global(.dark-mode) .control-btn {
  background-color: rgba(45, 45, 45, 0.9);
  color: #e0e0e0;
}

.control-btn:disabled {
  @apply opacity-50 cursor-not-allowed;
}

.control-btn:not(:disabled):hover {
  background-color: rgba(240, 240, 240, 0.95);
}

:global(.dark-mode) .control-btn:not(:disabled):hover {
  background-color: rgba(60, 60, 60, 0.95);
}

/* Image container cursor */
.image-container {
  cursor: default;
}

.image-container.cursor-grab {
  cursor: grab;
}

.image-container.cursor-grabbing {
  cursor: grabbing;
}
</style>
