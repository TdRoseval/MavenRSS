import type { SettingsData } from '@/types/settings';

type SettingsLike = Partial<SettingsData> & Record<string, unknown>;

interface EmbeddingModelLike {
  modelname?: unknown;
  baseurl?: unknown;
}

export function isEnabledSetting(value: unknown): boolean {
  return value === true || value === 'true';
}

function normalizeString(value: unknown): string {
  if (typeof value === 'string') {
    return value.trim();
  }

  if (value === null || value === undefined) {
    return '';
  }

  return String(value).trim();
}

function isValidEmbeddingModel(model: unknown): boolean {
  if (!model || typeof model !== 'object') {
    return false;
  }

  const embeddingModel = model as EmbeddingModelLike;
  return (
    normalizeString(embeddingModel.modelname) !== '' &&
    normalizeString(embeddingModel.baseurl) !== ''
  );
}

export function hasValidEmbeddingModelConfig(settings: SettingsLike): boolean {
  const rawModels = normalizeString(settings.ai_embedding_models);
  if (rawModels === '') {
    return false;
  }

  try {
    const models = JSON.parse(rawModels);
    return Array.isArray(models) && models.some((model) => isValidEmbeddingModel(model));
  } catch {
    return false;
  }
}

export function hasAIEnhancedModePrerequisites(settings: SettingsLike): boolean {
  return (
    hasValidEmbeddingModelConfig(settings) &&
    isEnabledSetting(settings.summary_enabled) &&
    normalizeString(settings.summary_provider) === 'ai' &&
    isEnabledSetting(settings.translation_enabled) &&
    isEnabledSetting(settings.ai_search_enabled) &&
    isEnabledSetting(settings.ai_chat_enabled) &&
    isEnabledSetting(settings.ai_fusion_enabled) &&
    isEnabledSetting(settings.ai_recommendation_enabled)
  );
}

export function isAIEnhancedModeEffectivelyEnabled(settings: SettingsLike): boolean {
  return isEnabledSetting(settings.ai_enhanced_mode) && hasAIEnhancedModePrerequisites(settings);
}
