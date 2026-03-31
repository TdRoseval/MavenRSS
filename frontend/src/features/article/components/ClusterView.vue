<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from 'vue';
import ClusterList from './ClusterList.vue';
import ClusterDetail from './ClusterDetail.vue';
import { useArticleStore } from '@/features/article/store';
import { useClusterStore } from '@/stores/cluster';

interface Props {
  isSidebarOpen?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  isSidebarOpen: true,
});

const emit = defineEmits<{
  toggleSidebar: [];
}>();

const articleStore = useArticleStore();
const clusterStore = useClusterStore();
const isMobile = ref(window.innerWidth < 768);
const mobileView = ref<'list' | 'detail'>('list');

function handleResize() {
  const wasMobile = isMobile.value;
  isMobile.value = window.innerWidth < 768;

  if (wasMobile && !isMobile.value && mobileView.value === 'detail') {
    mobileView.value = 'list';
  }
}

async function loadClusterData() {
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

onMounted(() => {
  window.addEventListener('resize', handleResize);
  loadClusterData().catch((error) => {
    console.error('Failed to load cluster data:', error);
  });
});

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize);
});

watch(
  () => [articleStore.currentFilter, articleStore.currentFeedId, articleStore.currentCategory],
  () => {
    clusterStore.currentClusterId = null;
    mobileView.value = 'list';
    loadClusterData().catch((error) => {
      console.error('Failed to reload cluster data:', error);
    });
  }
);
</script>

<template>
  <div class="flex h-full w-full overflow-hidden relative">
    <div v-if="isMobile" class="flex-1 flex flex-col h-full w-full relative">
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
      <div class="resizer hidden md:block"></div>
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
