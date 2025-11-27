<script setup lang="ts">
import { useModalClose } from '@/composables/ui/useModalClose';

interface Props {
  title?: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  isDanger?: boolean;
}

withDefaults(defineProps<Props>(), {
  title: 'Confirm',
  confirmText: 'Confirm',
  cancelText: 'Cancel',
  isDanger: false,
});

const emit = defineEmits<{
  confirm: [];
  cancel: [];
  close: [];
}>();

// Modal close handling - ESC should cancel
useModalClose(() => handleCancel());

function handleConfirm() {
  emit('confirm');
  emit('close');
}

function handleCancel() {
  emit('cancel');
  emit('close');
}
</script>

<template>
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
    @click.self="handleCancel"
    data-modal-open="true"
  >
    <div
      class="bg-bg-primary max-w-md w-full mx-4 rounded-xl shadow-2xl border border-border overflow-hidden animate-fade-in"
    >
      <div class="p-5 border-b border-border">
        <h3 class="text-lg font-semibold m-0">{{ title }}</h3>
      </div>

      <div class="p-5">
        <p class="m-0 text-text-primary">{{ message }}</p>
      </div>

      <div class="p-5 border-t border-border bg-bg-secondary flex justify-end gap-3">
        <button @click="handleCancel" class="btn-secondary">{{ cancelText }}</button>
        <button @click="handleConfirm" :class="isDanger ? 'btn-danger' : 'btn-primary'">
          {{ confirmText }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.btn-primary {
  @apply bg-accent text-white border-none px-5 py-2.5 rounded-lg cursor-pointer font-semibold hover:bg-accent-hover transition-colors;
}
.btn-danger {
  @apply bg-transparent border border-red-300 text-red-600 px-5 py-2.5 rounded-lg cursor-pointer font-semibold hover:bg-red-50 dark:hover:bg-red-900/20 dark:border-red-400 dark:text-red-400 transition-colors;
}
.btn-secondary {
  @apply bg-transparent border border-border text-text-primary px-5 py-2.5 rounded-lg cursor-pointer font-medium hover:bg-bg-tertiary transition-colors;
}
.animate-fade-in {
  animation: modalFadeIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes modalFadeIn {
  from {
    transform: translateY(-20px);
    opacity: 0;
  }
  to {
    transform: translateY(0);
    opacity: 1;
  }
}
</style>
