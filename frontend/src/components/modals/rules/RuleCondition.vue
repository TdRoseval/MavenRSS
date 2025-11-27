<script setup lang="ts">
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { PhX, PhProhibit } from '@phosphor-icons/vue';

interface Condition {
  id: number;
  logic?: 'and' | 'or' | null;
  negate: boolean;
  field: string;
  operator?: string | null;
  value: string;
  values: string[];
}

interface FieldOption {
  value: string;
  labelKey: string;
  multiSelect: boolean;
  booleanField?: boolean;
}

interface Props {
  condition: Condition;
  index: number;
  fieldOptions: FieldOption[];
  feedNames: string[];
  feedCategories: string[];
  textOperatorOptions: Array<{ value: string; labelKey: string }>;
  booleanOptions: Array<{ value: string; labelKey: string }>;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  update: [condition: Condition];
  remove: [];
  toggleNegate: [];
}>();

const { t, locale } = useI18n();

const openDropdown = ref(false);

// Helper functions
function isDateField(field: string): boolean {
  return field === 'published_after' || field === 'published_before';
}

function isMultiSelectField(field: string): boolean {
  return field === 'feed_name' || field === 'feed_category';
}

function isBooleanField(field: string): boolean {
  return field === 'is_read' || field === 'is_favorite' || field === 'is_hidden';
}

function needsOperator(field: string): boolean {
  return field === 'article_title';
}

function getMultiSelectOptions(): string[] {
  if (props.condition.field === 'feed_name') {
    return props.feedNames;
  } else if (props.condition.field === 'feed_category') {
    return props.feedCategories;
  }
  return [];
}

function toggleMultiSelectValue(val: string): void {
  const condition = { ...props.condition };
  const idx = condition.values.indexOf(val);
  if (idx > -1) {
    condition.values.splice(idx, 1);
  } else {
    condition.values.push(val);
  }
  emit('update', condition);
}

function getMultiSelectDisplayText(): string {
  const field = props.fieldOptions.find((f) => f.value === props.condition.field);
  if (!field) return '';

  if (!props.condition.values || props.condition.values.length === 0) {
    return t(field.labelKey);
  }

  if (props.condition.values.length === 1) {
    return props.condition.values[0];
  }

  const firstItem = props.condition.values[0];
  const totalCount = props.condition.values.length;
  const remaining = totalCount - 1;

  if (locale.value === 'zh-CN') {
    return `${firstItem} ${t('andNMore', { count: totalCount })}`;
  }
  return `${firstItem} ${t('andNMore', { count: remaining })}`;
}

function onFieldChange(): void {
  const condition = { ...props.condition };
  if (isDateField(condition.field)) {
    condition.operator = null;
    condition.value = '';
    condition.values = [];
  } else if (isMultiSelectField(condition.field)) {
    condition.operator = 'contains';
    condition.value = '';
    condition.values = [];
  } else if (isBooleanField(condition.field)) {
    condition.operator = null;
    condition.value = 'true';
    condition.values = [];
  } else {
    condition.operator = 'contains';
    condition.value = '';
    condition.values = [];
  }
  emit('update', condition);
}

function updateValue(value: string): void {
  const condition = { ...props.condition, value };
  emit('update', condition);
}

function updateOperator(operator: string): void {
  const condition = { ...props.condition, operator };
  emit('update', condition);
}
</script>

<template>
  <div class="condition-card">
    <div class="flex items-start gap-2">
      <!-- Negate button -->
      <button
        @click="emit('toggleNegate')"
        :class="['negate-btn', condition.negate ? 'active' : '']"
        :title="t('notCondition')"
      >
        <PhProhibit :size="16" />
      </button>

      <!-- Field selector -->
      <select
        :value="condition.field"
        @change="
          (e) => {
            condition.field = (e.target as HTMLSelectElement).value;
            onFieldChange();
          }
        "
        class="input-field"
      >
        <option v-for="opt in fieldOptions" :key="opt.value" :value="opt.value">
          {{ t(opt.labelKey) }}
        </option>
      </select>

      <!-- Operator selector (for article_title) -->
      <select
        v-if="needsOperator(condition.field)"
        :value="condition.operator"
        @change="(e) => updateOperator((e.target as HTMLSelectElement).value)"
        class="input-field"
      >
        <option v-for="opt in textOperatorOptions" :key="opt.value" :value="opt.value">
          {{ t(opt.labelKey) }}
        </option>
      </select>

      <!-- Value input: Multi-select dropdown -->
      <div v-if="isMultiSelectField(condition.field)" class="multi-select-container">
        <button @click="openDropdown = !openDropdown" class="multi-select-btn">
          <span class="flex-1 text-left">{{ getMultiSelectDisplayText() }}</span>
          <span class="text-text-secondary text-xs ml-2">▼</span>
        </button>
        <div v-if="openDropdown" class="dropdown top-full mt-1">
          <div class="dropdown-list">
            <label v-for="val in getMultiSelectOptions()" :key="val" class="dropdown-item">
              <input
                type="checkbox"
                :checked="condition.values.includes(val)"
                @change="toggleMultiSelectValue(val)"
                class="checkbox"
              />
              {{ val }}
            </label>
          </div>
        </div>
      </div>

      <!-- Value input: Date -->
      <input
        v-else-if="isDateField(condition.field)"
        type="date"
        :value="condition.value"
        @input="(e) => updateValue((e.target as HTMLInputElement).value)"
        class="input-field"
      />

      <!-- Value input: Boolean -->
      <select
        v-else-if="isBooleanField(condition.field)"
        :value="condition.value"
        @change="(e) => updateValue((e.target as HTMLSelectElement).value)"
        class="input-field"
      >
        <option v-for="opt in booleanOptions" :key="opt.value" :value="opt.value">
          {{ t(opt.labelKey) }}
        </option>
      </select>

      <!-- Value input: Text -->
      <input
        v-else
        type="text"
        :value="condition.value"
        @input="(e) => updateValue((e.target as HTMLInputElement).value)"
        :placeholder="t('inputValue')"
        class="input-field"
      />

      <!-- Delete button -->
      <button @click="emit('remove')" class="delete-btn" :title="t('delete')">
        <PhX :size="18" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.condition-card {
  @apply p-2 sm:p-3 rounded-lg bg-bg-secondary border border-border;
}

.negate-btn {
  @apply flex items-center gap-1 px-2 py-2 rounded-md border transition-all cursor-pointer;
}

.negate-btn:not(.active) {
  @apply text-text-secondary bg-bg-primary border-border;
}

.negate-btn.active {
  @apply border-red-400 text-red-500;
}

.negate-btn.active:hover {
  @apply bg-red-500/10 border-red-500 text-red-500;
}

.multi-select-container {
  @apply relative;
}

.multi-select-btn {
  @apply w-full p-2 border border-border rounded-md bg-bg-primary text-text-primary text-sm;
}

.multi-select-btn:hover {
  @apply flex items-center justify-between cursor-pointer border-accent transition-colors;
}

.dropdown {
  @apply absolute left-0 right-0 border border-border rounded-md bg-bg-primary;
}

.dropdown-list {
  @apply max-h-40 overflow-y-auto z-50 shadow-lg;
}

.dropdown-item {
  @apply flex items-center gap-2 px-3 py-2 cursor-pointer text-sm text-text-primary;
}

.dropdown-item:hover {
  @apply bg-bg-tertiary;
}

.checkbox {
  @apply w-4 h-4 accent-accent cursor-pointer;
}

.delete-btn {
  @apply p-2 rounded-lg text-red-500 transition-colors cursor-pointer;
}

.delete-btn:hover {
  @apply bg-red-500/10;
}
</style>
