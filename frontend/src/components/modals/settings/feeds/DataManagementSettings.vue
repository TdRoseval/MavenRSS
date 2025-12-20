<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { ref, type Ref } from 'vue';
import {
  PhHardDrives,
  PhUpload,
  PhDownload,
  PhBroom,
  PhMagnifyingGlass,
} from '@phosphor-icons/vue';

const { t } = useI18n();

const emit = defineEmits<{
  'import-opml': [event: Event];
  'export-opml': [];
  'cleanup-database': [];
  'discover-all': [];
}>();

const opmlInput: Ref<HTMLInputElement | null> = ref(null);

function clickFileInput() {
  opmlInput.value?.click();
}

function handleImportOPML(event: Event) {
  emit('import-opml', event);
  // Reset the input value so the same file can be selected again
  if (opmlInput.value) {
    opmlInput.value.value = '';
  }
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
</script>

<template>
  <div class="setting-group">
    <label
      class="font-semibold mb-2 sm:mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2"
    >
      <PhHardDrives :size="14" class="sm:w-4 sm:h-4" />
      {{ t('dataManagement') }}
    </label>
    <div class="flex flex-col sm:flex-row gap-2 sm:gap-3 mb-2 sm:mb-3">
      <button
        class="btn-secondary flex-1 justify-center text-sm sm:text-base"
        @click="clickFileInput"
      >
        <PhUpload :size="18" class="sm:w-5 sm:h-5" /> {{ t('importOPML') }}
      </button>
      <input
        ref="opmlInput"
        type="file"
        accept=".opml,.xml,application/xml,text/xml"
        class="hidden"
        @change="handleImportOPML"
      />
      <button
        class="btn-secondary flex-1 justify-center text-sm sm:text-base"
        @click="handleExportOPML"
      >
        <PhDownload :size="18" class="sm:w-5 sm:h-5" /> {{ t('exportOPML') }}
      </button>
    </div>
    <div class="flex mb-2 sm:mb-3">
      <button
        class="btn-danger flex-1 justify-center text-sm sm:text-base"
        @click="handleCleanupDatabase"
      >
        <PhBroom :size="18" class="sm:w-5 sm:h-5" /> {{ t('cleanDatabase') }}
      </button>
    </div>
    <div class="flex flex-col sm:flex-row gap-2 sm:gap-3 mb-2 sm:mb-3">
      <button
        class="btn-primary flex-1 justify-center text-sm sm:text-base"
        @click="handleDiscoverAll"
      >
        <PhMagnifyingGlass :size="18" class="sm:w-5 sm:h-5" />
        {{ t('discoverAllFeeds') }}
      </button>
    </div>
    <p class="text-xs text-text-secondary mb-2">
      {{ t('discoverAllFeedsDesc') }}
    </p>
  </div>
</template>

<style scoped>
@reference "../../../../style.css";

.btn-primary {
  @apply bg-accent text-white px-3 sm:px-4 py-1.5 sm:py-2 rounded-md cursor-pointer flex items-center gap-1.5 sm:gap-2 font-semibold hover:bg-accent-hover transition-colors shadow-sm;
}
.btn-primary:disabled {
  @apply opacity-50 cursor-not-allowed;
}
.btn-secondary {
  @apply bg-bg-tertiary border border-border text-text-primary px-3 sm:px-4 py-1.5 sm:py-2 rounded-md cursor-pointer flex items-center gap-1.5 sm:gap-2 font-medium hover:bg-bg-secondary transition-colors;
}
.btn-secondary:disabled {
  @apply opacity-50 cursor-not-allowed;
}
.btn-danger {
  @apply bg-red-50 border border-red-300 text-red-600 px-3 sm:px-4 py-1.5 sm:py-2 rounded-md cursor-pointer flex items-center gap-1.5 sm:gap-2 font-semibold hover:bg-red-100 transition-colors;
}
.btn-danger:disabled {
  @apply opacity-50 cursor-not-allowed;
}
</style>

<style>
/* Dark mode styles need to be global to work with body.dark-mode */
.dark-mode .btn-danger {
  background-color: rgba(127, 29, 29, 0.2) !important;
  border-color: #f87171 !important;
  color: #f87171 !important;
}
.dark-mode .btn-danger:hover {
  background-color: rgba(127, 29, 29, 0.3) !important;
}
</style>
