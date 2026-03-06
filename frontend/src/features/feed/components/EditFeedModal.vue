<script setup lang="ts">
import type { Feed } from '@/types/models';
import FeedFormModal from './FeedFormModal.vue';

interface Props {
  feed: Feed;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  close: [];
  updated: [];
}>();

function handleClose() {
  emit('close');
}

function handleUpdated() {
  emit('updated');
}
</script>

<template>
  <FeedFormModal mode="edit" :feed="props.feed" @close="handleClose" @updated="handleUpdated" />
</template>
