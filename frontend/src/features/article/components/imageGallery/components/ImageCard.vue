<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { PhImage, PhHeart } from '@phosphor-icons/vue';
import { computed } from 'vue';
import type { Article } from '@/types/models';
import { getProxiedMediaUrl } from '@/shared/lib/mediaProxy';

interface Props {
  article: Article;
  imageCount: number;
  showTextOverlay: boolean;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  click: [];
  favorite: [event: Event];
  contextMenu: [event: MouseEvent];
}>();

/**
 * Get proxied image URL for cover image
 * Cover images (image_url) are always cached to ensure they display correctly
 * This prevents hotlinking issues and ensures consistent loading
 */
const proxiedImageUrl = computed(() => {
  // Always use proxy with force_cache=true for cover images
  // Cover images are the main image shown in the gallery grid
  return getProxiedMediaUrl(props.article.image_url, undefined, true);
});

/**
 * Handle favorite button click
 * Emits the favorite event and closes any open context menu
 */
function handleFavoriteClick(event: Event): void {
  emit('favorite', event);
  // Close any open context menu by dispatching a click event to document
  // The ContextMenu component's handleClickOutside will catch this
  document.dispatchEvent(new MouseEvent('click', { bubbles: true }));
}

/**
 * Format date for display
 * @param dateString - ISO date string
 * @returns Formatted date string
 */
function formatDate(dateString: string): string {
  const { t } = useI18n();
  const date = new Date(dateString);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  const days = Math.floor(diff / (1000 * 60 * 60 * 24));

  if (days === 0) {
    const hours = Math.floor(diff / (1000 * 60 * 60));
    if (hours === 0) {
      const minutes = Math.floor(diff / (1000 * 60));
      return minutes <= 0
        ? t('common.time.justNow')
        : t('common.time.minutesAgo', { count: minutes });
    }
    return t('common.time.hoursAgo', { count: hours });
  } else if (days < 7) {
    return t('common.time.daysAgo', { count: days });
  }
  return date.toLocaleDateString();
}
</script>

<template>
  <div
    class="cursor-pointer group"
    @click="emit('click')"
    @contextmenu="emit('contextMenu', $event)"
  >
    <!-- Image container -->
    <div
      class="relative overflow-hidden rounded-lg bg-bg-secondary transition-transform duration-200 hover:scale-[1.02]"
    >
      <img :src="proxiedImageUrl" :alt="article.title" class="w-full h-auto block" loading="lazy" />

      <!-- Image count indicator -->
      <div
        v-if="imageCount > 1"
        class="absolute bottom-2 left-2 px-2 py-1 rounded-full bg-black/60 text-white text-xs font-semibold backdrop-blur-sm z-10 flex items-center gap-1 transition-all duration-200"
        :class="{ 'group-hover:bottom-20': !showTextOverlay }"
      >
        <PhImage :size="14" />
        <span class="ml-1">{{ imageCount }}</span>
      </div>

      <!-- Favorite button overlay -->
      <div
        class="absolute inset-0 bg-black/0 hover:bg-black/30 transition-all duration-200 flex items-start justify-end p-2"
      >
        <button
          class="opacity-0 group-hover:opacity-100 transition-opacity duration-200 bg-black/50 rounded-full p-1.5 hover:bg-black/70"
          @click="handleFavoriteClick($event)"
        >
          <PhHeart
            :size="20"
            :weight="article.is_favorite ? 'fill' : 'regular'"
            :class="article.is_favorite ? 'text-red-500' : 'text-white'"
          />
        </button>
      </div>

      <!-- Hover overlay when text is hidden -->
      <div
        v-if="!showTextOverlay"
        class="absolute inset-x-0 bottom-0 p-3 bg-gradient-to-t from-black/80 via-black/50 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-200"
      >
        <p class="text-sm font-medium text-white line-clamp-2 mb-1">
          {{ article.title }}
        </p>
        <div class="flex items-center justify-between text-xs text-white/80">
          <span class="truncate flex-1">{{ article.feed_title }}</span>
          <span class="ml-2 shrink-0">{{ formatDate(article.published_at) }}</span>
        </div>
      </div>
    </div>

    <!-- Text info (always shown when showTextOverlay is true) -->
    <div v-if="showTextOverlay" class="p-2">
      <p class="text-sm font-medium text-text-primary line-clamp-2 mb-1">
        {{ article.title }}
      </p>
      <div class="flex items-center justify-between text-xs text-text-secondary">
        <span class="truncate flex-1">{{ article.feed_title }}</span>
        <span class="ml-2 shrink-0">{{ formatDate(article.published_at) }}</span>
      </div>
    </div>
  </div>
</template>
