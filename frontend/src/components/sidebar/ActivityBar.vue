<script setup lang="ts">
import {
  PhListDashes,
  PhSquaresFour,
  PhTray,
  PhStar,
  PhClockCountdown,
  PhImages,
  PhPlus,
  PhGear,
  PhTextOutdent,
  PhSidebar,
  PhUsers,
  PhSignOut,
  PhSparkle,
} from '@phosphor-icons/vue';
import { useAuthStore } from '@/stores/auth';
import { computed, ref, onMounted, onBeforeUnmount } from 'vue';
import { useI18n } from 'vue-i18n';
import { useArticleFilter } from '@/features/article/composables/useArticleFilter';
import { authFetchJson } from '@/shared/lib/authFetch';
import { isAIEnhancedModeEffectivelyEnabled, isEnabledSetting } from '@/shared/lib/aiEnhancedMode';
import LogoSvg from '../../../public/assets/logo.svg';
import { useArticleStore } from '@/features/article/store';

const articleStore = useArticleStore();
const authStore = useAuthStore();
const { t } = useI18n();
const { clearAllFilters } = useArticleFilter();

const isAdmin = computed(() => authStore.user?.role === 'admin');
const showLogoutConfirm = ref(false);

function handleLogoutConfirm() {
  showLogoutConfirm.value = false;
  authStore.logout();
  window.location.replace(window.location.href);
}

function cancelLogout() {
  showLogoutConfirm.value = false;
}

interface Props {
  isCollapsed?: boolean;
  isMobile?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  isCollapsed: false,
  isMobile: false,
});

const emit = defineEmits<{
  'select-filter': [filterType: string];
  'add-feed': [];
  settings: [];
  'toggle-feed-drawer': [];
  ready: [{ expanded: boolean; pinned: boolean }];
  'toggle-activity-bar': [];
  'open-user-management': [];
}>();

interface NavItem {
  id: string;
  icon: any;
  label: string;
  activeIcon?: any;
  filterType:
    | 'all'
    | 'unread'
    | 'favorites'
    | 'readLater'
    | 'imageGallery'
    | 'dailyRecommendations';
}

const navItems = computed<NavItem[]>(() => [
  {
    id: 'all',
    icon: PhListDashes,
    activeIcon: PhSquaresFour,
    label: t('sidebar.activity.allArticles'),
    filterType: 'all',
  },
  {
    id: 'unread',
    icon: PhTray,
    label: t('sidebar.feedList.unread'),
    filterType: 'unread',
  },
  {
    id: 'favorites',
    icon: PhStar,
    label: t('sidebar.activity.favorites'),
    filterType: 'favorites',
  },
  {
    id: 'readLater',
    icon: PhClockCountdown,
    label: t('sidebar.activity.readLater'),
    filterType: 'readLater',
  },
  {
    id: 'imageGallery',
    icon: PhImages,
    label: t('sidebar.activity.imageGallery'),
    filterType: 'imageGallery',
  },
  {
    id: 'dailyRecommendations',
    icon: PhSparkle,
    activeIcon: PhSparkle,
    label: t('sidebar.activity.dailyRecommendations'),
    filterType: 'dailyRecommendations',
  },
]);

const imageGalleryEnabled = ref(false);
const aiRecommendationEnabled = ref(false);

async function loadFeatureSettings() {
  try {
    const data = await authFetchJson<any>('/api/settings');
    imageGalleryEnabled.value = isEnabledSetting(data.image_gallery_enabled);
    aiRecommendationEnabled.value = isEnabledSetting(data.ai_recommendation_enabled);
    articleStore.setAIEnhancedMode(isAIEnhancedModeEffectivelyEnabled(data));
  } catch (e) {
    console.error('Failed to load settings:', e);
  }
}

const savedPinnedState = localStorage.getItem('FeedListPinned');
const savedExpandedState = localStorage.getItem('FeedListExpanded');

const isFeedListPinned = ref(savedPinnedState === 'true' || savedPinnedState === null);
const isFeedListExpanded = ref(savedExpandedState === 'true' || savedExpandedState === null);

function saveDrawerState() {
  localStorage.setItem('FeedListPinned', String(isFeedListPinned.value));
  localStorage.setItem('FeedListExpanded', String(isFeedListExpanded.value));
}

function loadDrawerState() {
  const pinned = localStorage.getItem('FeedListPinned');
  const expanded = localStorage.getItem('FeedListExpanded');
  isFeedListPinned.value = pinned === 'true' || pinned === null;
  isFeedListExpanded.value = expanded === 'true' || expanded === null;
}

onMounted(async () => {
  if (authStore.isAuthenticated) {
    await loadFeatureSettings();
  }
  loadDrawerState();

  emit('ready', {
    expanded: isFeedListExpanded.value,
    pinned: isFeedListPinned.value,
  });

  window.addEventListener('image-gallery-setting-changed', handleImageGallerySettingChanged);
  window.addEventListener('ai-recommendation-setting-changed', handleAIRecommendationSettingChanged);
  window.addEventListener('settings-updated', handleSettingsUpdated);
});

onBeforeUnmount(() => {
  window.removeEventListener('image-gallery-setting-changed', handleImageGallerySettingChanged);
  window.removeEventListener(
    'ai-recommendation-setting-changed',
    handleAIRecommendationSettingChanged
  );
  window.removeEventListener('settings-updated', handleSettingsUpdated);
});

function handleImageGallerySettingChanged(e: Event) {
  const customEvent = e as CustomEvent<{ enabled: boolean }>;
  imageGalleryEnabled.value = customEvent.detail.enabled;
}

function handleAIRecommendationSettingChanged(e: Event) {
  const customEvent = e as CustomEvent<{ enabled: boolean }>;
  aiRecommendationEnabled.value = customEvent.detail.enabled;
}

function handleSettingsUpdated() {
  if (!authStore.isAuthenticated) {
    return;
  }
  loadFeatureSettings();
}

function handleNavClick(item: NavItem) {
  clearAllFilters();
  articleStore.setFilter(item.filterType);
  emit('select-filter', item.filterType);
}

function toggleFeedList() {
  isFeedListExpanded.value = !isFeedListExpanded.value;
  saveDrawerState();
  emit('toggle-feed-drawer');
}

function pinFeedList() {
  isFeedListPinned.value = true;
  isFeedListExpanded.value = true;
  saveDrawerState();
  emit('toggle-feed-drawer');
}

function unpinFeedList() {
  isFeedListPinned.value = false;
  saveDrawerState();
  emit('toggle-feed-drawer');
}

function handleFeedListStateChange(expanded: boolean, pinned?: boolean) {
  isFeedListExpanded.value = expanded;
  if (pinned !== undefined) {
    isFeedListPinned.value = pinned;
  }
  saveDrawerState();
}

function isVisible(item: NavItem) {
  if (item.id === 'imageGallery') {
    return imageGalleryEnabled.value;
  }
  if (item.id === 'dailyRecommendations') {
    return aiRecommendationEnabled.value;
  }
  return true;
}

defineExpose({
  toggleFeedList,
  pinFeedList,
  unpinFeedList,
  handleFeedListStateChange,
  loadDrawerState,
  get isFeedListExpanded() {
    return isFeedListExpanded.value;
  },
  get isFeedListPinned() {
    return isFeedListPinned.value;
  },
});
</script>

<template>
  <Transition name="activity-bar-slide">
    <div
      v-if="!props.isCollapsed"
      class="smart-activity-bar flex flex-col items-center py-3 bg-bg-tertiary border-r border-border h-full select-none shrink-0 relative z-30"
    >
      <div class="mb-6">
        <img :src="LogoSvg" alt="MavenRSS" class="w-6 h-6" />
      </div>

      <div class="w-8 h-px bg-border mb-3"></div>

      <div
        class="flex-1 flex flex-col items-center gap-1 w-full overflow-y-auto overflow-x-hidden nav-items-container"
      >
        <TransitionGroup name="nav-item">
          <button
            v-for="item in navItems"
            v-show="isVisible(item)"
            :key="item.id"
            :class="[
              'relative flex items-center justify-center text-text-secondary flex-shrink-0 transition-all hover:text-accent',
              articleStore.currentFilter === item.filterType ? 'text-accent' : '',
            ]"
            style="width: 44px; height: 44px"
            :title="item.label"
            @click="handleNavClick(item)"
          >
            <component
              :is="
                articleStore.currentFilter === item.filterType
                  ? item.activeIcon || item.icon
                  : item.icon
              "
              :size="24"
              :weight="articleStore.currentFilter === item.filterType ? 'fill' : 'regular'"
              :class="[
                articleStore.currentFilter === item.filterType ? 'text-accent scale-105' : '',
                'transition-all',
              ]"
            />

            <span
              v-if="item.id === 'all' && articleStore.unreadCounts?.total > 0"
              class="absolute bottom-0.5 right-0.5 min-w-[14px] h-[14px] px-0.5 text-[9px] font-medium flex items-center justify-center rounded-full text-white"
              style="background-color: #999999"
            >
              {{ articleStore.unreadCounts?.total > 99 ? '99+' : articleStore.unreadCounts?.total }}
            </span>
          </button>
        </TransitionGroup>
      </div>

      <div class="flex flex-col items-center gap-1 mt-auto w-full">
        <button
          class="w-11 h-11 flex items-center justify-center text-text-secondary hover:text-accent transition-colors"
          :title="t('sidebar.feedList.toggleFeedDrawer')"
          @click="emit('toggle-feed-drawer')"
        >
          <PhTextOutdent :size="24" />
        </button>

        <button
          class="w-11 h-11 flex items-center justify-center text-text-secondary hover:text-accent transition-colors"
          :title="t('sidebar.activity.addFeed')"
          @click="emit('add-feed')"
        >
          <PhPlus :size="24" />
        </button>

        <button
          v-if="isAdmin"
          class="w-11 h-11 flex items-center justify-center text-text-secondary hover:text-accent transition-colors"
          :title="t('admin.title')"
          @click="emit('open-user-management')"
        >
          <PhUsers :size="24" />
        </button>

        <button
          class="w-11 h-11 flex items-center justify-center text-text-secondary hover:text-accent transition-colors"
          :title="t('sidebar.activity.settings')"
          @click="emit('settings')"
        >
          <PhGear :size="24" />
        </button>

        <button
          class="w-11 h-11 flex items-center justify-center text-text-secondary hover:text-red-500 transition-colors"
          :title="t('common.logout')"
          @click="showLogoutConfirm = true"
        >
          <PhSignOut :size="24" />
        </button>

        <button
          class="md:hidden w-11 h-11 flex items-center justify-center text-text-secondary hover:text-accent transition-colors"
          :title="t('sidebar.activity.hideSidebar')"
          @click="emit('toggle-activity-bar')"
        >
          <PhSidebar :size="24" />
        </button>
      </div>

      <div
        v-if="showLogoutConfirm"
        class="fixed inset-0 z-[100] flex items-center justify-center bg-black/50 px-4"
      >
        <div class="w-full max-w-sm rounded-xl border border-border bg-bg-primary p-5 shadow-xl">
          <h3 class="text-base font-semibold text-text-primary mb-2">
            {{ t('common.logout') }}
          </h3>
          <p class="text-sm text-text-secondary mb-5">
            {{ t('common.logoutConfirm') }}
          </p>
          <div class="flex justify-end gap-3">
            <button class="btn-secondary" @click="cancelLogout">
              {{ t('common.cancel') }}
            </button>
            <button class="btn-danger" @click="handleLogoutConfirm">
              {{ t('common.confirm') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
@reference "../../style.css";
</style>
