<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { useAuthStore } from '@/stores/auth';
import {
  PhUser,
  PhEnvelope,
  PhGithubLogo,
} from '@phosphor-icons/vue';
import { openInBrowser } from '@/utils/browser';

const { t } = useI18n();
const authStore = useAuthStore();

function openGitHubRepo() {
  openInBrowser('https://github.com/TdRoseval/MavenRSS');
}
</script>

<template>
  <div class="py-6 sm:py-10 px-2">
    <!-- 账号信息 -->
    <div class="space-y-4 mb-8">
      <h3 class="text-lg sm:text-xl font-bold text-center mb-6">{{ t('auth.userInfo.title') }}</h3>

      <!-- 用户名 -->
      <div class="bg-bg-secondary p-4 rounded-lg border border-border">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-full bg-accent/10 flex items-center justify-center">
            <PhUser :size="20" class="text-accent" />
          </div>
          <div class="flex-1 min-w-0">
            <p class="text-xs text-text-secondary mb-1">{{ t('auth.userInfo.username') }}</p>
            <p class="text-text-primary font-medium truncate">{{ authStore.user?.username }}</p>
          </div>
        </div>
      </div>

      <!-- 邮箱 -->
      <div class="bg-bg-secondary p-4 rounded-lg border border-border">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-full bg-accent/10 flex items-center justify-center">
            <PhEnvelope :size="20" class="text-accent" />
          </div>
          <div class="flex-1 min-w-0">
            <p class="text-xs text-text-secondary mb-1">{{ t('auth.userInfo.email') }}</p>
            <p class="text-text-primary font-medium truncate">{{ authStore.user?.email }}</p>
          </div>
        </div>
      </div>

      <!-- 提示 -->
      <p class="text-xs text-text-secondary text-center mt-4">
        {{ t('auth.userInfo.readonlyHint') }}
      </p>
    </div>

    <!-- GitHub 链接 -->
    <div class="mt-8 pt-4 border-t border-border">
      <div class="flex justify-center">
        <button
          type="button"
          class="inline-flex items-center gap-1.5 sm:gap-2 text-accent hover:text-accent-hover transition-colors text-xs sm:text-sm font-medium"
          @click="openGitHubRepo"
        >
          <PhGithubLogo :size="20" class="sm:w-6 sm:h-6" />
          {{ t('setting.about.viewOnGitHub') }}
        </button>
      </div>
    </div>

    <!-- Copyright 信息 -->
    <div class="mt-6 pt-4 text-center">
      <p class="text-text-secondary text-xs">© 2026 MavenRSS. All rights reserved.</p>
      <p class="text-text-secondary text-xs">Open source and available under GPL-3.0 License.</p>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../../style.css";
</style>
