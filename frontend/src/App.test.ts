import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia } from 'pinia';
import { createI18n } from 'vue-i18n';
import en from './i18n/locales/en';
import App from './App.vue';

// Create stub components for complex child components
const createStub = (name: string) => ({
  name,
  template: '<div class="stub-component"><slot /></div>',
});

describe('App', () => {
  it('renders properly', () => {
    const pinia = createPinia();
    const i18n = createI18n({
      legacy: false,
      locale: 'en',
      messages: { en },
    });

    const wrapper = mount(App, {
      global: {
        plugins: [pinia, i18n],
        stubs: {
          Sidebar: createStub('Sidebar'),
          ArticleList: createStub('ArticleList'),
          ArticleDetail: createStub('ArticleDetail'),
          ImageGalleryView: createStub('ImageGalleryView'),
          AddFeedModal: createStub('AddFeedModal'),
          EditFeedModal: createStub('EditFeedModal'),
          SettingsModal: createStub('SettingsModal'),
          DiscoverFeedsModal: createStub('DiscoverFeedsModal'),
          UpdateAvailableDialog: createStub('UpdateAvailableDialog'),
          ContextMenu: createStub('ContextMenu'),
          ConfirmDialog: createStub('ConfirmDialog'),
          InputDialog: createStub('InputDialog'),
          MultiSelectDialog: createStub('MultiSelectDialog'),
          Toast: createStub('Toast'),
        },
        mocks: {
          $window: {
            showToast: vi.fn(),
            showConfirm: vi.fn(() => Promise.resolve(true)),
          },
        },
      },
    });

    // Check that the app container is rendered
    expect(wrapper.find('.app-container').exists()).toBe(true);
  });
});
