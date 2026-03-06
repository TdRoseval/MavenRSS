<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { PhTextT, PhTextIndent, PhTextAa } from '@phosphor-icons/vue';
import { SettingGroup, SettingItem, NumberControl } from '@/components/settings';
import '@/components/settings/styles.css';
import type { SettingsData } from '@/types/settings';
import { getRecommendedFonts } from '@/shared/lib/fontDetector';

const { t } = useI18n();

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:settings': [settings: SettingsData];
}>();

// Font categories
const availableFonts = ref<{
  serif: string[];
  sansSerif: string[];
  monospace: string[];
}>({
  serif: [],
  sansSerif: [],
  monospace: [],
});

// Computed values for display (handle string/number conversion)
const displayContentSize = computed(() => {
  return parseInt(props.settings.content_font_size as any) || 16;
});
const displayLineHeight = computed(() => {
  return parseFloat(props.settings.content_line_height as any) || 1.6;
});

// Load system fonts on mount
onMounted(() => {
  try {
    availableFonts.value = getRecommendedFonts();
  } catch (error) {
    console.error('Failed to detect system fonts:', error);
  }
});

function updateSetting(key: keyof SettingsData, value: any) {
  emit('update:settings', {
    ...props.settings,
    [key]: value,
  });
}
</script>

<template>
  <SettingGroup :icon="PhTextT" :title="t('setting.tab.typography')">
    <!-- Content Font Family -->
    <SettingItem :icon="PhTextT" :title="t('setting.typography.contentFontFamily')">
      <template #description>
        <div class="text-xs text-text-secondary hidden sm:block">
          {{ t('setting.typography.contentFontFamilyDesc') }}
        </div>
      </template>
      <select
        :value="settings.content_font_family"
        class="input-field w-36 sm:w-48 text-xs sm:text-sm max-h-60"
        @change="updateSetting('content_font_family', ($event.target as HTMLSelectElement).value)"
      >
        <optgroup :label="t('setting.typography.fontSystem')">
          <option value="system">{{ t('setting.typography.fontSystemDefault') }}</option>
        </optgroup>

        <optgroup v-if="availableFonts.serif.length > 0" :label="t('setting.typography.fontSerif')">
          <option value="serif">{{ t('setting.typography.fontSerifDefault') }}</option>
          <option
            v-for="font in availableFonts.serif"
            :key="font"
            :value="font"
            :style="{ fontFamily: font + ', serif' }"
          >
            {{ font }}
          </option>
        </optgroup>

        <optgroup
          v-if="availableFonts.sansSerif.length > 0"
          :label="t('setting.typography.fontSansSerif')"
        >
          <option value="sans-serif">{{ t('setting.typography.fontSansSerifDefault') }}</option>
          <option
            v-for="font in availableFonts.sansSerif"
            :key="font"
            :value="font"
            :style="{ fontFamily: font + ', sans-serif' }"
          >
            {{ font }}
          </option>
        </optgroup>

        <optgroup
          v-if="availableFonts.monospace.length > 0"
          :label="t('setting.typography.fontMonospace')"
        >
          <option value="monospace">{{ t('setting.typography.fontMonospaceDefault') }}</option>
          <option
            v-for="font in availableFonts.monospace"
            :key="font"
            :value="font"
            :style="{ fontFamily: font + ', monospace' }"
          >
            {{ font }}
          </option>
        </optgroup>
      </select>
    </SettingItem>

    <!-- Content Font Size -->
    <SettingItem :icon="PhTextAa" :title="t('setting.typography.contentFontSize')">
      <template #description>
        <div class="text-xs text-text-secondary hidden sm:block">
          {{ t('setting.typography.contentFontSizeDesc') }}
        </div>
      </template>
      <NumberControl
        :model-value="displayContentSize"
        :min="10"
        :max="24"
        suffix="px"
        @update:model-value="(v) => updateSetting('content_font_size', isNaN(v) ? 16 : v)"
      />
    </SettingItem>

    <!-- Content Line Height -->
    <SettingItem :icon="PhTextIndent" :title="t('setting.typography.contentLineHeight')">
      <template #description>
        <div class="text-xs text-text-secondary hidden sm:block">
          {{ t('setting.typography.contentLineHeightDesc') }}
        </div>
      </template>
      <NumberControl
        :model-value="displayLineHeight"
        :min="1"
        :max="3"
        :step="0.1"
        @update:model-value="
          (v) => updateSetting('content_line_height', isNaN(v) ? '1.6' : v.toString())
        "
      />
    </SettingItem>
  </SettingGroup>
</template>

<style scoped>
@reference "../../../../style.css";

.input-field {
  @apply p-1.5 sm:p-2.5 border border-border rounded-md bg-bg-secondary text-text-primary focus:border-accent focus:outline-none transition-colors;
}
</style>
