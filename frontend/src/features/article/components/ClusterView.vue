<script setup lang="ts">
import { ref, computed } from 'vue';
import ClusterList from './ClusterList.vue';
import ClusterDetail from './ClusterDetail.vue';

interface Props {
  isSidebarOpen?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  isSidebarOpen: true,
});

const emit = defineEmits<{
  toggleSidebar: [];
}>();

const isMobile = ref(window.innerWidth < 768);
const mobileView = ref<'list' | 'detail'>('list');

function handleResize() {
  const wasMobile = isMobile.value;
  isMobile.value = window.innerWidth < 768;

  if (wasMobile && !isMobile.value && mobileView.value === 'detail') {
    mobileView.value = 'list';
  }
}

window.addEventListener('resize', handleResize);

function openClusterOnMobile() {
  mobileView.value = 'detail';
}

function closeClusterOnMobile() {
  mobileView.value = 'list';
}
</script>

<template>
  <div class="flex h-full w-full overflow-hidden relative">
    <!-- Mobile View -->
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

    <!-- Desktop View -->
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
