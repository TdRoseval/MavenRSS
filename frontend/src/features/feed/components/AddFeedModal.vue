<script setup lang="ts">
import FeedFormModal from './FeedFormModal.vue';

const emit = defineEmits<{
  close: [];
  added: [];
}>();

function handleClose() {
  emit('close');
}

function handleAdded() {
  emit('added');
}
</script>

<template>
  <FeedFormModal mode="add" @close="handleClose" @added="handleAdded" />
</template>
