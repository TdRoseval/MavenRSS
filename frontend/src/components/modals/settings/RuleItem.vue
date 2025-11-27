<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { PhFunnel, PhListChecks, PhPlay, PhPencil, PhTrash } from '@phosphor-icons/vue';
import type { Condition } from '@/composables/rules/useRuleOptions';

const { t } = useI18n();

interface Rule {
  id: number;
  name: string;
  enabled: boolean;
  conditions: Condition[];
  actions: string[];
}

interface Props {
  rule: Rule;
  isApplying: boolean;
}

defineProps<Props>();

const emit = defineEmits<{
  'toggle-enabled': [];
  apply: [];
  edit: [];
  delete: [];
}>();

// Format condition for display
function formatCondition(rule: Rule): string {
  if (!rule.conditions || rule.conditions.length === 0) {
    return t('conditionAlways');
  }

  // Simplified display - show first condition
  const first = rule.conditions[0];
  let text = formatSingleCondition(first);

  if (rule.conditions.length > 1) {
    text += ` ${t('andNMore', { count: rule.conditions.length - 1 })}`;
  }

  return text;
}

function formatSingleCondition(condition: Condition): string {
  const fieldLabels: Record<string, string> = {
    feed_name: t('feedName'),
    feed_category: t('feedCategory'),
    article_title: t('articleTitle'),
    published_after: t('publishedAfter'),
    published_before: t('publishedBefore'),
    is_read: t('readStatus'),
    is_favorite: t('favoriteStatus'),
    is_hidden: t('hiddenStatus'),
  };

  const field = fieldLabels[condition.field] || condition.field;
  const value =
    condition.value || (condition.values && condition.values.length > 0 ? condition.values[0] : '');

  if (condition.negate) {
    return `${t('not')} ${field}: ${value}`;
  }

  return `${field}: ${value}`;
}

// Format actions for display
function formatActions(rule: Rule): string {
  if (!rule.actions || rule.actions.length === 0) {
    return '-';
  }

  const actionLabels: Record<string, string> = {
    favorite: t('actionFavorite'),
    unfavorite: t('actionUnfavorite'),
    hide: t('actionHide'),
    unhide: t('actionUnhide'),
    mark_read: t('actionMarkRead'),
    mark_unread: t('actionMarkUnread'),
  };

  return rule.actions.map((a: string) => actionLabels[a] || a).join(', ');
}
</script>

<template>
  <div class="rule-item">
    <div class="flex items-start gap-2 sm:gap-4">
      <!-- Toggle and Info -->
      <div class="flex-1 flex items-start gap-2 sm:gap-3 min-w-0">
        <input
          type="checkbox"
          :checked="rule.enabled"
          @change="emit('toggle-enabled')"
          class="toggle mt-1"
        />
        <div class="flex-1 min-w-0">
          <div
            class="font-medium mb-1 text-sm sm:text-base truncate"
            :class="{ 'text-text-secondary': !rule.enabled }"
          >
            {{ rule.name || t('rules') + ' #' + rule.id }}
          </div>
          <div class="text-xs text-text-secondary flex flex-wrap items-center gap-1 sm:gap-2">
            <span class="condition-badge">
              <PhFunnel :size="12" />
              {{ formatCondition(rule) }}
            </span>
            <span class="text-text-tertiary">→</span>
            <span class="action-badge">
              <PhListChecks :size="12" />
              {{ formatActions(rule) }}
            </span>
          </div>
        </div>
      </div>

      <!-- Action buttons -->
      <div class="flex items-center gap-1 sm:gap-2 shrink-0">
        <button
          @click="emit('apply')"
          class="action-btn"
          :disabled="isApplying"
          :title="t('applyRuleNow')"
        >
          <PhPlay v-if="!isApplying" :size="18" class="sm:w-5 sm:h-5" />
          <span v-else class="animate-spin text-sm">⟳</span>
        </button>
        <button @click="emit('edit')" class="action-btn" :title="t('editRule')">
          <PhPencil :size="18" class="sm:w-5 sm:h-5" />
        </button>
        <button @click="emit('delete')" class="action-btn danger" :title="t('deleteRule')">
          <PhTrash :size="18" class="sm:w-5 sm:h-5" />
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.rule-item {
  @apply p-2 sm:p-3 rounded-lg bg-bg-secondary border border-border;
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

.condition-badge,
.action-badge {
  @apply inline-flex items-center gap-1 px-1.5 sm:px-2 py-0.5 sm:py-1 rounded text-[10px] sm:text-xs bg-bg-tertiary;
}

.action-btn {
  @apply p-1.5 sm:p-2 rounded-lg bg-transparent border-none cursor-pointer text-text-secondary hover:bg-bg-tertiary hover:text-text-primary transition-colors;
}

.action-btn.danger:hover {
  @apply text-red-500 bg-red-500/10;
}

.action-btn:disabled {
  @apply opacity-50 cursor-not-allowed;
}

.animate-spin {
  animation: spin 1s linear infinite;
  display: inline-block;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}
</style>
