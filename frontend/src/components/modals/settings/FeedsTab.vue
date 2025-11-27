<script setup lang="ts">
import { useAppStore } from '@/stores/app';
import { useI18n } from 'vue-i18n';
import { ref, computed, type Ref } from 'vue';
import {
  PhHardDrives,
  PhUpload,
  PhDownload,
  PhBroom,
  PhRss,
  PhPlus,
  PhTrash,
  PhFolder,
  PhPencil,
  PhMagnifyingGlass,
  PhCode,
  PhSortAscending,
} from '@phosphor-icons/vue';
import type { Feed } from '@/types/models';

const store = useAppStore();
const { t } = useI18n();

const emit = defineEmits<{
  'import-opml': [event: Event];
  'export-opml': [];
  'cleanup-database': [];
  'add-feed': [];
  'edit-feed': [feed: Feed];
  'delete-feed': [id: number];
  'batch-delete': [ids: number[]];
  'batch-move': [ids: number[]];
  'discover-all': [];
}>();

const selectedFeeds: Ref<number[]> = ref([]);
const opmlInput: Ref<HTMLInputElement | null> = ref(null);

// Sorting state
type SortField = 'name' | 'date' | 'category';
type SortDirection = 'asc' | 'desc';
const sortField = ref<SortField>('name');
const sortDirection = ref<SortDirection>('asc');

const sortedFeeds = computed(() => {
  if (!store.feeds) return [];
  const feeds = [...store.feeds];

  feeds.sort((a, b) => {
    let comparison = 0;

    if (sortField.value === 'name') {
      comparison = a.title.localeCompare(b.title, undefined, { sensitivity: 'base' });
    } else if (sortField.value === 'date') {
      // Use feed ID as proxy for add time (higher ID = newer)
      comparison = a.id - b.id;
    } else if (sortField.value === 'category') {
      const catA = a.category || '';
      const catB = b.category || '';
      comparison = catA.localeCompare(catB, undefined, { sensitivity: 'base' });
    }

    return sortDirection.value === 'asc' ? comparison : -comparison;
  });

  return feeds;
});

const isAllSelected = computed(() => {
  return store.feeds && store.feeds.length > 0 && selectedFeeds.value.length === store.feeds.length;
});

function toggleSort(field: SortField) {
  if (sortField.value === field) {
    sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc';
  } else {
    sortField.value = field;
    sortDirection.value = 'asc';
  }
}

function clickFileInput() {
  opmlInput.value?.click();
}

function toggleSelectAll(e: Event) {
  const target = e.target as HTMLInputElement;
  if (!store.feeds) return;
  if (target.checked) {
    selectedFeeds.value = store.feeds.map((f) => f.id);
  } else {
    selectedFeeds.value = [];
  }
}

function handleImportOPML(event: Event) {
  emit('import-opml', event);
}

function handleExportOPML() {
  emit('export-opml');
}

function handleCleanupDatabase() {
  emit('cleanup-database');
}

function handleDiscoverAll() {
  emit('discover-all');
}

function handleAddFeed() {
  emit('add-feed');
}

function handleEditFeed(feed: Feed) {
  emit('edit-feed', feed);
}

function handleDeleteFeed(id: number) {
  emit('delete-feed', id);
}

function handleBatchDelete() {
  if (selectedFeeds.value.length === 0) return;
  emit('batch-delete', selectedFeeds.value);
  selectedFeeds.value = [];
}

function handleBatchMove() {
  if (selectedFeeds.value.length === 0) return;
  emit('batch-move', selectedFeeds.value);
  selectedFeeds.value = [];
}

function getFavicon(url: string): string {
  try {
    return `https://www.google.com/s2/favicons?domain=${new URL(url).hostname}`;
  } catch {
    return '';
  }
}

function isScriptFeed(feed: Feed): boolean {
  return !!feed.script_path;
}

async function openScriptsFolder() {
  try {
    await fetch('/api/scripts/open', { method: 'POST' });
    window.showToast(t('scriptsFolderOpened'), 'success');
  } catch (e) {
    console.error('Failed to open scripts folder:', e);
  }
}
</script>

<template>
  <div class="space-y-4 sm:space-y-6">
    <div class="setting-group">
      <label
        class="font-semibold mb-2 sm:mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2"
      >
        <PhHardDrives :size="14" class="sm:w-4 sm:h-4" />
        {{ t('dataManagement') }}
      </label>
      <div class="flex flex-col sm:flex-row gap-2 sm:gap-3 mb-2 sm:mb-3">
        <button
          @click="clickFileInput"
          class="btn-secondary flex-1 justify-center text-sm sm:text-base"
        >
          <PhUpload :size="18" class="sm:w-5 sm:h-5" /> {{ t('importOPML') }}
        </button>
        <input type="file" ref="opmlInput" class="hidden" @change="handleImportOPML" />
        <button
          @click="handleExportOPML"
          class="btn-secondary flex-1 justify-center text-sm sm:text-base"
        >
          <PhDownload :size="18" class="sm:w-5 sm:h-5" /> {{ t('exportOPML') }}
        </button>
      </div>
      <div class="flex mb-2 sm:mb-3">
        <button
          @click="handleCleanupDatabase"
          class="btn-danger flex-1 justify-center text-sm sm:text-base"
        >
          <PhBroom :size="18" class="sm:w-5 sm:h-5" /> {{ t('cleanDatabase') }}
        </button>
      </div>
      <div class="flex flex-col sm:flex-row gap-2 sm:gap-3 mb-2 sm:mb-3">
        <button
          @click="handleDiscoverAll"
          class="btn-primary flex-1 justify-center text-sm sm:text-base"
        >
          <PhMagnifyingGlass :size="18" class="sm:w-5 sm:h-5" />
          {{ t('discoverAllFeeds') }}
        </button>
      </div>
      <p class="text-xs text-text-secondary mb-2">
        {{ t('discoverAllFeedsDesc') }}
      </p>
    </div>

    <div class="setting-group">
      <label
        class="font-semibold mb-2 sm:mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2"
      >
        <PhRss :size="14" class="sm:w-4 sm:h-4" />
        {{ t('manageFeeds') }}
      </label>

      <div class="flex flex-wrap gap-1.5 sm:gap-2 mb-2 text-xs sm:text-sm">
        <button @click="handleAddFeed" class="btn-secondary py-1.5 px-2.5 sm:px-3">
          <PhPlus :size="14" class="sm:w-4 sm:h-4" />
          <span class="hidden sm:inline">{{ t('addFeed') }}</span
          ><span class="sm:hidden">{{ t('addFeed').split(' ')[0] }}</span>
        </button>
        <button
          @click="handleBatchDelete"
          class="btn-danger py-1.5 px-2.5 sm:px-3"
          :disabled="selectedFeeds.length === 0"
        >
          <PhTrash :size="14" class="sm:w-4 sm:h-4" />
          <span class="hidden sm:inline">{{ t('deleteSelected') }}</span
          ><span class="sm:hidden">{{ t('delete') }}</span>
        </button>
        <button
          @click="handleBatchMove"
          class="btn-secondary py-1.5 px-2.5 sm:px-3"
          :disabled="selectedFeeds.length === 0"
        >
          <PhFolder :size="14" class="sm:w-4 sm:h-4" />
          <span class="hidden sm:inline">{{ t('moveSelected') }}</span
          ><span class="sm:hidden">{{ t('move') }}</span>
        </button>
      </div>

      <div class="border border-border rounded-lg bg-bg-secondary">
        <!-- Table Header -->
        <div
          class="flex items-center justify-between p-1.5 sm:p-2 border-b border-border bg-bg-tertiary"
        >
          <label class="flex items-center gap-1.5 sm:gap-2 cursor-pointer select-none">
            <input
              type="checkbox"
              :checked="isAllSelected"
              @change="toggleSelectAll"
              class="w-3.5 h-3.5 sm:w-4 sm:h-4 rounded border-border text-accent focus:ring-2 focus:ring-accent cursor-pointer"
            />
            <span class="hidden sm:inline text-xs sm:text-sm">{{ t('selectAll') }}</span>
          </label>
          <div class="flex items-center gap-1">
            <PhSortAscending :size="12" class="text-text-secondary" />
            <button
              @click="toggleSort('name')"
              :class="[
                'px-1.5 py-0.5 text-xs rounded transition-colors',
                sortField === 'name'
                  ? 'bg-accent text-white'
                  : 'bg-bg-secondary text-text-primary hover:bg-bg-primary',
              ]"
            >
              {{ t('sortByName') }}
              <span v-if="sortField === 'name'">{{ sortDirection === 'asc' ? '↑' : '↓' }}</span>
            </button>
            <button
              @click="toggleSort('date')"
              :class="[
                'px-1.5 py-0.5 text-xs rounded transition-colors',
                sortField === 'date'
                  ? 'bg-accent text-white'
                  : 'bg-bg-secondary text-text-primary hover:bg-bg-primary',
              ]"
            >
              {{ t('sortByDate') }}
              <span v-if="sortField === 'date'">{{ sortDirection === 'asc' ? '↑' : '↓' }}</span>
            </button>
            <button
              @click="toggleSort('category')"
              :class="[
                'px-1.5 py-0.5 text-xs rounded transition-colors',
                sortField === 'category'
                  ? 'bg-accent text-white'
                  : 'bg-bg-secondary text-text-primary hover:bg-bg-primary',
              ]"
            >
              {{ t('sortByCategory') }}
              <span v-if="sortField === 'category'">{{ sortDirection === 'asc' ? '↑' : '↓' }}</span>
            </button>
          </div>
        </div>

        <!-- Scrollable Content -->
        <div class="overflow-y-auto max-h-48 sm:max-h-80">
          <!-- Feed Rows -->
          <div
            v-for="feed in sortedFeeds"
            :key="feed.id"
            class="flex items-center p-1.5 sm:p-2 border-b border-border last:border-0 bg-bg-primary hover:bg-bg-secondary gap-1.5 sm:gap-2"
          >
            <input
              type="checkbox"
              :value="feed.id"
              v-model="selectedFeeds"
              class="w-3.5 h-3.5 sm:w-4 sm:h-4 shrink-0 rounded border-border text-accent focus:ring-2 focus:ring-accent cursor-pointer"
            />
            <div class="w-4 h-4 flex items-center justify-center shrink-0">
              <img
                :src="getFavicon(feed.url)"
                class="w-full h-full object-contain"
                @error="
                  ($event: Event) => {
                    const target = $event.target as HTMLImageElement;
                    if (target) target.style.display = 'none';
                  }
                "
              />
            </div>
            <div class="truncate flex-1 min-w-0">
              <div class="font-medium truncate text-xs sm:text-sm">{{ feed.title }}</div>
              <div class="text-xs text-text-secondary truncate hidden sm:block">
                <span v-if="feed.category" class="inline-flex items-center gap-1">
                  <PhFolder :size="10" class="inline" />
                  {{ feed.category }}
                  <span class="mx-1">•</span>
                </span>
                <span v-if="isScriptFeed(feed)" class="inline-flex items-center gap-1">
                  <PhCode :size="10" class="inline" />
                  <button
                    @click.stop="openScriptsFolder"
                    class="text-accent hover:underline"
                    :title="t('openScriptsFolder')"
                  >
                    {{ feed.script_path }}
                  </button>
                </span>
                <span v-else>{{ feed.url }}</span>
              </div>
            </div>
            <div class="flex gap-0.5 sm:gap-1 shrink-0">
              <button
                @click="handleEditFeed(feed)"
                class="text-accent hover:bg-bg-tertiary p-1 rounded text-sm"
                :title="t('edit')"
              >
                <PhPencil :size="14" class="sm:w-4 sm:h-4" />
              </button>
              <button
                @click="handleDeleteFeed(feed.id)"
                class="text-red-500 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 p-1 rounded text-sm"
                :title="t('delete')"
              >
                <PhTrash :size="14" class="sm:w-4 sm:h-4" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.btn-primary {
  @apply bg-accent text-white px-3 sm:px-4 py-1.5 sm:py-2 rounded-md cursor-pointer flex items-center gap-1.5 sm:gap-2 font-semibold hover:bg-accent-hover transition-colors shadow-sm;
}
.btn-primary:disabled {
  @apply opacity-50 cursor-not-allowed;
}
.btn-secondary {
  @apply bg-transparent border border-border text-text-primary px-3 sm:px-4 py-1.5 sm:py-2 rounded-md cursor-pointer flex items-center gap-1.5 sm:gap-2 font-medium hover:bg-bg-tertiary transition-colors;
}
.btn-secondary:disabled {
  @apply opacity-50 cursor-not-allowed;
}
.btn-danger {
  @apply bg-transparent border border-red-300 text-red-600 px-3 sm:px-4 py-1.5 sm:py-2 rounded-md cursor-pointer flex items-center gap-1.5 sm:gap-2 font-semibold hover:bg-red-50 dark:hover:bg-red-900/20 dark:border-red-400 dark:text-red-400 transition-colors;
}
.btn-danger:disabled {
  @apply opacity-50 cursor-not-allowed;
}
</style>
