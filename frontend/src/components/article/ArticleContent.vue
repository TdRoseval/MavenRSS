<script setup lang="ts">
import { ref, watch, onMounted, computed, nextTick } from 'vue';
import { useI18n } from 'vue-i18n';
import type { Article } from '@/types/models';
import ArticleTitle from './parts/ArticleTitle.vue';
import ArticleSummary from './parts/ArticleSummary.vue';
import ArticleLoading from './parts/ArticleLoading.vue';
import ArticleBody from './parts/ArticleBody.vue';
import AudioPlayer from './parts/AudioPlayer.vue';
import VideoPlayer from './parts/VideoPlayer.vue';
import { useArticleSummary } from '@/composables/article/useArticleSummary';
import { useArticleTranslation } from '@/composables/article/useArticleTranslation';
import { useArticleRendering } from '@/composables/article/useArticleRendering';
import {
  extractTextWithPlaceholders,
  restorePreservedElements,
  hasOnlyPreservedContent,
} from '@/composables/article/useContentTranslation';
import './ArticleContent.css';

interface SummaryResult {
  summary: string;
  sentence_count: number;
  is_too_short: boolean;
  error?: string;
}

interface Props {
  article: Article;
  articleContent: string;
  isLoadingContent: boolean;
  attachImageEventListeners?: () => void;
  showTranslations?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  showTranslations: true,
  attachImageEventListeners: undefined,
});

const { t } = useI18n();

// Use composables for summary and translation
const {
  summarySettings,
  loadSummarySettings,
  generateSummary: generateSummaryComposable,
  isSummaryLoading,
} = useArticleSummary();

const { translationSettings, loadTranslationSettings } = useArticleTranslation();

// Use composable for enhanced rendering (math formulas, etc.)
const { enhanceRendering, renderMathFormulas, highlightCodeBlocks } = useArticleRendering();

// Computed properties for easier access
const summaryEnabled = computed(() => summarySettings.value.enabled);
const translationEnabled = computed(() => translationSettings.value.enabled);
const targetLanguage = computed(() => translationSettings.value.targetLang);

// Current article summary
const summaryResult = ref<SummaryResult | null>(null);
const isLoadingSummary = computed(() =>
  props.article ? isSummaryLoading(props.article.id) : false
);

// Additional state for summary translation
const translatedSummary = ref('');
const isTranslatingSummary = ref(false);

// Additional state for translation
const translatedTitle = ref('');
const isTranslatingTitle = ref(false);
const isTranslatingContent = ref(false);
const lastTranslatedArticleId = ref<number | null>(null);

// Load settings using composables
async function loadSettings() {
  await loadSummarySettings();
  await loadTranslationSettings();
}

// Translate text using the API
async function translateText(text: string): Promise<string> {
  if (!text || !translationEnabled.value) return '';

  try {
    const res = await fetch('/api/articles/translate-text', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        text: text,
        target_language: targetLanguage.value,
      }),
    });

    if (res.ok) {
      const data = await res.json();
      return data.translated_text || '';
    } else {
      console.error('Error translating text:', res.status);
      window.showToast(t('errorTranslatingContent'), 'error');
    }
  } catch (e) {
    console.error('Error translating text:', e);
    window.showToast(t('errorTranslating'), 'error');
  }
  return '';
}

// Generate summary for the current article
async function generateSummary(article: Article) {
  if (!summaryEnabled.value || !article) {
    return;
  }

  summaryResult.value = null;
  translatedSummary.value = '';

  const result = await generateSummaryComposable(article);
  summaryResult.value = result;

  // Auto-translate summary if translation is enabled
  if (translationEnabled.value && result?.summary && !result.is_too_short) {
    isTranslatingSummary.value = true;
    translatedSummary.value = await translateText(result.summary);
    isTranslatingSummary.value = false;
  }
}

// Translate title
async function translateTitle(article: Article) {
  if (!translationEnabled.value || !article?.title) return;

  isTranslatingTitle.value = true;
  translatedTitle.value = await translateText(article.title);
  isTranslatingTitle.value = false;
}

// Translate content paragraphs while preserving inline elements (formulas, code, images)
async function translateContentParagraphs(content: string) {
  if (!translationEnabled.value || !content) return;

  // Prevent duplicate translations for the same article
  if (lastTranslatedArticleId.value === props.article?.id) {
    return;
  }

  isTranslatingContent.value = true;
  lastTranslatedArticleId.value = props.article?.id || null;

  // Wait for content to render
  await nextTick();

  // Find all text elements in the prose content
  const proseContainer = document.querySelector('.prose-content');
  if (!proseContainer) {
    isTranslatingContent.value = false;
    return;
  }

  // Remove any existing translations first
  const existingTranslations = proseContainer.querySelectorAll('.translation-text');
  existingTranslations.forEach((el) => el.remove());

  // Find all translatable elements
  // For lists: translate individual li items, translation stays inside the same li
  // For tables: translate td/th cells, translation stays inside the same cell
  // For blockquotes: translate inner paragraphs, not the blockquote itself
  const textTags = [
    'P',
    'H1',
    'H2',
    'H3',
    'H4',
    'H5',
    'H6',
    'LI',
    'TD',
    'TH',
    'FIGCAPTION',
    'DT',
    'DD',
  ];
  const elements = proseContainer.querySelectorAll(textTags.join(','));

  // Track which elements we've already translated to avoid duplicates
  const translatedElements = new Set<HTMLElement>();

  for (const el of elements) {
    const htmlEl = el as HTMLElement;

    // Skip if inside a translation element
    if (htmlEl.closest('.translation-text')) continue;

    // Skip if already has translation inside
    if (htmlEl.querySelector('.translation-text')) continue;

    // Skip if we've already translated this element
    if (translatedElements.has(htmlEl)) continue;

    // Skip if this element's parent was already translated
    // This prevents duplicate translations of nested elements
    let hasTranslatedAncestor = false;
    let ancestor = htmlEl.parentElement;
    while (ancestor && ancestor !== proseContainer) {
      if (translatedElements.has(ancestor)) {
        hasTranslatedAncestor = true;
        break;
      }
      ancestor = ancestor.parentElement;
    }
    if (hasTranslatedAncestor) continue;

    // Skip elements that are entirely technical content (no translatable text)
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

    // Skip elements that only contain preserved content (no translatable text)
    if (hasOnlyPreservedContent(htmlEl)) {
      continue;
    }

    // Extract text with placeholders for inline elements (formulas, code, images) and hyperlinks
    const {
      text: textWithPlaceholders,
      preservedElements,
      hyperlinks,
    } = extractTextWithPlaceholders(htmlEl);

    if (!textWithPlaceholders || textWithPlaceholders.length < 2) continue;

    // Translate the text (with placeholders and link markers)
    const translatedText = await translateText(textWithPlaceholders);

    // Skip if translation is same as original or empty
    if (!translatedText || translatedText === textWithPlaceholders) continue;

    // Restore preserved elements and hyperlinks in the translated text
    const translatedHTML = restorePreservedElements(translatedText, preservedElements, hyperlinks);

    // Determine how to insert translation based on element type
    const tagName = htmlEl.tagName;

    if (
      tagName === 'LI' ||
      tagName === 'TD' ||
      tagName === 'TH' ||
      tagName === 'DD' ||
      tagName === 'DT'
    ) {
      // For list items, table cells, definition list items: append translation inside the same element
      const translationEl = document.createElement('div');
      translationEl.className = 'translation-text translation-inline';
      translationEl.innerHTML = translatedHTML;
      htmlEl.appendChild(translationEl);
    } else if (htmlEl.closest('blockquote')) {
      // For elements inside blockquote: append translation inside, styled differently
      const translationEl = document.createElement('div');
      translationEl.className = 'translation-text translation-blockquote';
      translationEl.innerHTML = translatedHTML;
      htmlEl.appendChild(translationEl);
    } else {
      // For standalone paragraphs, headings, figcaption: insert after as sibling
      const translationEl = document.createElement('div');
      translationEl.className = 'translation-text';
      translationEl.innerHTML = translatedHTML;
      htmlEl.parentNode?.insertBefore(translationEl, htmlEl.nextSibling);
    }

    // Mark this element as translated
    translatedElements.add(htmlEl);
  }

  // Re-apply rendering enhancements to translation elements (for math formulas)
  await nextTick();
  proseContainer.querySelectorAll('.translation-text').forEach((el) => {
    renderMathFormulas(el as HTMLElement);
    highlightCodeBlocks(el as HTMLElement);
  });

  // Re-attach ALL event listeners after translation modifies the DOM
  // This includes unwrapping images from links, attaching image handlers, and link handlers
  await reattachImageInteractions();

  isTranslatingContent.value = false;
}

async function reattachImageInteractions() {
  if (!props.attachImageEventListeners || !props.articleContent) return;
  await nextTick();
  props.attachImageEventListeners();
}

// Watch for article changes and regenerate summary + translations
watch(
  () => props.article?.id,
  (newId, oldId) => {
    if (newId !== oldId) {
      summaryResult.value = null;
      translatedSummary.value = '';
      translatedTitle.value = '';
      lastTranslatedArticleId.value = null; // Reset translation tracking

      if (props.article) {
        if (summaryEnabled.value) {
          // Delay summary generation when switching articles
          setTimeout(() => generateSummary(props.article), 100);
        }
        if (translationEnabled.value) {
          translateTitle(props.article);
        }
      }
    }
  }
);

// Watch for content loading completion only
watch(
  () => props.isLoadingContent,
  (isLoading, wasLoading) => {
    if (wasLoading && !isLoading && props.article) {
      // Enhance rendering first (math formulas, etc.)
      nextTick(() => {
        enhanceRendering('.prose-content');
      });

      // Delay summary generation to prioritize content display
      if (summaryEnabled.value) {
        setTimeout(() => generateSummary(props.article), 100);
      }
      if (translationEnabled.value && lastTranslatedArticleId.value !== props.article.id) {
        nextTick(() => translateContentParagraphs(props.articleContent));
      }
    }
  }
);

onMounted(async () => {
  await loadSettings();
  if (props.article) {
    // Enhance rendering if content is already loaded
    if (props.articleContent && !props.isLoadingContent) {
      await nextTick();
      enhanceRendering('.prose-content');
    }

    if (summaryEnabled.value && props.articleContent) {
      // Delay summary generation to ensure content displays first
      setTimeout(() => generateSummary(props.article), 100);
    }
    if (translationEnabled.value) {
      translateTitle(props.article);
      if (props.articleContent && !props.isLoadingContent) {
        nextTick(() => translateContentParagraphs(props.articleContent));
      }
    }
  }
});

// Ensure image interactions stay attached when content is (re)rendered
watch(
  () => props.articleContent,
  (content) => {
    if (content) {
      reattachImageInteractions();
    }
  },
  { immediate: true }
);
</script>

<template>
  <div class="flex-1 overflow-y-auto bg-bg-primary p-3 sm:p-6">
    <div
      class="max-w-3xl mx-auto bg-bg-primary"
      :class="{ 'hide-translations': !showTranslations }"
    >
      <ArticleTitle
        :article="article"
        :translated-title="translatedTitle"
        :is-translating-title="isTranslatingTitle"
        :translation-enabled="translationEnabled"
      />

      <!-- Audio Player (if article has audio) -->
      <AudioPlayer
        v-if="article.audio_url"
        :audio-url="article.audio_url"
        :article-title="article.title"
      />

      <!-- Video Player (if article has video) -->
      <VideoPlayer
        v-if="article.video_url"
        :video-url="article.video_url"
        :article-title="article.title"
      />

      <ArticleSummary
        :summary-result="summaryResult"
        :is-loading-summary="isLoadingSummary"
        :translated-summary="translatedSummary"
        :is-translating-summary="isTranslatingSummary"
        :translation-enabled="translationEnabled"
      />

      <ArticleLoading v-if="isLoadingContent" />

      <ArticleBody
        v-else
        :article-content="articleContent"
        :is-translating-content="isTranslatingContent"
        :has-media-content="!!(article.audio_url || article.video_url)"
      />
    </div>
  </div>
</template>
