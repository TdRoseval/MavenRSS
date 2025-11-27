<script setup lang="ts">
import { useAppStore } from '@/stores/app';
import { useI18n } from 'vue-i18n';
import { ref, onMounted, watch, type Ref } from 'vue';
import { PhLightning, PhPlus } from '@phosphor-icons/vue';
import RuleEditorModal from '../rules/RuleEditorModal.vue';
import RuleItem from './RuleItem.vue';
import type { Condition } from '@/composables/rules/useRuleOptions';

const store = useAppStore();
const { t } = useI18n();

interface Rule {
  id: number;
  name: string;
  enabled: boolean;
  conditions: Condition[];
  actions: string[];
}

interface SettingsData {
  rules: string;
  [key: string]: unknown;
}

interface Props {
  settings: SettingsData;
}

const props = defineProps<Props>();

// Rules list
const rules: Ref<Rule[]> = ref([]);

// Modal states
const showRuleEditor = ref(false);
const editingRule: Ref<Rule | null> = ref(null);
const applyingRuleId: Ref<number | null> = ref(null);

// Load rules from settings
onMounted(() => {
  loadRules();
});

function loadRules() {
  if (props.settings.rules) {
    try {
      const parsed =
        typeof props.settings.rules === 'string'
          ? JSON.parse(props.settings.rules)
          : props.settings.rules;
      rules.value = Array.isArray(parsed) ? parsed : [];
    } catch (e) {
      console.error('Error parsing rules:', e);
      rules.value = [];
    }
  }
}

// Watch for settings changes
watch(
  () => props.settings.rules,
  () => {
    loadRules();
  },
  { immediate: true }
);

// Save rules to settings
async function saveRules() {
  try {
    props.settings.rules = JSON.stringify(rules.value);
    await fetch('/api/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ rules: props.settings.rules }),
    });
  } catch (e) {
    console.error('Error saving rules:', e);
  }
}

// Add new rule
function addRule() {
  editingRule.value = null;
  showRuleEditor.value = true;
}

// Edit existing rule
function editRule(rule: Rule): void {
  editingRule.value = { ...rule };
  showRuleEditor.value = true;
}

// Delete rule
async function deleteRule(ruleId: number): Promise<void> {
  const confirmed = await window.showConfirm({
    title: t('ruleDeleteConfirmTitle'),
    message: t('ruleDeleteConfirmMessage'),
    confirmText: t('delete'),
    cancelText: t('cancel'),
    isDanger: true,
  });

  if (!confirmed) return;

  rules.value = rules.value.filter((r) => r.id !== ruleId);
  await saveRules();
  window.showToast(t('ruleDeletedSuccess'), 'success');
}

// Toggle rule enabled state
async function toggleRuleEnabled(rule: Rule): Promise<void> {
  rule.enabled = !rule.enabled;
  await saveRules();
}

// Save rule from editor
async function handleSaveRule(rule: Rule): Promise<void> {
  // Check if this is a new rule (editingRule is null or has no id)
  const isNew = !editingRule.value || !editingRule.value.id;

  if (editingRule.value && editingRule.value.id) {
    // Update existing rule
    const index = rules.value.findIndex((r) => r.id === rule.id);
    if (index !== -1) {
      rules.value[index] = rule;
    }
  } else {
    // Add new rule
    rule.id = Date.now();
    rule.enabled = true;
    rules.value.push(rule);
  }

  await saveRules();
  showRuleEditor.value = false;
  window.showToast(t('ruleSavedSuccess'), 'success');

  // Apply rule to existing articles when adding a new rule
  if (isNew && rule.enabled) {
    await applyRule(rule);
  }
}

// Apply rule now
async function applyRule(rule: Rule): Promise<void> {
  applyingRuleId.value = rule.id;

  try {
    const res = await fetch('/api/rules/apply', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(rule),
    });

    if (res.ok) {
      const data = await res.json();
      window.showToast(t('ruleAppliedSuccess', { count: data.affected }), 'success');
      store.fetchArticles();
      store.fetchUnreadCounts();
    } else {
      window.showToast(t('errorSavingSettings'), 'error');
    }
  } catch (e) {
    console.error('Error applying rule:', e);
    window.showToast(t('errorSavingSettings'), 'error');
  } finally {
    applyingRuleId.value = null;
  }
}
</script>

<template>
  <div class="space-y-4 sm:space-y-6">
    <div class="setting-group">
      <label
        class="font-semibold mb-2 sm:mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2"
      >
        <PhLightning :size="14" class="sm:w-4 sm:h-4" />
        {{ t('rules') }}
      </label>

      <!-- Header with description and add button -->
      <div class="setting-item mb-2 sm:mb-3">
        <div class="flex-1 flex items-center sm:items-start gap-2 sm:gap-3 min-w-0">
          <PhLightning :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
          <div class="flex-1 min-w-0">
            <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">{{ t('rules') }}</div>
            <div class="text-xs text-text-secondary hidden sm:block">{{ t('rulesDesc') }}</div>
          </div>
        </div>
        <button @click="addRule" class="btn-primary">
          <PhPlus :size="16" class="sm:w-5 sm:h-5" />
          <span class="hidden sm:inline">{{ t('addRule') }}</span>
        </button>
      </div>

      <!-- Empty state -->
      <div v-if="rules.length === 0" class="empty-state">
        <PhLightning :size="48" class="mx-auto mb-3 opacity-30" />
        <p class="text-text-secondary text-sm sm:text-base">{{ t('noRules') }}</p>
        <p class="text-text-secondary text-xs mt-1">{{ t('noRulesHint') }}</p>
      </div>

      <!-- Rules List -->
      <div v-else class="space-y-2 sm:space-y-3">
        <RuleItem
          v-for="rule in rules"
          :key="rule.id"
          :rule="rule"
          :is-applying="applyingRuleId === rule.id"
          @toggle-enabled="toggleRuleEnabled(rule)"
          @apply="applyRule(rule)"
          @edit="editRule(rule)"
          @delete="deleteRule(rule.id)"
        />
      </div>
    </div>

    <!-- Rule Editor Modal -->
    <RuleEditorModal
      v-if="showRuleEditor"
      :show="showRuleEditor"
      :rule="editingRule"
      @close="showRuleEditor = false"
      @save="handleSaveRule"
    />
  </div>
</template>

<style scoped>
.setting-item {
  @apply flex items-center sm:items-start justify-between gap-2 sm:gap-4 p-2 sm:p-3 rounded-lg bg-bg-secondary border border-border;
}

.btn-primary {
  @apply bg-accent text-white border-none px-3 py-2 sm:px-4 sm:py-2.5 rounded-lg cursor-pointer flex items-center gap-1 sm:gap-2 font-medium hover:bg-accent-hover transition-colors text-sm sm:text-base;
}

.empty-state {
  @apply text-center py-8 sm:py-12;
}
</style>
