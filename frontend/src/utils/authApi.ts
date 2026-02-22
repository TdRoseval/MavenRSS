import type {
  AuthResponse,
  LoginRequest,
  RegisterRequest,
  RefreshRequest,
  User,
  UserQuota,
  PendingRegistration,
} from '@/types/auth';

const API_BASE = '/api';

async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
  const token = localStorage.getItem('mrrss_auth');
  let authHeader = {};
  if (token) {
    try {
      const authData = JSON.parse(token);
      if (authData.accessToken) {
        authHeader = {
          Authorization: `Bearer ${authData.accessToken}`,
        };
      }
    } catch {}
  }

  const response = await fetch(`${API_BASE}${url}`, {
    headers: {
      'Content-Type': 'application/json',
      ...authHeader,
      ...options.headers,
    },
    ...options,
  });

  if (!response.ok) {
    let errorMessage = 'Request failed';
    try {
      const errorData = await response.json();
      errorMessage = errorData.error || errorMessage;
    } catch {}
    throw new Error(errorMessage);
  }

  return response.json();
}

export const authApi = {
  async login(data: LoginRequest): Promise<AuthResponse> {
    return request<AuthResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  async register(data: RegisterRequest): Promise<{ message: string }> {
    return request<{ message: string }>('/auth/register', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  async refresh(data: RefreshRequest): Promise<AuthResponse> {
    return request<AuthResponse>('/auth/refresh', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  async logout(data: RefreshRequest): Promise<{ message: string }> {
    return request<{ message: string }>('/auth/logout', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  async getMe(): Promise<{ user: User; quota?: UserQuota; template_available: boolean }> {
    return request('/auth/me');
  },

  async checkTemplateAvailable(): Promise<{
    available: boolean;
    inherited: boolean;
    has_template: boolean;
  }> {
    return request('/auth/template-available');
  },

  async inheritTemplate(): Promise<{ message: string }> {
    return request('/auth/inherit-template', {
      method: 'POST',
    });
  },

  admin: {
    async getPendingRegistrations(
      page: number = 1,
      pageSize: number = 20
    ): Promise<{
      registrations: PendingRegistration[];
      total: number;
      page: number;
      page_size: number;
    }> {
      return request(`/admin/pending-registrations?page=${page}&page_size=${pageSize}`);
    },

    async approveRegistration(
      registrationId: number
    ): Promise<{ message: string; user_id: number }> {
      return request('/admin/approve-registration', {
        method: 'POST',
        body: JSON.stringify({ registration_id: registrationId }),
      });
    },

    async rejectRegistration(registrationId: number): Promise<{ message: string }> {
      return request('/admin/reject-registration', {
        method: 'POST',
        body: JSON.stringify({ registration_id: registrationId }),
      });
    },

    async getUsers(
      page: number = 1,
      pageSize: number = 20
    ): Promise<{ users: User[]; total: number; page: number; page_size: number }> {
      return request(`/admin/users?page=${page}&page_size=${pageSize}`);
    },

    async getUser(id: number): Promise<User> {
      return request(`/admin/users/${id}`);
    },

    async updateUser(
      id: number,
      data: Partial<{ username: string; email: string; role: string; status: string }>
    ): Promise<User> {
      return request(`/admin/users/${id}`, {
        method: 'PUT',
        body: JSON.stringify(data),
      });
    },

    async deleteUser(id: number): Promise<{ message: string }> {
      return request(`/admin/users/${id}`, {
        method: 'DELETE',
      });
    },

    async getUserQuota(id: number): Promise<UserQuota> {
      return request(`/admin/users/${id}/quota`);
    },

    async updateUserQuota(
      id: number,
      data: Partial<{
        max_feeds: number;
        max_articles: number;
        max_ai_tokens: number;
        max_ai_concurrency: number;
        max_feed_fetch_concurrency: number;
        max_db_query_concurrency: number;
        max_storage_mb: number;
      }>
    ): Promise<UserQuota> {
      return request(`/admin/users/${id}/quota`, {
        method: 'PUT',
        body: JSON.stringify(data),
      });
    },

    async createTemplateUser(data: {
      username: string;
      email: string;
      password: string;
    }): Promise<{ message: string; user_id: number }> {
      return request('/admin/create-template', {
        method: 'POST',
        body: JSON.stringify(data),
      });
    },

    async getTemplateUser(): Promise<User> {
      return request('/admin/template');
    },
  },
};
