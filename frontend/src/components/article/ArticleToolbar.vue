<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import {
  PhArrowLeft,
  PhGlobe,
  PhArticle,
  PhEnvelopeOpen,
  PhEnvelope,
  PhStar,
  PhClockCountdown,
  PhArrowSquareOut,
  PhTranslate,
} from '@phosphor-icons/vue';
import type { Article } from '@/types/models';

const { t } = useI18n();

interface Props {
  article: Article;
  showContent: boolean;
  showTranslations?: boolean;
}

withDefaults(defineProps<Props>(), {
  showTranslations: true,
});

defineEmits<{
  close: [];
  toggleContentView: [];
  toggleRead: [];
  toggleFavorite: [];
  toggleReadLater: [];
  openOriginal: [];
  toggleTranslations: [];
}>();
</script>

<template>
  <div
    class="h-[44px] sm:h-[50px] px-3 sm:px-5 border-b border-border flex justify-between items-center bg-bg-primary shrink-0"
  >
    <button
      class="md:hidden flex items-center gap-1.5 sm:gap-2 text-text-secondary hover:text-text-primary text-sm sm:text-base"
      @click="$emit('close')"
    >
      <PhArrowLeft :size="18" class="sm:w-5 sm:h-5" />
      <span class="hidden xs:inline">{{ t('back') }}</span>
    </button>
    <div class="flex gap-1 sm:gap-2 ml-auto">
      <button
        class="action-btn"
        :title="showContent ? t('viewOriginal') : t('viewContent')"
        @click="$emit('toggleContentView')"
      >
        <PhGlobe v-if="showContent" :size="18" class="sm:w-5 sm:h-5" />
        <PhArticle v-else :size="18" class="sm:w-5 sm:h-5" />
      </button>
      <button
        v-if="showContent"
        class="action-btn"
        :title="showTranslations ? t('hideTranslations') : t('showTranslations')"
        @click="$emit('toggleTranslations')"
      >
        <PhTranslate
          :size="18"
          class="sm:w-5 sm:h-5"
          :weight="showTranslations ? 'fill' : 'regular'"
        />
      </button>
      <button
        class="action-btn"
        :title="article.is_read ? t('markAsUnread') : t('markAsRead')"
        @click="$emit('toggleRead')"
      >
        <PhEnvelopeOpen v-if="article.is_read" :size="18" class="sm:w-5 sm:h-5" />
        <PhEnvelope v-else :size="18" class="sm:w-5 sm:h-5" />
      </button>
      <button
        :class="[
          'action-btn',
          article.is_favorite ? 'text-yellow-500 hover:text-yellow-600' : 'hover:text-yellow-500',
        ]"
        :title="article.is_favorite ? t('removeFromFavorite') : t('addToFavorite')"
        @click="$emit('toggleFavorite')"
      >
        <PhStar
          :size="18"
          class="sm:w-5 sm:h-5"
          :weight="article.is_favorite ? 'fill' : 'regular'"
        />
      </button>
      <button
        :class="[
          'action-btn',
          article.is_read_later ? 'text-blue-500 hover:text-blue-600' : 'hover:text-blue-500',
        ]"
        :title="article.is_read_later ? t('removeFromReadLater') : t('addToReadLater')"
        @click="$emit('toggleReadLater')"
      >
        <PhClockCountdown
          :size="18"
          class="sm:w-5 sm:h-5"
          :weight="article.is_read_later ? 'fill' : 'regular'"
        />
      </button>
      <button class="action-btn" :title="t('openInBrowser')" @click="$emit('openOriginal')">
        <PhArrowSquareOut :size="18" class="sm:w-5 sm:h-5" />
      </button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.action-btn {
  @apply text-lg sm:text-xl cursor-pointer text-text-secondary p-1 sm:p-1.5 rounded-md transition-colors hover:bg-bg-tertiary hover:text-text-primary;
}
</style>
