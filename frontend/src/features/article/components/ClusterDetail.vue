<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  PhSparkle,
  PhX,
  PhClockCountdown,
  PhStar,
  PhEnvelope,
  PhEnvelopeOpen,
  PhTranslate,
  PhCaretLeft,
  PhCaretRight,
} from '@phosphor-icons/vue';
import type { Cluster, DailyRecommendationItem } from '@/types/models';
import { useClusterStore } from '@/stores/cluster';
import { useArticleStore } from '@/features/article/store';
import { formatDate as formatDateUtil } from '@/shared/lib/date';
import DOMPurify from 'dompurify';
import { marked } from 'marked';
import { useReadingTimeTracker } from '../composables/useReadingTimeTracker';
import { useSettings } from '@/composables/core/useSettings';
import { authPost } from '@/shared/lib/authFetch';
import {
  extractTextWithPlaceholders,
  restorePreservedElements,
} from '@/features/article/composables/useContentTranslation';
import { useArticleRendering } from '../composables/useArticleRendering';

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
const articleStore = useArticleStore();
const { settings, fetchSettings } = useSettings();
const { renderMathFormulas, highlightCodeBlocks } = useArticleRendering();

const cluster = ref<Cluster | null>(null);
const isLoading = ref(false);
const isForceTranslating = ref(false);
const contentArticleRef = ref<HTMLElement | null>(null);
const translatedClusterTitle = ref('');
const translatedClusterSummary = ref('');
const hasBodyTranslations = ref(false);
const forceTranslateRunId = ref(0);

const formatDateWithI18n = (dateStr: string): string => {
  return formatDateUtil(dateStr, locale.value, t);
};

const formattedContent = computed(() => {
  const content = cluster.value?.merged_content || '';
  if (!content) return '';

  const html = marked.parse(content, { async: false }) as string;
  return DOMPurify.sanitize(html);
});

const feedTitles = computed(() => cluster.value?.feed_titles?.filter(Boolean) || []);
const authors = computed(() => cluster.value?.authors?.filter(Boolean) || []);

const translationEnabled = computed(
  () => settings.value.translation_enabled === true || settings.value.translation_enabled === 'true'
);

const translationOnlyMode = computed(
  () =>
    settings.value.translation_only_mode === true ||
    settings.value.translation_only_mode === 'true'
);

const targetLanguage = computed(() => settings.value.target_language || 'zh');

const originalClusterTitle = computed(
  () => cluster.value?.display_title || cluster.value?.merged_title || ''
);

const originalClusterSummary = computed(() => cluster.value?.merged_summary || '');

const hasTranslatedTitle = computed(
  () =>
    translatedClusterTitle.value.trim() !== '' &&
    translatedClusterTitle.value.trim() !== originalClusterTitle.value.trim()
);

const hasTranslatedSummary = computed(
  () =>
    translatedClusterSummary.value.trim() !== '' &&
    translatedClusterSummary.value.trim() !== originalClusterSummary.value.trim()
);

const clusterTitle = computed(() => {
  if (hasTranslatedTitle.value) {
    return translatedClusterTitle.value;
  }
  return originalClusterTitle.value;
});

const clusterOriginalTitle = computed(() => {
  if (!hasTranslatedTitle.value || translationOnlyMode.value) {
    return '';
  }
  return originalClusterTitle.value;
});

const clusterSummary = computed(() => {
  if (hasTranslatedSummary.value) {
    return translatedClusterSummary.value;
  }
  return originalClusterSummary.value;
});

const clusterOriginalSummary = computed(() => {
  if (!hasTranslatedSummary.value || translationOnlyMode.value) {
    return '';
  }
  return originalClusterSummary.value;
});

function clearBodyTranslations() {
  if (!contentArticleRef.value) {
    hasBodyTranslations.value = false;
    return;
  }

  const existingTranslations = contentArticleRef.value.querySelectorAll('.translation-text');
  existingTranslations.forEach((el) => el.remove());
  hasBodyTranslations.value = false;
}

function resetTranslationState() {
  translatedClusterTitle.value = '';
  translatedClusterSummary.value = '';
  clearBodyTranslations();
}

async function enhanceRenderedContent() {
  await nextTick();
  if (!contentArticleRef.value) {
    return;
  }

  renderMathFormulas(contentArticleRef.value);
  highlightCodeBlocks(contentArticleRef.value);
}

async function loadClusterDetail(id: number) {
  resetTranslationState();

  const currentCluster = clusterStore.currentCluster;
  if (currentCluster && currentCluster.id === id) {
    cluster.value = { ...currentCluster };
  }

  isLoading.value = !cluster.value?.merged_content;

  try {
    cluster.value = await clusterStore.fetchClusterDetail(id);
    await enhanceRenderedContent();
  } catch (error) {
    console.error('Failed to load cluster detail:', error);
    if (!cluster.value) {
      window.showToast(t('common.errors.savingSettings'), 'error');
    }
  } finally {
    isLoading.value = false;
  }
}

async function translateClusterField(text: string, force: boolean, preemptive: boolean) {
  if (!text.trim()) {
    return '';
  }

  const response = await authPost<any>('/api/articles/translate-text', {
    text,
    target_language: targetLanguage.value,
    force,
    high_priority: true,
    preemptive,
  });

  return response?.translated_text || '';
}

function isTranslationRunActive(runId: number) {
  return isForceTranslating.value && forceTranslateRunId.value === runId;
}

async function forceTranslateClusterParagraphs(runId: number, preemptive: boolean) {
  await nextTick();

  const proseContainer = contentArticleRef.value;
  if (!proseContainer) {
    return;
  }

  clearBodyTranslations();

  const textTags = ['P', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'LI', 'TD', 'TH', 'FIGCAPTION', 'DT', 'DD'];
  const translatedElements = new Set<HTMLElement>();
  const allElements = Array.from(proseContainer.querySelectorAll(textTags.join(',')));

  allElements.sort((a, b) => {
    const getDepth = (el: Element): number => {
      let depth = 0;
      let parent = el.parentElement;
      while (parent && parent !== proseContainer) {
        depth++;
        parent = parent.parentElement;
      }
      return depth;
    };
    return getDepth(a) - getDepth(b);
  });

  const canContainNestedTranslatableElements = (el: HTMLElement): boolean => {
    const nestableTags = ['LI', 'BLOCKQUOTE', 'DD', 'DT', 'TD', 'TH'];
    return nestableTags.includes(el.tagName);
  };

  const getNestedTranslatableChildren = (el: HTMLElement): Element[] => {
    return Array.from(el.children).filter((child) => textTags.includes(child.tagName));
  };

  let firstRequest = preemptive;

  for (const el of allElements) {
    if (!isTranslationRunActive(runId)) {
      return;
    }

    const htmlEl = el as HTMLElement;

    if (htmlEl.closest('.translation-text')) continue;
    if (htmlEl.querySelector('.translation-text')) continue;
    if (translatedElements.has(htmlEl)) continue;

    let hasTranslatedAncestor = false;
    let ancestor = htmlEl.parentElement;
    while (ancestor && ancestor !== proseContainer) {
      if (translatedElements.has(ancestor)) {
        if (canContainNestedTranslatableElements(htmlEl)) {
          const ancestorNested = getNestedTranslatableChildren(ancestor as HTMLElement);
          if (ancestorNested.length > 0) {
            ancestor = ancestor.parentElement;
            continue;
          }
        }
        hasTranslatedAncestor = true;
        break;
      }
      ancestor = ancestor.parentElement;
    }
    if (hasTranslatedAncestor) continue;

    if (
      htmlEl.closest('pre') ||
      htmlEl.tagName === 'CODE' ||
      htmlEl.closest('kbd') ||
      htmlEl.classList.contains('katex') ||
      htmlEl.classList.contains('katex-display') ||
      htmlEl.classList.contains('katex-inline')
    ) {
      continue;
    }

    const { text, preservedElements, hyperlinks } = extractTextWithPlaceholders(htmlEl);
    if (!text.trim()) {
      continue;
    }

    const translatedText = await translateClusterField(text, true, firstRequest);
    firstRequest = false;

    if (!isTranslationRunActive(runId)) {
      return;
    }

    if (!translatedText.trim()) {
      continue;
    }

    const translatedHTML = restorePreservedElements(
      translatedText,
      preservedElements,
      hyperlinks
    );

    const translationEl = document.createElement('div');
    if (
      htmlEl.tagName === 'LI' ||
      htmlEl.tagName === 'TD' ||
      htmlEl.tagName === 'TH' ||
      htmlEl.tagName === 'DD' ||
      htmlEl.tagName === 'DT'
    ) {
      translationEl.className = 'translation-text translation-inline';
      translationEl.innerHTML = translatedHTML;
      htmlEl.appendChild(translationEl);
    } else if (htmlEl.closest('blockquote')) {
      translationEl.className = 'translation-text translation-blockquote';
      translationEl.innerHTML = translatedHTML;
      htmlEl.appendChild(translationEl);
    } else {
      translationEl.className = 'translation-text';
      translationEl.innerHTML = translatedHTML;
      htmlEl.parentNode?.insertBefore(translationEl, htmlEl.nextSibling);
    }

    translatedElements.add(htmlEl);
    hasBodyTranslations.value = true;
  }

  await nextTick();
  proseContainer.querySelectorAll('.translation-text').forEach((el) => {
    renderMathFormulas(el as HTMLElement);
    highlightCodeBlocks(el as HTMLElement);
  });
}

async function forceTranslateCluster() {
  if (!cluster.value || !translationEnabled.value || !targetLanguage.value) {
    return;
  }

  const runId = forceTranslateRunId.value + 1;
  forceTranslateRunId.value = runId;
  isForceTranslating.value = true;
  translatedClusterTitle.value = '';
  translatedClusterSummary.value = '';
  clearBodyTranslations();

  try {
    let preemptive = true;

    if (originalClusterTitle.value.trim()) {
      translatedClusterTitle.value = await translateClusterField(
        originalClusterTitle.value,
        true,
        preemptive
      );
      preemptive = false;
    }

    if (!isTranslationRunActive(runId)) {
      return;
    }

    if (originalClusterSummary.value.trim()) {
      translatedClusterSummary.value = await translateClusterField(
        originalClusterSummary.value,
        true,
        preemptive
      );
      preemptive = false;
    }

    if (!isTranslationRunActive(runId)) {
      return;
    }

    await forceTranslateClusterParagraphs(runId, preemptive);

    if (isTranslationRunActive(runId)) {
      window.showToast(t('article.action.forceTranslateSuccess'), 'success');
    }
  } catch (error) {
    console.error('Failed to force translate cluster:', error);
    window.showToast(t('common.errors.translatingContent'), 'error');
  } finally {
    if (forceTranslateRunId.value === runId) {
      isForceTranslating.value = false;
    }
  }
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

// Effective cluster list matching ClusterList's displayedClusters order
const navigationClusters = computed<Cluster[]>(() => {
  const source =
    articleStore.currentFilter === 'dailyRecommendations'
      ? clusterStore.dailyRecommendations.map((item: DailyRecommendationItem) => item.cluster)
      : clusterStore.clusters;

  if (!articleStore.showOnlyUnread) {
    return source;
  }

  // Keep the currently open cluster in the list so navigation stays stable
  return source.filter(
    (item: Cluster) => !item.is_read || item.id === clusterStore.currentClusterId
  );
});

const currentClusterIndex = computed(() => {
  if (!clusterStore.currentClusterId) return -1;
  return navigationClusters.value.findIndex((item) => item.id === clusterStore.currentClusterId);
});

const hasPreviousCluster = computed(
  () => currentClusterIndex.value > 0 && navigationClusters.value.length > 0
);

const hasNextCluster = computed(
  () =>
    currentClusterIndex.value >= 0 && currentClusterIndex.value < navigationClusters.value.length - 1
);

function scrollClusterIntoView(clusterId: number) {
  setTimeout(() => {
    const clusterEl = document.querySelector(`[data-cluster-id="${clusterId}"]`);
    if (clusterEl) {
      clusterEl.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }
  }, 50);
}

function navigateToCluster(target: Cluster) {
  clusterStore.currentClusterId = target.id;
  clusterStore.reportClusterClick(target.id);

  if (!target.is_read) {
    clusterStore.markClusterRead(target.id, true).catch((e: unknown) => {
      console.error('Error marking cluster as read:', e);
    });
  }

  scrollClusterIntoView(target.id);
}

function goToPreviousCluster() {
  if (!hasPreviousCluster.value) return;
  navigateToCluster(navigationClusters.value[currentClusterIndex.value - 1]);
}

function goToNextCluster() {
  if (!hasNextCluster.value) return;
  navigateToCluster(navigationClusters.value[currentClusterIndex.value + 1]);
}

// Auto-load the next page when the current cluster becomes the last one in the
// list, so the next/previous navigation bar can continue past the current page
watch([currentClusterIndex, () => navigationClusters.value.length], ([index, length]) => {
  if (index < 0 || length === 0 || index !== length - 1) return;

  // Daily recommendations are a fixed set with no pagination
  if (articleStore.currentFilter === 'dailyRecommendations') return;

  clusterStore.loadMore();
});

watch(
  () => clusterStore.currentClusterId,
  (newId) => {
    forceTranslateRunId.value += 1;
    isForceTranslating.value = false;

    if (newId) {
      loadClusterDetail(newId);
    } else {
      cluster.value = null;
      resetTranslationState();
    }
  },
  { immediate: true }
);

watch(
  () => cluster.value?.merged_content,
  async () => {
    await enhanceRenderedContent();
  }
);

function handleForceTranslateEvent() {
  if (
    cluster.value &&
    translationEnabled.value &&
    targetLanguage.value &&
    !isForceTranslating.value
  ) {
    forceTranslateCluster();
  }
}

onMounted(() => {
  fetchSettings().catch((error) => {
    console.error('Failed to load translation settings for cluster detail:', error);
  });
  window.addEventListener('force-translate-cluster', handleForceTranslateEvent);
});

onBeforeUnmount(() => {
  forceTranslateRunId.value += 1;
  isForceTranslating.value = false;
  window.removeEventListener('force-translate-cluster', handleForceTranslateEvent);
});

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
      'flex-1 min-w-0 bg-bg-primary flex flex-col h-full absolute w-full md:static md:w-auto z-30 transition-transform duration-300',
      clusterStore.currentClusterId ? 'translate-x-0 md:translate-x-0' : 'translate-x-full',
    ]"
  >
    <div
      v-if="isMobile && cluster"
      class="flex items-center gap-2 px-3 py-2 border-b border-border bg-bg-secondary"
    >
      <span class="flex-1 truncate text-sm font-medium text-center pr-8">{{ clusterTitle }}</span>
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

        <div class="flex items-center gap-0.5 sm:gap-1 pl-2" data-cluster-toolbar>
          <button
            v-if="translationEnabled"
            class="flex items-center justify-center w-8 h-8 sm:w-9 sm:h-9 rounded-lg hover:bg-bg-tertiary text-text-secondary transition-colors disabled:opacity-50"
            :title="t('article.action.forceReTranslate')"
            :disabled="isForceTranslating"
            @click="forceTranslateCluster"
          >
            <PhTranslate
              :size="18"
              class="sm:w-5 sm:h-5"
              :class="{ 'animate-spin': isForceTranslating }"
            />
          </button>
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

      <div
        :class="[
          'flex-1 overflow-y-auto w-full custom-scrollbar',
          isMobile && (hasPreviousCluster || hasNextCluster) ? 'pb-16' : '',
        ]"
      >
        <div
          class="max-w-[800px] w-[95%] sm:w-[90%] md:w-[85%] mx-auto py-6 sm:py-8 lg:py-12 px-4 sm:px-0"
        >
          <header class="mb-6 sm:mb-8 text-center sm:text-left">
            <h1
              class="text-2xl sm:text-3xl lg:text-4xl font-bold text-text-primary leading-tight font-serif mb-2 sm:mb-3"
            >
              {{ clusterTitle }}
            </h1>
            <p
              v-if="clusterOriginalTitle"
              class="text-sm sm:text-base leading-7 text-text-secondary mb-4 sm:mb-6"
            >
              {{ clusterOriginalTitle }}
            </p>

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
            v-if="clusterSummary"
            class="bg-bg-secondary p-4 rounded-lg mb-8 border border-border"
          >
            <h3 class="font-semibold text-lg mb-2">{{ t('article.cluster.summaryTitle') }}</h3>
            <p class="text-text-secondary leading-relaxed">{{ clusterSummary }}</p>
            <p
              v-if="clusterOriginalSummary"
              class="mt-3 border-t border-dashed border-border pt-3 text-sm leading-7 text-text-secondary/80"
            >
              {{ clusterOriginalSummary }}
            </p>
          </div>

          <div :class="{ 'translation-only-mode': translationOnlyMode && hasBodyTranslations }">
            <article
              ref="contentArticleRef"
              class="prose prose-content w-full"
              v-html="formattedContent"
            />
          </div>

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
                  {{ article.translated_title || article.title }}
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

    <!-- Navigation buttons - placed outside flex container for fixed positioning -->
    <div
      v-if="hasPreviousCluster || hasNextCluster"
      :class="[
        'flex items-center bg-bg-primary px-3 py-1.5',
        isMobile
          ? 'fixed bottom-0 left-0 right-0 border-t border-border z-20 justify-end gap-2'
          : [
              'absolute bottom-0 left-0 right-0',
              hasPreviousCluster && hasNextCluster
                ? 'justify-between'
                : hasPreviousCluster
                  ? 'justify-start'
                  : 'justify-end',
            ],
      ]"
    >
      <button
        v-if="hasPreviousCluster"
        :title="t('article.navigation.previousCluster') || 'Previous cluster'"
        :class="[
          'flex items-center gap-1.5 px-2 py-1 rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed',
          isMobile
            ? 'text-text-secondary hover:bg-bg-tertiary'
            : 'text-text-secondary/70 hover:text-text-primary hover:bg-bg-secondary/50',
        ]"
        @click="goToPreviousCluster"
      >
        <PhCaretLeft :size="16" />
        <span v-if="!isMobile" class="text-xs">{{
          t('article.navigation.previousCluster') || 'Previous'
        }}</span>
      </button>

      <button
        v-if="hasNextCluster"
        :title="t('article.navigation.nextCluster') || 'Next cluster'"
        :class="[
          'flex items-center gap-1.5 px-2 py-1 rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed',
          isMobile
            ? 'text-text-secondary hover:bg-bg-tertiary'
            : 'text-text-secondary/70 hover:text-text-primary hover:bg-bg-secondary/50',
        ]"
        @click="goToNextCluster"
      >
        <span v-if="!isMobile" class="text-xs">{{
          t('article.navigation.nextCluster') || 'Next'
        }}</span>
        <PhCaretRight :size="16" />
      </button>
    </div>
  </main>
</template>

<style src="./ArticleContent.css"></style>

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
