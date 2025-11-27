<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { PhCode, PhBookOpen } from '@phosphor-icons/vue';
import { useModalClose } from '@/composables/ui/useModalClose';

const { t } = useI18n();

type FeedType = 'url' | 'script';

const feedType = ref<FeedType>('url');
const title = ref('');
const url = ref('');
const category = ref('');
const scriptPath = ref('');
const isSubmitting = ref(false);

// Available scripts from the scripts directory
const availableScripts = ref<Array<{ name: string; path: string; type: string }>>([]);
const scriptsDir = ref('');

const emit = defineEmits<{
  close: [];
  added: [];
}>();

// Modal close handling
useModalClose(() => close());

onMounted(async () => {
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

async function addFeed() {
  if (!isFormValid.value) return;
  isSubmitting.value = true;

  try {
    const body: Record<string, string> = {
      category: category.value,
      title: title.value,
    };

    if (feedType.value === 'url') {
      body.url = url.value;
    } else {
      body.script_path = scriptPath.value;
    }

    const res = await fetch('/api/feeds/add', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });

    if (res.ok) {
      emit('added');
      title.value = '';
      url.value = '';
      category.value = '';
      scriptPath.value = '';
      window.showToast(t('feedAddedSuccess'), 'success');
      close();
    } else {
      window.showToast(t('errorAddingFeed'), 'error');
    }
  } catch (e) {
    console.error(e);
    window.showToast(t('errorAddingFeed'), 'error');
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
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 backdrop-blur-sm p-4"
    @click.self="close"
    data-modal-open="true"
  >
    <div
      class="bg-bg-primary w-full max-w-md rounded-2xl shadow-2xl border border-border overflow-hidden animate-fade-in"
    >
      <div class="p-5 border-b border-border flex justify-between items-center">
        <h3 class="text-lg font-semibold m-0">{{ t('addNewFeed') }}</h3>
        <span
          @click="close"
          class="text-2xl cursor-pointer text-text-secondary hover:text-text-primary"
          >&times;</span
        >
      </div>
      <div class="p-6">
        <div class="mb-4">
          <label class="block mb-1.5 font-semibold text-sm text-text-secondary"
            >{{ t('title') }} ({{ t('optional') }})</label
          >
          <input
            v-model="title"
            type="text"
            :placeholder="t('titlePlaceholder')"
            class="input-field"
          />
        </div>

        <!-- URL Input (default mode) -->
        <div v-if="feedType === 'url'" class="mb-4">
          <label class="block mb-1.5 font-semibold text-sm text-text-secondary">{{
            t('rssUrl')
          }}</label>
          <input
            v-model="url"
            type="text"
            :placeholder="t('rssUrlPlaceholder')"
            class="input-field"
          />
          <div class="mt-2">
            <button
              type="button"
              @click="feedType = 'script'"
              class="text-sm text-accent hover:underline"
            >
              {{ t('useCustomScript') }}
            </button>
          </div>
        </div>

        <!-- Script Selection (advanced mode) -->
        <div v-else class="mb-4">
          <label class="block mb-1.5 font-semibold text-sm text-text-secondary">{{
            t('selectScript')
          }}</label>
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
            class="text-sm text-text-secondary bg-bg-secondary rounded-md p-3 border border-border"
          >
            <p class="mb-2">{{ t('noScriptsFound') }}</p>
          </div>
          <div class="flex items-center justify-between mt-2">
            <button
              type="button"
              @click="feedType = 'url'"
              class="text-sm text-accent hover:underline"
            >
              {{ t('useRssUrl') }}
            </button>
            <div class="flex items-center gap-3">
              <a
                href="https://github.com/WCY-dt/MrRSS/blob/main/docs/CUSTOM_SCRIPTS.md"
                target="_blank"
                rel="noopener noreferrer"
                class="text-sm text-accent hover:underline flex items-center gap-1"
              >
                <PhBookOpen :size="14" />
                {{ t('scriptDocumentation') }}
              </a>
              <button
                type="button"
                @click="openScriptsFolder"
                class="text-sm text-accent hover:underline flex items-center gap-1"
              >
                <PhCode :size="14" />
                {{ t('openScriptsFolder') }}
              </button>
            </div>
          </div>
        </div>

        <div class="mb-4">
          <label class="block mb-1.5 font-semibold text-sm text-text-secondary">{{
            t('categoryOptional')
          }}</label>
          <input
            v-model="category"
            type="text"
            :placeholder="t('categoryPlaceholder')"
            class="input-field"
          />
        </div>
      </div>
      <div class="p-5 border-t border-border bg-bg-secondary text-right">
        <button @click="addFeed" :disabled="isSubmitting || !isFormValid" class="btn-primary">
          {{ isSubmitting ? t('adding') : t('addSubscription') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.input-field {
  @apply w-full p-2.5 border border-border rounded-md bg-bg-secondary text-text-primary text-sm focus:border-accent focus:outline-none transition-colors;
}
.btn-primary {
  @apply bg-accent text-white border-none px-5 py-2.5 rounded-lg cursor-pointer font-semibold hover:bg-accent-hover transition-colors disabled:opacity-70;
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
