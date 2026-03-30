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
import { authDelete } from '@/shared/lib/authFetch';
import { useAIProfiles } from '@/composables/ai/useAIProfiles';

const { t } = useI18n();
const { hasProfiles, fetchProfiles } = useAIProfiles();

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();

function updateSetting(key: keyof SettingsData, value: any) {
  emit('update:settings', {
    ...props.settings,
    [key]: value,
  });
}

const isDeleting = ref(false);

onMounted(() => {
  if (!hasProfiles.value) {
    fetchProfiles();
  }
});

// Check if AI enhanced mode can be enabled
const isAIEnhancedModeAvailable = computed(() => {
  const settings = props.settings;
  
  let hasEmbeddings = false;
  try {
    const models = JSON.parse(settings.ai_embedding_models || '[]');
    hasEmbeddings = Array.isArray(models) && models.length > 0;
  } catch(e) { /* ignore */ }

  return (
    hasProfiles.value &&
    hasEmbeddings &&
    settings.summary_enabled === true &&
    settings.summary_provider === 'ai' &&
    settings.translation_enabled === true &&
    settings.ai_search_enabled === true &&
    settings.ai_chat_enabled === true
  );
});

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
</script>

<template>
  <SettingGroup :icon="PhRobot" :title="t('setting.ai.aiFeatures')">
    <!-- AI Search -->
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

    <!-- AI Chat -->
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

    <!-- AI Enhanced Mode -->
    <TipBox
      v-if="!isAIEnhancedModeAvailable"
      type="warning"
      :title="t('setting.ai.aiEnhancedModeDisabled')"
    />
    <SettingWithToggle
      :icon="PhRocket"
      :title="t('setting.ai.aiEnhancedMode')"
      :description="t('setting.ai.aiEnhancedModeDesc')"
      :model-value="isAIEnhancedModeAvailable ? props.settings.ai_enhanced_mode : false"
      :disabled="!isAIEnhancedModeAvailable"
      @update:model-value="updateSetting('ai_enhanced_mode', $event)"
    />
  </SettingGroup>
</template>

<style scoped>
@reference "../../../../style.css";
</style>
