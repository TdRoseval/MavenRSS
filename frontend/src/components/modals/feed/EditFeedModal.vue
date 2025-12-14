<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { PhCode, PhBookOpen, PhCaretDown, PhCaretRight } from '@phosphor-icons/vue';
import type { Feed } from '@/types/models';
import { useModalClose } from '@/composables/ui/useModalClose';
import { useAppStore } from '@/stores/app';

const { t } = useI18n();
const store = useAppStore();

// Check if image gallery feature is enabled
const imageGalleryEnabled = ref(false);

type FeedType = 'url' | 'script';
type ProxyMode = 'global' | 'custom' | 'none';
type RefreshMode = 'global' | 'fixed' | 'intelligent' | 'custom';

interface Props {
  feed: Feed;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  close: [];
  updated: [];
}>();

const feedType = ref<FeedType>('url');
const title = ref('');
const url = ref('');
const category = ref('');
const categorySelection = ref('');
const showCustomCategory = ref(false);
const scriptPath = ref('');
const hideFromTimeline = ref(false);
const isImageMode = ref(false);

// Proxy settings
const proxyMode = ref<ProxyMode>('global');
const proxyType = ref('http');
const proxyHost = ref('');
const proxyPort = ref('');
const proxyUsername = ref('');
const proxyPassword = ref('');

// Refresh settings
const refreshMode = ref<RefreshMode>('global');
const refreshInterval = ref(30);

const isSubmitting = ref(false);
const showAdvancedSettings = ref(false);

// Available scripts from the scripts directory
const availableScripts = ref<Array<{ name: string; path: string; type: string }>>([]);
const scriptsDir = ref('');

// Get unique categories from existing feeds
const existingCategories = computed(() => {
  const categories = new Set<string>();
  store.feeds.forEach((feed) => {
    if (feed.category && feed.category.trim() !== '') {
      categories.add(feed.category);
    }
  });
  return Array.from(categories).sort();
});

// Watch for category selection changes
function handleCategoryChange() {
  if (categorySelection.value === '__custom__') {
    showCustomCategory.value = true;
    category.value = '';
  } else {
    showCustomCategory.value = false;
    category.value = categorySelection.value;
  }
}

// Modal close handling
useModalClose(() => close());

onMounted(async () => {
  title.value = props.feed.title;
  url.value = props.feed.url;
  category.value = props.feed.category;
  scriptPath.value = props.feed.script_path || '';
  hideFromTimeline.value = props.feed.hide_from_timeline || false;
  isImageMode.value = props.feed.is_image_mode || false;

  // Load image gallery enabled setting
  async function loadImageGallerySetting() {
    try {
      const res = await fetch('/api/settings');
      if (res.ok) {
        const data = await res.json();
        imageGalleryEnabled.value = data.image_gallery_enabled === 'true';
      }
    } catch (e) {
      console.error('Failed to load settings:', e);
    }
  }
  
  await loadImageGallerySetting();
  
  // Listen for settings changes
  window.addEventListener('image-gallery-setting-changed', (e: Event) => {
    const customEvent = e as CustomEvent;
    imageGalleryEnabled.value = customEvent.detail.enabled;
  });

  // Initialize proxy settings
  if (props.feed.proxy_url) {
    proxyMode.value = 'custom';
    // Parse proxy URL: protocol://[username:password@]host:port
    try {
      const proxyUrlObj = new URL(props.feed.proxy_url);
      proxyType.value = proxyUrlObj.protocol.replace(':', '');
      proxyHost.value = proxyUrlObj.hostname;
      proxyPort.value = proxyUrlObj.port;
      proxyUsername.value = proxyUrlObj.username;
      proxyPassword.value = proxyUrlObj.password;
    } catch (e) {
      // Fallback for invalid URL format
      console.error('Failed to parse proxy URL:', e);
      window.showToast(t('invalidProxyUrl'), 'error');
    }
  } else if (props.feed.proxy_enabled) {
    proxyMode.value = 'global';
  } else {
    proxyMode.value = 'none';
  }

  // Initialize refresh settings
  const interval = props.feed.refresh_interval || 0;
  if (interval === 0) {
    refreshMode.value = 'global';
  } else if (interval === -1) {
    refreshMode.value = 'intelligent';
  } else {
    refreshMode.value = 'custom';
    refreshInterval.value = interval;
  }

  // Initialize category selection
  if (category.value && existingCategories.value.includes(category.value)) {
    categorySelection.value = category.value;
  } else if (category.value) {
    // If category doesn't exist in list, show custom input
    showCustomCategory.value = true;
  }

  // Determine feed type based on whether it has a script path
  if (props.feed.script_path) {
    feedType.value = 'script';
  }

  await loadScripts();
});

async function loadScripts() {
  try {
    const res = await fetch('/api/scripts/list');
    if (res.ok) {
      const data = await res.json();
      availableScripts.value = data.scripts || [];
      scriptsDir.value = data.scripts_dir || '';
    }
  } catch (e) {
    console.error('Failed to load scripts:', e);
  }
}

function close() {
  emit('close');
}

const isFormValid = computed(() => {
  if (feedType.value === 'url') {
    return url.value.trim() !== '';
  } else {
    return scriptPath.value.trim() !== '';
  }
});

function buildProxyUrl(): string {
  if (proxyMode.value !== 'custom' || !proxyHost.value || !proxyPort.value) {
    return '';
  }

  let auth = '';
  if (proxyUsername.value) {
    auth = proxyPassword.value
      ? `${proxyUsername.value}:${proxyPassword.value}@`
      : `${proxyUsername.value}@`;
  }

  return `${proxyType.value}://${auth}${proxyHost.value}:${proxyPort.value}`;
}

function getRefreshInterval(): number {
  // Return 0 for global, -1 for intelligent, or the custom interval
  switch (refreshMode.value) {
    case 'global':
      return 0;
    case 'intelligent':
      return -1;
    case 'custom':
      return refreshInterval.value;
    default:
      return 0;
  }
}

async function save() {
  if (!isFormValid.value) return;
  isSubmitting.value = true;

  try {
    const body: Record<string, string | number | boolean> = {
      id: props.feed.id,
      title: title.value,
      category: category.value,
      hide_from_timeline: hideFromTimeline.value,
      is_image_mode: isImageMode.value,
      refresh_interval: getRefreshInterval(),
    };

    // Handle proxy settings
    if (proxyMode.value === 'custom') {
      body.proxy_enabled = true;
      body.proxy_url = buildProxyUrl();
    } else if (proxyMode.value === 'global') {
      body.proxy_enabled = true;
      body.proxy_url = '';
    } else {
      body.proxy_enabled = false;
      body.proxy_url = '';
    }

    if (feedType.value === 'url') {
      body.url = url.value;
      body.script_path = '';
    } else {
      body.url = scriptPath.value ? 'script://' + scriptPath.value : props.feed.url;
      body.script_path = scriptPath.value;
    }

    const res = await fetch('/api/feeds/update', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });

    if (res.ok) {
      emit('updated');
      window.showToast(t('feedUpdatedSuccess'), 'success');
      close();
    } else {
      window.showToast(t('errorUpdatingFeed'), 'error');
    }
  } catch (e) {
    console.error(e);
    window.showToast(t('errorUpdatingFeed'), 'error');
  } finally {
    isSubmitting.value = false;
  }
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
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 backdrop-blur-sm p-2 sm:p-4"
    data-modal-open="true"
    @click.self="close"
  >
    <div
      class="bg-bg-primary w-full max-w-md h-full sm:h-auto sm:max-h-[90vh] flex flex-col rounded-none sm:rounded-2xl shadow-2xl border border-border overflow-hidden animate-fade-in"
    >
      <div class="p-3 sm:p-5 border-b border-border flex justify-between items-center shrink-0">
        <h3 class="text-base sm:text-lg font-semibold m-0">{{ t('editFeed') }}</h3>
        <span
          class="text-2xl cursor-pointer text-text-secondary hover:text-text-primary"
          @click="close"
          >&times;</span
        >
      </div>
      <div class="flex-1 overflow-y-auto p-4 sm:p-6">
        <div class="mb-3 sm:mb-4">
          <label
            class="block mb-1 sm:mb-1.5 font-semibold text-xs sm:text-sm text-text-secondary"
            >{{ t('title') }}</label
          >
          <input v-model="title" type="text" class="input-field" />
        </div>

        <!-- URL Input (default mode) -->
        <div v-if="feedType === 'url'" class="mb-3 sm:mb-4">
          <label
            class="block mb-1 sm:mb-1.5 font-semibold text-xs sm:text-sm text-text-secondary"
            >{{ t('rssUrl') }}</label
          >
          <input v-model="url" type="text" class="input-field" />
          <div class="mt-2">
            <button
              type="button"
              class="text-xs sm:text-sm text-accent hover:underline"
              @click="feedType = 'script'"
            >
              {{ t('useCustomScript') }}
            </button>
          </div>
        </div>

        <!-- Script Selection (advanced mode) -->
        <div v-else class="mb-3 sm:mb-4">
          <label
            class="block mb-1 sm:mb-1.5 font-semibold text-xs sm:text-sm text-text-secondary"
            >{{ t('selectScript') }}</label
          >
          <div v-if="availableScripts.length > 0" class="mb-2">
            <select v-model="scriptPath" class="input-field">
              <option value="">{{ t('selectScriptPlaceholder') }}</option>
              <option v-for="script in availableScripts" :key="script.path" :value="script.path">
                {{ script.name }} ({{ script.type }})
              </option>
            </select>
          </div>
          <div
            v-else
            class="text-xs sm:text-sm text-text-secondary bg-bg-secondary rounded-md p-2 sm:p-3 border border-border"
          >
            <p class="mb-2">{{ t('noScriptsFound') }}</p>
          </div>
          <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between mt-2 gap-2">
            <button
              type="button"
              class="text-xs sm:text-sm text-accent hover:underline"
              @click="feedType = 'url'"
            >
              {{ t('useRssUrl') }}
            </button>
            <div class="flex flex-wrap items-center gap-2 sm:gap-3">
              <a
                href="https://github.com/WCY-dt/MrRSS/blob/main/docs/CUSTOM_SCRIPTS.md"
                target="_blank"
                rel="noopener noreferrer"
                class="text-xs sm:text-sm text-accent hover:underline flex items-center gap-1"
              >
                <PhBookOpen :size="14" />
                {{ t('scriptDocumentation') }}
              </a>
              <button
                type="button"
                class="text-xs sm:text-sm text-accent hover:underline flex items-center gap-1"
                @click="openScriptsFolder"
              >
                <PhCode :size="14" />
                {{ t('openScriptsFolder') }}
              </button>
            </div>
          </div>
        </div>

        <div class="mb-3 sm:mb-4">
          <label
            class="block mb-1 sm:mb-1.5 font-semibold text-xs sm:text-sm text-text-secondary"
            >{{ t('category') }}</label
          >
          <select
            v-if="!showCustomCategory"
            v-model="categorySelection"
            class="input-field w-full"
            @change="handleCategoryChange"
          >
            <option value="">{{ t('uncategorized') }}</option>
            <option v-for="cat in existingCategories" :key="cat" :value="cat">{{ cat }}</option>
            <option value="__custom__">{{ t('customCategory') }}</option>
          </select>
          <div v-else class="flex gap-2">
            <input
              v-model="category"
              type="text"
              :placeholder="t('enterCategoryName')"
              class="input-field flex-1"
              autofocus
            />
            <button
              type="button"
              class="px-3 py-2 text-xs sm:text-sm text-text-secondary hover:text-text-primary border border-border rounded-md hover:bg-bg-tertiary transition-colors"
              @click="
                showCustomCategory = false;
                categorySelection = category;
              "
            >
              {{ t('cancel') }}
            </button>
          </div>
        </div>

        <!-- Advanced Settings Toggle -->
        <div class="mb-3 sm:mb-4">
          <button
            type="button"
            class="flex items-center gap-1 text-xs sm:text-sm text-accent hover:text-accent-hover transition-colors"
            @click="showAdvancedSettings = !showAdvancedSettings"
          >
            <PhCaretRight
              v-if="!showAdvancedSettings"
              :size="12"
              class="transition-transform duration-200"
            />
            <PhCaretDown v-else :size="12" class="transition-transform duration-200" />
            <span class="hover:underline">
              {{ showAdvancedSettings ? t('hideAdvancedSettings') : t('showAdvancedSettings') }}
            </span>
          </button>
        </div>

        <!-- Advanced Settings Section (Collapsible) -->
        <div v-if="showAdvancedSettings" class="mb-3 sm:mb-4 space-y-3 sm:space-y-4">
          <!-- Image Mode Toggle (only shown if image gallery is enabled) -->
          <div v-if="imageGalleryEnabled" class="p-3 rounded-lg bg-bg-secondary border border-border">
            <label class="flex items-center justify-between cursor-pointer">
              <div>
                <span class="font-semibold text-xs sm:text-sm text-text-primary">{{
                  t('imageMode')
                }}</span>
                <p class="text-[10px] sm:text-xs text-text-secondary mt-0.5">
                  {{ t('imageModeDesc') }}
                </p>
              </div>
              <input v-model="isImageMode" type="checkbox" class="toggle" />
            </label>
          </div>

          <!-- Hide from Timeline Toggle -->
          <div class="p-3 rounded-lg bg-bg-secondary border border-border">
            <label class="flex items-center justify-between cursor-pointer">
              <div>
                <span class="font-semibold text-xs sm:text-sm text-text-primary">{{
                  t('hideFromTimeline')
                }}</span>
                <p class="text-[10px] sm:text-xs text-text-secondary mt-0.5">
                  {{ t('hideFromTimelineDesc') }}
                </p>
              </div>
              <input v-model="hideFromTimeline" type="checkbox" class="toggle" />
            </label>
          </div>

          <!-- Proxy Settings -->
          <div class="p-3 rounded-lg bg-bg-secondary border border-border space-y-3">
            <div>
              <label class="block mb-1.5 font-semibold text-xs sm:text-sm text-text-primary">
                {{ t('feedProxy') }}
              </label>
              <p class="text-[10px] sm:text-xs text-text-secondary mb-2">
                {{ t('feedProxyDesc') }}
              </p>
              <select v-model="proxyMode" class="input-field w-full">
                <option value="global">{{ t('useGlobalProxy') }}</option>
                <option value="custom">{{ t('useCustomProxy') }}</option>
                <option value="none">{{ t('noProxy') }}</option>
              </select>
            </div>

            <!-- Custom Proxy Configuration -->
            <div v-if="proxyMode === 'custom'" class="space-y-2.5 pl-3 border-l-2 border-accent/30">
              <!-- Proxy Type -->
              <div>
                <label class="block mb-1 text-[10px] sm:text-xs font-medium text-text-secondary">
                  {{ t('feedProxyType') }}
                </label>
                <select v-model="proxyType" class="input-field w-full text-xs sm:text-sm">
                  <option value="http">{{ t('httpProxy') }}</option>
                  <option value="https">{{ t('httpsProxy') }}</option>
                  <option value="socks5">{{ t('socks5Proxy') }}</option>
                </select>
              </div>

              <!-- Proxy Host and Port -->
              <div class="grid grid-cols-3 gap-2">
                <div class="col-span-2">
                  <label class="block mb-1 text-[10px] sm:text-xs font-medium text-text-secondary">
                    {{ t('feedProxyHost') }} <span class="text-red-500">*</span>
                  </label>
                  <input
                    v-model="proxyHost"
                    type="text"
                    :placeholder="t('proxyHostPlaceholder')"
                    :class="[
                      'input-field text-xs sm:text-sm',
                      proxyMode === 'custom' && !proxyHost.trim() ? 'border-red-500' : '',
                    ]"
                  />
                </div>
                <div>
                  <label class="block mb-1 text-[10px] sm:text-xs font-medium text-text-secondary">
                    {{ t('feedProxyPort') }} <span class="text-red-500">*</span>
                  </label>
                  <input
                    v-model="proxyPort"
                    type="text"
                    placeholder="8080"
                    :class="[
                      'input-field text-center text-xs sm:text-sm',
                      proxyMode === 'custom' && !proxyPort.trim() ? 'border-red-500' : '',
                    ]"
                  />
                </div>
              </div>

              <!-- Proxy Authentication (Optional) -->
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="block mb-1 text-[10px] sm:text-xs font-medium text-text-secondary">
                    {{ t('feedProxyUsername') }}
                  </label>
                  <input
                    v-model="proxyUsername"
                    type="text"
                    :placeholder="t('proxyUsernamePlaceholder')"
                    class="input-field text-xs sm:text-sm"
                  />
                </div>
                <div>
                  <label class="block mb-1 text-[10px] sm:text-xs font-medium text-text-secondary">
                    {{ t('feedProxyPassword') }}
                  </label>
                  <input
                    v-model="proxyPassword"
                    type="password"
                    :placeholder="t('proxyPasswordPlaceholder')"
                    class="input-field text-xs sm:text-sm"
                  />
                </div>
              </div>
            </div>
          </div>

          <!-- Refresh Settings -->
          <div class="p-3 rounded-lg bg-bg-secondary border border-border space-y-3">
            <div>
              <label class="block mb-1.5 font-semibold text-xs sm:text-sm text-text-primary">
                {{ t('feedRefreshMode') }}
              </label>
              <p class="text-[10px] sm:text-xs text-text-secondary mb-2">
                {{ t('feedRefreshModeDesc') }}
              </p>
              <select v-model="refreshMode" class="input-field w-full">
                <option value="global">{{ t('useGlobalRefresh') }}</option>
                <option value="intelligent">{{ t('useIntelligentInterval') }}</option>
                <option value="custom">{{ t('useCustomInterval') }}</option>
              </select>
            </div>

            <!-- Custom Refresh Interval -->
            <div v-if="refreshMode === 'custom'" class="pl-3 border-l-2 border-accent/30">
              <label class="block mb-1 text-[10px] sm:text-xs font-medium text-text-secondary">
                {{ t('feedRefreshInterval') }}
              </label>
              <div class="flex items-center gap-2">
                <input
                  v-model.number="refreshInterval"
                  type="number"
                  min="5"
                  max="1440"
                  :placeholder="t('feedRefreshIntervalPlaceholder')"
                  class="input-field flex-1 text-xs sm:text-sm"
                />
                <span class="text-xs text-text-secondary shrink-0">{{ t('minutesShort') }}</span>
              </div>
              <p class="text-[10px] text-text-secondary mt-1">
                {{ t('feedRefreshIntervalDesc') }}
              </p>
            </div>
          </div>
        </div>
      </div>
      <div class="p-3 sm:p-5 border-t border-border bg-bg-secondary text-right shrink-0">
        <button
          :disabled="isSubmitting || !isFormValid"
          class="btn-primary text-sm sm:text-base"
          @click="save"
        >
          {{ isSubmitting ? t('saving') : t('saveChanges') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../style.css";

.input-field {
  @apply w-full p-2 sm:p-2.5 border border-border rounded-md bg-bg-tertiary text-text-primary text-xs sm:text-sm focus:border-accent focus:outline-none transition-colors;
}
.btn-primary {
  @apply bg-accent text-white border-none px-4 sm:px-5 py-2 sm:py-2.5 rounded-lg cursor-pointer font-semibold hover:bg-accent-hover transition-colors disabled:opacity-70;
}
.toggle {
  @apply w-10 h-5 appearance-none bg-bg-tertiary rounded-full relative cursor-pointer border border-border transition-colors checked:bg-accent checked:border-accent shrink-0;
}
.toggle::after {
  content: '';
  @apply absolute top-0.5 left-0.5 w-3.5 h-3.5 bg-white rounded-full shadow-sm transition-transform;
}
.toggle:checked::after {
  transform: translateX(20px);
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
</style>
