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
} from '@phosphor-icons/vue';
import { useAuthStore } from '@/stores/auth';
import { computed } from 'vue';
import { ref, onMounted } from 'vue';
import { useAppStore } from '@/stores/app';
import { useI18n } from 'vue-i18n';
import { useArticleFilter } from '@/composables/article/useArticleFilter';
import { authFetchJson } from '@/utils/authFetch';
import LogoSvg from '../../../public/assets/logo.svg';

const store = useAppStore();
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
  filterType: 'all' | 'unread' | 'favorites' | 'readLater' | 'imageGallery';
}

const navItems: NavItem[] = [
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
];

const imageGalleryEnabled = ref(false);

async function loadImageGallerySetting() {
  try {
    const data = await authFetchJson<any>('/api/settings');
    imageGalleryEnabled.value = data.image_gallery_enabled === 'true';
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
    await loadImageGallerySetting();
  }
  loadDrawerState();

  emit('ready', {
    expanded: isFeedListExpanded.value,
    pinned: isFeedListPinned.value,
  });

  window.addEventListener('image-gallery-setting-changed', (e: Event) => {
    const customEvent = e as CustomEvent;
    imageGalleryEnabled.value = customEvent.detail.enabled;
  });
});

function handleNavClick(item: NavItem) {
  clearAllFilters();
  store.setFilter(item.filterType);
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
        <img :src="LogoSvg" alt="MrRSS" class="w-6 h-6" />
      </div>

      <div class="w-8 h-px bg-border mb-3"></div>

      <div
        class="flex-1 flex flex-col items-center gap-1 w-full overflow-y-auto overflow-x-hidden nav-items-container"
      >
        <TransitionGroup name="nav-item">
          <button
            v-for="item in navItems"
            v-show="item.id !== 'imageGallery' || imageGalleryEnabled"
            :key="item.id"
            :class="[
              'relative flex items-center justify-center text-text-secondary flex-shrink-0 transition-all hover:text-accent',
              store.currentFilter === item.filterType ? 'text-accent' : '',
            ]"
            style="width: 44px; height: 44px"
            :title="item.label"
            @click="handleNavClick(item)"
          >
            <component
              :is="
                store.currentFilter === item.filterType ? item.activeIcon || item.icon : item.icon
              "
              :size="24"
              :weight="store.currentFilter === item.filterType ? 'fill' : 'regular'"
              :class="[
                store.currentFilter === item.filterType ? 'text-accent scale-105' : '',
                'transition-all',
              ]"
            />

            <span
              v-if="item.id === 'all' && store.unreadCounts?.total > 0"
              class="absolute bottom-0.5 right-0.5 min-w-[14px] h-[14px] px-0.5 text-[9px] font-medium flex items-center justify-center rounded-full text-white"
              style="background-color: #999999"
            >
              {{ store.unreadCounts?.total > 99 ? '99+' : store.unreadCounts?.total }}
            </span>
          </button>
        </TransitionGroup>
      </div>

      <div class="flex flex-col items-center gap-1 mt-auto w-full">
        <button
          class="relative flex items-center justify-center text-text-secondary flex-shrink-0 transition-all hover:text-accent"
          style="width: 44px; height: 44px"
          :title="t('sidebar.activity.addFeed')"
          @click="emit('add-feed')"
        >
          <PhPlus :size="24" weight="regular" class="transition-all" />
        </button>

        <button
          class="relative flex items-center justify-center text-text-secondary flex-shrink-0 transition-all hover:text-accent"
          style="width: 44px; height: 44px"
          :title="
            isFeedListExpanded
              ? t('sidebar.activity.collapseFeedList')
              : t('sidebar.activity.expandFeedList')
          "
          @click="toggleFeedList"
        >
          <PhSidebar :size="24" :weight="isFeedListExpanded ? 'fill' : 'regular'" />
        </button>

        <button
          class="relative flex items-center justify-center text-text-secondary flex-shrink-0 transition-all hover:text-accent"
          style="width: 44px; height: 44px"
          :title="t('setting.tab.settings')"
          @click="emit('settings')"
        >
          <PhGear :size="24" weight="regular" class="transition-all" />
        </button>

        <button
          v-if="isAdmin"
          class="relative flex items-center justify-center text-text-secondary flex-shrink-0 transition-all hover:text-accent"
          style="width: 44px; height: 44px"
          :title="t('admin.title')"
          @click="emit('open-user-management')"
        >
          <PhUsers :size="24" weight="regular" class="transition-all" />
        </button>

        <button
          type="button"
          class="relative flex items-center justify-center text-text-secondary flex-shrink-0 transition-all hover:text-accent"
          style="width: 44px; height: 44px"
          :title="t('admin.logout')"
          @click="showLogoutConfirm = true"
        >
          <PhSignOut :size="24" weight="regular" class="transition-all" />
        </button>

        <div class="w-8 h-px bg-border my-2"></div>

        <button
          v-if="false && !isMobile"
          class="relative flex items-center justify-center text-text-secondary flex-shrink-0 transition-all hover:text-accent"
          style="width: 44px; height: 44px"
          :title="t('sidebar.activity.collapseActivityBar')"
          @click="emit('toggle-activity-bar')"
        >
          <PhTextOutdent :size="24" weight="regular" class="transition-all" />
        </button>
      </div>
    </div>
  </Transition>

  <Teleport to="body">
    <div v-if="showLogoutConfirm" class="logout-modal-overlay" @click="cancelLogout">
      <div class="logout-modal" @click.stop>
        <h3>{{ t('admin.logout') }}</h3>
        <p>{{ t('admin.confirmLogout') }}</p>
        <div class="logout-modal-buttons">
          <button class="btn-cancel" @click="cancelLogout">{{ t('admin.cancel') }}</button>
          <button class="btn-confirm" @click="handleLogoutConfirm">{{ t('admin.logout') }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
@reference "../../style.css";

.activity-bar-slide-enter-active {
  transition:
    transform 0.25s cubic-bezier(0.4, 0, 0.2, 1),
    opacity 0.2s ease;
  will-change: transform, opacity;
}

.activity-bar-slide-leave-active {
  transition:
    transform 0.2s cubic-bezier(0.4, 0, 0.2, 1),
    opacity 0.18s ease;
  will-change: transform, opacity;
}

.activity-bar-slide-enter-from {
  opacity: 0;
  transform: translateX(-12px);
}

.activity-bar-slide-leave-to {
  opacity: 0;
  transform: translateX(-12px);
}

.activity-bar-slide-enter-to,
.activity-bar-slide-leave-from {
  opacity: 1;
  transform: translateX(0);
}

.smart-activity-bar {
  width: 56px;
  min-width: 56px;
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  z-index: 15;
  backface-visibility: hidden;
  -webkit-font-smoothing: antialiased;
}

@media (max-width: 767px) {
  .smart-activity-bar {
    z-index: 60;
  }
}

.nav-items-container {
  transition: height 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.nav-item-enter-active,
.nav-item-leave-active {
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  will-change: opacity, transform;
}

.nav-item-enter-from {
  opacity: 0;
  transform: scale(0.9) translateY(-10px);
}

.nav-item-leave-to {
  opacity: 0;
  transform: scale(0.9) translateY(10px);
}

.nav-item-move {
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  will-change: transform;
}

.smart-activity-bar button .ph,
.smart-activity-bar button svg {
  transition:
    transform 0.2s cubic-bezier(0.4, 0, 0.2, 1),
    color 0.2s ease;
  will-change: transform;
}

.smart-activity-bar button {
  transition:
    color 0.2s ease,
    background-color 0.2s ease;
  will-change: color, background-color;
}

@media (max-width: 1400px) {
  .smart-activity-bar {
    width: 48px;
    min-width: 48px;
  }

  button[style*='width: 44px'] {
    width: 40px !important;
    height: 40px !important;
  }
}

@media (max-width: 767px) {
  .smart-activity-bar {
    width: 44px;
    min-width: 44px;
  }

  button[style*='width: 44px'] {
    width: 36px !important;
    height: 36px !important;
  }
}
</style>

<style>
.logout-modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
}

.logout-modal {
  background: white;
  padding: 24px;
  border-radius: 8px;
  min-width: 300px;
  text-align: center;
}

.logout-modal h3 {
  margin: 0 0 16px;
  color: #333;
}

.logout-modal p {
  margin: 0 0 24px;
  color: #666;
}

.logout-modal-buttons {
  display: flex;
  gap: 12px;
  justify-content: center;
}

.btn-cancel,
.btn-confirm {
  padding: 8px 24px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
  border: none;
}

.btn-cancel {
  background: #f1f5f9;
  color: #333;
}

.btn-confirm {
  background: #dc3545;
  color: white;
}
</style>
