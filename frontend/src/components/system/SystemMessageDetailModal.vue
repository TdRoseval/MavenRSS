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

const metadata = computed<Record<string, any>>(() => {
  const raw = systemMessageStore.activeMessage?.metadata_json;
  if (!raw) {
    return {};
  }
  try {
    return JSON.parse(raw);
  } catch {
    return {};
  }
});

function formatDate(value?: string): string {
  if (!value) {
    return '-';
  }
  return new Date(value).toLocaleString();
}
</script>

<template>
  <BaseModal
    :title="systemMessageStore.activeMessage?.title || t('notifications.details')"
    size="2xl"
    max-height="80vh"
    @close="emit('close')"
  >
    <div v-if="systemMessageStore.activeMessage" class="space-y-4 p-5">
      <div class="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
        <div class="rounded-xl bg-bg-secondary p-3">
          <div class="text-text-tertiary">{{ t('notifications.kind') }}</div>
          <div class="mt-1 break-all font-semibold text-text-primary">
            {{ systemMessageStore.activeMessage.kind }}
          </div>
        </div>
        <div class="rounded-xl bg-bg-secondary p-3">
          <div class="text-text-tertiary">{{ t('notifications.updatedAt') }}</div>
          <div class="mt-1 font-semibold text-text-primary">
            {{ formatDate(systemMessageStore.activeMessage.updated_at) }}
          </div>
        </div>
        <div class="rounded-xl bg-bg-secondary p-3">
          <div class="text-text-tertiary">{{ t('notifications.sampleSize') }}</div>
          <div class="mt-1 font-semibold text-text-primary">
            {{ metadata.sample_size ?? '-' }}
          </div>
        </div>
        <div class="rounded-xl bg-bg-secondary p-3">
          <div class="text-text-tertiary">{{ t('notifications.unnormalizedCount') }}</div>
          <div class="mt-1 font-semibold text-text-primary">
            {{ metadata.unnormalized_count ?? '-' }}
          </div>
        </div>
      </div>

      <div v-if="metadata.unnormalized_ratio !== undefined" class="rounded-xl bg-bg-secondary p-3 text-sm">
        <div class="text-text-tertiary">{{ t('notifications.unnormalizedRatio') }}</div>
        <div class="mt-1 font-semibold text-text-primary">
          {{ (Number(metadata.unnormalized_ratio) * 100).toFixed(1) }}%
        </div>
      </div>

      <div v-if="metadata.trigger_scope" class="rounded-xl bg-bg-secondary p-3 text-sm">
        <div class="text-text-tertiary">{{ t('notifications.blockedScope') }}</div>
        <div class="mt-1 break-all font-semibold text-text-primary">
          {{ metadata.trigger_scope }}
        </div>
      </div>

      <div class="rounded-2xl border border-border bg-bg-secondary p-4">
        <div class="mb-2 text-xs font-medium uppercase tracking-[0.16em] text-text-tertiary">
          {{ t('notifications.body') }}
        </div>
        <pre class="whitespace-pre-wrap break-words text-sm leading-6 text-text-primary">{{
          systemMessageStore.activeMessage.body
        }}</pre>
      </div>
    </div>
  </BaseModal>
</template>
