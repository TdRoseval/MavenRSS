<script setup lang="ts">
import { ref, computed } from 'vue';
import { PhSpeakerHigh, PhPlay, PhPause } from '@phosphor-icons/vue';
import { useI18n } from 'vue-i18n';

interface Props {
  audioUrl: string;
  articleTitle: string;
}

defineProps<Props>();

const { t } = useI18n();

const audioRef = ref<HTMLAudioElement | null>(null);
const isPlaying = ref(false);
const currentTime = ref(0);
const duration = ref(0);

// Format time in MM:SS format
function formatTime(seconds: number): string {
  if (!isFinite(seconds)) return '0:00';
  const mins = Math.floor(seconds / 60);
  const secs = Math.floor(seconds % 60);
  return `${mins}:${secs.toString().padStart(2, '0')}`;
}

// Toggle play/pause
async function togglePlay() {
  if (!audioRef.value) return;

  if (isPlaying.value) {
    audioRef.value.pause();
  } else {
    try {
      await audioRef.value.play();
    } catch (error) {
      console.error('Failed to play audio:', error);
      window.showToast(t('audioPlaybackError'), 'error');
    }
  }
}

// Handle audio events
function onPlay() {
  isPlaying.value = true;
}

function onPause() {
  isPlaying.value = false;
}

function onTimeUpdate() {
  if (!audioRef.value) return;
  currentTime.value = audioRef.value.currentTime;
}

function onLoadedMetadata() {
  if (!audioRef.value) return;
  duration.value = audioRef.value.duration;
}

function onEnded() {
  isPlaying.value = false;
  currentTime.value = 0;
}

// Seek to position
function seek(event: MouseEvent) {
  if (!audioRef.value) return;
  const progressBar = event.currentTarget as HTMLElement;
  const rect = progressBar.getBoundingClientRect();
  const clickX = event.clientX - rect.left;
  const percentage = clickX / rect.width;
  audioRef.value.currentTime = percentage * duration.value;
}

// Computed progress percentage
const progressPercentage = computed(() => {
  if (!duration.value) return 0;
  return (currentTime.value / duration.value) * 100;
});

// Extract filename from audio URL
const downloadFilename = computed(() => {
  try {
    const url = new URL(props.audioUrl);
    const pathname = url.pathname;
    const filename = pathname.substring(pathname.lastIndexOf('/') + 1);
    // If filename has no extension or is empty, use article title with .mp3
    if (!filename || !filename.includes('.')) {
      return `${props.articleTitle}.mp3`;
    }
    return filename;
  } catch {
    // Fallback if URL parsing fails
    return `${props.articleTitle}.mp3`;
  }
});
</script>

<template>
  <div class="bg-bg-secondary border border-border rounded-lg p-4 mb-4 sm:mb-6">
    <div class="flex items-center gap-3 mb-3">
      <PhSpeakerHigh :size="20" class="text-accent flex-shrink-0" />
      <span class="text-sm font-medium text-text-primary">{{ t('podcastAudio') }}</span>
    </div>

    <!-- Audio element (hidden) -->
    <audio
      ref="audioRef"
      :src="audioUrl"
      @play="onPlay"
      @pause="onPause"
      @timeupdate="onTimeUpdate"
      @loadedmetadata="onLoadedMetadata"
      @ended="onEnded"
      preload="metadata"
    />

    <!-- Custom audio controls -->
    <div class="flex items-center gap-3">
      <!-- Play/Pause button -->
      <button
        @click="togglePlay"
        class="flex items-center justify-center w-10 h-10 rounded-full bg-accent hover:bg-accent/90 transition-colors flex-shrink-0"
        :title="isPlaying ? t('pause') : t('play')"
      >
        <PhPlay v-if="!isPlaying" :size="20" class="text-white ml-0.5" />
        <PhPause v-else :size="20" class="text-white" />
      </button>

      <!-- Progress bar -->
      <div class="flex-1 flex items-center gap-2">
        <span class="text-xs text-text-secondary min-w-[40px] text-right">{{
          formatTime(currentTime)
        }}</span>
        <div class="flex-1 h-1.5 bg-bg-tertiary rounded-full cursor-pointer relative" @click="seek">
          <div
            class="h-full bg-accent rounded-full transition-all duration-100"
            :style="{ width: `${progressPercentage}%` }"
          />
        </div>
        <span class="text-xs text-text-secondary min-w-[40px]">{{ formatTime(duration) }}</span>
      </div>
    </div>

    <!-- Download link -->
    <div class="mt-3 pt-3 border-t border-border">
      <a
        :href="audioUrl"
        :download="downloadFilename"
        class="text-xs text-accent hover:underline flex items-center gap-1"
        target="_blank"
      >
        {{ t('downloadAudio') }}
      </a>
    </div>
  </div>
</template>
