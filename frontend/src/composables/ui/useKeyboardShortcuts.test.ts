import { defineComponent, h } from 'vue';
import { mount } from '@vue/test-utils';
import { createPinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useKeyboardShortcuts } from './useKeyboardShortcuts';
import { useArticleStore } from '@/features/article/store';
import { useClusterStore } from '@/stores/cluster';

describe('useKeyboardShortcuts detail scrolling', () => {
  let wrapper: ReturnType<typeof mount> | undefined;
  let detail: HTMLElement | undefined;
  let scrollableContent: HTMLElement | undefined;

  beforeEach(() => {
    detail = document.createElement('main');
    detail.dataset.clusterDetail = '';
    scrollableContent = document.createElement('div');
    scrollableContent.className = 'overflow-y-auto';
    Object.defineProperty(scrollableContent, 'clientHeight', { value: 500 });
    detail.appendChild(scrollableContent);
    document.body.appendChild(detail);

    wrapper = mount(
      defineComponent({
        setup() {
          const articleStore = useArticleStore();
          articleStore.setAIEnhancedMode(true);
          articleStore.currentFilter = 'clusters';

          const clusterStore = useClusterStore();
          clusterStore.currentClusterId = 1;

          useKeyboardShortcuts({
            onOpenSettings: vi.fn(),
            onAddFeed: vi.fn(),
            onMarkAllRead: vi.fn(async () => {}),
          });
          return () => h('div');
        },
      }),
      {
        attachTo: document.body,
        global: { plugins: [createPinia()] },
      }
    );
  });

  afterEach(() => {
    wrapper?.unmount();
    detail?.remove();
  });

  it('scrolls an AI cluster detail with arrow keys without requiring focus', () => {
    const scrollTo = vi.fn();
    scrollableContent!.scrollTo = scrollTo;

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown' }));

    expect(scrollTo).toHaveBeenCalledWith({ top: 100, behavior: 'smooth' });
  });

  it('uses the cluster detail viewport for Space paging', () => {
    const scrollTo = vi.fn();
    scrollableContent!.scrollTop = 20;
    scrollableContent!.scrollTo = scrollTo;

    window.dispatchEvent(new KeyboardEvent('keydown', { key: ' ' }));

    expect(scrollTo).toHaveBeenCalledWith({ top: 470, behavior: 'smooth' });
  });
});
