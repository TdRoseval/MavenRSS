<script setup lang="ts">
import { useAppStore } from '@/stores/app';
import { useI18n } from 'vue-i18n';
import { PhPlus, PhGear, PhMagnifyingGlass, PhX } from '@phosphor-icons/vue';
import { useSidebar } from '@/composables/core/useSidebar';
import SidebarNavItem from './SidebarNavItem.vue';
import SidebarCategory from './SidebarCategory.vue';

const store = useAppStore();
const { t } = useI18n();

interface Props {
  isOpen?: boolean;
}

defineProps<Props>();

const emit = defineEmits<{
  toggle: [];
}>();

const {
  tree,
  categoryUnreadCounts,
  toggleCategory,
  isCategoryOpen,
  searchQuery,
  onFeedContextMenu,
  onCategoryContextMenu,
} = useSidebar();

const emitShowAddFeed = () => window.dispatchEvent(new CustomEvent('show-add-feed'));
const emitShowSettings = () => window.dispatchEvent(new CustomEvent('show-settings'));
</script>

<template>
  <aside
    :class="[
      'sidebar flex flex-col bg-bg-secondary border-r border-border h-full transition-transform duration-300 absolute z-20 md:relative md:translate-x-0',
      isOpen ? 'translate-x-0' : '-translate-x-full',
    ]"
  >
    <div class="p-3 sm:p-5 border-b border-border flex justify-between items-center">
      <h2 class="m-0 text-base sm:text-lg font-bold flex items-center gap-1.5 sm:gap-2 text-accent">
        <img src="/assets/logo.svg" alt="Logo" class="h-6 sm:h-7 w-auto" />
        <span class="xs:inline">{{ t('appName') }}</span>
      </h2>
    </div>

    <nav class="p-2 sm:p-3 space-y-1">
      <SidebarNavItem
        :label="t('allArticles')"
        :is-active="store.currentFilter === 'all'"
        icon="all"
        :unread-count="store.unreadCounts.total"
        @click="store.setFilter('all')"
      />
      <SidebarNavItem
        :label="t('unread')"
        :is-active="store.currentFilter === 'unread'"
        icon="unread"
        @click="store.setFilter('unread')"
      />
      <SidebarNavItem
        :label="t('favorites')"
        :is-active="store.currentFilter === 'favorites'"
        icon="favorites"
        @click="store.setFilter('favorites')"
      />
    </nav>

    <!-- Search Box (kept outside scrollable list so it doesn't scroll) -->
    <div class="px-2 sm:px-3 pt-2 border-t border-border bg-bg-secondary z-10">
      <div class="mb-3">
        <div
          class="flex items-center bg-bg-secondary border border-border rounded-lg px-3 py-2 focus-within:border-accent transition-colors"
        >
          <PhMagnifyingGlass :size="18" class="text-text-secondary mr-2 flex-shrink-0" />
          <input
            v-model="searchQuery"
            type="text"
            :placeholder="t('searchFeeds')"
            class="w-full bg-transparent border-none outline-none text-text-primary text-sm placeholder-text-secondary"
          />
          <button
            v-if="searchQuery"
            @click="searchQuery = ''"
            class="ml-2 p-0.5 text-text-secondary hover:text-text-primary hover:bg-bg-tertiary rounded transition-colors flex-shrink-0"
            :title="t('clear')"
          >
            <PhX :size="16" />
          </button>
        </div>
      </div>
    </div>

    <div class="flex-1 overflow-y-auto p-1.5 sm:p-2">
      <!-- Categories -->
      <SidebarCategory
        v-for="(data, name) in tree.tree"
        :key="name"
        :name="name"
        :feeds="data._feeds"
        :is-open="isCategoryOpen(name)"
        :is-active="store.currentCategory === name"
        :unread-count="categoryUnreadCounts[name] || 0"
        :current-feed-id="store.currentFeedId"
        :feed-unread-counts="store.unreadCounts.feedCounts"
        @toggle="toggleCategory(name)"
        @select-category="store.setCategory(name)"
        @select-feed="store.setFeed"
        @category-context-menu="(e) => onCategoryContextMenu(e, name)"
        @feed-context-menu="onFeedContextMenu"
      />

      <!-- Uncategorized -->
      <SidebarCategory
        v-if="tree.uncategorized.length > 0"
        :name="t('uncategorized')"
        :feeds="tree.uncategorized"
        :is-open="isCategoryOpen('uncategorized')"
        :is-active="false"
        :is-uncategorized="true"
        :unread-count="categoryUnreadCounts['uncategorized'] || 0"
        :current-feed-id="store.currentFeedId"
        :feed-unread-counts="store.unreadCounts.feedCounts"
        @toggle="toggleCategory('uncategorized')"
        @select-feed="store.setFeed"
        @category-context-menu="(e) => onCategoryContextMenu(e, 'uncategorized')"
        @feed-context-menu="onFeedContextMenu"
      />
    </div>

    <div class="p-2 sm:p-4 border-t border-border flex gap-1.5 sm:gap-2">
      <button @click="emitShowAddFeed" class="footer-btn" :title="t('addFeed')">
        <PhPlus :size="18" class="sm:w-5 sm:h-5" />
      </button>
      <button @click="emitShowSettings" class="footer-btn" :title="t('settings')">
        <PhGear :size="18" class="sm:w-5 sm:h-5" />
      </button>
    </div>
  </aside>
  <!-- Overlay for mobile -->
  <div v-if="isOpen" @click="emit('toggle')" class="fixed inset-0 bg-black/50 z-10 md:hidden"></div>
</template>

<style scoped>
.sidebar {
  width: 16rem;
}
@media (min-width: 768px) {
  .sidebar {
    width: var(--sidebar-width, 16rem);
  }
}
.footer-btn {
  @apply flex-1 flex items-center justify-center gap-2 p-2 sm:p-2.5 text-text-secondary rounded-lg text-lg sm:text-xl hover:bg-bg-tertiary hover:text-text-primary transition-colors;
}
</style>
