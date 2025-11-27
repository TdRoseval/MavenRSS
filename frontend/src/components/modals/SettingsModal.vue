<script setup lang="ts">
import { useAppStore } from '@/stores/app';
import { useI18n } from 'vue-i18n';
import { ref, onMounted, type Ref } from 'vue';
import GeneralTab from './settings/GeneralTab.vue';
import FeedsTab from './settings/FeedsTab.vue';
import ShortcutsTab from './settings/ShortcutsTab.vue';
import RulesTab from './settings/RulesTab.vue';
import AboutTab from './settings/AboutTab.vue';
import DiscoverAllFeedsModal from './discovery/DiscoverAllFeedsModal.vue';
import { PhGear } from '@phosphor-icons/vue';
import type { TabName } from '@/types/settings';
import type { ThemePreference } from '@/stores/app';
import { useSettings } from '@/composables/core/useSettings';
import { useAppUpdates } from '@/composables/core/useAppUpdates';
import { useFeedManagement } from '@/composables/feed/useFeedManagement';
import { useModalClose } from '@/composables/ui/useModalClose';

const store = useAppStore();
const { t } = useI18n();

// Use composables
const { settings, fetchSettings, applySettings } = useSettings();
const {
  updateInfo,
  checkingUpdates,
  downloadingUpdate,
  installingUpdate,
  downloadProgress,
  checkForUpdates: handleCheckUpdates,
  downloadAndInstallUpdate: handleDownloadInstallUpdate,
} = useAppUpdates();
const {
  handleImportOPML,
  handleExportOPML,
  handleCleanupDatabase,
  handleAddFeed,
  handleEditFeed,
  handleDeleteFeed,
  handleBatchDelete,
  handleBatchMove,
} = useFeedManagement();

const emit = defineEmits<{
  close: [];
}>();

const activeTab: Ref<TabName> = ref('general');
const showDiscoverAllModal = ref(false);

// Modal close handling
useModalClose(() => emit('close'));

onMounted(async () => {
  try {
    const data = await fetchSettings();
    applySettings(data, (theme: string) => store.setTheme(theme as ThemePreference));
  } catch (e) {
    console.error('Error loading settings:', e);
  }
});

function handleDiscoverAll() {
  showDiscoverAllModal.value = true;
}
</script>

<template>
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-2 sm:p-4"
    @click.self="emit('close')"
    data-modal-open="true"
  >
    <div
      class="bg-bg-primary w-full max-w-4xl h-full sm:h-[700px] sm:max-h-[90vh] flex flex-col rounded-none sm:rounded-2xl shadow-2xl border border-border overflow-hidden animate-fade-in"
    >
      <div class="p-3 sm:p-5 border-b border-border flex justify-between items-center shrink-0">
        <h3 class="text-base sm:text-lg font-semibold m-0 flex items-center gap-2">
          <PhGear :size="20" class="sm:w-6 sm:h-6" />
          {{ t('settingsTitle') }}
        </h3>
        <span
          @click="emit('close')"
          class="text-2xl cursor-pointer text-text-secondary hover:text-text-primary"
          >&times;</span
        >
      </div>

      <div
        class="flex border-b border-border bg-bg-secondary shrink-0 overflow-x-auto scrollbar-hide"
      >
        <button
          @click="activeTab = 'general'"
          :class="['tab-btn', activeTab === 'general' ? 'active' : '']"
        >
          {{ t('general') }}
        </button>
        <button
          @click="activeTab = 'feeds'"
          :class="['tab-btn', activeTab === 'feeds' ? 'active' : '']"
        >
          {{ t('feeds') }}
        </button>
        <button
          @click="activeTab = 'rules'"
          :class="['tab-btn', activeTab === 'rules' ? 'active' : '']"
        >
          {{ t('rules') }}
        </button>
        <button
          @click="activeTab = 'shortcuts'"
          :class="['tab-btn', activeTab === 'shortcuts' ? 'active' : '']"
        >
          {{ t('shortcuts') }}
        </button>
        <button
          @click="activeTab = 'about'"
          :class="['tab-btn', activeTab === 'about' ? 'active' : '']"
        >
          {{ t('about') }}
        </button>
      </div>

      <div class="flex-1 overflow-y-auto p-3 sm:p-6 min-h-0">
        <GeneralTab v-if="activeTab === 'general'" :settings="settings" />

        <FeedsTab
          v-if="activeTab === 'feeds'"
          @import-opml="handleImportOPML"
          @export-opml="handleExportOPML"
          @cleanup-database="handleCleanupDatabase"
          @add-feed="handleAddFeed"
          @edit-feed="handleEditFeed"
          @delete-feed="handleDeleteFeed"
          @batch-delete="handleBatchDelete"
          @batch-move="handleBatchMove"
          @discover-all="handleDiscoverAll"
        />

        <RulesTab v-if="activeTab === 'rules'" :settings="settings" />

        <ShortcutsTab v-if="activeTab === 'shortcuts'" :settings="settings" />

        <AboutTab
          v-if="activeTab === 'about'"
          :update-info="updateInfo"
          :checking-updates="checkingUpdates"
          :downloading-update="downloadingUpdate"
          :installing-update="installingUpdate"
          :download-progress="downloadProgress"
          @check-updates="handleCheckUpdates"
          @download-install-update="handleDownloadInstallUpdate"
        />
      </div>
    </div>

    <!-- Discover All Feeds Modal -->
    <DiscoverAllFeedsModal :show="showDiscoverAllModal" @close="showDiscoverAllModal = false" />
  </div>
</template>

<style scoped>
.tab-btn {
  @apply px-3 sm:px-5 py-2 sm:py-3 bg-transparent border-b-2 border-transparent text-text-secondary font-semibold cursor-pointer hover:text-text-primary transition-all relative whitespace-nowrap text-sm sm:text-base;
}
.tab-btn:hover {
  background-color: rgba(128, 128, 128, 0.1);
}
.tab-btn.active {
  @apply text-accent border-accent;
  background-color: rgba(128, 128, 128, 0.05);
}
.tab-btn.active::after {
  content: '';
  position: absolute;
  bottom: -2px;
  left: 0;
  right: 0;
  height: 2px;
  background: linear-gradient(90deg, transparent, var(--accent-color), transparent);
  animation: shimmer 2s ease-in-out infinite;
}
@keyframes shimmer {
  0%,
  100% {
    opacity: 0.5;
  }
  50% {
    opacity: 1;
  }
}
.btn-primary {
  @apply bg-accent text-white border-none px-5 py-2.5 rounded-lg cursor-pointer font-semibold hover:bg-accent-hover transition-colors;
}
.animate-fade-in {
  animation: modalFadeIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes modalFadeIn {
  from {
    transform: translateY(-20px);
    opacity: 0;
  }
  to {
    transform: translateY(0);
    opacity: 1;
  }
}
.scrollbar-hide {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
.scrollbar-hide::-webkit-scrollbar {
  display: none;
}
</style>
