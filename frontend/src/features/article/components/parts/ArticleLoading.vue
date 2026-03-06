<script setup lang="ts">
import { PhArticle } from '@phosphor-icons/vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
</script>

<template>
  <!-- Loading state with proper background -->
  <div class="flex flex-col items-center justify-center py-12 sm:py-16 bg-bg-primary">
    <div class="relative mb-4 sm:mb-6">
      <!-- Outer pulsing ring -->
      <div
        class="absolute inset-0 rounded-full border-2 sm:border-4 border-accent animate-ping opacity-20"
      ></div>
      <!-- Middle spinning ring -->
      <div
        class="absolute inset-0 rounded-full border-2 sm:border-4 border-t-accent border-r-transparent border-b-transparent border-l-transparent animate-spin"
      ></div>
      <!-- Inner icon -->
      <div class="relative bg-bg-secondary rounded-full p-4 sm:p-6">
        <PhArticle :size="48" class="text-accent sm:w-16 sm:h-16" />
      </div>
    </div>
    <p class="text-base sm:text-lg font-medium text-text-primary mb-1 sm:mb-2">
      {{ t('article.content.loadingContent') }}
    </p>
    <p class="text-xs sm:text-sm text-text-secondary px-4 text-center">
      {{ t('article.content.fetchingArticleContent') }}
    </p>
  </div>
</template>
