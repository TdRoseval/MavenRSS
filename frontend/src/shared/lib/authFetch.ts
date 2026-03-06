import { useAuthStore } from '@/stores/auth';

let isRefreshing = false;
let refreshPromise: Promise<boolean> | null = null;

async function refreshTokens(): Promise<boolean> {
  const authStore = useAuthStore();
  
  if (!authStore.refreshToken) {
    return false;
  }

  try {
    const response = await fetch('/api/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: authStore.refreshToken }),
    });

    if (!response.ok) {
      return false;
    }

    const data = await response.json();
    authStore.setAuth(data.access_token, data.refresh_token, data.user);
    return true;
  } catch {
    return false;
  }
}

async function tryRefreshToken(): Promise<boolean> {
  if (isRefreshing && refreshPromise) {
    return refreshPromise;
  }

  isRefreshing = true;
  refreshPromise = refreshTokens().finally(() => {
    isRefreshing = false;
    refreshPromise = null;
  });

  return refreshPromise;
}

export async function authFetch(url: string, options: RequestInit = {}): Promise<Response> {
  const authStore = useAuthStore();

  const headers = new Headers(options.headers);
  if (authStore.accessToken) {
    headers.set('Authorization', `Bearer ${authStore.accessToken}`);
  }

  const response = await fetch(url, {
    ...options,
    headers,
  });

  if (response.status === 401 && authStore.refreshToken) {
    const refreshed = await tryRefreshToken();
    
    if (refreshed) {
      const newHeaders = new Headers(options.headers);
      if (authStore.accessToken) {
        newHeaders.set('Authorization', `Bearer ${authStore.accessToken}`);
      }
      
      return fetch(url, {
        ...options,
        headers: newHeaders,
      });
    } else {
      authStore.clearStorage();
    }
  }

  return response;
}

export async function authFetchJson<T = any>(url: string, options: RequestInit = {}): Promise<T> {
  const response = await authFetch(url, options);

  if (!response.ok) {
    const contentType = response.headers.get('content-type') || '';
    if (contentType.includes('application/json')) {
      const error = await response.json();
      const errorMessage = error.error?.message || error.error || error.message || 'Request failed';
      throw new Error(typeof errorMessage === 'string' ? errorMessage : 'Request failed');
    }
    throw new Error(`Request failed: ${response.status}`);
  }

  const text = await response.text();
  if (!text) {
    return {} as T;
  }
  return JSON.parse(text);
}

export async function authGet<T = any>(url: string): Promise<T> {
  return authFetchJson<T>(url, { method: 'GET' });
}

export async function authPost<T = any>(url: string, data?: any): Promise<T> {
  return authFetchJson<T>(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: data ? JSON.stringify(data) : undefined,
  });
}

export async function authPut<T = any>(url: string, data?: any): Promise<T> {
  return authFetchJson<T>(url, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: data ? JSON.stringify(data) : undefined,
  });
}

export async function authDelete<T = any>(url: string): Promise<T> {
  return authFetchJson<T>(url, { method: 'DELETE' });
}
