import { ref, type Ref } from 'vue';
import { useI18n } from 'vue-i18n';
import type { Article } from '@/types/models';

interface SummarySettings {
  enabled: boolean;
  length: string;
  provider: string;
}

interface SummaryResult {
  summary: string;
  sentence_count: number;
  is_too_short: boolean;
  error?: string;
}

export function useArticleSummary() {
  const { t } = useI18n();
  const summarySettings = ref<SummarySettings>({
    enabled: false,
    length: 'medium',
    provider: 'local',
  });
  const summaryCache: Ref<Map<number, SummaryResult>> = ref(new Map());
  const loadingSummaries: Ref<Set<number>> = ref(new Set());

  // Load summary settings
  async function loadSummarySettings(): Promise<void> {
    try {
      const res = await fetch('/api/settings');
      const data = await res.json();
      summarySettings.value = {
        enabled: data.summary_enabled === 'true',
        length: data.summary_length || 'medium',
        provider: data.summary_provider || 'local',
      };
    } catch (e) {
      console.error('Error loading summary settings:', e);
    }
  }

  // Generate summary for an article
  async function generateSummary(article: Article): Promise<SummaryResult | null> {
    if (!summarySettings.value.enabled) {
      return null;
    }

    // Check cache first
    if (summaryCache.value.has(article.id)) {
      return summaryCache.value.get(article.id) || null;
    }

    // Check if already loading
    if (loadingSummaries.value.has(article.id)) {
      return null;
    }

    loadingSummaries.value.add(article.id);

    try {
      const res = await fetch('/api/articles/summarize', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          article_id: article.id,
          length: summarySettings.value.length,
        }),
      });

      if (res.ok) {
        const data: SummaryResult = await res.json();
        summaryCache.value.set(article.id, data);

        // Show notification if AI limit was reached
        if (data.limit_reached) {
          window.showToast(t('aiLimitReached'), 'warning');
        }

        return data;
      }
    } catch (e) {
      console.error('Error generating summary:', e);
    } finally {
      loadingSummaries.value.delete(article.id);
    }

    return null;
  }

  // Get summary from cache
  function getCachedSummary(articleId: number): SummaryResult | null {
    return summaryCache.value.get(articleId) || null;
  }

  // Check if summary is loading
  function isSummaryLoading(articleId: number): boolean {
    return loadingSummaries.value.has(articleId);
  }

  // Clear cache for a specific article or all
  function clearSummaryCache(articleId?: number): void {
    if (articleId !== undefined) {
      summaryCache.value.delete(articleId);
    } else {
      summaryCache.value.clear();
    }
  }

  // Update summary settings from event
  function handleSummarySettingsChange(enabled: boolean, length: string, provider?: string): void {
    summarySettings.value = {
      enabled,
      length,
      provider: provider || summarySettings.value.provider,
    };
    // Clear cache when settings change to regenerate with new settings
    clearSummaryCache();
  }

  return {
    summarySettings,
    loadingSummaries,
    loadSummarySettings,
    generateSummary,
    getCachedSummary,
    isSummaryLoading,
    clearSummaryCache,
    handleSummarySettingsChange,
  };
}
