import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import en from '@/i18n/locales/en';
import ArticleSummary from './ArticleSummary.vue';

function mountSummary(loading: boolean) {
  const i18n = createI18n({ legacy: false, locale: 'en', messages: { en } });
  return mount(ArticleSummary, {
    props: {
      summaryResult: null,
      isLoadingSummary: loading,
      translationEnabled: false,
    },
    global: { plugins: [i18n] },
  });
}

// The loading timer must only run while a summary is actually loading. Before
// the fix it ticked every 100ms for the whole component lifetime.
describe('ArticleSummary loading timer', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('does not run an interval when not loading', () => {
    const spy = vi.spyOn(window, 'setInterval');
    mountSummary(false);
    expect(spy).not.toHaveBeenCalled();
  });

  it('starts the timer when loading begins and stops it when loading ends', async () => {
    const setIntervalSpy = vi.spyOn(window, 'setInterval');
    const clearIntervalSpy = vi.spyOn(window, 'clearInterval');

    const wrapper = mountSummary(false);
    setIntervalSpy.mockClear();

    await wrapper.setProps({ isLoadingSummary: true });
    expect(setIntervalSpy).toHaveBeenCalledTimes(1);

    await wrapper.setProps({ isLoadingSummary: false });
    expect(clearIntervalSpy).toHaveBeenCalledTimes(1);

    // Unmount after already stopped must not clear again.
    wrapper.unmount();
    expect(clearIntervalSpy).toHaveBeenCalledTimes(1);
  });
});