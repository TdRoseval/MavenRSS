<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  PhSparkle,
  PhX,
  PhClockCountdown,
  PhStar,
  PhEnvelope,
  PhEnvelopeOpen,
} from '@phosphor-icons/vue';
import type { Cluster } from '@/types/models';
import { useClusterStore } from '@/stores/cluster';
import { formatDate as formatDateUtil } from '@/shared/lib/date';
import DOMPurify from 'dompurify';
import { marked } from 'marked';
import { useReadingTimeTracker } from '../composables/useReadingTimeTracker';

// Activate reading time tracker for AI Enhanced Mode (Level 2 deep-read feedback)
useReadingTimeTracker();

interface Props {
  isMobile?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  isMobile: false,
});

const emit = defineEmits<{
  close: [];
}>();

const { t, locale } = useI18n();
const clusterStore = useClusterStore();

const cluster = ref<Cluster | null>(null);
const isLoading = ref(false);

const formatDateWithI18n = (dateStr: string): string => {
  return formatDateUtil(dateStr, locale.value, t);
};

const formattedContent = computed(() => {
  if (!cluster.value?.merged_content) return '';

  const html = marked.parse(cluster.value.merged_content, { async: false }) as string;
  return DOMPurify.sanitize(html);
});
const feedTitles = computed(() => cluster.value?.feed_titles?.filter(Boolean) || []);
const authors = computed(() => cluster.value?.authors?.filter(Boolean) || []);

async function loadClusterDetail(id: number) {
  isLoading.value = true;
  cluster.value = await clusterStore.fetchClusterDetail(id);
  isLoading.value = false;
}

async function toggleRead(): Promise<void> {
  if (!cluster.value) {
    return;
  }

  const nextValue = !cluster.value.is_read;
  const currentId = cluster.value.id;
  cluster.value.is_read = nextValue;

  try {
    await clusterStore.markClusterRead(currentId, nextValue);
  } catch (error) {
    cluster.value.is_read = !nextValue;
    console.error('Error toggling cluster read state:', error);
    window.showToast(t('common.errors.savingSettings'), 'error');
  }
}

async function toggleFavorite(): Promise<void> {
  if (!cluster.value) {
    return;
  }

  const currentId = cluster.value.id;
  const previousValue = cluster.value.is_favorite;
  cluster.value.is_favorite = !previousValue;

  try {
    await clusterStore.toggleClusterFavorite({
      ...cluster.value,
      is_favorite: previousValue,
    });
  } catch (error) {
    cluster.value.is_favorite = previousValue;
    console.error('Error toggling cluster favorite state:', error);
    window.showToast(t('common.errors.savingSettings'), 'error');
    return;
  }

  clusterStore.updateClusterState(currentId, { is_favorite: cluster.value.is_favorite });
}

async function toggleReadLater(): Promise<void> {
  if (!cluster.value) {
    return;
  }

  const previousReadLater = cluster.value.is_read_later;
  const previousRead = cluster.value.is_read;
  cluster.value.is_read_later = !previousReadLater;
  if (cluster.value.is_read_later) {
    cluster.value.is_read = false;
  }

  try {
    await clusterStore.toggleClusterReadLater({
      ...cluster.value,
      is_read_later: previousReadLater,
      is_read: previousRead,
    });
  } catch (error) {
    cluster.value.is_read_later = previousReadLater;
    cluster.value.is_read = previousRead;
    console.error('Error toggling cluster read-later state:', error);
    window.showToast(t('common.errors.savingSettings'), 'error');
  }
}

watch(
  () => clusterStore.currentClusterId,
  (newId) => {
    if (newId) {
      loadClusterDetail(newId);
    } else {
      cluster.value = null;
    }
  },
  { immediate: true }
);

function handleClose() {
  clusterStore.currentClusterId = null;
  if (props.isMobile) {
    emit('close');
  }
}
</script>

<template>
  <main
    :class="[
      'flex-1 bg-bg-primary flex flex-col h-full absolute w-full md:static md:w-auto z-30 transition-transform duration-300',
      clusterStore.currentClusterId ? 'translate-x-0 md:translate-x-0' : 'translate-x-full',
    ]"
  >
    <!-- Mobile header with back button -->
    <div
      v-if="isMobile && cluster"
      class="flex items-center gap-2 px-3 py-2 border-b border-border bg-bg-secondary"
    >
      <span class="flex-1 truncate text-sm font-medium text-center pr-8">{{
        cluster.merged_title
      }}</span>
      <button
        class="flex items-center justify-center p-2 -mr-2 rounded-lg hover:bg-bg-tertiary transition-colors"
        :title="t('article.navigation.backToList')"
        @click="handleClose"
      >
        <PhX :size="20" />
      </button>
    </div>

    <div
      v-if="!clusterStore.currentClusterId && !isLoading"
      class="hidden md:flex flex-col items-center justify-center h-full text-text-secondary text-center px-4"
    >
      <PhSparkle :size="48" class="mb-4 sm:mb-5 opacity-50 sm:w-16 sm:h-16" />
      <p class="text-sm sm:text-base">{{ t('article.cluster.selectPrompt') }}</p>
    </div>

    <div
      v-else-if="isLoading"
      class="flex-1 flex flex-col items-center justify-center p-8 text-text-secondary"
    >
      <div
        class="w-8 h-8 border-4 border-accent border-t-transparent rounded-full animate-spin mb-4"
      />
      <div>{{ t('article.cluster.loadingDetail') }}</div>
    </div>

    <div v-else-if="cluster" class="flex flex-col h-full bg-bg-primary overflow-hidden">
      <div
        class="flex items-center justify-between p-2 border-b border-border bg-bg-secondary shrink-0 min-h-[48px] sm:min-h-[56px]"
      >
        <div
          class="flex items-center gap-1 sm:gap-2 overflow-x-auto no-scrollbar mask-edges-right pr-4"
        >
          <div class="flex gap-1.5 p-1 bg-bg-tertiary rounded-lg">
            <span class="text-sm font-semibold px-2 py-1">{{
              t('article.cluster.sourceLabel')
            }}</span>
          </div>
        </div>

        <div class="flex items-center gap-0.5 sm:gap-1 pl-2">
          <button
            class="flex items-center justify-center w-8 h-8 sm:w-9 sm:h-9 rounded-lg hover:bg-bg-tertiary text-text-secondary transition-colors"
            :title="
              cluster.is_read ? t('article.action.markAsUnread') : t('article.action.markAsRead')
            "
            @click="toggleRead"
          >
            <PhEnvelopeOpen v-if="cluster.is_read" :size="18" class="sm:w-5 sm:h-5" />
            <PhEnvelope v-else :size="18" class="sm:w-5 sm:h-5" />
          </button>
          <button
            class="flex items-center justify-center w-8 h-8 sm:w-9 sm:h-9 rounded-lg transition-colors"
            :class="
              cluster.is_favorite
                ? 'text-yellow-500 hover:bg-bg-tertiary hover:text-yellow-600'
                : 'text-text-secondary hover:bg-bg-tertiary hover:text-yellow-500'
            "
            :title="
              cluster.is_favorite
                ? t('article.action.removeFromFavorite')
                : t('article.action.addToFavorite')
            "
            @click="toggleFavorite"
          >
            <PhStar
              :size="18"
              class="sm:w-5 sm:h-5"
              :weight="cluster.is_favorite ? 'fill' : 'regular'"
            />
          </button>
          <button
            class="flex items-center justify-center w-8 h-8 sm:w-9 sm:h-9 rounded-lg transition-colors"
            :class="
              cluster.is_read_later
                ? 'text-blue-500 hover:bg-bg-tertiary hover:text-blue-600'
                : 'text-text-secondary hover:bg-bg-tertiary hover:text-blue-500'
            "
            :title="
              cluster.is_read_later
                ? t('article.action.removeFromReadLater')
                : t('article.action.addToReadLater')
            "
            @click="toggleReadLater"
          >
            <PhClockCountdown
              :size="18"
              class="sm:w-5 sm:h-5"
              :weight="cluster.is_read_later ? 'fill' : 'regular'"
            />
          </button>
          <button
            v-if="!isMobile"
            class="hidden md:flex items-center justify-center w-8 h-8 sm:w-9 sm:h-9 rounded-lg hover:bg-bg-tertiary text-text-secondary transition-colors"
            :title="t('common.close')"
            @click="handleClose"
          >
            <PhX :size="20" class="sm:w-[22px] sm:h-[22px]" />
          </button>
        </div>
      </div>

      <!-- Content -->
      <div class="flex-1 overflow-y-auto w-full custom-scrollbar">
        <div
          class="max-w-[800px] w-[95%] sm:w-[90%] md:w-[85%] mx-auto py-6 sm:py-8 lg:py-12 px-4 sm:px-0"
        >
          <header class="mb-6 sm:mb-8 text-center sm:text-left">
            <h1
              class="text-2xl sm:text-3xl lg:text-4xl font-bold text-text-primary leading-tight font-serif mb-4 sm:mb-6"
            >
              {{ cluster.merged_title }}
            </h1>

            <div
              class="flex flex-wrap items-center justify-center sm:justify-start gap-x-4 gap-y-2 text-sm sm:text-base text-text-secondary"
            >
              <div class="flex items-center gap-1.5 font-medium text-accent">
                <span>{{ t('article.cluster.mergedFrom', { count: cluster.article_count }) }}</span>
              </div>
              <span class="hidden sm:inline text-border">•</span>
              <div class="flex items-center gap-1.5">
                <span class="font-medium">{{ t('article.cluster.statusLabel') }}:</span>
                <span class="capitalize">{{ cluster.status }}</span>
              </div>
              <span class="hidden sm:inline text-border">•</span>
              <span class="whitespace-nowrap">{{ formatDateWithI18n(cluster.created_at) }}</span>
            </div>
            <div class="mt-4 space-y-3">
              <div class="flex flex-wrap items-start justify-center sm:justify-start gap-2">
                <span
                  class="px-2.5 py-1 rounded-full bg-bg-secondary text-accent text-xs sm:text-sm font-medium"
                >
                  {{ t('article.cluster.sourceLabel') }}
                </span>
                <span
                  v-for="feedTitle in feedTitles"
                  :key="feedTitle"
                  class="px-2.5 py-1 rounded-full bg-bg-secondary text-text-secondary text-xs sm:text-sm border border-border"
                >
                  {{ feedTitle }}
                </span>
              </div>
              <div
                v-if="authors.length"
                class="flex flex-wrap items-start justify-center sm:justify-start gap-2"
              >
                <span class="text-xs sm:text-sm font-medium text-text-secondary">
                  {{ t('article.cluster.authorLabel') }}
                </span>
                <span
                  v-for="author in authors"
                  :key="author"
                  class="px-2.5 py-1 rounded-full bg-bg-secondary text-text-secondary text-xs sm:text-sm border border-border"
                >
                  {{ author }}
                </span>
              </div>
            </div>
          </header>

          <hr class="border-border my-6 sm:my-8" />

          <div
            v-if="cluster.merged_summary"
            class="bg-bg-secondary p-4 rounded-lg mb-8 border border-border"
          >
            <h3 class="font-semibold text-lg mb-2">{{ t('article.cluster.summaryTitle') }}</h3>
            <p class="text-text-secondary leading-relaxed">{{ cluster.merged_summary }}</p>
          </div>

          <article class="prose-content w-full" v-html="formattedContent" />

          <div
            v-if="cluster.articles && cluster.articles.length > 0"
            class="mt-12 pt-8 border-t border-border"
          >
            <h3 class="font-semibold text-xl mb-4 text-text-primary">
              {{ t('article.cluster.sourceArticlesTitle') }}
            </h3>
            <ul class="space-y-3">
              <li
                v-for="article in cluster.articles"
                :key="article.id"
                class="flex flex-col gap-1 p-3 bg-bg-secondary rounded-lg border border-border"
              >
                <a
                  :href="article.url"
                  target="_blank"
                  class="font-medium text-accent hover:underline line-clamp-1"
                >
                  {{ article.title }}
                </a>
                <span class="text-xs text-text-secondary">
                  {{ article.feed_title || t('article.cluster.unknownFeed') }} •
                  {{ formatDateWithI18n(article.published_at) }}
                  <template v-if="article.author"> • {{ article.author }} </template>
                </span>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  </main>
</template>

<style scoped>
@reference "../../style.css";

.prose-content {
  @apply text-text-primary;
  font-family: inherit;
  font-size: 1.125rem;
  line-height: 1.75;
}

.prose-content :deep(h1) {
  @apply text-2xl font-bold mt-8 mb-4 font-serif;
}

.prose-content :deep(h2) {
  @apply text-xl font-bold mt-8 mb-4 font-serif;
}

.prose-content :deep(h3) {
  @apply text-lg font-bold mt-6 mb-3 font-serif;
}

.prose-content :deep(p) {
  @apply mb-5;
}

.prose-content :deep(a) {
  @apply text-accent hover:underline;
}

.prose-content :deep(ul) {
  @apply list-disc pl-6 mb-5;
}

.prose-content :deep(ol) {
  @apply list-decimal pl-6 mb-5;
}

.prose-content :deep(blockquote) {
  @apply pl-4 border-l-4 border-border text-text-secondary italic my-5;
}
</style>
