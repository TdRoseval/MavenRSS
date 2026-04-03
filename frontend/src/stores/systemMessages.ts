import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { authGet, authPut } from '@/shared/lib/authFetch';
import type {
  SystemMessage,
  SystemMessageListResponse,
  SystemMessageUnreadCountResponse,
} from '@/types/models';

export const useSystemMessageStore = defineStore('systemMessages', () => {
  const POLL_INTERVAL_MS = 30000;

  const messages = ref<SystemMessage[]>([]);
  const unreadCount = ref(0);
  const isLoading = ref(false);
  const isUnreadCountLoading = ref(false);
  const hasLoaded = ref(false);
  const isCenterOpen = ref(false);
  const activeMessage = ref<SystemMessage | null>(null);

  let pollTimer: ReturnType<typeof setInterval> | null = null;

  const hasUnread = computed(() => unreadCount.value > 0);

  async function fetchMessages(): Promise<SystemMessage[]> {
    isLoading.value = true;
    try {
      const response = await authGet<SystemMessageListResponse>('/api/system-messages');
      messages.value = response.messages || [];
      hasLoaded.value = true;
      unreadCount.value = messages.value.filter((message) => !message.is_read).length;
      return messages.value;
    } finally {
      isLoading.value = false;
    }
  }

  async function fetchUnreadCount(): Promise<number> {
    isUnreadCountLoading.value = true;
    try {
      const response = await authGet<SystemMessageUnreadCountResponse>(
        '/api/system-messages/unread-count'
      );
      unreadCount.value = response.unread_count || 0;
      return unreadCount.value;
    } finally {
      isUnreadCountLoading.value = false;
    }
  }

  async function markRead(id: number): Promise<void> {
    const target = messages.value.find((message) => message.id === id);
    if (!target || target.is_read) {
      return;
    }

    await authPut('/api/system-messages/read', { id });
    target.is_read = true;
    target.read_at = new Date().toISOString();
    unreadCount.value = Math.max(
      0,
      unreadCount.value - 1
    );
    if (activeMessage.value?.id === id) {
      activeMessage.value = { ...target };
    }
  }

  async function markAllRead(): Promise<void> {
    if (!messages.value.some((message) => !message.is_read)) {
      return;
    }

    await authPut('/api/system-messages/mark-all-read');
    const readAt = new Date().toISOString();
    messages.value = messages.value.map((message) => ({
      ...message,
      is_read: true,
      read_at: readAt,
    }));
    unreadCount.value = 0;
    if (activeMessage.value) {
      activeMessage.value = {
        ...activeMessage.value,
        is_read: true,
        read_at: readAt,
      };
    }
  }

  async function openCenter(): Promise<void> {
    isCenterOpen.value = true;
    await Promise.all([fetchMessages(), fetchUnreadCount()]);
  }

  function closeCenter(): void {
    isCenterOpen.value = false;
  }

  async function openDetail(message: SystemMessage): Promise<void> {
    activeMessage.value = message;
    if (!message.is_read) {
      await markRead(message.id);
    }
  }

  function closeDetail(): void {
    activeMessage.value = null;
  }

  async function refreshIfOpen(): Promise<void> {
    if (isCenterOpen.value) {
      await fetchMessages();
      return;
    }
    await fetchUnreadCount();
  }

  function startPolling(): void {
    if (pollTimer) {
      return;
    }
    void fetchUnreadCount();
    pollTimer = setInterval(() => {
      void refreshIfOpen();
    }, POLL_INTERVAL_MS);
  }

  function stopPolling(): void {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  return {
    messages,
    unreadCount,
    isLoading,
    isUnreadCountLoading,
    hasLoaded,
    isCenterOpen,
    activeMessage,
    hasUnread,
    fetchMessages,
    fetchUnreadCount,
    markRead,
    markAllRead,
    openCenter,
    closeCenter,
    openDetail,
    closeDetail,
    startPolling,
    stopPolling,
  };
});
