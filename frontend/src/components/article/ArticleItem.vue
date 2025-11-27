<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { PhEyeSlash, PhStar } from '@phosphor-icons/vue';
import type { Article } from '@/types/models';
import { formatDate as formatDateUtil } from '@/utils/date';

interface Props {
  article: Article;
  isActive: boolean;
}

defineProps<Props>();

const emit = defineEmits<{
  click: [];
  contextmenu: [event: MouseEvent];
  observeElement: [element: Element | null];
}>();

const { t, locale } = useI18n();

function formatDate(dateStr: string): string {
  return formatDateUtil(dateStr, locale.value === 'zh-CN' ? 'zh-CN' : 'en-US');
}

function handleImageError(event: Event) {
  const target = event.target as HTMLImageElement;
  target.style.display = 'none';
}
</script>

<template>
  <div
    :data-article-id="article.id"
    :ref="(el) => emit('observeElement', el as Element | null)"
    @click="emit('click')"
    @contextmenu="emit('contextmenu', $event)"
    :class="[
      'article-card',
      article.is_read ? 'read' : '',
      article.is_favorite ? 'favorite' : '',
      article.is_hidden ? 'hidden' : '',
      isActive ? 'active' : '',
    ]"
  >
    <img
      v-if="article.image_url"
      :src="article.image_url"
      class="w-16 h-12 sm:w-20 sm:h-[60px] object-cover rounded bg-bg-tertiary shrink-0 border border-border"
      @error="handleImageError"
    />

    <div class="flex-1 min-w-0">
      <div class="flex items-start gap-1.5 sm:gap-2">
        <h4
          v-if="!article.translated_title || article.translated_title === article.title"
          class="flex-1 m-0 mb-1 sm:mb-1.5 text-sm sm:text-base font-semibold leading-snug text-text-primary"
        >
          {{ article.title }}
        </h4>
        <div v-else class="flex-1">
          <h4
            class="m-0 mb-0.5 sm:mb-1 text-sm sm:text-base font-semibold leading-snug text-text-primary"
          >
            {{ article.translated_title }}
          </h4>
          <div class="text-[10px] sm:text-xs text-text-secondary italic mb-0.5 sm:mb-1">
            {{ article.title }}
          </div>
        </div>
        <PhEyeSlash
          v-if="article.is_hidden"
          :size="18"
          class="text-text-secondary flex-shrink-0 sm:w-5 sm:h-5"
          :title="t('hideArticle')"
        />
      </div>

      <div
        class="flex justify-between items-center text-[10px] sm:text-xs text-text-secondary mt-1.5 sm:mt-2"
      >
        <span class="font-medium text-accent truncate flex-1 min-w-0 mr-2">
          {{ article.feed_title }}
        </span>
        <div class="flex items-center gap-1 sm:gap-2 shrink-0">
          <PhStar
            v-if="article.is_favorite"
            :size="14"
            class="text-yellow-500 sm:w-[18px] sm:h-[18px]"
            weight="fill"
          />
          <span class="whitespace-nowrap">{{ formatDate(article.published_at) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.article-card {
  @apply p-2 sm:p-3 border-b border-border cursor-pointer transition-colors flex gap-2 sm:gap-3 relative;
}

.article-card:hover {
  @apply bg-bg-tertiary;
}

.article-card.active {
  @apply bg-bg-tertiary border-l-2 sm:border-l-[3px] border-l-accent;
}

.article-card.read h4 {
  @apply text-text-secondary font-normal;
}

.article-card.read .text-sm {
  @apply text-text-secondary opacity-80;
}

.article-card.favorite {
  background-color: rgba(255, 215, 0, 0.05);
}

.article-card.hidden {
  @apply opacity-60 bg-gray-100 dark:bg-gray-800;
}

.article-card.hidden:hover {
  @apply opacity-80;
}
</style>
