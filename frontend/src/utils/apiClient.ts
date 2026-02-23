import type { Article } from '@/types/models';
import { useAuthStore } from '@/stores/auth';

interface AuthResponse {
  access_token: string;
  refresh_token: string;
  user: any;
}

/**
 * API Client for making HTTP requests to the MavenRSS backend
 * Provides consistent error handling, loading state management, and user feedback
 */
export class ApiClient {
  private static instance: ApiClient;
  private loadingRequests: Set<string> = new Set();
  private networkStatus: 'online' | 'offline' = 'online';
  private authStore: any = null;
  private isRefreshing: boolean = false;
  private refreshPromise: Promise<any> | null = null;

  private constructor() {
    // Monitor network status
    this.setupNetworkMonitoring();
  }

  public static getInstance(): ApiClient {
    if (!ApiClient.instance) {
      ApiClient.instance = new ApiClient();
    }
    return ApiClient.instance;
  }

  private getAuthStore() {
    if (!this.authStore) {
      this.authStore = useAuthStore();
    }
    return this.authStore;
  }

  private setupNetworkMonitoring(): void {
    window.addEventListener('online', () => {
      this.networkStatus = 'online';
      if (window.showToast) {
        window.showToast('Network connection restored', 'success');
      }
    });

    window.addEventListener('offline', () => {
      this.networkStatus = 'offline';
      if (window.showToast) {
        window.showToast('Network connection lost. Some features may be unavailable.', 'error');
      }
    });
  }

  /**
   * Makes a GET request to the specified endpoint
   * @param endpoint - API endpoint to call
   * @param params - URL parameters to include
   * @param options - Additional fetch options
   * @returns Promise with the response data
   */
  async get<T>(endpoint: string, params?: Record<string, any>, options?: RequestInit): Promise<T> {
    const url = this.buildUrl(endpoint, params);
    return this.request<T>(url, {
      ...options,
      method: 'GET',
    });
  }

  /**
   * Makes a POST request to the specified endpoint
   * @param endpoint - API endpoint to call
   * @param data - Data to send in the request body
   * @param params - URL parameters to include
   * @param options - Additional fetch options
   * @returns Promise with the response data
   */
  async post<T>(
    endpoint: string,
    data?: any,
    params?: Record<string, any>,
    options?: RequestInit
  ): Promise<T> {
    const url = this.buildUrl(endpoint, params);
    return this.request<T>(url, {
      ...options,
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  /**
   * Makes a PUT request to the specified endpoint
   * @param endpoint - API endpoint to call
   * @param data - Data to send in the request body
   * @param params - URL parameters to include
   * @param options - Additional fetch options
   * @returns Promise with the response data
   */
  async put<T>(
    endpoint: string,
    data?: any,
    params?: Record<string, any>,
    options?: RequestInit
  ): Promise<T> {
    const url = this.buildUrl(endpoint, params);
    return this.request<T>(url, {
      ...options,
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  /**
   * Makes a DELETE request to the specified endpoint
   * @param endpoint - API endpoint to call
   * @param params - URL parameters to include
   * @param options - Additional fetch options
   * @returns Promise with the response data
   */
  async delete<T>(
    endpoint: string,
    params?: Record<string, any>,
    options?: RequestInit
  ): Promise<T> {
    const url = this.buildUrl(endpoint, params);
    return this.request<T>(url, {
      ...options,
      method: 'DELETE',
    });
  }

  /**
   * Builds a URL with the specified endpoint and parameters
   * @param endpoint - API endpoint
   * @param params - URL parameters
   * @returns Full URL string
   */
  private buildUrl(endpoint: string, params?: Record<string, any>): string {
    let url = endpoint;
    if (!url.startsWith('/api/')) {
      url = `/api/${url}`;
    }

    if (params && Object.keys(params).length > 0) {
      const searchParams = new URLSearchParams();
      Object.entries(params).forEach(([key, value]) => {
        if (value !== undefined && value !== null) {
          searchParams.append(key, String(value));
        }
      });
      url += `?${searchParams.toString()}`;
    }

    return url;
  }

  /**
   * Refresh access token using refresh token
   * @returns Promise with new auth data
   */
  private async refreshAccessToken(): Promise<void> {
    const authStore = this.getAuthStore();

    if (!authStore.refreshToken) {
      throw new Error('No refresh token available');
    }

    const response = await fetch('/api/auth/refresh', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ refresh_token: authStore.refreshToken }),
    });

    if (!response.ok) {
      throw new Error('Failed to refresh token');
    }

    const data: AuthResponse = await response.json();
    authStore.setAuth(data.access_token, data.refresh_token, data.user);
  }

  /**
   * Makes an HTTP request with consistent error handling
   * @param url - Full URL to request
   * @param options - Fetch options
   * @returns Promise with the response data
   */
  private async request<T>(
    url: string,
    options: RequestInit,
    isRetry: boolean = false
  ): Promise<T> {
    // Check network status
    if (this.networkStatus === 'offline') {
      throw new Error('Network connection is offline');
    }

    // Track loading state
    const requestKey = `${options.method || 'GET'} ${url}`;
    this.loadingRequests.add(requestKey);

    try {
      // If already refreshing, wait for it to complete
      if (this.isRefreshing && this.refreshPromise && !isRetry) {
        await this.refreshPromise;
        // Retry the original request after refresh
        return this.request<T>(url, options, true);
      }

      // Add authorization header if access token is available
      const authStore = this.getAuthStore();
      const headers = new Headers(options.headers || {});

      if (authStore.accessToken) {
        headers.set('Authorization', `Bearer ${authStore.accessToken}`);
      }

      const response = await fetch(url, {
        ...options,
        headers,
      });

      // Handle 401 Unauthorized - try to refresh token first
      if (response.status === 401 && !isRetry) {
        if (!this.isRefreshing) {
          this.isRefreshing = true;
          this.refreshPromise = this.refreshAccessToken();

          try {
            await this.refreshPromise;
            // Refresh succeeded, retry the request
            return this.request<T>(url, options, true);
          } catch (refreshError) {
            // Refresh failed, clear auth
            authStore.clearStorage();
            throw new Error('Authentication required');
          } finally {
            this.isRefreshing = false;
            this.refreshPromise = null;
          }
        }
      }

      // Handle 401 on retry - clear auth
      if (response.status === 401 && isRetry) {
        authStore.clearStorage();
        throw new Error('Authentication required');
      }

      // Check if response is OK
      if (!response.ok) {
        const contentType = response.headers.get('content-type') || '';
        let errorText: string;

        if (contentType.includes('application/json')) {
          const errorData = await response.json();
          errorText = errorData.error || errorData.message || response.statusText;
        } else {
          errorText = await response.text();
        }

        throw new Error(errorText || `API error: ${response.status}`);
      }

      // Parse response as JSON
      const text = await response.text();
      if (!text) {
        return {} as T;
      }
      const data = JSON.parse(text);
      return data as T;
    } catch (error) {
      // Handle network errors
      if (error instanceof Error) {
        console.error(`API request failed: ${url}`, error);
        // Show user-friendly error message
        if (window.showToast && error.message !== 'Authentication required') {
          window.showToast(`API request failed: ${error.message}`, 'error');
        }
      }
      throw error;
    } finally {
      // Remove from loading requests
      this.loadingRequests.delete(requestKey);
    }
  }

  /**
   * Checks if any API requests are currently loading
   * @returns boolean indicating if any requests are loading
   */
  isLoading(): boolean {
    return this.loadingRequests.size > 0;
  }

  /**
   * Gets the current network status
   * @returns current network status
   */
  getNetworkStatus(): 'online' | 'offline' {
    return this.networkStatus;
  }
}

// Export singleton instance
export const apiClient = ApiClient.getInstance();

// Export common API functions for convenience
export async function fetchArticles(params?: {
  page?: number;
  limit?: number;
  filter?: string;
  feed_id?: number;
  category?: string;
}): Promise<Article[]> {
  return apiClient.get<Article[]>('/articles', params);
}

export async function fetchFeeds(): Promise<any[]> {
  return apiClient.get<any[]>('/feeds');
}

export async function fetchUnreadCounts(): Promise<any> {
  return apiClient.get<any>('/articles/unread-counts');
}

export async function markArticleAsRead(articleId: number, read: boolean): Promise<void> {
  return apiClient.post<void>('/articles/read', { id: articleId, read });
}

export async function toggleArticleFavorite(articleId: number): Promise<void> {
  return apiClient.post<void>('/articles/favorite', { id: articleId });
}

export async function toggleArticleReadLater(articleId: number): Promise<void> {
  return apiClient.post<void>('/articles/toggle-read-later', { id: articleId });
}

export async function refreshFeeds(): Promise<void> {
  return apiClient.post<void>('/refresh');
}

export async function fetchSettings(): Promise<any> {
  return apiClient.get<any>('/settings');
}

export async function saveSettings(settings: any): Promise<void> {
  return apiClient.post<void>('/settings', settings);
}
