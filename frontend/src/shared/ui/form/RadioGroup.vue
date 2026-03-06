<script setup lang="ts">
import { type Component } from 'vue';

interface Option {
  value: string;
  label: string;
  description?: string;
  icon?: Component;
}

defineProps<{
  modelValue: string;
  options: Option[];
}>();

defineEmits<{
  (e: 'update:modelValue', value: string): void;
}>();
</script>

<template>
  <div class="space-y-2">
    <div
      v-for="option in options"
      :key="option.value"
      class="relative flex cursor-pointer rounded-lg border p-4 shadow-sm focus:outline-none"
      :class="[
        modelValue === option.value
          ? 'border-accent-primary bg-accent-primary/5 ring-1 ring-accent-primary'
          : 'border-border-default hover:bg-bg-secondary',
      ]"
      @click="$emit('update:modelValue', option.value)"
    >
      <div class="flex w-full items-center justify-between">
        <div class="flex items-center">
          <div class="text-sm">
            <div class="flex items-center gap-2 font-medium text-text-primary">
              <component :is="option.icon" v-if="option.icon" class="h-4 w-4" />
              {{ option.label }}
            </div>
            <div v-if="option.description" class="mt-1 text-xs text-text-secondary">
              {{ option.description }}
            </div>
          </div>
        </div>
        <div
          class="flex h-4 w-4 shrink-0 items-center justify-center rounded-full border border-border-default"
          :class="[
            modelValue === option.value
              ? 'border-accent-primary bg-accent-primary'
              : 'bg-transparent',
          ]"
        >
          <div v-if="modelValue === option.value" class="h-1.5 w-1.5 rounded-full bg-white" />
        </div>
      </div>
    </div>
  </div>
</template>
