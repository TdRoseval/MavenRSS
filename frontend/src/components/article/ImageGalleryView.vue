<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, nextTick, watch } from 'vue';
import { useAppStore } from '@/stores/app';
import { useI18n } from 'vue-i18n';
import type { Article } from '@/types/models';
import {
  PhImage,
  PhHeart,
  PhList,
  PhFloppyDisk,
  PhGlobe,
  PhX,
  PhTextT,
  PhTextTSlash,
} from '@phosphor-icons/vue';
import { openInBrowser } from '@/utils/browser';

const store = useAppStore();
const { t } = useI18n();

interface Props {
  isSidebarOpen?: boolean;
}

defineProps<Props>();

const emit = defineEmits<{
  toggleSidebar: [];
}>();

// Constants
const ITEMS_PER_PAGE = 30;
const SCROLL_THRESHOLD_PX = 500; // Start loading more items when user is 500px from bottom

const articles = ref<Article[]>([]);
const isLoading = ref(false);
const page = ref(1);
const hasMore = ref(true);
const selectedArticle = ref<Article | null>(null);
const showImageViewer = ref(false);
const allImages = ref<string[]>([]);
const currentImageIndex = ref(0);
const currentImageLoading = ref(false);
const columns = ref<Article[][]>([]);
const columnCount = ref(4);
const containerRef = ref<HTMLElement | null>(null);
// eslint-disable-next-line no-undef
let resizeObserver: ResizeObserver | null = null;
const contextMenu = ref<{ show: boolean; x: number; y: number; article: Article | null }>({
  show: false,
  x: 0,
  y: 0,
  article: null,
});
const imageCountCache = ref<Map<number, number>>(new Map());
const showTextOverlay = ref(true);

// Load showTextOverlay preference from localStorage
const savedShowTextOverlay = localStorage.getItem('imageGalleryShowTextOverlay');
if (savedShowTextOverlay !== null) {
  showTextOverlay.value = savedShowTextOverlay === 'true';
}

// Watch for changes and save to localStorage
watch(showTextOverlay, (newValue) => {
  localStorage.setItem('imageGalleryShowTextOverlay', String(newValue));
});

// Compute which feed ID to fetch (if viewing a specific feed)
const feedId = computed(() => store.currentFeedId);

// Fetch image gallery articles
async function fetchImages(loadMore = false) {
  if (isLoading.value) return;

  isLoading.value = true;
  try {
    let url = `/api/articles/images?page=${page.value}&limit=${ITEMS_PER_PAGE}`;
    if (feedId.value) {
      url += `&feed_id=${feedId.value}`;
    }

    const res = await fetch(url);
    if (res.ok) {
      const data = await res.json();

      // Validate that data is an array
      if (!Array.isArray(data)) {
        console.error('API response is not an array:', data);
        return;
      }

      const newArticles = data;

      if (loadMore) {
        articles.value = [...articles.value, ...newArticles];
      } else {
        articles.value = newArticles;
      }

      hasMore.value = newArticles.length >= ITEMS_PER_PAGE;

      // Preload image counts for new articles
      newArticles.forEach((article: Article) => {
        if (!imageCountCache.value.has(article.id)) {
          fetchImageCount(article.id);
        }
      });
    }
  } catch (e) {
    console.error('Failed to load images:', e);
  } finally {
    isLoading.value = false;
  }
}

// Fetch image count for an article
async function fetchImageCount(articleId: number) {
  try {
    const res = await fetch(`/api/articles/extract-images?id=${articleId}`);
    if (res.ok) {
      const data = await res.json();
      if (data.images && Array.isArray(data.images)) {
        imageCountCache.value.set(articleId, data.images.length);
      }
    }
  } catch (e) {
    console.error('Failed to fetch image count:', e);
  }
}

// Get image count for an article
function getImageCount(article: Article): number {
  return imageCountCache.value.get(article.id) || 1;
}

// Calculate number of columns based on container width dynamically
function calculateColumns() {
  if (!containerRef.value) return;
  const width = containerRef.value.offsetWidth;

  // Target column width: 250px for optimal image viewing
  // Minimum 2 columns, no maximum
  const targetColumnWidth = 250;
  const calculatedColumns = Math.floor(width / targetColumnWidth);

  // Ensure at least 2 columns
  columnCount.value = Math.max(2, calculatedColumns);

  // Rearrange columns after calculating new count
  arrangeColumns();
}

// Arrange articles into columns by time, balancing heights
function arrangeColumns() {
  if (articles.value.length === 0) {
    columns.value = [];
    return;
  }

  // Initialize columns
  const cols: Article[][] = Array.from({ length: columnCount.value }, () => []);
  const colHeights: number[] = Array(columnCount.value).fill(0);

  // Sort articles by published date (newest first)
  const sortedArticles = [...articles.value].sort((a, b) => {
    return new Date(b.published_at).getTime() - new Date(a.published_at).getTime();
  });

  // Place each article in the shortest column
  sortedArticles.forEach((article) => {
    const shortestColIndex = colHeights.indexOf(Math.min(...colHeights));
    cols[shortestColIndex].push(article);
    // Estimate height: 200px for image + 80px for info
    colHeights[shortestColIndex] += 280;
  });

  columns.value = cols;
}

// Handle scroll for infinite loading
function handleScroll() {
  if (!containerRef.value) return;

  const scrollTop = containerRef.value.scrollTop;
  const containerHeight = containerRef.value.clientHeight;
  const scrollHeight = containerRef.value.scrollHeight;

  if (
    scrollTop + containerHeight >= scrollHeight - SCROLL_THRESHOLD_PX &&
    !isLoading.value &&
    hasMore.value
  ) {
    // Increment page before fetching
    const nextPage = page.value + 1;
    page.value = nextPage;
    fetchImages(true);
  }
}

// Toggle favorite
async function toggleFavorite(article: Article, event: Event) {
  event.stopPropagation();
  try {
    const res = await fetch(`/api/articles/favorite?id=${article.id}`, {
      method: 'POST',
    });
    if (res.ok) {
      article.is_favorite = !article.is_favorite;
    }
  } catch (e) {
    console.error('Failed to toggle favorite:', e);
  }
}

// Open image viewer
async function openImage(article: Article) {
  selectedArticle.value = article;
  showImageViewer.value = true;
  currentImageLoading.value = true;

  // Fetch all images from the article
  await fetchArticleImages(article);

  // Mark as read
  if (!article.is_read) {
    markAsRead(article);
  }
}

// Fetch all images from article content
async function fetchArticleImages(article: Article) {
  try {
    const res = await fetch(`/api/articles/extract-images?id=${article.id}`);
    if (res.ok) {
      const data = await res.json();
      if (data.images && Array.isArray(data.images) && data.images.length > 0) {
        allImages.value = data.images;
        // Find the index of the article's main image
        currentImageIndex.value = data.images.findIndex((img: string) => img === article.image_url);
        if (currentImageIndex.value < 0) {
          currentImageIndex.value = 0;
        }
      } else {
        // Fallback to just the article's main image
        allImages.value = [article.image_url || ''];
        currentImageIndex.value = 0;
      }
    } else {
      // Fallback on error
      allImages.value = [article.image_url || ''];
      currentImageIndex.value = 0;
    }
  } catch (e) {
    console.error('Failed to fetch article images:', e);
    // Fallback on error
    allImages.value = [article.image_url || ''];
    currentImageIndex.value = 0;
  }
}

// Navigate to previous image
function previousImage() {
  if (currentImageIndex.value > 0) {
    currentImageIndex.value--;
  } else {
    // Wrap to last image
    currentImageIndex.value = allImages.value.length - 1;
  }
  // Reset loading state
  currentImageLoading.value = true;
}

// Navigate to next image
function nextImage() {
  if (currentImageIndex.value < allImages.value.length - 1) {
    currentImageIndex.value++;
  } else {
    // Wrap to first image
    currentImageIndex.value = 0;
  }
  // Reset loading state
  currentImageLoading.value = true;
}

// Handle image load
function handleImageLoad() {
  currentImageLoading.value = false;
}

// Handle image error
function handleImageError() {
  currentImageLoading.value = false;
}

// Mark article as read
async function markAsRead(article: Article) {
  try {
    const res = await fetch(`/api/articles/read?id=${article.id}&read=true`, {
      method: 'POST',
    });
    if (res.ok) {
      article.is_read = true;
    }
  } catch (e) {
    console.error('Failed to mark as read:', e);
  }
}

// Close image viewer
function closeImageViewer() {
  showImageViewer.value = false;
  selectedArticle.value = null;
  allImages.value = [];
  currentImageIndex.value = 0;
}

// Format date
function formatDate(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  const days = Math.floor(diff / (1000 * 60 * 60 * 24));

  if (days === 0) {
    const hours = Math.floor(diff / (1000 * 60 * 60));
    if (hours === 0) {
      const minutes = Math.floor(diff / (1000 * 60));
      return minutes <= 0 ? t('justNow') : t('minutesAgo', { count: minutes });
    }
    return t('hoursAgo', { count: hours });
  } else if (days < 7) {
    return t('daysAgo', { count: days });
  }
  return date.toLocaleDateString();
}

// Handle right-click context menu
function handleContextMenu(event: MouseEvent, article: Article) {
  event.preventDefault();
  event.stopPropagation();
  contextMenu.value = {
    show: true,
    x: event.clientX,
    y: event.clientY,
    article,
  };
}

// Close context menu
function closeContextMenu() {
  contextMenu.value.show = false;
}

// Download image
async function downloadImage(article: Article) {
  try {
    const response = await fetch(article.image_url || '');
    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${article.title}.jpg`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    window.URL.revokeObjectURL(url);
  } catch (e) {
    console.error('Failed to download image:', e);
    window.showToast(t('downloadFailed'), 'error');
  }
  closeContextMenu();
}

// Open original article
function openOriginal(article: Article) {
  openInBrowser(article.url);
  closeContextMenu();
}

// Handle keyboard shortcuts
function handleKeyDown(e: KeyboardEvent) {
  // Only handle keyboard when image viewer is open
  if (!showImageViewer.value) return;

  if (e.key === 'Escape') {
    closeImageViewer();
  } else if (e.key === 'ArrowLeft') {
    e.preventDefault();
    previousImage();
  } else if (e.key === 'ArrowRight') {
    e.preventDefault();
    nextImage();
  }
}

// Watch for articles changes and rearrange
watch(articles, () => {
  nextTick(() => {
    arrangeColumns();
  });
});

// Watch for feed ID changes and refetch
watch(feedId, async () => {
  // Close image viewer when switching feeds
  showImageViewer.value = false;
  selectedArticle.value = null;
  allImages.value = [];
  currentImageIndex.value = 0;

  page.value = 1;
  articles.value = [];
  hasMore.value = true;
  await fetchImages();
  // Recalculate columns after fetching new articles
  await nextTick();
  calculateColumns();
});

onMounted(() => {
  fetchImages();
  if (containerRef.value) {
    containerRef.value.addEventListener('scroll', handleScroll);
  }
  window.addEventListener('click', closeContextMenu);
  window.addEventListener('keydown', handleKeyDown);

  // Set up ResizeObserver to watch for container size changes
  if (containerRef.value) {
    // eslint-disable-next-line no-undef
    resizeObserver = new ResizeObserver(() => {
      calculateColumns();
    });
    resizeObserver.observe(containerRef.value);
  }
});

onUnmounted(() => {
  if (containerRef.value) {
    containerRef.value.removeEventListener('scroll', handleScroll);
  }
  if (resizeObserver && containerRef.value) {
    resizeObserver.unobserve(containerRef.value);
    resizeObserver.disconnect();
    resizeObserver = null;
  }
  window.removeEventListener('click', closeContextMenu);
  window.removeEventListener('keydown', handleKeyDown);
});
</script>

<template>
  <div
    ref="containerRef"
    class="flex flex-col flex-1 h-full overflow-y-auto bg-bg-primary scroll-smooth"
  >
    <!-- Header -->
    <div
      class="sticky top-0 z-10 bg-bg-primary border-b border-border p-2 sm:p-4 flex items-center gap-3"
    >
      <button
        class="p-2 rounded-lg hover:bg-bg-tertiary text-text-primary transition-colors md:hidden"
        :title="t('toggleSidebar')"
        @click="emit('toggleSidebar')"
      >
        <PhList :size="24" />
      </button>
      <div class="flex items-center gap-2 sm:gap-2 flex-1">
        <h1 class="text-base sm:text-lg font-bold text-text-primary line-height-fixed-32">
          {{ t('imageGallery') }}
        </h1>
      </div>
      <button
        class="p-2 rounded hover:bg-bg-tertiary text-text-primary transition-colors"
        :title="showTextOverlay ? t('hideText') : t('showText')"
        @click="showTextOverlay = !showTextOverlay"
      >
        <PhTextTSlash v-if="showTextOverlay" :size="20" />
        <PhTextT v-else :size="20" />
      </button>
    </div>

    <!-- Masonry Grid -->
    <div v-if="articles.length > 0" class="p-4 flex gap-4">
      <div v-for="(column, colIndex) in columns" :key="colIndex" class="flex-1 flex flex-col gap-4">
        <div
          v-for="article in column"
          :key="article.id"
          class="cursor-pointer group"
          @click="openImage(article)"
          @contextmenu="handleContextMenu($event, article)"
        >
          <div
            class="relative overflow-hidden rounded-lg bg-bg-secondary transition-transform duration-200 hover:scale-[1.02]"
          >
            <img
              :src="article.image_url"
              :alt="article.title"
              class="w-full h-auto block"
              loading="lazy"
            />
            <!-- Image count indicator -->
            <div
              v-if="getImageCount(article) > 1"
              class="absolute bottom-2 left-2 px-2 py-1 rounded-full bg-black/60 text-white text-xs font-semibold backdrop-blur-sm z-10 flex items-center gap-1"
            >
              <PhImage :size="14" />
              <span class="ml-1">{{ getImageCount(article) }}</span>
            </div>
            <div
              class="absolute inset-0 bg-black/0 hover:bg-black/30 transition-all duration-200 flex items-start justify-end p-2"
            >
              <button
                class="opacity-0 group-hover:opacity-100 transition-opacity duration-200 bg-black/50 rounded-full p-1.5 hover:bg-black/70"
                @click="toggleFavorite(article, $event)"
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
      </div>
    </div>

    <!-- Empty State -->
    <div
      v-else-if="!isLoading"
      class="flex flex-col items-center justify-center h-full w-full gap-4"
    >
      <PhImage :size="64" class="text-text-secondary opacity-50" />
      <p class="text-text-secondary">{{ t('noArticles') }}</p>
    </div>

    <!-- Loading Indicator -->
    <div v-if="isLoading" class="flex justify-center py-8">
      <div
        class="w-8 h-8 border-4 border-accent border-t-transparent rounded-full animate-spin"
      ></div>
    </div>

    <!-- Image Viewer Modal -->
    <div
      v-if="showImageViewer && selectedArticle"
      class="fixed inset-0 z-50 bg-black/90 flex flex-col p-4"
      data-image-viewer="true"
      @click="closeImageViewer"
    >
      <button
        class="absolute top-4 right-4 w-8 h-8 bg-black/50 hover:bg-black/70 rounded-full text-white flex items-center justify-center transition-colors duration-200 z-10 shrink-0"
        @click="closeImageViewer"
      >
        <PhX :size="20" />
      </button>

      <!-- Image counter (when multiple images) -->
      <div
        v-if="allImages.length > 1"
        class="absolute top-4 left-4 px-2 py-1 rounded text-white text-sm font-medium min-w-[60px] text-center"
        style="
          z-index: 11;
          text-shadow:
            0 1px 3px rgba(0, 0, 0, 0.8),
            0 1px 2px rgba(0, 0, 0, 0.6);
        "
      >
        {{ currentImageIndex + 1 }} / {{ allImages.length }}
      </div>

      <!-- Navigation buttons (when multiple images) -->
      <template v-if="allImages.length > 1">
        <button
          class="absolute top-1/2 left-4 -translate-y-1/2 w-12 h-12 rounded text-white text-4xl flex items-center justify-center transition-all duration-200 hover:scale-110 active:scale-95 z-10"
          style="
            text-shadow:
              0 1px 3px rgba(0, 0, 0, 0.8),
              0 1px 2px rgba(0, 0, 0, 0.6);
          "
          @click.stop="previousImage"
        >
          ‹
        </button>
        <button
          class="absolute top-1/2 right-4 -translate-y-1/2 w-12 h-12 rounded text-white text-4xl flex items-center justify-center transition-all duration-200 hover:scale-110 active:scale-95 z-10"
          style="
            text-shadow:
              0 1px 3px rgba(0, 0, 0, 0.8),
              0 1px 2px rgba(0, 0, 0, 0.6);
          "
          @click.stop="nextImage"
        >
          ›
        </button>
      </template>

      <div class="flex-1 flex items-center justify-center min-h-0 relative" @click.stop>
        <!-- Loading placeholder -->
        <div
          v-if="currentImageLoading"
          class="absolute inset-0 flex items-center justify-center z-10"
        >
          <div
            class="w-12 h-12 border-4 border-white/20 border-t-white rounded-full animate-spin"
          ></div>
        </div>

        <img
          :src="allImages[currentImageIndex] || selectedArticle.image_url"
          :alt="selectedArticle.title"
          class="h-full w-full object-contain"
          :class="{ 'opacity-0': currentImageLoading, 'transition-opacity duration-200': true }"
          @load="handleImageLoad"
          @error="handleImageError"
        />
      </div>

      <div class="bg-bg-primary px-3 py-3 rounded-md shrink-0" @click.stop>
        <div class="flex items-center justify-between gap-4 mb-2">
          <h2 class="text-base font-bold text-text-primary flex-1 line-clamp-2">
            {{ selectedArticle.title }}
          </h2>
          <a
            :href="selectedArticle.url"
            target="_blank"
            rel="noopener noreferrer"
            class="px-3 py-1.5 bg-accent hover:bg-accent-hover text-white rounded-md text-sm whitespace-nowrap transition-colors duration-200"
          >
            {{ t('viewOriginal') }}
          </a>
        </div>
        <div class="flex items-center gap-4 text-sm text-text-secondary">
          <span class="truncate flex-1">{{ selectedArticle.feed_title }}</span>
          <span class="shrink-0">{{ formatDate(selectedArticle.published_at) }}</span>
        </div>
      </div>
    </div>

    <!-- Context Menu -->
    <div
      v-if="contextMenu.show && contextMenu.article"
      class="fixed z-50 bg-bg-primary border border-border rounded-lg shadow-lg py-1 min-w-[180px]"
      :style="{ top: contextMenu.y + 'px', left: contextMenu.x + 'px' }"
      @click.stop
    >
      <button
        class="w-full px-4 py-2 flex items-center gap-3 text-sm text-text-primary hover:bg-bg-tertiary active:bg-bg-secondary transition-colors cursor-pointer"
        @click="downloadImage(contextMenu.article)"
      >
        <PhFloppyDisk :size="16" />
        <span>{{ t('downloadImage') }}</span>
      </button>
      <button
        class="w-full px-4 py-2 flex items-center gap-3 text-sm text-text-primary hover:bg-bg-tertiary active:bg-bg-secondary transition-colors cursor-pointer"
        @click="openOriginal(contextMenu.article)"
      >
        <PhGlobe :size="16" />
        <span>{{ t('openInBrowser') }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
/* Define keyframes for spinner animation */
@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}
</style>
