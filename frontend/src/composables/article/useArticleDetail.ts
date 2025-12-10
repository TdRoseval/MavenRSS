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

  async function toggleReadLater() {
    if (!article.value) return;
    const newState = !article.value.is_read_later;
    article.value.is_read_later = newState;
    // When adding to read later, also mark as unread
    if (newState) {
      article.value.is_read = false;
    }
    try {
      await fetch(`/api/articles/toggle-read-later?id=${article.value.id}`, { method: 'POST' });
      store.fetchUnreadCounts();
    } catch (e) {
      console.error('Error toggling read later:', e);
      // Revert on error
      article.value.is_read_later = !newState;
    }
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
    // Attach image event handlers BEFORE unwrapping from links to ensure click events work properly on images within anchor tags.
    const images = document.querySelectorAll<HTMLImageElement>('.prose img');
    images.forEach((img) => {
      img.style.cursor = 'pointer';
      // Left click - open image viewer
      img.addEventListener('click', (e: Event) => {
        e.preventDefault();
        e.stopPropagation(); // Prevent event bubbling to parent link elements
        imageViewerSrc.value = img.src;
        imageViewerAlt.value = img.alt || '';
      });
      // Right click - show context menu for saving
      img.addEventListener('contextmenu', (e: MouseEvent) => {
        e.preventDefault();
        e.stopPropagation(); // Prevent event bubbling to parent link elements
        // Use global context menu system
        window.dispatchEvent(
          new CustomEvent('open-context-menu', {
            detail: {
              x: e.clientX,
              y: e.clientY,
              items: [
                {
                  label: t('viewImage'),
                  action: 'view',
                  icon: 'PhMagnifyingGlassPlus',
                },
                {
                  label: t('downloadImage'),
                  action: 'download',
                  icon: 'PhDownloadSimple',
                },
              ],
              data: { src: img.src },
              callback: (action: string, data: { src: string }) => {
                if (action === 'view') {
                  imageViewerSrc.value = data.src;
                  imageViewerAlt.value = '';
                } else if (action === 'download') {
                  downloadImage(data.src);
                }
              },
            },
          })
        );
      });
    });

    // Unwrap images from anchor tags to prevent navigation conflicts
    const imagesInLinks = document.querySelectorAll<HTMLImageElement>('.prose a img');
    imagesInLinks.forEach((img) => {
      const anchor = img.closest('a');
      if (anchor && anchor.parentNode) {
        // Replace the anchor with just the image
        anchor.parentNode.insertBefore(img, anchor);
        anchor.remove();
      }
    });

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
  }

  function closeImageViewer() {
    imageViewerSrc.value = null;
    imageViewerAlt.value = '';
  }

  // Download image from URL
  async function downloadImage(src: string) {
    try {
      const response = await fetch(src);
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const blob = await response.blob();

      // Extract and sanitize filename from URL
      let filename = 'image';
      try {
        const url = new URL(src);
        const pathname = url.pathname;
        const pathSegments = pathname.split('/').filter((segment) => segment.length > 0);
        if (pathSegments.length > 0) {
          const lastSegment = pathSegments[pathSegments.length - 1];
          filename = lastSegment.split('?')[0].replace(/[^a-zA-Z0-9._-]/g, '_') || 'image';
        }
      } catch {
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
      window.open(src, '_blank');
    }
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
    toggleReadLater,
    openOriginal,
    toggleContentView,
    closeImageViewer,
    downloadImage,

    // Translations
    t,
  };
}
