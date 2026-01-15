<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { PhBookOpen } from '@phosphor-icons/vue';
import { openInBrowser } from '@/utils/browser';

interface Props {
  mode: 'add' | 'edit';
  url: string;
  xpathType: 'HTML+XPath' | 'XML+XPath';
  xpathItem: string;
  xpathItemTitle: string;
  xpathItemContent: string;
  xpathItemUri: string;
  xpathItemAuthor: string;
  xpathItemTimestamp: string;
  xpathItemTimeFormat: string;
  xpathItemThumbnail: string;
  xpathItemCategories: string;
  xpathItemUid: string;
  isUrlInvalid?: boolean;
  isXpathItemInvalid?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  isUrlInvalid: false,
  isXpathItemInvalid: false,
});

const emit = defineEmits<{
  'update:url': [value: string];
  'update:xpath-type': [value: string];
  'update:xpath-item': [value: string];
  'update:xpath-item-title': [value: string];
  'update:xpath-item-content': [value: string];
  'update:xpath-item-uri': [value: string];
  'update:xpath-item-author': [value: string];
  'update:xpath-item-timestamp': [value: string];
  'update:xpath-item-time-format': [value: string];
  'update:xpath-item-thumbnail': [value: string];
  'update:xpath-item-categories': [value: string];
  'update:xpath-item-uid': [value: string];
}>();

const { t, locale } = useI18n();

function openDocumentation() {
  const docUrl = locale.value.startsWith('zh')
    ? 'https://github.com/WCY-dt/MrRSS/blob/main/docs/XPATH_MODE.zh.md'
    : 'https://github.com/WCY-dt/MrRSS/blob/main/docs/XPATH_MODE.md';
  openInBrowser(docUrl);
}

// Hardcoded XPath placeholders - same across all languages
const xpathPlaceholders = {
  xpathItem: '//div[contains(@class, "post")]',
  xpathItemTitle: './/h1[contains(@class, "title")]',
  xpathItemUri: './/a[contains(@class, "link")]/@href',
  xpathItemContent: './/div[contains(@class, "content")]',
  xpathItemAuthor: './/span[contains(@class, "author")]',
  xpathItemTimestamp: './/time/@datetime',
  xpathItemTimeFormat: '2006-01-02 15:04:05',
  xpathItemThumbnail: './/img/@src',
  xpathItemCategories: './/span[contains(@class, "tag")]',
  xpathItemUid: './/article/@id',
};
</script>

<template>
  <div class="mb-3 sm:mb-4">
    <div class="mb-3">
      <label class="block mb-1 sm:mb-1.5 font-semibold text-xs sm:text-sm text-text-secondary"
        >{{ t('sourceUrl') }} <span class="text-red-500">*</span></label
      >
      <input
        :value="props.url"
        type="text"
        :placeholder="t('sourceUrlPlaceholder')"
        :class="['input-field', props.mode === 'add' && props.isUrlInvalid ? 'border-red-500' : '']"
        @input="emit('update:url', ($event.target as HTMLInputElement).value)"
      />
    </div>

    <div class="mb-3">
      <label class="block mb-1 sm:mb-1.5 font-semibold text-xs sm:text-sm text-text-secondary">{{
        t('xpathType')
      }}</label>
      <select
        :value="props.xpathType"
        class="input-field"
        @change="emit('update:xpath-type', ($event.target as HTMLSelectElement).value)"
      >
        <option value="HTML+XPath">{{ t('htmlXpath') }}</option>
        <option value="XML+XPath">{{ t('xmlXpath') }}</option>
      </select>
    </div>

    <div class="mb-3">
      <label class="block mb-1 sm:mb-1.5 font-semibold text-xs sm:text-sm text-text-secondary"
        >{{ t('xpathItem') }} <span class="text-red-500">*</span></label
      >
      <input
        :value="props.xpathItem"
        type="text"
        :placeholder="xpathPlaceholders.xpathItem"
        :class="[
          'input-field',
          props.mode === 'add' && props.isXpathItemInvalid ? 'border-red-500' : '',
        ]"
        @input="emit('update:xpath-item', ($event.target as HTMLInputElement).value)"
      />
      <div class="text-xs text-text-secondary mt-1">{{ t('xpathItemHelp') }}</div>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-3">
      <div>
        <label class="block mb-1 font-semibold text-xs text-text-secondary">{{
          t('xpathItemTitle')
        }}</label>
        <input
          :value="props.xpathItemTitle"
          type="text"
          :placeholder="xpathPlaceholders.xpathItemTitle"
          class="input-field"
          @input="emit('update:xpath-item-title', ($event.target as HTMLInputElement).value)"
        />
      </div>
      <div>
        <label class="block mb-1 font-semibold text-xs text-text-secondary">{{
          t('xpathItemUri')
        }}</label>
        <input
          :value="props.xpathItemUri"
          type="text"
          :placeholder="xpathPlaceholders.xpathItemUri"
          class="input-field"
          @input="emit('update:xpath-item-uri', ($event.target as HTMLInputElement).value)"
        />
      </div>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-3">
      <div>
        <label class="block mb-1 font-semibold text-xs text-text-secondary">{{
          t('xpathItemContent')
        }}</label>
        <input
          :value="props.xpathItemContent"
          type="text"
          :placeholder="xpathPlaceholders.xpathItemContent"
          class="input-field"
          @input="emit('update:xpath-item-content', ($event.target as HTMLInputElement).value)"
        />
      </div>
      <div>
        <label class="block mb-1 font-semibold text-xs text-text-secondary">{{
          t('xpathItemAuthor')
        }}</label>
        <input
          :value="props.xpathItemAuthor"
          type="text"
          :placeholder="xpathPlaceholders.xpathItemAuthor"
          class="input-field"
          @input="emit('update:xpath-item-author', ($event.target as HTMLInputElement).value)"
        />
      </div>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-3">
      <div>
        <label class="block mb-1 font-semibold text-xs text-text-secondary">{{
          t('xpathItemTimestamp')
        }}</label>
        <input
          :value="props.xpathItemTimestamp"
          type="text"
          :placeholder="xpathPlaceholders.xpathItemTimestamp"
          class="input-field"
          @input="emit('update:xpath-item-timestamp', ($event.target as HTMLInputElement).value)"
        />
      </div>
      <div>
        <label class="block mb-1 font-semibold text-xs text-text-secondary">{{
          t('xpathItemTimeFormat')
        }}</label>
        <input
          :value="props.xpathItemTimeFormat"
          type="text"
          :placeholder="xpathPlaceholders.xpathItemTimeFormat"
          class="input-field"
          @input="emit('update:xpath-item-time-format', ($event.target as HTMLInputElement).value)"
        />
      </div>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-3">
      <div>
        <label class="block mb-1 font-semibold text-xs text-text-secondary">{{
          t('xpathItemThumbnail')
        }}</label>
        <input
          :value="props.xpathItemThumbnail"
          type="text"
          :placeholder="xpathPlaceholders.xpathItemThumbnail"
          class="input-field"
          @input="emit('update:xpath-item-thumbnail', ($event.target as HTMLInputElement).value)"
        />
      </div>
      <div>
        <label class="block mb-1 font-semibold text-xs text-text-secondary">{{
          t('xpathItemCategories')
        }}</label>
        <input
          :value="props.xpathItemCategories"
          type="text"
          :placeholder="xpathPlaceholders.xpathItemCategories"
          class="input-field"
          @input="emit('update:xpath-item-categories', ($event.target as HTMLInputElement).value)"
        />
      </div>
    </div>

    <div class="mb-3">
      <label class="block mb-1 font-semibold text-xs text-text-secondary">{{
        t('xpathItemUid')
      }}</label>
      <input
        :value="props.xpathItemUid"
        type="text"
        :placeholder="xpathPlaceholders.xpathItemUid"
        class="input-field"
        @input="emit('update:xpath-item-uid', ($event.target as HTMLInputElement).value)"
      />
    </div>

    <div class="flex flex-col sm:flex-row gap-2 sm:gap-3 mt-4">
      <button
        type="button"
        class="text-xs sm:text-sm text-accent hover:underline flex items-center gap-1"
        @click="openDocumentation"
      >
        <PhBookOpen :size="14" />
        {{ t('xpathDocumentation') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../style.css";

.input-field {
  @apply w-full p-2 sm:p-2.5 border border-border rounded-md bg-bg-tertiary text-text-primary text-xs sm:text-sm focus:border-accent focus:outline-none transition-colors;
}
</style>
