import type { Article } from '@/types/models';

/**
 * API Client for making HTTP requests to the MrRSS backend
 * Provides consistent error handling, loading state management, and user feedback
 */
export class ApiClient {
  private static instance: ApiClient;
  private loadingRequests: Set<string> = new Set();
  private networkStatus: 'online' | 'offline' = 'online';

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
  async post<T>(endpoint: string, data?: any, params?: Record<string, any>, options?: RequestInit): Promise<T> {
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
  async put<T>(endpoint: string, data?: any, params?: Record<string, any>, options?: RequestInit): Promise<T> {
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
  async delete<T>(endpoint: string, params?: Record<string, any>, options?: RequestInit): Promise<T> {
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
   * Makes an HTTP request with consistent error handling
   * @param url - Full URL to request
   * @param options - Fetch options
   * @returns Promise with the response data
   */
  private async request<T>(url: string, options: RequestInit): Promise<T> {
    // Check network status
    if (this.networkStatus === 'offline') {
      throw new Error('Network connection is offline');
    }

    // Track loading state
    const requestKey = `${options.method || 'GET'} ${url}`;
    this.loadingRequests.add(requestKey);

    try {
      const response = await fetch(url, options);

      // Check if response is OK
      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`API error: ${errorText || response.statusText}`);
      }

      // Parse response as JSON
      const data = await response.json();
      return data as T;
    } catch (error) {
      // Handle network errors
      if (error instanceof Error) {
        console.error(`API request failed: ${url}`, error);
        // Show user-friendly error message
        if (window.showToast) {
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
