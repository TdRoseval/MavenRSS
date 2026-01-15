import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia } from 'pinia';
import { createI18n } from 'vue-i18n';
import en from './i18n/locales/en';
import App from './App.vue';

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
      },
    });
    // Check that the app container is rendered
    expect(wrapper.find('.app-container').exists()).toBe(true);
    // Check that key UI elements are present
    expect(wrapper.text()).toContain('All Articles');
  });
});
