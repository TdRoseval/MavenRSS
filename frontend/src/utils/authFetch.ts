import { useAuthStore } from '@/stores/auth';

/**
 * Authenticated fetch wrapper - automatically adds Authorization header
 */
export async function authFetch(url: string, options: RequestInit = {}): Promise<Response> {
  const authStore = useAuthStore();

  const headers = new Headers(options.headers);
  if (authStore.accessToken) {
    headers.set('Authorization', `Bearer ${authStore.accessToken}`);
  }

  return fetch(url, {
    ...options,
    headers,
  });
}

/**
 * Authenticated fetch that returns JSON
 */
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

/**
 * Helper for GET requests with auth
 */
export async function authGet<T = any>(url: string): Promise<T> {
  return authFetchJson<T>(url, { method: 'GET' });
}

/**
 * Helper for POST requests with auth
 */
export async function authPost<T = any>(url: string, data?: any): Promise<T> {
  return authFetchJson<T>(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: data ? JSON.stringify(data) : undefined,
  });
}

/**
 * Helper for PUT requests with auth
 */
export async function authPut<T = any>(url: string, data?: any): Promise<T> {
  return authFetchJson<T>(url, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: data ? JSON.stringify(data) : undefined,
  });
}

/**
 * Helper for DELETE requests with auth
 */
export async function authDelete<T = any>(url: string): Promise<T> {
  return authFetchJson<T>(url, { method: 'DELETE' });
}
