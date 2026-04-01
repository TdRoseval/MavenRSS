<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import { useI18n } from 'vue-i18n';
import { PhEyeSlash, PhStar, PhClockCountdown } from '@phosphor-icons/vue';
import type { Cluster } from '@/types/models';
import { formatDate as formatDateUtil } from '@/shared/lib/date';
import { getProxiedMediaUrl, isMediaCacheEnabled } from '@/shared/lib/mediaProxy';
import { useShowPreviewImages } from '@/composables/ui/useShowPreviewImages';
import { useAuthStore } from '@/stores/auth';
import { useSettings } from '@/composables/core/useSettings';
import { apiClient } from '@/shared/lib/apiClient';
import { imageCache } from '@/shared/lib/imageCache';

interface Props {
  cluster: Cluster;
  isActive: boolean;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  click: [];
  contextmenu: [event: MouseEvent];
  observeElement: [element: Element | null];
  hoverMarkAsRead: [clusterId: number];
}>();

const { t, locale } = useI18n();
const { showPreviewImages } = useShowPreviewImages();
const { settings } = useSettings();
const authStore = useAuthStore();

const compactMode = computed(() => {
  return settings.value.layout_mode === 'compact';
});
const feedTitlesText = computed(() => props.cluster.feed_titles?.filter(Boolean).join(' · ') || '');
const authorsText = computed(() => props.cluster.authors?.filter(Boolean).join(' · ') || '');
const clusterImageSource = computed(() => {
  if (props.cluster.image_url) {
    return props.cluster.image_url;
  }

  return props.cluster.articles?.find((article) => article.image_url)?.image_url || '';
});

const formatDateWithI18n = (dateStr: string): string => {
  return formatDateUtil(dateStr, locale.value, t);
};

const mediaCacheEnabled = ref(false);
const hoverMarkAsRead = ref(false);
const imageFailed = ref(false);
const imageLoading = ref(true);
const imageInViewport = ref(false);
const imageContainerRef = ref<HTMLDivElement | null>(null);
let hoverTimeout: ReturnType<typeof setTimeout> | null = null;
let sharedObserver: IntersectionObserver | null = null;
const observerTargets = new WeakMap<Element, () => void>();

const imageUrl = computed(() => {
  if (!clusterImageSource.value) return '';

  const finalUrl = mediaCacheEnabled.value
    ? getProxiedMediaUrl(clusterImageSource.value)
    : clusterImageSource.value;

  return imageCache.getImageUrl(finalUrl);
});

const shouldShowImage = computed(() => {
  return showPreviewImages.value && !!clusterImageSource.value && !imageFailed.value;
});

onMounted(() => {
  if ('IntersectionObserver' in window && imageContainerRef.value) {
    if (!sharedObserver) {
      sharedObserver = new IntersectionObserver(
        (entries) => {
          entries.forEach((entry) => {
            const callback = observerTargets.get(entry.target);
            if (callback && entry.isIntersecting) {
              callback();
            }
          });
        },
        {
          rootMargin: '200px',
          threshold: 0,
        }
      );
    }

    const callback = () => {
      imageInViewport.value = true;
      if (sharedObserver && imageContainerRef.value) {
        sharedObserver.unobserve(imageContainerRef.value);
        observerTargets.delete(imageContainerRef.value);
      }
    };

    observerTargets.set(imageContainerRef.value, callback);
    sharedObserver.observe(imageContainerRef.value);
  } else {
    imageInViewport.value = true;
  }

  isMediaCacheEnabled().then((enabled) => {
    mediaCacheEnabled.value = enabled;
  });
});

onBeforeUnmount(() => {
  if (hoverTimeout) {
    clearTimeout(hoverTimeout);
  }
  if (sharedObserver && imageContainerRef.value) {
    sharedObserver.unobserve(imageContainerRef.value);
    observerTargets.delete(imageContainerRef.value);
  }
});

function handleImageLoad(event: Event) {
  const target = event.target as HTMLImageElement;
  const url = target.src;

  imageCache.markAsLoaded(url);
  imageLoading.value = false;
  imageFailed.value = false;
  target.style.opacity = '1';
}

function handleImageError(event: Event) {
  const target = event.target as HTMLImageElement;
  const url = target.src;

  imageLoading.value = false;
  imageFailed.value = true;
  imageCache.handleLoadError(url);
}

function handleMouseEnter() {
  if (!hoverMarkAsRead.value || props.cluster.is_read || props.cluster.is_read_later) {
    return;
  }

  hoverTimeout = setTimeout(() => {
    markAsRead();
  }, 300);
}

function handleMouseLeave() {
  if (hoverTimeout) {
    clearTimeout(hoverTimeout);
    hoverTimeout = null;
  }
}

async function markAsRead() {
  if (!authStore.isAuthenticated || props.cluster.is_read) return;

  try {
    await apiClient.put('/clusters/read', { id: props.cluster.id, read: true });
    emit('hoverMarkAsRead', props.cluster.id);
  } catch (e) {
    console.error('Error marking as read on hover:', e);
  }
}
</script>

<template>
  <div
    :ref="(el) => emit('observeElement', el as Element | null)"
    class="article-card"
    :class="[
      cluster.is_read ? 'read' : '',
      cluster.is_favorite ? 'favorite' : '',
      cluster.is_hidden ? 'hidden' : '',
      cluster.is_read_later ? 'read-later' : '',
      isActive ? 'active' : '',
      compactMode ? 'compact' : '',
    ]"
    @click="emit('click')"
    @contextmenu="emit('contextmenu', $event)"
    @mouseenter="handleMouseEnter"
    @mouseleave="handleMouseLeave"
  >
    <div
      v-if="shouldShowImage"
      ref="imageContainerRef"
      class="article-thumbnail-placeholder"
      :class="{ 'compact-thumbnail': compactMode }"
    >
      <img
        v-if="imageInViewport && imageUrl"
        :src="imageUrl"
        :alt="cluster.display_title || cluster.merged_title"
        class="article-thumbnail"
        :class="{ 'image-loaded': !imageLoading }"
        decoding="async"
        @load="handleImageLoad"
        @error="handleImageError"
      />
      <div
        v-if="imageLoading && imageInViewport"
        class="article-thumbnail article-thumbnail-loading"
      />
    </div>

    <div class="flex-1 min-w-0">
      <div class="flex items-start gap-1.5 sm:gap-2">
        <h4
          class="flex-1 m-0 font-semibold leading-snug text-text-primary article-title"
          :class="{
            'text-base sm:text-base mb-0.5 sm:mb-1': !compactMode,
            'text-[14px] mb-0 compact-title': compactMode,
            'read-title text-text-secondary font-normal': cluster.is_read && compactMode,
          }"
        >
          {{ cluster.display_title || cluster.merged_title }}
        </h4>

        <PhEyeSlash
          v-if="cluster.is_hidden"
          :size="18"
          class="text-text-secondary flex-shrink-0 sm:w-5 sm:h-5"
        />

        <!-- Compact mode icons on the right -->
        <div
          v-if="compactMode"
          class="flex items-center gap-1.5 sm:gap-2 shrink-0 ml-1 self-center"
        >
          <PhClockCountdown
            v-if="cluster.is_read_later"
            :size="16"
            class="text-blue-500"
            weight="fill"
          />
          <PhStar v-if="cluster.is_favorite" :size="16" class="text-yellow-500" weight="fill" />
        </div>
      </div>

      <!-- Feed source name and time -->
      <div
        class="flex justify-between items-center text-[11px] sm:text-xs text-text-secondary"
        :class="{ 'mt-0 sm:mt-1': !compactMode, 'mt-0': compactMode }"
      >
        <span class="flex items-center gap-1.5 truncate flex-1 min-w-0 mr-2">
          <span class="font-semibold text-orange-500">{{ t('article.cluster.sourceLabel') }}</span>
          <span
            class="text-[11px] sm:text-[11px] text-text-secondary opacity-75 truncate max-w-[120px]"
          >
            {{ t('article.cluster.articleCount', { count: cluster.article_count }) }}
          </span>
        </span>
        <div class="flex items-center gap-1 sm:gap-2 shrink-0 min-h-[14px] sm:min-h-[18px]">
          <template v-if="!compactMode">
            <PhClockCountdown
              v-if="cluster.is_read_later"
              :size="14"
              class="text-blue-500 sm:w-[18px] sm:h-[18px]"
              weight="fill"
            />
            <PhStar
              v-if="cluster.is_favorite"
              :size="14"
              class="text-yellow-500 sm:w-[18px] sm:h-[18px]"
              weight="fill"
            />
          </template>
          <span class="whitespace-nowrap">{{ formatDateWithI18n(cluster.created_at) }}</span>
        </div>
      </div>
      <div
        v-if="feedTitlesText"
        class="mt-1 text-[11px] sm:text-xs font-medium text-blue-500 line-clamp-1 break-all"
      >
        {{ feedTitlesText }}
      </div>
      <div
        v-if="authorsText"
        class="mt-1 text-[11px] sm:text-xs text-text-secondary/80 line-clamp-1 break-all"
      >
        {{ authorsText }}
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.article-card {
  @apply py-2 px-1.5 sm:p-3 border-b border-border cursor-pointer transition-colors flex gap-2 sm:gap-3 relative border-l-2 sm:border-l-[3px] border-l-transparent;
}

.article-card.compact {
  @apply py-1 px-2;
}

.article-card:hover {
  @apply bg-bg-tertiary;
}

.article-card.active {
  @apply bg-bg-tertiary border-l-accent;
}

.article-card.read h4 {
  @apply text-text-secondary font-normal;
}

.article-card.favorite {
  background-color: rgba(255, 215, 0, 0.05);
}

.article-card.read-later {
  background-color: rgba(59, 130, 246, 0.05);
}

.article-title {
  word-break: break-word;
  overflow-wrap: anywhere;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  display: -webkit-box;
  overflow: hidden;
}

.article-title.compact-title {
  -webkit-line-clamp: 1;
}

.article-thumbnail {
  @apply w-16 h-12 sm:w-20 sm:h-[60px] object-cover rounded bg-bg-tertiary shrink-0 border border-border;
  contain: layout style paint;
  will-change: auto;
  opacity: 0;
  transition: opacity 0.2s ease-in-out;
}

.article-card.compact .article-thumbnail {
  @apply w-12 h-9 sm:w-14 sm:h-[42px];
}

.article-thumbnail.image-loaded {
  opacity: 1;
}

.article-thumbnail-placeholder {
  @apply w-16 h-12 sm:w-20 sm:h-[60px] shrink-0 border border-border rounded overflow-hidden bg-bg-tertiary;
  contain: layout style paint;
  flex-shrink: 0;
}

.article-thumbnail-placeholder.compact-thumbnail {
  @apply w-12 h-9 sm:w-14 sm:h-[42px];
}

.article-thumbnail-loading {
  @apply w-full h-full bg-bg-tertiary animate-pulse;
  contain: layout style;
}

@media (max-width: 1400px) {
  .article-thumbnail,
  .article-thumbnail-placeholder {
    width: 56px !important;
    height: 42px !important;
  }
}
</style>
