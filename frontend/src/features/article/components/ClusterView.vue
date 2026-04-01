<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from 'vue';
import { useI18n } from 'vue-i18n';
import ClusterList from './ClusterList.vue';
import ClusterDetail from './ClusterDetail.vue';
import { useArticleStore } from '@/features/article/store';
import { useClusterStore } from '@/stores/cluster';
import { useResizablePanels } from '@/composables/ui/useResizablePanels';

interface Props {
  isSidebarOpen?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  isSidebarOpen: true,
});

const emit = defineEmits<{
  toggleSidebar: [];
}>();

const { t } = useI18n();
const articleStore = useArticleStore();
const clusterStore = useClusterStore();
const { startResizeArticleList } = useResizablePanels();
const isMobile = ref(window.innerWidth < 768);
const mobileView = ref<'list' | 'detail'>('list');
const showRecentFailureModal = ref(false);

function handleResize() {
  const wasMobile = isMobile.value;
  isMobile.value = window.innerWidth < 768;

  if (wasMobile && !isMobile.value && mobileView.value === 'detail') {
    mobileView.value = 'list';
  }
}

async function loadClusterData() {
  if (clusterStore.shouldBlockClusterView) {
    return;
  }

  if (articleStore.currentFilter === 'dailyRecommendations') {
    const dates = await clusterStore.fetchDailyRecommendationDates();
    if (dates.length > 0) {
      await clusterStore.fetchDailyRecommendations(
        clusterStore.selectedRecommendationDate || dates[0]
      );
    } else {
      clusterStore.dailyRecommendations = [];
    }
    return;
  }

  await clusterStore.fetchClusters();
}

function openClusterOnMobile() {
  mobileView.value = 'detail';
}

function closeClusterOnMobile() {
  mobileView.value = 'list';
}

function openRecentFailureModal() {
  showRecentFailureModal.value = true;
}

function closeRecentFailureModal() {
  showRecentFailureModal.value = false;
}

function getFailurePreview(message?: string) {
  if (!message) {
    return '';
  }
  return message.length > 120 ? `${message.slice(0, 120)}...` : message;
}

function getFailureStageLabel(stage?: string) {
  switch (stage) {
    case 'summary':
      return t('article.cluster.processingPendingSummary');
    case 'translation':
      return t('article.cluster.processingPendingTranslation');
    case 'embedding':
      return t('article.cluster.processingPendingEmbedding');
    case 'clustering':
      return t('article.cluster.processingPendingClustering');
    case 'recommendation':
      return t('article.cluster.processingPendingRecommendationTask');
    default:
      return t('article.cluster.processingUnknownFailureStage');
  }
}

onMounted(() => {
  window.addEventListener('resize', handleResize);
  clusterStore
    .startAIProcessingPolling()
    .then(() => loadClusterData())
    .catch((error) => {
      console.error('Failed to initialize AI processing status:', error);
    });
});

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize);
  clusterStore.stopAIProcessingPolling();
});

watch(
  () => clusterStore.shouldBlockClusterView,
  (isBlocked, wasBlocked) => {
    if (isBlocked) {
      clusterStore.clearData();
      return;
    }

    if (wasBlocked && !isBlocked) {
      loadClusterData().catch((error) => {
        console.error('Failed to load cluster data after AI processing completed:', error);
      });
    }
  }
);

watch(
  () => clusterStore.aiProcessingStatus?.has_interest_vector,
  (hasInterestVector, previousValue) => {
    if (typeof previousValue !== 'boolean' || hasInterestVector === previousValue) {
      return;
    }

    if (clusterStore.shouldBlockClusterView || articleStore.currentFilter === 'dailyRecommendations') {
      return;
    }

    clusterStore.currentClusterId = null;
    mobileView.value = 'list';
    loadClusterData().catch((error) => {
      console.error('Failed to reload cluster data after interest vector changed:', error);
    });
  }
);

watch(
  () => [articleStore.currentFilter, articleStore.currentFeedId, articleStore.currentCategory],
  () => {
    clusterStore.currentClusterId = null;
    mobileView.value = 'list';

    if (clusterStore.shouldBlockClusterView) {
      clusterStore.clearData();
      return;
    }

    loadClusterData().catch((error) => {
      console.error('Failed to reload cluster data:', error);
    });
  }
);

watch(
  () => clusterStore.aiProcessingStatus?.recent_failure_message,
  (message) => {
    if (!message) {
      showRecentFailureModal.value = false;
    }
  }
);
</script>

<template>
  <div class="flex h-full w-full overflow-hidden relative">
    <template v-if="clusterStore.shouldBlockClusterView">
      <div class="flex h-full w-full items-center justify-center bg-bg-primary px-6">
        <div class="w-full max-w-xl rounded-2xl border border-border bg-bg-secondary p-6 sm:p-8">
          <div class="text-sm font-medium uppercase tracking-[0.18em] text-text-tertiary">
            {{ t('article.cluster.processingLabel') }}
          </div>
          <h3 class="mt-3 text-xl font-semibold text-text-primary">
            {{ t('article.cluster.processingTitle') }}
          </h3>
          <p class="mt-2 text-sm leading-6 text-text-secondary">
            {{ t('article.cluster.processingDescription') }}
          </p>

          <div class="mt-6">
            <div class="mb-2 flex items-center justify-between text-sm text-text-secondary">
              <span>{{ t('article.cluster.processingProgress') }}</span>
              <span>{{ clusterStore.aiProcessingProgressPercent }}%</span>
            </div>
            <div class="h-3 overflow-hidden rounded-full bg-bg-tertiary">
              <div
                class="h-full rounded-full bg-accent transition-[width] duration-500 ease-out"
                :style="{ width: `${clusterStore.aiProcessingProgressPercent}%` }"
              />
            </div>
          </div>

          <div class="mt-4 grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
            <div class="rounded-xl bg-bg-primary p-3">
              <div class="text-text-tertiary">{{ t('article.cluster.processingEligible') }}</div>
              <div class="mt-1 text-lg font-semibold text-text-primary">
                {{ clusterStore.aiProcessingStatus?.eligible_articles ?? 0 }}
              </div>
            </div>
            <div class="rounded-xl bg-bg-primary p-3">
              <div class="text-text-tertiary">{{ t('article.cluster.processingCompleted') }}</div>
              <div class="mt-1 text-lg font-semibold text-text-primary">
                {{ clusterStore.aiProcessingStatus?.completed_articles ?? 0 }}
              </div>
            </div>
            <div class="rounded-xl bg-bg-primary p-3">
              <div class="text-text-tertiary">{{ t('article.cluster.processingPending') }}</div>
              <div class="mt-1 text-lg font-semibold text-text-primary">
                {{ clusterStore.aiProcessingStatus?.pending_articles ?? 0 }}
              </div>
            </div>
            <div class="rounded-xl bg-bg-primary p-3">
              <div class="text-text-tertiary">{{ t('article.cluster.processingQueued') }}</div>
              <div class="mt-1 text-lg font-semibold text-text-primary">
                {{ clusterStore.aiProcessingStatus?.queued_tasks ?? 0 }}
              </div>
            </div>
          </div>

          <div class="mt-6 rounded-2xl border border-border/70 bg-bg-primary p-4">
            <div class="text-sm font-medium text-text-primary">
              {{ t('article.cluster.processingBreakdownTitle') }}
            </div>
            <p class="mt-1 text-xs leading-5 text-text-tertiary">
              {{ t('article.cluster.processingBreakdownDescription') }}
            </p>

            <div class="mt-4 grid grid-cols-5 gap-2 text-xs">
              <div class="rounded-xl bg-bg-secondary p-2.5">
                <div class="leading-4 text-text-tertiary">
                  {{ t('article.cluster.processingPendingSummary') }}
                </div>
                <div class="mt-1 text-base font-semibold text-text-primary">
                  {{ clusterStore.aiProcessingStatus?.pending_summary_articles ?? 0 }}
                </div>
              </div>
              <div class="rounded-xl bg-bg-secondary p-2.5">
                <div class="leading-4 text-text-tertiary">
                  {{ t('article.cluster.processingPendingTranslation') }}
                </div>
                <div class="mt-1 text-base font-semibold text-text-primary">
                  {{ clusterStore.aiProcessingStatus?.pending_translation_articles ?? 0 }}
                </div>
              </div>
              <div class="rounded-xl bg-bg-secondary p-2.5">
                <div class="leading-4 text-text-tertiary">
                  {{ t('article.cluster.processingPendingEmbedding') }}
                </div>
                <div class="mt-1 text-base font-semibold text-text-primary">
                  {{ clusterStore.aiProcessingStatus?.pending_embedding_articles ?? 0 }}
                </div>
              </div>
              <div class="rounded-xl bg-bg-secondary p-2.5">
                <div class="leading-4 text-text-tertiary">
                  {{ t('article.cluster.processingPendingClustering') }}
                </div>
                <div class="mt-1 text-base font-semibold text-text-primary">
                  {{ clusterStore.aiProcessingStatus?.pending_clustering_articles ?? 0 }}
                </div>
              </div>
              <div class="rounded-xl bg-bg-secondary p-2.5">
                <div class="leading-4 text-text-tertiary">
                  {{ t('article.cluster.processingWorkers') }}
                </div>
                <div class="mt-1 text-base font-semibold text-text-primary">
                  {{ clusterStore.aiProcessingStatus?.active_worker_tasks ?? 0 }}
                </div>
              </div>
            </div>

            <p
              v-if="(clusterStore.aiProcessingStatus?.pending_recommendation_days ?? 0) > 0"
              class="mt-4 text-xs leading-5 text-text-tertiary"
            >
              {{ t('article.cluster.processingAdditionalRecommendation', {
                count: clusterStore.aiProcessingStatus?.pending_recommendation_days ?? 0,
              }) }}
            </p>

            <div
              v-if="clusterStore.aiProcessingStatus?.recent_failure_message"
              class="mt-4 rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-950"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="font-medium">
                  {{ t('article.cluster.processingRecentFailureTitle') }}
                </div>
                <button
                  type="button"
                  class="rounded-lg border border-amber-300 bg-white/80 px-3 py-1 text-xs font-medium text-amber-950 transition hover:bg-white"
                  @click="openRecentFailureModal"
                >
                  {{ t('article.cluster.processingRecentFailureView') }}
                </button>
              </div>
              <div class="mt-2 leading-6">
                {{
                  t('article.cluster.processingRecentFailureStage', {
                    stage: getFailureStageLabel(clusterStore.aiProcessingStatus?.recent_failure_stage),
                  })
                }}
              </div>
              <div
                v-if="clusterStore.aiProcessingStatus?.recent_failure_article_title"
                class="mt-1 leading-6"
              >
                {{
                  t('article.cluster.processingRecentFailureArticle', {
                    title: clusterStore.aiProcessingStatus?.recent_failure_article_title,
                  })
                }}
              </div>
              <div
                v-if="clusterStore.aiProcessingStatus?.recent_failure_model"
                class="mt-1 leading-6"
              >
                {{
                  t('article.cluster.processingRecentFailureModel', {
                    model: clusterStore.aiProcessingStatus?.recent_failure_model,
                  })
                }}
              </div>
              <div
                v-if="clusterStore.aiProcessingStatus?.recent_failure_endpoint"
                class="mt-1 break-all leading-6"
              >
                {{
                  t('article.cluster.processingRecentFailureEndpoint', {
                    endpoint: clusterStore.aiProcessingStatus?.recent_failure_endpoint,
                  })
                }}
              </div>
              <div class="mt-2 rounded-lg bg-white/70 p-3 text-xs leading-5 text-amber-900">
                {{ getFailurePreview(clusterStore.aiProcessingStatus?.recent_failure_message) }}
              </div>
              <div
                v-if="(clusterStore.aiProcessingStatus?.recent_failure_count ?? 0) > 1"
                class="mt-2 text-xs leading-5 text-amber-900"
              >
                {{
                  t('article.cluster.processingRecentFailureCount', {
                    count: clusterStore.aiProcessingStatus?.recent_failure_count ?? 0,
                  })
                }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <Teleport to="body">
        <div
          v-if="showRecentFailureModal && clusterStore.aiProcessingStatus?.recent_failure_message"
          class="fixed inset-0 z-[120] flex items-center justify-center bg-black/45 px-4 py-6"
          @click.self="closeRecentFailureModal"
        >
          <div class="w-full max-w-2xl rounded-2xl border border-border bg-bg-secondary shadow-2xl">
            <div class="flex items-center justify-between border-b border-border px-5 py-4">
              <div>
                <h3 class="text-lg font-semibold text-text-primary">
                  {{ t('article.cluster.processingRecentFailureTitle') }}
                </h3>
                <p class="mt-1 text-sm text-text-secondary">
                  {{
                    t('article.cluster.processingRecentFailureStage', {
                      stage: getFailureStageLabel(clusterStore.aiProcessingStatus?.recent_failure_stage),
                    })
                  }}
                </p>
              </div>
              <button
                type="button"
                class="rounded-lg border border-border bg-bg-primary px-3 py-1.5 text-sm text-text-secondary transition hover:text-text-primary"
                @click="closeRecentFailureModal"
              >
                {{ t('article.cluster.processingRecentFailureClose') }}
              </button>
            </div>

            <div class="max-h-[70vh] space-y-3 overflow-y-auto px-5 py-4 text-sm text-text-primary">
              <div v-if="clusterStore.aiProcessingStatus?.recent_failure_article_title">
                {{
                  t('article.cluster.processingRecentFailureArticle', {
                    title: clusterStore.aiProcessingStatus?.recent_failure_article_title,
                  })
                }}
              </div>
              <div v-if="clusterStore.aiProcessingStatus?.recent_failure_model">
                {{
                  t('article.cluster.processingRecentFailureModel', {
                    model: clusterStore.aiProcessingStatus?.recent_failure_model,
                  })
                }}
              </div>
              <div
                v-if="clusterStore.aiProcessingStatus?.recent_failure_endpoint"
                class="break-all"
              >
                {{
                  t('article.cluster.processingRecentFailureEndpoint', {
                    endpoint: clusterStore.aiProcessingStatus?.recent_failure_endpoint,
                  })
                }}
              </div>
              <div
                v-if="(clusterStore.aiProcessingStatus?.recent_failure_count ?? 0) > 1"
                class="text-text-secondary"
              >
                {{
                  t('article.cluster.processingRecentFailureCount', {
                    count: clusterStore.aiProcessingStatus?.recent_failure_count ?? 0,
                  })
                }}
              </div>

              <div class="rounded-xl bg-bg-primary p-4">
                <div class="mb-2 text-xs font-medium uppercase tracking-[0.16em] text-text-tertiary">
                  {{ t('article.cluster.processingRecentFailureMessage') }}
                </div>
                <pre class="whitespace-pre-wrap break-words text-sm leading-6 text-text-primary">{{
                  clusterStore.aiProcessingStatus?.recent_failure_message
                }}</pre>
              </div>
            </div>
          </div>
        </div>
      </Teleport>
    </template>

    <div v-else-if="isMobile" class="flex-1 flex flex-col h-full w-full relative">
      <div
        :class="[
          'absolute inset-0 z-10 transition-opacity duration-200',
          mobileView === 'list' ? 'opacity-100 visible' : 'opacity-0 invisible pointer-events-none',
        ]"
      >
        <ClusterList
          :is-mobile="true"
          :is-sidebar-open="isSidebarOpen"
          @toggle-sidebar="emit('toggleSidebar')"
          @select-cluster="openClusterOnMobile"
        />
      </div>

      <div
        :class="[
          'absolute inset-0 z-20 transition-transform duration-300',
          mobileView === 'detail' ? 'translate-x-0' : 'translate-x-full',
        ]"
      >
        <ClusterDetail :is-mobile="true" @close="closeClusterOnMobile" />
      </div>
    </div>

    <template v-else>
      <ClusterList :is-sidebar-open="isSidebarOpen" @toggle-sidebar="emit('toggleSidebar')" />
      <div class="resizer hidden md:block" @mousedown="startResizeArticleList"></div>
      <ClusterDetail />
    </template>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.resizer {
  width: 4px;
  cursor: col-resize;
  background-color: transparent;
  flex-shrink: 0;
  transition: background-color 0.2s;
  z-index: 10;
  margin-left: -2px;
  margin-right: -2px;
}
.resizer:hover,
.resizer:active {
  background-color: var(--color-accent, #3b82f6);
}
</style>
