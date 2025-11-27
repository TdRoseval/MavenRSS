import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue';
import { useAppStore } from '@/stores/app';
import { useI18n } from 'vue-i18n';
import { BrowserOpenURL } from '@/wailsjs/wailsjs/runtime/runtime';
import type { Article } from '@/types/models';

type ViewMode = 'original' | 'rendered';
type RenderAction = 'showContent' | 'showOriginal' | null;

interface ViewModeChangeEvent extends Event {
  detail: {
    mode: ViewMode;
  };
}

interface RenderActionEvent extends Event {
  detail: {
    action: RenderAction;
  };
}

export function useArticleDetail() {
  const store = useAppStore();
  const { t, locale } = useI18n();

  const article = computed<Article | undefined>(() =>
    store.articles.find((a) => a.id === store.currentArticleId)
  );
  const showContent = ref(false);
  const articleContent = ref('');
  const isLoadingContent = ref(false);
  const currentArticleId = ref<number | null>(null);
  const defaultViewMode = ref<ViewMode>('original');
  const pendingRenderAction = ref<RenderAction>(null);
  const userPreferredMode = ref<ViewMode | null>(null); // Remember user's manual choice
  const imageViewerSrc = ref<string | null>(null);
  const imageViewerAlt = ref('');

  // Watch for article changes and apply view mode
  watch(
    () => store.currentArticleId,
    async (newId, oldId) => {
      if (newId && newId !== oldId) {
        // Reset content when switching articles
        articleContent.value = '';
        currentArticleId.value = null;

        // Check if there's a pending render action from context menu
        if (pendingRenderAction.value) {
          // Apply the explicit action instead of default
          if (pendingRenderAction.value === 'showContent') {
            showContent.value = true;
            userPreferredMode.value = 'rendered';
            await fetchArticleContent();
          } else if (pendingRenderAction.value === 'showOriginal') {
            showContent.value = false;
            userPreferredMode.value = 'original';
          }
          pendingRenderAction.value = null; // Clear the pending action
        } else {
          // Apply user's preferred mode or default view mode
          const preferredMode = userPreferredMode.value || defaultViewMode.value;
          if (preferredMode === 'rendered') {
            showContent.value = true;
            await fetchArticleContent();
          } else {
            showContent.value = false;
          }
        }
      }
    }
  );

  // Listen for default view mode changes from settings
  window.addEventListener('default-view-mode-changed', (e: Event) => {
    const event = e as ViewModeChangeEvent;
    defaultViewMode.value = event.detail.mode;
    // Reset user preference when default changes
    userPreferredMode.value = null;
  });

  function close() {
    store.currentArticleId = null;
    showContent.value = false;
    articleContent.value = '';
    currentArticleId.value = null;
  }

  function toggleRead() {
    if (!article.value) return;
    const newState = !article.value.is_read;
    article.value.is_read = newState;
    fetch(`/api/articles/read?id=${article.value.id}&read=${newState}`, { method: 'POST' });
  }

  function toggleFavorite() {
    if (!article.value) return;
    const newState = !article.value.is_favorite;
    article.value.is_favorite = newState;
    fetch(`/api/articles/favorite?id=${article.value.id}`, { method: 'POST' });
  }

  function openOriginal() {
    if (article.value) BrowserOpenURL(article.value.url);
  }

  async function toggleContentView() {
    if (!showContent.value) {
      // Switching to content view - fetch content if needed
      if (!article.value) return;
      // Check if we need to fetch content (different article or no content yet)
      if (currentArticleId.value !== article.value.id) {
        await fetchArticleContent();
      }
    }
    showContent.value = !showContent.value;
    // Remember user's preference
    userPreferredMode.value = showContent.value ? 'rendered' : 'original';
  }

  async function fetchArticleContent() {
    if (!article.value) return;

    isLoadingContent.value = true;
    currentArticleId.value = article.value.id; // Track which article we're loading
    try {
      const res = await fetch(`/api/articles/content?id=${article.value.id}`);
      if (res.ok) {
        const data = await res.json();
        articleContent.value = data.content || '';
        // Wait for DOM to update, then attach event listeners
        await nextTick();
        attachContentEventListeners();
      } else {
        console.error('Failed to fetch article content');
        articleContent.value = '';
      }
    } catch (e) {
      console.error('Error fetching article content:', e);
      articleContent.value = '';
    } finally {
      isLoadingContent.value = false;
    }
  }

  // Attach event listeners to links and images in rendered content
  function attachContentEventListeners() {
    // Handle all links - open in default browser
    const links = document.querySelectorAll('.prose a');
    links.forEach((link) => {
      link.addEventListener('click', (e: Event) => {
        e.preventDefault();
        const href = link.getAttribute('href');
        if (href) {
          BrowserOpenURL(href);
        }
      });
    });

    // Handle all images - make them clickable for zoom/pan
    const images = document.querySelectorAll<HTMLImageElement>('.prose img');
    images.forEach((img) => {
      img.style.cursor = 'pointer';
      img.addEventListener('click', (e: Event) => {
        e.preventDefault();
        imageViewerSrc.value = img.src;
        imageViewerAlt.value = img.alt || '';
      });
    });
  }

  function closeImageViewer() {
    imageViewerSrc.value = null;
    imageViewerAlt.value = '';
  }

  // Listen for render content event from context menu
  async function handleRenderContent(e: Event) {
    const event = e as RenderActionEvent;
    if (!article.value) return;

    const action = event.detail?.action || 'showContent';

    // Mark as read when rendering content
    if (!article.value.is_read) {
      article.value.is_read = true;
      fetch(`/api/articles/read?id=${article.value.id}&read=true`, { method: 'POST' });
    }

    if (action === 'showContent') {
      // Check if we need to fetch content for this article
      if (currentArticleId.value !== article.value.id) {
        await fetchArticleContent();
      }
      showContent.value = true;
      userPreferredMode.value = 'rendered';
    } else if (action === 'showOriginal') {
      showContent.value = false;
      userPreferredMode.value = 'original';
    }
  }

  // Listen for explicit render action from context menu (before article selection)
  function handleExplicitRenderAction(e: Event) {
    const event = e as RenderActionEvent;
    pendingRenderAction.value = event.detail?.action;
  }

  // Handle toggle content view from keyboard shortcut
  function handleToggleContentView() {
    if (article.value) {
      toggleContentView();
    }
  }

  // Handle reset user preference from normal article selection
  function handleResetUserPreference() {
    userPreferredMode.value = null;
  }

  onMounted(async () => {
    window.addEventListener('render-article-content', handleRenderContent);
    window.addEventListener('explicit-render-action', handleExplicitRenderAction);
    window.addEventListener('toggle-content-view', handleToggleContentView);
    window.addEventListener('reset-user-view-preference', handleResetUserPreference);

    // Load default view mode from settings
    try {
      const res = await fetch('/api/settings');
      const data = await res.json();
      defaultViewMode.value = data.default_view_mode || 'original';
    } catch (e) {
      console.error('Error loading settings:', e);
    }
  });

  onBeforeUnmount(() => {
    window.removeEventListener('render-article-content', handleRenderContent);
    window.removeEventListener('explicit-render-action', handleExplicitRenderAction);
    window.removeEventListener('toggle-content-view', handleToggleContentView);
    window.removeEventListener('reset-user-view-preference', handleResetUserPreference);
  });

  return {
    // Reactive state
    article,
    showContent,
    articleContent,
    isLoadingContent,
    imageViewerSrc,
    imageViewerAlt,
    locale,

    // Functions
    close,
    toggleRead,
    toggleFavorite,
    openOriginal,
    toggleContentView,
    closeImageViewer,

    // Translations
    t,
  };
}
