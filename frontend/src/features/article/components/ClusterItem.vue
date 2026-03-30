<script setup lang="ts">
import { ref, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { PhEyeSlash, PhStar, PhClockCountdown, PhSparkle } from '@phosphor-icons/vue';
import type { Cluster } from '@/types/models';
import { formatDate as formatDateUtil } from '@/shared/lib/date';
import { useAuthStore } from '@/stores/auth';
import { useSettings } from '@/composables/core/useSettings';
import { apiClient } from '@/shared/lib/apiClient';

interface Props {
  cluster: Cluster;
  isActive: boolean;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  click: [];
  contextmenu: [event: MouseEvent];
  observeElement: [element: Element | null];
  hoverMarkAsRead: [clusterId: number];
}>();

const { t, locale } = useI18n();
const { settings } = useSettings();
const authStore = useAuthStore();

const compactMode = computed(() => {
  return settings.value.layout_mode === 'compact';
});
const feedTitlesText = computed(() => props.cluster.feed_titles?.filter(Boolean).join(' · ') || '');
const authorsText = computed(() => props.cluster.authors?.filter(Boolean).join(' · ') || '');

const formatDateWithI18n = (dateStr: string): string => {
  return formatDateUtil(dateStr, locale.value, t);
};

const hoverMarkAsRead = ref(false);
let hoverTimeout: ReturnType<typeof setTimeout> | null = null;

function handleMouseEnter() {
  if (!hoverMarkAsRead.value || props.cluster.is_read || props.cluster.is_read_later) {
    return;
  }

  hoverTimeout = setTimeout(() => {
    markAsRead();
  }, 300);
}

function handleMouseLeave() {
  if (hoverTimeout) {
    clearTimeout(hoverTimeout);
    hoverTimeout = null;
  }
}

async function markAsRead() {
  if (!authStore.isAuthenticated || props.cluster.is_read) return;

  try {
    await apiClient.put('/clusters/read', { id: props.cluster.id, read: true });
    emit('hoverMarkAsRead', props.cluster.id);
  } catch (e) {
    console.error('Error marking as read on hover:', e);
  }
}
</script>

<template>
  <div
    :ref="(el) => emit('observeElement', el as Element | null)"
    class="article-card"
    :class="[
      cluster.is_read ? 'read' : '',
      cluster.is_favorite ? 'favorite' : '',
      cluster.is_hidden ? 'hidden' : '',
      cluster.is_read_later ? 'read-later' : '',
      isActive ? 'active' : '',
      compactMode ? 'compact' : '',
    ]"
    @click="emit('click')"
    @contextmenu="emit('contextmenu', $event)"
    @mouseenter="handleMouseEnter"
    @mouseleave="handleMouseLeave"
  >
    <!-- Simple AI or cluster icon instead of thumbnail for now -->
    <div
      class="article-thumbnail-placeholder flex items-center justify-center bg-blue-50 dark:bg-blue-900/20 text-blue-500"
      :class="{ 'compact-thumbnail': compactMode }"
    >
      <PhSparkle :size="compactMode ? 20 : 28" weight="fill" />
    </div>

    <div class="flex-1 min-w-0">
      <div class="flex items-start gap-1.5 sm:gap-2">
        <h4
          class="flex-1 m-0 font-semibold leading-snug text-text-primary article-title"
          :class="{
            'text-base sm:text-base mb-0.5 sm:mb-1': !compactMode,
            'text-[14px] mb-0 compact-title': compactMode,
            'read-title text-text-secondary font-normal': cluster.is_read && compactMode,
          }"
        >
          {{ cluster.merged_title }}
        </h4>

        <PhEyeSlash
          v-if="cluster.is_hidden"
          :size="18"
          class="text-text-secondary flex-shrink-0 sm:w-5 sm:h-5"
        />

        <!-- Compact mode icons on the right -->
        <div
          v-if="compactMode"
          class="flex items-center gap-1.5 sm:gap-2 shrink-0 ml-1 self-center"
        >
          <PhClockCountdown
            v-if="cluster.is_read_later"
            :size="16"
            class="text-blue-500"
            weight="fill"
          />
          <PhStar v-if="cluster.is_favorite" :size="16" class="text-yellow-500" weight="fill" />
        </div>
      </div>

      <!-- Feed source name and time -->
      <div
        class="flex justify-between items-center text-[11px] sm:text-xs text-text-secondary"
        :class="{ 'mt-0 sm:mt-1': !compactMode, 'mt-0': compactMode }"
      >
        <span class="flex items-center gap-1.5 truncate flex-1 min-w-0 mr-2">
          <span class="font-medium text-blue-500">{{ t('article.cluster.sourceLabel') }}</span>
          <span
            class="text-[11px] sm:text-[11px] text-text-secondary opacity-75 truncate max-w-[120px]"
          >
            {{ t('article.cluster.articleCount', { count: cluster.article_count }) }}
          </span>
        </span>
        <div class="flex items-center gap-1 sm:gap-2 shrink-0 min-h-[14px] sm:min-h-[18px]">
          <template v-if="!compactMode">
            <PhClockCountdown
              v-if="cluster.is_read_later"
              :size="14"
              class="text-blue-500 sm:w-[18px] sm:h-[18px]"
              weight="fill"
            />
            <PhStar
              v-if="cluster.is_favorite"
              :size="14"
              class="text-yellow-500 sm:w-[18px] sm:h-[18px]"
              weight="fill"
            />
          </template>
          <span class="whitespace-nowrap">{{ formatDateWithI18n(cluster.created_at) }}</span>
        </div>
      </div>
      <div
        v-if="feedTitlesText"
        class="mt-1 text-[11px] sm:text-xs text-text-secondary line-clamp-1 break-all"
      >
        {{ feedTitlesText }}
      </div>
      <div
        v-if="authorsText"
        class="mt-1 text-[11px] sm:text-xs text-text-secondary/80 line-clamp-1 break-all"
      >
        {{ authorsText }}
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.article-card {
  @apply py-2 px-1.5 sm:p-3 border-b border-border cursor-pointer transition-colors flex gap-2 sm:gap-3 relative border-l-2 sm:border-l-[3px] border-l-transparent;
}

.article-card.compact {
  @apply py-1 px-2;
}

.article-card:hover {
  @apply bg-bg-tertiary;
}

.article-card.active {
  @apply bg-bg-tertiary border-l-accent;
}

.article-card.read h4 {
  @apply text-text-secondary font-normal;
}

.article-card.favorite {
  background-color: rgba(255, 215, 0, 0.05);
}

.article-card.read-later {
  background-color: rgba(59, 130, 246, 0.05);
}

.article-title {
  word-break: break-word;
  overflow-wrap: anywhere;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  display: -webkit-box;
  overflow: hidden;
}

.article-title.compact-title {
  -webkit-line-clamp: 1;
}

.article-thumbnail-placeholder {
  @apply w-16 h-12 sm:w-20 sm:h-[60px] shrink-0 border border-border rounded overflow-hidden;
  contain: layout style;
  flex-shrink: 0;
}

.article-thumbnail-placeholder.compact-thumbnail {
  @apply w-12 h-9 sm:w-14 sm:h-[42px];
}

@media (max-width: 1400px) {
  .article-thumbnail-placeholder {
    width: 56px !important;
    height: 42px !important;
  }
}
</style>
