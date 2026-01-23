<script setup lang="ts">
import { useI18n } from 'vue-i18n';

import type { ProxyMode, RefreshMode } from '@/composables/feed/useFeedForm';

interface Props {
  imageGalleryEnabled: boolean;
  isImageMode: boolean;
  hideFromTimeline: boolean;
  articleViewMode: 'global' | 'webpage' | 'rendered' | 'external';
  autoExpandContent: 'global' | 'enabled' | 'disabled';
  proxyMode: ProxyMode;
  proxyType: string;
  proxyHost: string;
  proxyPort: string;
  proxyUsername: string;
  proxyPassword: string;
  refreshMode: RefreshMode;
  refreshInterval: number;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:isImageMode': [value: boolean];
  'update:hideFromTimeline': [value: boolean];
  'update:articleViewMode': [value: 'global' | 'webpage' | 'rendered' | 'external'];
  'update:autoExpandContent': [value: 'global' | 'enabled' | 'disabled'];
  'update:proxyMode': [value: ProxyMode];
  'update:proxyType': [value: string];
  'update:proxyHost': [value: string];
  'update:proxyPort': [value: string];
  'update:proxyUsername': [value: string];
  'update:proxyPassword': [value: string];
  'update:refreshMode': [value: RefreshMode];
  'update:refreshInterval': [value: number];
}>();

const { t } = useI18n();
</script>

<template>
  <!-- Advanced Settings Section (Collapsible) -->
  <div class="mb-3 sm:mb-4 space-y-3 sm:space-y-4">
    <!-- Image Mode Toggle (only shown if image gallery is enabled) -->
    <div
      v-if="props.imageGalleryEnabled"
      class="p-3 rounded-lg bg-bg-secondary border border-border"
    >
      <label class="flex items-center justify-between cursor-pointer">
        <div>
          <span class="font-semibold text-xs sm:text-sm text-text-primary">{{
            t('setting.feed.imageMode')
          }}</span>
          <p class="text-[10px] sm:text-xs text-text-secondary mt-0.5">
            {{ t('setting.feed.imageModeDesc') }}
          </p>
        </div>
        <input
          :checked="props.isImageMode"
          type="checkbox"
          class="toggle"
          @change="emit('update:isImageMode', ($event.target as HTMLInputElement).checked)"
        />
      </label>
    </div>

    <!-- Hide from Timeline Toggle -->
    <div class="p-3 rounded-lg bg-bg-secondary border border-border">
      <label class="flex items-center justify-between cursor-pointer">
        <div>
          <span class="font-semibold text-xs sm:text-sm text-text-primary">{{
            t('setting.reading.hideFromTimeline')
          }}</span>
          <p class="text-[10px] sm:text-xs text-text-secondary mt-0.5">
            {{ t('setting.reading.hideFromTimelineDesc') }}
          </p>
        </div>
        <input
          :checked="props.hideFromTimeline"
          type="checkbox"
          class="toggle"
          @change="emit('update:hideFromTimeline', ($event.target as HTMLInputElement).checked)"
        />
      </label>
    </div>

    <!-- Article View Mode -->
    <div class="p-3 rounded-lg bg-bg-secondary border border-border">
      <label class="block mb-1.5 font-semibold text-xs sm:text-sm text-text-primary">
        {{ t('setting.feed.articleViewMode') }}
      </label>
      <p class="text-[10px] sm:text-xs text-text-secondary mb-2">
        {{ t('setting.feed.articleViewModeDesc') }}
      </p>
      <select
        :value="props.articleViewMode"
        class="input-field w-full"
        @change="
          emit(
            'update:articleViewMode',
            ($event.target as HTMLSelectElement).value as
              | 'global'
              | 'webpage'
              | 'rendered'
              | 'external'
          )
        "
      >
        <option value="global">{{ t('setting.feed.useGlobalSettings') }}</option>
        <option value="webpage">{{ t('setting.reading.viewAsWebpage') }}</option>
        <option value="rendered">{{ t('setting.reading.viewAsRendered') }}</option>
        <option value="external">{{ t('article.action.viewModeExternal') }}</option>
      </select>
    </div>

    <!-- Auto Expand Content -->
    <div class="p-3 rounded-lg bg-bg-secondary border border-border">
      <label class="block mb-1.5 font-semibold text-xs sm:text-sm text-text-primary">
        {{ t('setting.feed.autoExpandContent') }}
      </label>
      <p class="text-[10px] sm:text-xs text-text-secondary mb-2">
        {{ t('setting.feed.autoExpandContentDesc') }}
      </p>
      <select
        :value="props.autoExpandContent"
        class="input-field w-full"
        @change="
          emit(
            'update:autoExpandContent',
            ($event.target as HTMLSelectElement).value as 'global' | 'enabled' | 'disabled'
          )
        "
      >
        <option value="global">{{ t('setting.feed.useGlobalSettings') }}</option>
        <option value="enabled">{{ t('common.form.enabled') }}</option>
        <option value="disabled">{{ t('common.form.disabled') }}</option>
      </select>
    </div>

    <!-- Proxy Settings -->
    <div class="p-3 rounded-lg bg-bg-secondary border border-border space-y-3">
      <div>
        <label class="block mb-1.5 font-semibold text-xs sm:text-sm text-text-primary">
          {{ t('modal.feed.proxy') }}
        </label>
        <p class="text-[10px] sm:text-xs text-text-secondary mb-2">
          {{ t('modal.feed.proxyDesc') }}
        </p>
        <select
          :value="props.proxyMode"
          class="input-field w-full"
          @change="emit('update:proxyMode', ($event.target as HTMLSelectElement).value)"
        >
          <option value="global">{{ t('setting.network.useGlobalProxy') }}</option>
          <option value="custom">{{ t('setting.network.useCustomProxy') }}</option>
          <option value="none">{{ t('setting.network.noProxy') }}</option>
        </select>
      </div>

      <!-- Custom Proxy Configuration -->
      <div v-if="props.proxyMode === 'custom'" class="space-y-2.5 pl-3 border-l-2 border-accent/30">
        <!-- Proxy Type -->
        <div>
          <label class="block mb-1 text-[10px] sm:text-xs font-medium text-text-secondary">
            {{ t('modal.feed.proxyType') }}
          </label>
          <select
            :value="props.proxyType"
            class="input-field w-full text-xs sm:text-sm"
            @change="emit('update:proxyType', ($event.target as HTMLSelectElement).value)"
          >
            <option value="http">{{ t('setting.network.httpProxy') }}</option>
            <option value="https">{{ t('setting.network.httpsProxy') }}</option>
            <option value="socks5">{{ t('setting.network.socks5Proxy') }}</option>
          </select>
        </div>

        <!-- Proxy Host and Port -->
        <div class="grid grid-cols-3 gap-2">
          <div class="col-span-2">
            <label class="block mb-1 text-[10px] sm:text-xs font-medium text-text-secondary">
              {{ t('modal.feed.proxyHost') }} <span class="text-red-500">*</span>
            </label>
            <input
              :value="props.proxyHost"
              type="text"
              :placeholder="t('setting.network.proxyHostPlaceholder')"
              :class="[
                'input-field text-xs sm:text-sm',
                props.proxyMode === 'custom' && !props.proxyHost.trim() ? 'border-red-500' : '',
              ]"
              @input="emit('update:proxyHost', ($event.target as HTMLInputElement).value)"
            />
          </div>
          <div>
            <label class="block mb-1 text-[10px] sm:text-xs font-medium text-text-secondary">
              {{ t('modal.feed.proxyPort') }} <span class="text-red-500">*</span>
            </label>
            <input
              :value="props.proxyPort"
              type="text"
              placeholder="8080"
              :class="[
                'input-field text-center text-xs sm:text-sm',
                props.proxyMode === 'custom' && !props.proxyPort.trim() ? 'border-red-500' : '',
              ]"
              @input="emit('update:proxyPort', ($event.target as HTMLInputElement).value)"
            />
          </div>
        </div>

        <!-- Proxy Authentication (Optional) -->
        <div class="grid grid-cols-2 gap-2">
          <div>
            <label class="block mb-1 text-[10px] sm:text-xs font-medium text-text-secondary">
              {{ t('modal.feed.proxyUsername') }}
            </label>
            <input
              :value="props.proxyUsername"
              type="text"
              :placeholder="t('setting.network.proxyUsernamePlaceholder')"
              class="input-field text-xs sm:text-sm"
              @input="emit('update:proxyUsername', ($event.target as HTMLInputElement).value)"
            />
          </div>
          <div>
            <label class="block mb-1 text-[10px] sm:text-xs font-medium text-text-secondary">
              {{ t('modal.feed.proxyPassword') }}
            </label>
            <input
              :value="props.proxyPassword"
              type="password"
              :placeholder="t('setting.network.proxyPasswordPlaceholder')"
              class="input-field text-xs sm:text-sm"
              @input="emit('update:proxyPassword', ($event.target as HTMLInputElement).value)"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- Refresh Settings -->
    <div class="p-3 rounded-lg bg-bg-secondary border border-border space-y-3">
      <div>
        <label class="block mb-1.5 font-semibold text-xs sm:text-sm text-text-primary">
          {{ t('modal.feed.refreshMode') }}
        </label>
        <p class="text-[10px] sm:text-xs text-text-secondary mb-2">
          {{ t('modal.feed.refreshModeDesc') }}
        </p>
        <select
          :value="props.refreshMode"
          class="input-field w-full"
          @change="emit('update:refreshMode', ($event.target as HTMLSelectElement).value)"
        >
          <option value="global">{{ t('setting.feed.useGlobalRefresh') }}</option>
          <option value="intelligent">{{ t('setting.feed.useIntelligentInterval') }}</option>
          <option value="custom">{{ t('setting.feed.useCustomInterval') }}</option>
          <option value="never">{{ t('setting.feed.neverRefresh') }}</option>
        </select>
      </div>

      <!-- Custom Refresh Interval -->
      <div v-if="props.refreshMode === 'custom'" class="pl-3 border-l-2 border-accent/30">
        <label class="block mb-1 text-[10px] sm:text-xs font-medium text-text-secondary">
          {{ t('modal.feed.refreshInterval') }}
        </label>
        <div class="flex items-center gap-2">
          <input
            :value="props.refreshInterval"
            type="number"
            min="5"
            max="1440"
            :placeholder="t('modal.feed.refreshIntervalPlaceholder')"
            class="input-field flex-1 text-xs sm:text-sm"
            @input="
              emit(
                'update:refreshInterval',
                parseInt(($event.target as HTMLInputElement).value) || 0
              )
            "
          />
          <span class="text-xs text-text-secondary shrink-0">{{
            t('common.time.minutesShort')
          }}</span>
        </div>
        <p class="text-[10px] text-text-secondary mt-1">
          {{ t('modal.feed.refreshIntervalDesc') }}
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../style.css";

.input-field {
  @apply w-full p-2 sm:p-2.5 border border-border rounded-md bg-bg-tertiary text-text-primary text-xs sm:text-sm focus:border-accent focus:outline-none transition-colors;
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
</style>
