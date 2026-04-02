<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  PhRobot,
  PhChatCircleText,
  PhTrash,
  PhBroom,
  PhMagnifyingGlass,
  PhRocket,
  PhSparkle,
} from '@phosphor-icons/vue';
import {
  TipBox,
  SettingGroup,
  SettingWithToggle,
  NestedSettingsContainer,
  SubSettingItem,
} from '@/components/settings';
import AIProfileSelector from './AIProfileSelector.vue';
import '@/components/settings/styles.css';
import type { SettingsData } from '@/types/settings';
import type { ClusterRenormalizeResponse } from '@/types/models';
import { authDelete, authPost } from '@/shared/lib/authFetch';
import { useAIProfiles } from '@/composables/ai/useAIProfiles';
import { hasValidEmbeddingModelConfig } from '@/shared/lib/aiEnhancedMode';

const { t } = useI18n();
const { profiles, hasProfiles, fetchProfiles } = useAIProfiles();

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();

function updateSetting(key: keyof SettingsData, value: any) {
  emit('update:settings', normalizeFeatureSettings({
    ...props.settings,
    [key]: value,
  }));
}

const isDeleting = ref(false);
const isReclusterNormalizing = ref(false);

onMounted(() => {
  if (!hasProfiles.value) {
    fetchProfiles();
  }
});

function hasValidProfile(profileID: string) {
  const normalizedProfileID = String(profileID || '').trim();
  if (normalizedProfileID === '') {
    return false;
  }

  return profiles.value.some((profile) => String(profile.id) === normalizedProfileID);
}

const hasValidFusionProfile = computed(() => {
  const profileID = String(props.settings.ai_fusion_profile_id || '').trim();
  if (profileID === '') {
    return false;
  }

  return hasValidProfile(profileID);
});

const hasValidRecommendationProfile = computed(() => {
  const profileID = String(props.settings.ai_recommendation_profile_id || '').trim();
  if (profileID === '') {
    return false;
  }

  return hasValidProfile(profileID);
});

function hasBaseAIFeaturePrerequisites(settings: SettingsData) {
  return (
    hasProfiles.value &&
    hasValidEmbeddingModelConfig(settings) &&
    settings.summary_enabled === true &&
    settings.summary_provider === 'ai' &&
    settings.translation_enabled === true &&
    settings.ai_search_enabled === true &&
    settings.ai_chat_enabled === true
  );
}

function canEnableRecommendation(settings: SettingsData) {
  return hasBaseAIFeaturePrerequisites(settings) && settings.ai_fusion_enabled === true;
}

function canEnableAIEnhancedMode(settings: SettingsData) {
  return (
    hasBaseAIFeaturePrerequisites(settings) &&
    settings.ai_fusion_enabled === true &&
    settings.ai_recommendation_enabled === true
  );
}

function normalizeFeatureSettings(settings: SettingsData): SettingsData {
  const nextSettings = { ...settings };

  if (!canEnableRecommendation(nextSettings) || !hasValidProfile(nextSettings.ai_fusion_profile_id)) {
    nextSettings.ai_recommendation_enabled = false;
  }

  if (
    !canEnableAIEnhancedMode(nextSettings) ||
    !hasValidProfile(nextSettings.ai_fusion_profile_id) ||
    !hasValidProfile(nextSettings.ai_recommendation_profile_id)
  ) {
    nextSettings.ai_enhanced_mode = false;
  }

  return nextSettings;
}

const canConfigureFusion = computed(() => hasBaseAIFeaturePrerequisites(props.settings));

const canConfigureRecommendation = computed(() => canEnableRecommendation(props.settings));

const canConfigureAIEnhancedMode = computed(() => canEnableAIEnhancedMode(props.settings));

const isAIEnhancedModeAvailable = computed(
  () =>
    canConfigureAIEnhancedMode.value &&
    hasValidFusionProfile.value &&
    hasValidRecommendationProfile.value
);

async function clearAllChatSessions() {
  const confirmed = await window.showConfirm({
    title: t('setting.ai.clearAllChats'),
    message: t('setting.ai.clearAllChatsConfirm'),
    isDanger: true,
  });
  if (!confirmed) return;

  isDeleting.value = true;
  try {
    const data = await authDelete('/api/ai/chat/sessions/delete-all');
    window.showToast(t('setting.ai.clearAllChatsSuccess', { count: data.count || 0 }), 'success');
  } catch (error) {
    console.error('Failed to clear chat sessions:', error);
    window.showToast(t('setting.ai.clearAllChatsFailed'), 'error');
  } finally {
    isDeleting.value = false;
  }
}

async function reclusterNormalizeArticles() {
  const confirmed = await window.showConfirm({
    title: t('setting.ai.reclusterNormalizeTitle'),
    message: t('setting.ai.reclusterNormalizeConfirm'),
    isDanger: true,
  });
  if (!confirmed) return;

  isReclusterNormalizing.value = true;
  try {
    const data = await authPost<ClusterRenormalizeResponse>('/api/clusters/recluster-normalize');
    if (data.scheduled) {
      window.showToast(t('setting.ai.reclusterNormalizeStarted'), 'success');
      return;
    }

    if (data.reason === 'busy') {
      window.showToast(t('setting.ai.reclusterNormalizeBusy'), 'warning');
      return;
    }

    window.showToast(t('setting.ai.reclusterNormalizeDisabled'), 'warning');
  } catch (error) {
    console.error('Failed to start article cluster renormalization:', error);
    window.showToast(t('setting.ai.reclusterNormalizeFailed'), 'error');
  } finally {
    isReclusterNormalizing.value = false;
  }
}
</script>

<template>
  <SettingGroup :icon="PhRobot" :title="t('setting.ai.aiFeatures')">
    <TipBox type="info" :title="t('setting.ai.isBeta')" />
    <SettingWithToggle
      :icon="PhMagnifyingGlass"
      :title="t('setting.ai.aiSearchEnabled')"
      :description="t('setting.ai.aiSearchEnabledDesc')"
      :model-value="props.settings.ai_search_enabled"
      @update:model-value="updateSetting('ai_search_enabled', $event)"
    />

    <NestedSettingsContainer v-if="props.settings.ai_search_enabled">
      <SubSettingItem
        :icon="PhRobot"
        :title="t('setting.ai.selectProfile')"
        :description="t('setting.ai.selectProfileForSearch')"
      >
        <AIProfileSelector
          :model-value="props.settings.ai_search_profile_id"
          @update:model-value="updateSetting('ai_search_profile_id', $event)"
        />
      </SubSettingItem>
    </NestedSettingsContainer>

    <SettingWithToggle
      :icon="PhChatCircleText"
      :title="t('setting.ai.aiChatEnabled')"
      :description="t('setting.ai.aiChatEnabledDesc')"
      :model-value="props.settings.ai_chat_enabled"
      @update:model-value="updateSetting('ai_chat_enabled', $event)"
    />

    <NestedSettingsContainer v-if="props.settings.ai_chat_enabled">
      <SubSettingItem
        :icon="PhRobot"
        :title="t('setting.ai.selectProfile')"
        :description="t('setting.ai.selectProfileForChat')"
      >
        <AIProfileSelector
          :model-value="props.settings.ai_chat_profile_id"
          @update:model-value="updateSetting('ai_chat_profile_id', $event)"
        />
      </SubSettingItem>

      <SubSettingItem
        :icon="PhTrash"
        :title="t('setting.ai.clearAllChats')"
        :description="t('setting.ai.clearAllChatsDesc')"
      >
        <button
          type="button"
          :disabled="isDeleting"
          class="btn-secondary"
          @click="clearAllChatSessions"
        >
          <PhBroom :size="16" class="sm:w-5 sm:h-5" />
          {{ isDeleting ? t('setting.database.cleaning') : t('setting.ai.clearAllChatsButton') }}
        </button>
      </SubSettingItem>
    </NestedSettingsContainer>

    <TipBox v-if="!canConfigureFusion" type="warning" :title="t('setting.ai.aiFusionDisabled')" />
    <TipBox
      v-else-if="props.settings.ai_fusion_enabled && !hasValidFusionProfile"
      type="warning"
      :title="t('setting.ai.aiFusionRequiresProfile')"
    />
    <SettingWithToggle
      :icon="PhRobot"
      :title="t('setting.ai.aiFusionEnabled')"
      :description="t('setting.ai.aiFusionEnabledDesc')"
      :model-value="props.settings.ai_fusion_enabled"
      :disabled="!canConfigureFusion"
      @update:model-value="updateSetting('ai_fusion_enabled', $event)"
    />

    <NestedSettingsContainer v-if="props.settings.ai_fusion_enabled">
      <SubSettingItem
        :icon="PhRobot"
        :title="t('setting.ai.selectFusionProfile')"
        :description="t('setting.ai.selectFusionProfileDesc')"
      >
        <AIProfileSelector
          :model-value="props.settings.ai_fusion_profile_id"
          @update:model-value="updateSetting('ai_fusion_profile_id', $event)"
        />
      </SubSettingItem>
    </NestedSettingsContainer>

    <TipBox
      v-if="!canConfigureRecommendation"
      type="warning"
      :title="t('setting.ai.aiRecommendationDisabled')"
    />
    <TipBox
      v-else-if="props.settings.ai_recommendation_enabled && !hasValidRecommendationProfile"
      type="warning"
      :title="t('setting.ai.aiRecommendationRequiresProfile')"
    />
    <SettingWithToggle
      :icon="PhSparkle"
      :title="t('setting.ai.aiRecommendationEnabled')"
      :description="t('setting.ai.aiRecommendationEnabledDesc')"
      :model-value="props.settings.ai_recommendation_enabled"
      :disabled="!canConfigureRecommendation"
      @update:model-value="updateSetting('ai_recommendation_enabled', $event)"
    />

    <NestedSettingsContainer v-if="props.settings.ai_recommendation_enabled">
      <SubSettingItem
        :icon="PhRobot"
        :title="t('setting.ai.selectRecommendationProfile')"
        :description="t('setting.ai.selectRecommendationProfileDesc')"
      >
        <AIProfileSelector
          :model-value="props.settings.ai_recommendation_profile_id"
          @update:model-value="updateSetting('ai_recommendation_profile_id', $event)"
        />
      </SubSettingItem>
    </NestedSettingsContainer>

    <TipBox
      v-if="!canConfigureAIEnhancedMode"
      type="warning"
      :title="t('setting.ai.aiEnhancedModeDisabled')"
    />
    <TipBox
      v-else-if="!hasValidFusionProfile"
      type="warning"
      :title="t('setting.ai.aiEnhancedModeRequiresFusionProfile')"
    />
    <TipBox
      v-else-if="!hasValidRecommendationProfile"
      type="warning"
      :title="t('setting.ai.aiEnhancedModeRequiresRecommendationProfile')"
    />
    <SettingWithToggle
      :icon="PhRocket"
      :title="t('setting.ai.aiEnhancedMode')"
      :description="t('setting.ai.aiEnhancedModeDesc')"
      :model-value="isAIEnhancedModeAvailable ? props.settings.ai_enhanced_mode : false"
      :disabled="!isAIEnhancedModeAvailable"
      @update:model-value="updateSetting('ai_enhanced_mode', $event)"
    />

    <NestedSettingsContainer v-if="props.settings.ai_enhanced_mode">
      <SubSettingItem
        :icon="PhBroom"
        :title="t('setting.ai.reclusterNormalizeTitle')"
        :description="t('setting.ai.reclusterNormalizeDesc')"
      >
        <button
          type="button"
          :disabled="isReclusterNormalizing"
          class="btn-secondary text-red-500 border-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 dark:border-red-400"
          @click="reclusterNormalizeArticles"
        >
          <PhBroom :size="16" class="sm:w-5 sm:h-5" />
          {{
            isReclusterNormalizing
              ? t('setting.ai.reclusterNormalizeStarting')
              : t('setting.ai.reclusterNormalizeButton')
          }}
        </button>
      </SubSettingItem>
    </NestedSettingsContainer>
  </SettingGroup>
</template>

<style scoped>
@reference "../../../../style.css";
</style>
