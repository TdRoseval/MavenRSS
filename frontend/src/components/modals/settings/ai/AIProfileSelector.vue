<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useAIProfiles } from '@/composables/ai/useAIProfiles';

const { t } = useI18n();
const { profiles, fetchProfiles } = useAIProfiles();

interface Props {
  modelValue: string | null;
  disabled?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
});

const emit = defineEmits<{
  'update:modelValue': [value: string | null];
}>();

// Load profiles on mount
onMounted(() => {
  if (profiles.value.length === 0) {
    fetchProfiles();
  }
});

// Computed selected value (keep as string)
// If no value is set, show the first profile but don't emit update
const selectedValue = computed(() => {
  if (props.modelValue === null || props.modelValue === '') {
    return profiles.value.length > 0 ? String(profiles.value[0].id) : '';
  }
  return String(props.modelValue);
});

// Handle select change (emit string value to match settings type)
function handleChange(event: Event) {
  const value = (event.target as HTMLSelectElement).value;
  if (value === '') {
    emit('update:modelValue', null);
  } else {
    emit('update:modelValue', value);
  }
}
</script>

<template>
  <div class="ai-profile-selector">
    <select
      :value="selectedValue"
      :disabled="disabled || profiles.length === 0"
      class="input-field select"
      :class="{ 'opacity-50 cursor-not-allowed': disabled || profiles.length === 0 }"
      @change="handleChange"
    >
      <option v-if="profiles.length === 0" value="" disabled>
        {{ t('setting.ai.noProfiles') }}
      </option>
      <option v-for="profile in profiles" :key="profile.id" :value="profile.id">
        {{ profile.name }}
      </option>
    </select>

    <!-- No profiles warning -->
    <div v-if="profiles.length === 0" class="text-xs text-text-tertiary mt-1">
      {{ t('setting.ai.noProfilesHint') }}
    </div>
  </div>
</template>

<style scoped>
@reference "../../../../style.css";

.input-field {
  @apply p-1.5 sm:p-2.5 border border-border rounded-md bg-bg-secondary text-text-primary focus:border-accent focus:outline-none transition-colors text-xs sm:text-sm;
  appearance: none;
  -webkit-appearance: none;
  -moz-appearance: none;
  padding-right: 2.5rem;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 20 20'%3E%3Cpath stroke='%236b7280' stroke-linecap='round' stroke-linejoin='round' stroke-width='1.5' d='M6 8l4 4 4-4'/%3E%3C/svg%3E");
  background-position: right 0.5rem center;
  background-repeat: no-repeat;
  background-size: 1.5em 1.5em;
}

.input-field:disabled {
  @apply cursor-not-allowed;
}
</style>
