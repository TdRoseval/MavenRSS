<template>
  <div class="login-container">
    <div class="login-card">
      <div class="login-header">
        <h1>{{ t('appName') }}</h1>
        <p class="subtitle">{{ t('login.welcome') }}</p>
      </div>

      <div class="tabs">
        <button :class="{ active: currentTab === 'login' }" @click="currentTab = 'login'">
          {{ t('login.login') }}
        </button>
        <button :class="{ active: currentTab === 'register' }" @click="currentTab = 'register'">
          {{ t('login.register') }}
        </button>
      </div>

      <form class="login-form" @submit.prevent="handleSubmit">
        <div v-if="currentTab === 'register'" class="form-group">
          <label for="email">{{ t('login.email') }}</label>
          <input
            id="email"
            v-model="formData.email"
            type="email"
            required
            :placeholder="t('login.email')"
          />
        </div>

        <div class="form-group">
          <label for="username">{{ t('login.username') }}</label>
          <input
            id="username"
            v-model="formData.username"
            type="text"
            required
            :placeholder="t('login.username')"
          />
        </div>

        <div class="form-group">
          <label for="password">{{ t('login.password') }}</label>
          <input
            id="password"
            v-model="formData.password"
            type="password"
            required
            :placeholder="t('login.password')"
          />
        </div>

        <div v-if="currentTab === 'login'" class="form-group remember-me">
          <label class="checkbox-label">
            <input v-model="rememberMe" type="checkbox" />
            <span>{{ t('login.rememberMe') }}</span>
          </label>
        </div>

        <div v-if="error" class="error-message">
          {{ error }}
        </div>

        <div v-if="successMessage" class="success-message">
          {{ successMessage }}
        </div>

        <button type="submit" class="submit-button" :disabled="loading">
          <span v-if="loading">{{
            currentTab === 'login' ? t('login.loggingIn') : t('login.registering')
          }}</span>
          <span v-else>{{
            currentTab === 'login' ? t('login.loginButton') : t('login.registerButton')
          }}</span>
        </button>
      </form>

      <div class="login-footer">
        <p v-if="currentTab === 'login'">
          {{ t('login.noAccount') }}
          <button class="link-button" @click="currentTab = 'register'">
            {{ t('login.register') }}
          </button>
        </p>
        <p v-else>
          {{ t('login.hasAccount') }}
          <button class="link-button" @click="currentTab = 'login'">{{ t('login.login') }}</button>
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useAuthStore } from '@/stores/auth';
import { authApi } from '@/utils/authApi';

const { t } = useI18n();

const emit = defineEmits<{
  (e: 'login'): void;
}>();

const authStore = useAuthStore();

const currentTab = ref<'login' | 'register'>('login');
const loading = ref(false);
const error = ref('');
const successMessage = ref('');
const rememberMe = ref(false);

const formData = reactive({
  username: '',
  email: '',
  password: '',
});

onMounted(() => {
  const remembered = authStore.loadFromStorage();
  if (remembered) {
    formData.username = remembered.username;
    formData.password = remembered.password;
    rememberMe.value = true;
  }
});

async function handleSubmit() {
  loading.value = true;
  error.value = '';
  successMessage.value = '';

  try {
    if (currentTab.value === 'login') {
      const result = await authApi.login({
        username: formData.username,
        password: formData.password,
      });

      authStore.setAuth(result.access_token, result.refresh_token, result.user);

      if (rememberMe.value) {
        authStore.saveRememberedCredentials(formData.username, formData.password);
      } else {
        authStore.clearRememberedCredentials();
      }

      emit('login');
    } else {
      await authApi.register({
        username: formData.username,
        email: formData.email,
        password: formData.password,
      });

      successMessage.value = 'Registration submitted successfully. Please wait for admin approval.';

      setTimeout(() => {
        currentTab.value = 'login';
        formData.email = '';
        formData.password = '';
      }, 2000);
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'An error occurred';
  } finally {
    loading.value = false;
  }
}
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 20px;
}

.login-card {
  background: white;
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  padding: 40px;
  width: 100%;
  max-width: 400px;
}

.login-header {
  text-align: center;
  margin-bottom: 30px;
}

.login-header h1 {
  font-size: 32px;
  font-weight: 700;
  color: #667eea;
  margin: 0 0 8px;
}

.subtitle {
  font-size: 16px;
  color: #64748b;
  margin: 0;
}

.tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 24px;
  background: #f1f5f9;
  padding: 4px;
  border-radius: 8px;
}

.tabs button {
  flex: 1;
  padding: 10px 16px;
  border: none;
  background: transparent;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  color: #64748b;
  cursor: pointer;
  transition: all 0.2s;
}

.tabs button:hover {
  background: #e2e8f0;
}

.tabs button.active {
  background: white;
  color: #667eea;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group label {
  font-size: 14px;
  font-weight: 500;
  color: #374151;
}

.form-group input {
  padding: 12px 14px;
  border: 2px solid #e2e8f0;
  border-radius: 8px;
  font-size: 14px;
  transition: border-color 0.2s;
}

.form-group input:focus {
  outline: none;
  border-color: #667eea;
}

.remember-me {
  flex-direction: row;
  align-items: center;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.checkbox-label input {
  width: auto;
}

.checkbox-label span {
  font-size: 14px;
  color: #64748b;
}

.error-message {
  padding: 12px;
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 8px;
  color: #dc2626;
  font-size: 14px;
}

.success-message {
  padding: 12px;
  background: #f0fdf4;
  border: 1px solid #bbf7d0;
  border-radius: 8px;
  color: #16a34a;
  font-size: 14px;
}

.submit-button {
  padding: 14px;
  border: none;
  border-radius: 8px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  font-size: 16px;
  font-weight: 600;
  cursor: pointer;
  transition:
    transform 0.2s,
    box-shadow 0.2s;
}

.submit-button:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
}

.submit-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.login-footer {
  margin-top: 24px;
  text-align: center;
}

.login-footer p {
  font-size: 14px;
  color: #64748b;
  margin: 0;
}

.link-button {
  border: none;
  background: transparent;
  color: #667eea;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  padding: 0;
}

.link-button:hover {
  text-decoration: underline;
}
</style>
