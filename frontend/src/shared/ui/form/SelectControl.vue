<script setup lang="ts">
interface Option {
  value: string | number;
  label: string;
  disabled?: boolean;
}

interface Props {
  modelValue: string | number;
  options: Array<Option>;
  disabled?: boolean;
  width?: string;
}

defineProps<Props>();

const emit = defineEmits<{
  'update:modelValue': [value: string | number];
}>();

function handleChange(event: Event) {
  const target = event.target as HTMLSelectElement;
  emit('update:modelValue', target.value);
}

const widthClass = (width?: string) => {
  switch (width) {
    case 'sm':
      return 'w-20 sm:w-24';
    case 'md':
      return 'w-32 sm:w-48';
    case 'lg':
      return 'w-48 sm:w-64';
    default:
      return width || 'w-24 sm:w-48';
  }
};
</script>

<template>
  <select
    class="input-field select"
    :class="[widthClass(width), { 'opacity-50 cursor-not-allowed': disabled }]"
    :disabled="disabled"
    :value="modelValue"
    @change="handleChange"
  >
    <option
      v-for="option in options"
      :key="option.value"
      :value="option.value"
      :disabled="option.disabled"
    >
      {{ option.label }}
    </option>
  </select>
</template>

<style scoped>
.input-field {
  @apply p-1.5 sm:p-2.5 border border-border rounded-md bg-bg-secondary text-text-primary focus:border-accent focus:outline-none transition-colors text-xs sm:text-sm;
}

.input-field:disabled {
  @apply cursor-not-allowed;
}
</style>
