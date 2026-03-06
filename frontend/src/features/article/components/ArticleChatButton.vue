<script setup lang="ts">
import { PhChatCircleText } from '@phosphor-icons/vue';
import { useI18n } from 'vue-i18n';

interface Props {
  onClick: () => void;
}

defineProps<Props>();

const { t } = useI18n();
</script>

<template>
  <button
    class="fixed bottom-12 right-6 w-14 h-14 bg-accent hover:bg-accent-hover text-white rounded-full shadow-lg flex items-center justify-center transition-all hover:scale-110 z-40"
    :title="t('article.chat.aiChat')"
    @click="onClick"
  >
    <PhChatCircleText :size="28" />
  </button>
</template>
