/**
 * Settings types for SettingsModal and related components
 */

export interface SettingsData {
  update_interval: number;
  translation_enabled: boolean;
  target_language: string;
  translation_provider: string;
  deepl_api_key: string;
  baidu_app_id: string;
  baidu_secret_key: string;
  ai_api_key: string;
  ai_endpoint: string;
  ai_model: string;
  auto_cleanup_enabled: boolean;
  max_cache_size_mb: number;
  max_article_age_days: number;
  language: string;
  theme: string;
  last_article_update: string;
  show_hidden_articles: boolean;
  default_view_mode: string;
  startup_on_boot: boolean;
  shortcuts: string;
  rules: string;
  summary_enabled: boolean;
  summary_length: string;
  [key: string]: unknown; // Allow additional properties
}

export interface UpdateInfo {
  has_update: boolean;
  current_version: string;
  latest_version: string;
  download_url: string;
  asset_name: string;
  error?: string;
}

export interface DownloadResponse {
  success: boolean;
  file_path: string;
}

export interface InstallResponse {
  success: boolean;
}

export type TabName = 'general' | 'feeds' | 'rules' | 'shortcuts' | 'about';
