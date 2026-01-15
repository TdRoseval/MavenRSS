<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { useSettings } from '@/composables/core/useSettings';
import { onMounted } from 'vue';
import {
  PhArrowLeft,
  PhGlobe,
  PhArticle,
  PhEnvelopeOpen,
  PhEnvelope,
  PhStar,
  PhClockCountdown,
  PhArrowSquareOut,
  PhShareNetwork,
} from '@phosphor-icons/vue';
import type { Article } from '@/types/models';

const { t } = useI18n();
const { settings, fetchSettings } = useSettings();

onMounted(async () => {
  try {
    await fetchSettings();
  } catch (e) {
    console.error('Error loading settings:', e);
  }
});

interface Props {
  article: Article | undefined;
  showContent: boolean;
}

defineProps<Props>();

const emit = defineEmits<{
  close: [];
  'toggle-read': [];
  'toggle-favorite': [];
  'toggle-read-later': [];
  'toggle-content': [];
  'open-original': [];
  'export-to-obsidian': [];
}>();
</script>

<template>
  <div
    v-if="article"
    class="header-bar bg-bg-primary border-b border-border p-2 sm:p-4 flex items-center justify-between gap-2 sm:gap-4"
  >
    <!-- Left: Back button -->
    <button class="btn-icon" :title="t('back')" @click="emit('close')">
      <PhArrowLeft :size="20" class="sm:w-6 sm:h-6" />
    </button>

    <!-- Center: Feed name -->
    <div class="flex-1 min-w-0 text-center">
      <h3 class="text-sm sm:text-base font-medium text-text-secondary truncate">
        {{ article.feed_name }}
      </h3>
    </div>

    <!-- Right: Actions -->
    <div class="flex items-center gap-1 sm:gap-2">
      <button
        class="btn-icon"
        :title="article.is_read ? t('markUnread') : t('markRead')"
        @click="emit('toggle-read')"
      >
        <PhEnvelopeOpen v-if="article.is_read" :size="20" class="sm:w-6 sm:h-6" />
        <PhEnvelope v-else :size="20" class="sm:w-6 sm:h-6" />
      </button>
      <button
        class="btn-icon"
        :title="article.is_favorite ? t('removeFromFavorite') : t('addToFavorite')"
        @click="emit('toggle-favorite')"
      >
        <PhStar
          :size="20"
          :weight="article.is_favorite ? 'fill' : 'regular'"
          class="sm:w-6 sm:h-6"
          :class="{ 'text-yellow-500': article.is_favorite }"
        />
      </button>
      <button
        class="btn-icon"
        :title="article.is_read_later ? t('removeFromReadLater') : t('addToReadLater')"
        @click="emit('toggle-read-later')"
      >
        <PhClockCountdown
          :size="20"
          :weight="article.is_read_later ? 'fill' : 'regular'"
          class="sm:w-6 sm:h-6"
          :class="{ 'text-blue-500': article.is_read_later }"
        />
      </button>
      <button
        class="btn-icon"
        :title="showContent ? t('showOriginal') : t('showRendered')"
        @click="emit('toggle-content')"
      >
        <PhArticle v-if="showContent" :size="20" class="sm:w-6 sm:h-6" />
        <PhGlobe v-else :size="20" class="sm:w-6 sm:h-6" />
      </button>
      <button class="btn-primary" :title="t('openOriginal')" @click="emit('open-original')">
        <PhArrowSquareOut :size="20" class="sm:w-6 sm:h-6" />
        <span class="hidden sm:inline">{{ t('openOriginal') }}</span>
      </button>
      <button
        v-if="settings.obsidian_enabled"
        class="btn-icon"
        :title="t('exportToObsidian')"
        @click="emit('export-to-obsidian')"
      >
        <PhShareNetwork :size="20" class="sm:w-6 sm:h-6" />
      </button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.btn-icon {
  @apply p-2 sm:p-2.5 rounded-lg bg-transparent border-none cursor-pointer text-text-secondary hover:bg-bg-tertiary hover:text-text-primary transition-colors;
}

.btn-primary {
  @apply bg-accent text-white px-3 py-2 sm:px-4 sm:py-2.5 rounded-lg cursor-pointer flex items-center gap-1 sm:gap-2 font-medium hover:bg-accent-hover transition-colors text-sm sm:text-base;
}
</style>
