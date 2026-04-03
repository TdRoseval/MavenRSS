import { ref, onBeforeUnmount } from 'vue';

const DEFAULT_SIDEBAR_WIDTH = 234;
export const DESKTOP_ARTICLE_LIST_MIN_WIDTH = 180;
export const DESKTOP_ARTICLE_LIST_MAX_WIDTH = 420;
export const COMPACT_ARTICLE_LIST_MIN_WIDTH = 220;
export const COMPACT_ARTICLE_LIST_MAX_WIDTH = 520;
const DEFAULT_ARTICLE_LIST_WIDTH = DESKTOP_ARTICLE_LIST_MAX_WIDTH;

const sidebarWidth = ref<number>(DEFAULT_SIDEBAR_WIDTH);
const articleListWidth = ref<number>(DEFAULT_ARTICLE_LIST_WIDTH);
const isResizingSidebar = ref<boolean>(false);
const isResizingArticleList = ref<boolean>(false);
const compactMode = ref<boolean>(false);
const userManuallyResized = ref<boolean>(false);
const initialMouseX = ref<number>(0);
const initialArticleListWidth = ref<number>(DEFAULT_ARTICLE_LIST_WIDTH);

export function getDefaultArticleListWidth(isCompactMode: boolean): number {
  return isCompactMode ? COMPACT_ARTICLE_LIST_MAX_WIDTH : DESKTOP_ARTICLE_LIST_MAX_WIDTH;
}

export function useResizablePanels() {
  // Set compact mode state (doesn't change width by itself)
  function setCompactMode(enabled: boolean): void {
    compactMode.value = enabled;
  }

  function setSidebarWidth(width: number): void {
    sidebarWidth.value = width;
  }

  // Set article list width (called when settings are loaded or user changes compact mode)
  function setArticleListWidth(width: number): void {
    articleListWidth.value = width;
    // Reset user resize flag when setting from settings
    userManuallyResized.value = false;
  }

  // Sidebar resize handlers
  function startResizeSidebar(): void {
    isResizingSidebar.value = true;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    window.addEventListener('mousemove', handleResizeSidebar);
    window.addEventListener('mouseup', stopResizeSidebar);
  }

  function handleResizeSidebar(): void {
    if (!isResizingSidebar.value) return;
    const newWidth = (window.event as MouseEvent).clientX;
    if (newWidth >= 120 && newWidth <= 320) {
      sidebarWidth.value = newWidth;
    }
  }

  function stopResizeSidebar(): void {
    isResizingSidebar.value = false;
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
    window.removeEventListener('mousemove', handleResizeSidebar);
    window.removeEventListener('mouseup', stopResizeSidebar);
  }

  // Article list resize handlers
  function startResizeArticleList(event: MouseEvent): void {
    isResizingArticleList.value = true;
    // Store initial mouse position and article list width
    initialMouseX.value = event.clientX;
    initialArticleListWidth.value = articleListWidth.value;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    window.addEventListener('mousemove', handleResizeArticleList);
    window.addEventListener('mouseup', stopResizeArticleList);
  }

  function handleResizeArticleList(): void {
    if (!isResizingArticleList.value) return;
    const currentMouseX = (window.event as MouseEvent).clientX;
    // Calculate the delta from the initial position and apply to initial width
    const deltaX = currentMouseX - initialMouseX.value;
    const newWidth = initialArticleListWidth.value + deltaX;
    // Keep detail pane roomy on desktop while still allowing manual expansion.
    const minWidth = compactMode.value
      ? COMPACT_ARTICLE_LIST_MIN_WIDTH
      : DESKTOP_ARTICLE_LIST_MIN_WIDTH;
    const maxWidth = compactMode.value
      ? COMPACT_ARTICLE_LIST_MAX_WIDTH
      : DESKTOP_ARTICLE_LIST_MAX_WIDTH;
    if (newWidth >= minWidth && newWidth <= maxWidth) {
      articleListWidth.value = newWidth;
      // Mark that user has manually resized
      userManuallyResized.value = true;
    }
  }

  function stopResizeArticleList(): void {
    isResizingArticleList.value = false;
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
    window.removeEventListener('mousemove', handleResizeArticleList);
    window.removeEventListener('mouseup', stopResizeArticleList);
  }

  // Cleanup
  onBeforeUnmount(() => {
    window.removeEventListener('mousemove', handleResizeSidebar);
    window.removeEventListener('mouseup', stopResizeSidebar);
    window.removeEventListener('mousemove', handleResizeArticleList);
    window.removeEventListener('mouseup', stopResizeArticleList);
  });

  return {
    sidebarWidth,
    articleListWidth,
    startResizeSidebar,
    startResizeArticleList,
    setSidebarWidth,
    setCompactMode,
    setArticleListWidth,
  };
}
