<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import BaseModal from '@/shared/ui/BaseModal.vue';
import { useSystemMessageStore } from '@/stores/systemMessages';

const emit = defineEmits<{
  close: [];
}>();

const { t } = useI18n();
const systemMessageStore = useSystemMessageStore();

const hasMessages = computed(() => systemMessageStore.messages.length > 0);

function formatDate(value?: string): string {
  if (!value) {
    return '-';
  }
  return new Date(value).toLocaleString();
}

function preview(body: string): string {
  const normalized = body.replace(/\s+/g, ' ').trim();
  if (normalized.length <= 140) {
    return normalized;
  }
  return `${normalized.slice(0, 140)}...`;
}

async function handleOpenDetail(id: number): Promise<void> {
  const message = systemMessageStore.messages.find((item) => item.id === id);
  if (!message) {
    return;
  }
  await systemMessageStore.openDetail(message);
}

async function handleMarkAllRead(): Promise<void> {
  await systemMessageStore.markAllRead();
}
</script>

<template>
  <BaseModal
    :title="t('notifications.title')"
    size="3xl"
    :z-index="90"
    max-height="85vh"
    body-class="p-0"
    @close="emit('close')"
  >
    <div class="flex items-center justify-between border-b border-border px-5 py-4">
      <div class="text-sm text-text-secondary">
        {{ t('notifications.subtitle', { count: systemMessageStore.unreadCount }) }}
      </div>
      <button
        class="rounded-lg border border-border bg-bg-secondary px-3 py-1.5 text-sm text-text-primary transition hover:bg-bg-tertiary disabled:cursor-not-allowed disabled:opacity-50"
        :disabled="!systemMessageStore.hasUnread"
        @click="handleMarkAllRead"
      >
        {{ t('notifications.markAllRead') }}
      </button>
    </div>

    <div v-if="systemMessageStore.isLoading" class="p-6 text-sm text-text-secondary">
      {{ t('common.state.loading') }}
    </div>

    <div v-else-if="!hasMessages" class="flex flex-col items-center justify-center gap-2 px-6 py-16">
      <h3 class="text-lg font-semibold text-text-primary">{{ t('notifications.emptyTitle') }}</h3>
      <p class="max-w-md text-center text-sm leading-6 text-text-secondary">
        {{ t('notifications.emptyDescription') }}
      </p>
    </div>

    <div v-else class="divide-y divide-border">
      <button
        v-for="message in systemMessageStore.messages"
        :key="message.id"
        class="flex w-full items-start gap-4 px-5 py-4 text-left transition hover:bg-bg-secondary"
        @click="handleOpenDetail(message.id)"
      >
        <span
          class="mt-1.5 h-2.5 w-2.5 shrink-0 rounded-full"
          :class="message.is_read ? 'bg-transparent border border-border' : 'bg-red-500'"
        />
        <div class="min-w-0 flex-1">
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0">
              <div class="truncate text-sm font-semibold text-text-primary">
                {{ message.title }}
              </div>
              <div class="mt-1 text-xs uppercase tracking-[0.14em] text-text-tertiary">
                {{ message.kind }}
              </div>
            </div>
            <div class="shrink-0 text-xs text-text-tertiary">
              {{ formatDate(message.updated_at) }}
            </div>
          </div>
          <p class="mt-2 text-sm leading-6 text-text-secondary">
            {{ preview(message.body) }}
          </p>
        </div>
      </button>
    </div>
  </BaseModal>
</template>
