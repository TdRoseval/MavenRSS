import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import type { User } from '@/types/auth';

const STORAGE_KEY = 'mavenrss_auth';
const REMEMBER_KEY = 'mavenrss_remember';

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  user: User | null;
}

export const useAuthStore = defineStore('auth', () => {
  const accessToken = ref<string | null>(null);
  const refreshToken = ref<string | null>(null);
  const user = ref<User | null>(null);
  const loading = ref(false);

  const isAuthenticated = computed(() => !!accessToken.value && !!user.value);
  const isAdmin = computed(() => user.value?.role === 'admin');
  const isTemplate = computed(() => user.value?.role === 'template');

  function loadFromStorage() {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      try {
        const data: AuthState = JSON.parse(stored);
        accessToken.value = data.accessToken;
        refreshToken.value = data.refreshToken;
        user.value = data.user;
      } catch {
        clearStorage();
      }
    }

    const remembered = localStorage.getItem(REMEMBER_KEY);
    if (remembered) {
      try {
        const data = JSON.parse(remembered);
        return data;
      } catch {
        localStorage.removeItem(REMEMBER_KEY);
      }
    }
    return null;
  }

  function saveToStorage() {
    const data: AuthState = {
      accessToken: accessToken.value,
      refreshToken: refreshToken.value,
      user: user.value,
    };
    localStorage.setItem(STORAGE_KEY, JSON.stringify(data));
  }

  function clearStorage() {
    localStorage.removeItem(STORAGE_KEY);
    accessToken.value = null;
    refreshToken.value = null;
    user.value = null;
  }

  function saveRememberedCredentials(username: string, password: string) {
    const data = { username, password };
    localStorage.setItem(REMEMBER_KEY, JSON.stringify(data));
  }

  function clearRememberedCredentials() {
    localStorage.removeItem(REMEMBER_KEY);
  }

  function setAuth(access: string, refresh: string, userData: User) {
    accessToken.value = access;
    refreshToken.value = refresh;
    user.value = userData;
    saveToStorage();
  }

  function updateUser(userData: User) {
    user.value = userData;
    saveToStorage();
  }

  function logout() {
    clearStorage();
    clearRememberedCredentials();
  }

  return {
    accessToken,
    refreshToken,
    user,
    loading,
    isAuthenticated,
    isAdmin,
    isTemplate,
    loadFromStorage,
    saveToStorage,
    clearStorage,
    saveRememberedCredentials,
    clearRememberedCredentials,
    setAuth,
    updateUser,
    logout,
  };
});
