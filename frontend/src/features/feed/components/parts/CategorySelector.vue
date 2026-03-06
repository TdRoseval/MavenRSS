<script setup lang="ts">
import { useI18n } from 'vue-i18n';

interface Props {
  category: string;
  categorySelection: string;
  showCustomCategory: boolean;
  existingCategories: string[];
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:category': [value: string];
  'update:categorySelection': [value: string];
  'update:showCustomCategory': [value: boolean];
  'handle-category-change': [value: string];
}>();

const { t } = useI18n();

function handleCategoryChange(value: string) {
  emit('handle-category-change', value);
}
</script>

<template>
  <div class="mb-3 sm:mb-4">
    <label class="block mb-1 sm:mb-1.5 font-semibold text-xs sm:text-sm text-text-secondary">{{
      t('common.form.category')
    }}</label>
    <select
      v-if="!props.showCustomCategory"
      :value="props.categorySelection"
      class="input-field w-full"
      @change="handleCategoryChange(($event.target as HTMLSelectElement).value)"
    >
      <option value="">{{ t('sidebar.feedList.uncategorized') }}</option>
      <option v-for="cat in props.existingCategories" :key="cat" :value="cat">{{ cat }}</option>
      <option value="__custom__">{{ t('modal.feed.customCategory') }}</option>
    </select>
    <div v-else class="flex gap-2">
      <input
        :value="props.category"
        type="text"
        :placeholder="t('modal.feed.enterCategoryName')"
        class="input-field flex-1"
        autofocus
        @input="emit('update:category', ($event.target as HTMLInputElement).value)"
      />
      <button
        type="button"
        class="px-3 py-2 text-xs sm:text-sm text-text-secondary hover:text-text-primary border border-border rounded-md hover:bg-bg-tertiary transition-colors"
        @click="
          emit('update:showCustomCategory', false);
          emit('update:categorySelection', '');
        "
      >
        {{ t('common.cancel') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../style.css";

.input-field {
  @apply w-full p-2 sm:p-2.5 border border-border rounded-md bg-bg-tertiary text-text-primary text-xs sm:text-sm focus:border-accent focus:outline-none transition-colors;
  box-sizing: border-box;
}

select.input-field {
  appearance: none;
  -webkit-appearance: none;
  -moz-appearance: none;
  padding-right: 2.5rem;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 20 20'%3E%3Cpath stroke='%236b7280' stroke-linecap='round' stroke-linejoin='round' stroke-width='1.5' d='M6 8l4 4 4-4'/%3E%3C/svg%3E");
  background-position: right 0.5rem center;
  background-repeat: no-repeat;
  background-size: 1.5em 1.5em;
}
</style>
