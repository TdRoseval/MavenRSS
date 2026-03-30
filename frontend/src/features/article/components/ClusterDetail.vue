<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { PhSparkle, PhX, PhClockCountdown, PhStar } from '@phosphor-icons/vue';
import type { Cluster } from '@/types/models';
import { useClusterStore } from '@/stores/cluster';
import { formatDate as formatDateUtil } from '@/shared/lib/date';
import DOMPurify from 'dompurify';
import { marked } from 'marked';

interface Props {
  isMobile?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  isMobile: false,
});

const emit = defineEmits<{
  close: [];
}>();

const { t, locale } = useI18n();
const clusterStore = useClusterStore();

const cluster = ref<Cluster | null>(null);
const isLoading = ref(false);

const formatDateWithI18n = (dateStr: string): string => {
  return formatDateUtil(dateStr, locale.value, t);
};

// Parse markdown securely using marked and DOMPurify
const formattedContent = computed(() => {
  if (!cluster.value?.merged_content) return '';
  
  const html = marked.parse(cluster.value.merged_content, { async: false }) as string;
  return DOMPurify.sanitize(html);
});

async function loadClusterDetail(id: number) {
  isLoading.value = true;
  cluster.value = await clusterStore.fetchClusterDetail(id);
  isLoading.value = false;
}

watch(
  () => clusterStore.currentClusterId,
  (newId) => {
    if (newId) {
      loadClusterDetail(newId);
    } else {
      cluster.value = null;
    }
  },
  { immediate: true }
);

function handleClose() {
  clusterStore.currentClusterId = null;
  if (props.isMobile) {
    emit('close');
  }
}
</script>

<template>
  <main
    :class="[
      'flex-1 bg-bg-primary flex flex-col h-full absolute w-full md:static md:w-auto z-30 transition-transform duration-300',
      clusterStore.currentClusterId ? 'translate-x-0 md:translate-x-0' : 'translate-x-full',
    ]"
  >
    <!-- Mobile header with back button -->
    <div
      v-if="isMobile && cluster"
      class="flex items-center gap-2 px-3 py-2 border-b border-border bg-bg-secondary"
    >
      <span class="flex-1 truncate text-sm font-medium text-center pr-8">{{ cluster.merged_title }}</span>
      <button
        class="flex items-center justify-center p-2 -mr-2 rounded-lg hover:bg-bg-tertiary transition-colors"
        title="Back to list"
        @click="handleClose"
      >
        <PhX :size="20" />
      </button>
    </div>

    <div
      v-if="!clusterStore.currentClusterId && !isLoading"
      class="hidden md:flex flex-col items-center justify-center h-full text-text-secondary text-center px-4"
    >
      <PhSparkle :size="48" class="mb-4 sm:mb-5 opacity-50 sm:w-16 sm:h-16" />
      <p class="text-sm sm:text-base">Select a cluster to view the AI fused article</p>
    </div>

    <div
      v-else-if="isLoading"
      class="flex-1 flex flex-col items-center justify-center p-8 text-text-secondary"
    >
      <div class="w-8 h-8 border-4 border-accent border-t-transparent rounded-full animate-spin mb-4" />
      <div>Loading fusion details...</div>
    </div>

    <div v-else-if="cluster" class="flex flex-col h-full bg-bg-primary overflow-hidden">
      <!-- Simple toolbar -->
      <div class="flex items-center justify-between p-2 border-b border-border bg-bg-secondary shrink-0 min-h-[48px] sm:min-h-[56px]">
        <div class="flex items-center gap-1 sm:gap-2 overflow-x-auto no-scrollbar mask-edges-right pr-4">
          <div class="flex gap-1.5 p-1 bg-bg-tertiary rounded-lg">
             <span class="text-sm font-semibold px-2 py-1">AI Fusion</span>
          </div>
        </div>
        
        <div class="flex items-center gap-0.5 sm:gap-1 pl-2">
           <button
            v-if="!isMobile"
            class="hidden md:flex items-center justify-center w-8 h-8 sm:w-9 sm:h-9 rounded-lg hover:bg-bg-tertiary text-text-secondary transition-colors"
            title="Close"
            @click="handleClose"
          >
            <PhX :size="20" class="sm:w-[22px] sm:h-[22px]" />
          </button>
        </div>
      </div>

      <!-- Content -->
      <div class="flex-1 overflow-y-auto w-full custom-scrollbar">
        <div class="max-w-[800px] w-[95%] sm:w-[90%] md:w-[85%] mx-auto py-6 sm:py-8 lg:py-12 px-4 sm:px-0">
          <!-- Article Header -->
          <header class="mb-6 sm:mb-8 text-center sm:text-left">
            <h1 class="text-2xl sm:text-3xl lg:text-4xl font-bold text-text-primary leading-tight font-serif mb-4 sm:mb-6">
              {{ cluster.merged_title }}
            </h1>

            <div class="flex flex-wrap items-center justify-center sm:justify-start gap-x-4 gap-y-2 text-sm sm:text-base text-text-secondary">
               <div class="flex items-center gap-1.5 font-medium text-accent">
                 <span>Merged from {{ cluster.article_count }} articles</span>
               </div>
               <span class="hidden sm:inline text-border">•</span>
               <div class="flex items-center gap-1.5">
                 <span class="font-medium">Status:</span>
                 <span class="capitalize">{{ cluster.status }}</span>
               </div>
               <span class="hidden sm:inline text-border">•</span>
               <span class="whitespace-nowrap">{{ formatDateWithI18n(cluster.created_at) }}</span>
            </div>
          </header>

          <!-- Divider -->
          <hr class="border-border my-6 sm:my-8" />

          <!-- Summary -->
          <div v-if="cluster.merged_summary" class="bg-bg-secondary p-4 rounded-lg mb-8 border border-border">
            <h3 class="font-semibold text-lg mb-2">Summary</h3>
            <p class="text-text-secondary leading-relaxed">{{ cluster.merged_summary }}</p>
          </div>

          <!-- Parsed Content -->
          <article 
            class="prose-content w-full" 
            v-html="formattedContent"
          />
          
          <div v-if="cluster.articles && cluster.articles.length > 0" class="mt-12 pt-8 border-t border-border">
            <h3 class="font-semibold text-xl mb-4 text-text-primary">Source Articles</h3>
            <ul class="space-y-3">
              <li v-for="article in cluster.articles" :key="article.id" class="flex flex-col gap-1 p-3 bg-bg-secondary rounded-lg border border-border">
                <a :href="article.url" target="_blank" class="font-medium text-accent hover:underline line-clamp-1">
                  {{ article.title }}
                </a>
                <span class="text-xs text-text-secondary">{{ article.feed_title }} • {{ formatDateWithI18n(article.published_at) }}</span>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  </main>
</template>

<style scoped>
@reference "../../style.css";

.prose-content {
  @apply text-text-primary;
  font-family: inherit;
  font-size: 1.125rem;
  line-height: 1.75;
}

.prose-content :deep(h1) {
  @apply text-2xl font-bold mt-8 mb-4 font-serif;
}

.prose-content :deep(h2) {
  @apply text-xl font-bold mt-8 mb-4 font-serif;
}

.prose-content :deep(h3) {
  @apply text-lg font-bold mt-6 mb-3 font-serif;
}

.prose-content :deep(p) {
  @apply mb-5;
}

.prose-content :deep(a) {
  @apply text-accent hover:underline;
}

.prose-content :deep(ul) {
  @apply list-disc pl-6 mb-5;
}

.prose-content :deep(ol) {
  @apply list-decimal pl-6 mb-5;
}

.prose-content :deep(blockquote) {
  @apply pl-4 border-l-4 border-border text-text-secondary italic my-5;
}
</style>
