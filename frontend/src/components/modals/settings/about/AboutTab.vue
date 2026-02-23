<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { useAuthStore } from '@/stores/auth';
import { PhUser, PhEnvelope, PhGithubLogo, PhDownloadSimple } from '@phosphor-icons/vue';
import { openInBrowser } from '@/utils/browser';
import { authApi } from '@/utils/authApi';
import { ref, onMounted } from 'vue';
import { useAppStore } from '@/stores/app';
import { useSettings } from '@/composables/core/useSettings';

const { t } = useI18n();
const authStore = useAuthStore();
const store = useAppStore();
const { fetchSettings } = useSettings();

const templateAvailable = ref(false);
const isLoading = ref(false);
const checkingTemplate = ref(true);

async function checkTemplateAvailable() {
  try {
    const result = await authApi.checkTemplateAvailable();
    templateAvailable.value = result.available;
  } catch (e) {
    console.error('Failed to check template availability:', e);
  } finally {
    checkingTemplate.value = false;
  }
}

async function handleInheritTemplate() {
  const isReinherit = authStore.user?.has_inherited;
  const message = isReinherit
    ? `<div>重新继承将覆盖当前的所有数据：</div>
       <div style="margin-top: 8px; color: #dc2626; font-weight: 700;">• 订阅源</div>
       <div style="color: #dc2626; font-weight: 700;">• 文章数据</div>
       <div style="color: #dc2626; font-weight: 700;">• AI 配置</div>
       <div style="color: #dc2626; font-weight: 700;">• 所有用户设置</div>
       <div style="margin-top: 8px;">确定要继续吗？</div>`
    : t('auth.userInfo.inheritTemplateConfirm');

  const confirmed = await window.showConfirm({
    title: t('auth.userInfo.inheritTemplate'),
    message: message,
    isDanger: isReinherit,
    useHtml: isReinherit,
  });
  if (!confirmed) return;

  isLoading.value = true;
  try {
    await authApi.inheritTemplate();
    window.showToast(t('auth.userInfo.inheritTemplateSuccess'), 'success');

    const meResult = await authApi.getMe();
    authStore.updateUser(meResult.user);
    templateAvailable.value = false;

    store.fetchFeeds();
    store.fetchArticles();

    // Directly refresh settings to show the inherited AI configuration
    await fetchSettings();
  } catch (e) {
    console.error('Failed to inherit template:', e);
    window.showToast((e as Error).message || 'Failed to inherit template', 'error');
  } finally {
    isLoading.value = false;
  }
}

function openGitHubRepo() {
  openInBrowser('https://github.com/TdRoseval/MavenRSS');
}

onMounted(() => {
  checkTemplateAvailable();
});
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

      <!-- 继承模板数据按钮 -->
      <div v-if="!checkingTemplate" class="bg-bg-secondary p-4 rounded-lg border border-border">
        <div class="space-y-3">
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-full bg-accent/10 flex items-center justify-center">
              <PhDownloadSimple :size="20" class="text-accent" />
            </div>
            <div class="flex-1 min-w-0">
              <p class="text-xs text-text-secondary mb-1">
                {{ t('auth.userInfo.inheritTemplate') }}
              </p>
              <p class="text-text-primary font-medium">
                {{
                  authStore.user?.has_inherited
                    ? t('auth.userInfo.inheritTemplateAlready')
                    : t('auth.userInfo.inheritTemplateDesc')
                }}
              </p>
            </div>
          </div>
          <button
            v-if="templateAvailable"
            type="button"
            class="w-full bg-accent text-white py-2.5 px-4 rounded-lg font-medium cursor-pointer hover:bg-accent-hover transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            :disabled="isLoading"
            @click="handleInheritTemplate"
          >
            {{
              isLoading
                ? t('auth.userInfo.inheritTemplateLoading')
                : authStore.user?.has_inherited
                  ? '重新继承模板'
                  : t('auth.userInfo.inheritTemplate')
            }}
          </button>
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
