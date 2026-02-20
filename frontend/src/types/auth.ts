export type UserRole = 'user' | 'admin' | 'template';

export interface User {
  id: number;
  username: string;
  email: string;
  role: UserRole;
  status: string;
  created_at: string;
  updated_at: string;
  inherited_from?: number;
  has_inherited: boolean;
}

export interface UserQuota {
  id: number;
  user_id: number;
  max_feeds: number;
  max_articles: number;
  max_ai_calls_per_day: number;
  max_storage_mb: number;
  used_feeds: number;
  used_articles: number;
  used_ai_calls_today: number;
  used_storage_mb: number;
  created_at: string;
  updated_at: string;
}

export interface PendingRegistration {
  id: number;
  username: string;
  email: string;
  created_at: string;
}

export interface AuthResponse {
  access_token: string;
  refresh_token: string;
  user: User;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface RegisterRequest {
  username: string;
  email: string;
  password: string;
}

export interface RefreshRequest {
  refresh_token: string;
}
