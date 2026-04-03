// Type definitions for models

export interface Tag {
  id: number;
  name: string;
  color: string; // Hex color code
  position?: number;
}

export interface Article {
  id: number;
  feed_id: number;
  feed_title?: string;
  feed_name?: string; // Alias for feed_title (used in filters/rules)
  title: string;
  original_title?: string;
  translated_title?: string;
  url: string;
  image_url?: string; // Article thumbnail image
  audio_url?: string; // Podcast audio file URL
  video_url?: string; // YouTube video embed URL
  published_at: string;
  is_read: boolean;
  is_favorite: boolean;
  is_hidden: boolean;
  is_read_later: boolean;
  author?: string; // Article author
  summary?: string; // Cached AI-generated summary
  freshrss_item_id?: string; // FreshRSS/Google Reader item ID
  cluster_id?: number; // ID of the cluster this article belongs to
  // Feed reference for translation settings
  feed?: {
    translate_articles: boolean;
  };
}

export interface Cluster {
  id: number;
  user_id: number;
  status: string; // 'pending_merge' | 'merging' | 'pending_embed' | 'complete'
  merged_title: string;
  display_title?: string;
  merged_summary: string;
  merged_content: string;
  image_url?: string;
  article_count: number;
  created_at: string;
  updated_at: string;
  is_read: boolean;
  is_favorite: boolean;
  is_read_later: boolean;
  is_hidden: boolean;
  recommendation_archive_date?: string;
  recommendation_score?: number;
  is_ai_recommended?: boolean;
  recommendation_profile_id?: number;
  feed_titles?: string[];
  authors?: string[];
  articles?: Article[];
}

export interface DailyRecommendationItem {
  recommendation_date: string;
  recommendation_rank: number;
  recommendation_score: number;
  recommendation_profile_id: number;
  latest_published_at?: string;
  cluster: Cluster;
}

export interface DailyRecommendationResponse {
  selected_date: string;
  available_dates: string[];
  recommendations: DailyRecommendationItem[];
  total: number;
}

export interface DailyRecommendationTaskStatus {
  is_enabled: boolean;
  has_task: boolean;
  recommendation_date?: string;
  trigger?: string;
  stage?: string;
  is_queued: boolean;
  is_running: boolean;
  is_waiting_for_idle: boolean;
  force: boolean;
  progress_percent: number;
  candidate_count: number;
  selected_count: number;
  saved_count: number;
  started_at?: string;
  updated_at?: string;
  last_error_message?: string;
  last_error_at?: string;
}

export interface DailyRecommendationRefreshResponse {
  scheduled: boolean;
  date: string;
  status: DailyRecommendationTaskStatus;
}

export interface AIProcessingStatus {
  is_enabled: boolean;
  has_interest_vector: boolean;
  is_renormalization_running?: boolean;
  is_config_frozen: boolean;
  is_stale: boolean;
  is_freeze_suspended: boolean;
  eligible_articles: number;
  pending_articles: number;
  completed_articles: number;
  pending_summary_articles: number;
  pending_translation_articles: number;
  pending_embedding_articles: number;
  pending_clustering_articles: number;
  pending_recommendation_days: number;
  progress_percent: number;
  queued_tasks: number;
  active_worker_tasks: number;
  active_async_work: number;
  is_cluster_pipeline_busy: boolean;
  is_cluster_fusion_running?: boolean;
  is_cluster_embedding_running?: boolean;
  pending_merge_clusters?: number;
  pending_embed_clusters?: number;
  cluster_phase?: string;
  renormalization_total_articles?: number;
  renormalization_pending_articles?: number;
  renormalization_completed_articles?: number;
  last_progress_at?: string;
  stalled_for_seconds?: number;
  recent_failure_stage?: string;
  recent_failure_message?: string;
  recent_failure_article_id?: number;
  recent_failure_article_title?: string;
  recent_failure_model?: string;
  recent_failure_endpoint?: string;
  recent_failure_at?: string;
  recent_failure_count?: number;
  embedding_health_blocked: boolean;
  embedding_health_sample_size: number;
  embedding_health_unnormalized_count: number;
  embedding_health_unnormalized_ratio: number;
}

export interface SystemMessage {
  id: number;
  user_id: number;
  kind: string;
  title: string;
  body: string;
  metadata_json?: string;
  is_read: boolean;
  read_at?: string;
  created_at: string;
  updated_at: string;
}

export interface SystemMessageListResponse {
  messages: SystemMessage[];
}

export interface SystemMessageUnreadCountResponse {
  unread_count: number;
}

export interface ClusterRenormalizeResponse {
  scheduled: boolean;
  reason?: 'busy' | 'disabled';
  message?: string;
}

export interface UserAIStats {
  ai_read_count: number;
  ai_total_read_time: number;
}

export interface Feed {
  id: number;
  url: string;
  title: string;
  category: string;
  last_fetched_at: string;
  position?: number; // Position within category for custom ordering
  is_discovered?: boolean;
  website_url?: string;
  image_url?: string;
  last_error?: string;
  script_path?: string;
  hide_from_timeline?: boolean;
  proxy_url?: string;
  proxy_enabled?: boolean;
  refresh_interval?: number;
  is_image_mode?: boolean;
  // XPath support
  type?: string;
  xpath_item?: string;
  xpath_item_title?: string;
  xpath_item_content?: string;
  xpath_item_uri?: string;
  xpath_item_author?: string;
  xpath_item_timestamp?: string;
  xpath_item_time_format?: string;
  xpath_item_thumbnail?: string;
  xpath_item_categories?: string;
  xpath_item_uid?: string;
  article_view_mode?: string; // Article view mode override ('global', 'webpage', 'rendered', 'external')
  auto_expand_content?: string; // Auto expand content mode ('global', 'enabled', 'disabled')
  // Email/Newsletter support
  email_address?: string;
  email_imap_server?: string;
  email_imap_port?: number;
  email_username?: string;
  email_password?: string;
  email_folder?: string;
  // FreshRSS integration
  is_freshrss_source?: boolean; // Whether this feed is from FreshRSS sync
  freshrss_stream_id?: string; // FreshRSS stream ID (e.g., "feed/http://...")
  // Translation settings
  translate_articles?: boolean; // Whether to translate articles in this feed (requires global translation_enabled)
  // Statistics
  latest_article_time?: string; // Latest article publish time
  articles_per_month?: number; // Average articles per month (calculated from last 90 days)
  last_update_status?: string; // Last update status ("success" or "failed")
  // Tags (populated by API handlers)
  tags?: Tag[]; // Tags assigned to this feed
}

export interface UnreadCounts {
  total: number;
  feedCounts: Record<number, number>;
}

export interface RefreshProgress {
  isRunning: boolean;
  errors?: Record<number, string>; // Map of feed ID to error message
  pool_task_count?: number; // Tasks currently in pool
  article_click_count?: number; // Article click triggered tasks
  queue_task_count?: number; // Tasks in queue
  pool_tasks?: PoolTaskInfo[]; // Detailed pool task information
  queue_tasks?: QueueTaskInfo[]; // Detailed queue task information (max 3)
}

export interface PoolTaskInfo {
  feed_id: number;
  feed_title: string;
  reason: number; // TaskReason enum value
  created_at: string;
}

export interface QueueTaskInfo {
  feed_id: number;
  feed_title: string;
  position: number;
}

export interface UpdateInfo {
  has_update: boolean;
  latest_version: string;
  current_version: string;
  download_url: string;
  release_notes: string;
  is_portable: boolean;
}

export interface Settings {
  update_interval: string;
  auto_cleanup_enabled: string;
  max_cache_size_mb: string;
  max_article_age_days: string;
  translation_enabled: string;
  target_language: string;
  translation_provider: string;
  deepl_api_key: string;
  language: string;
  theme: string;
  default_view_mode: string;
  show_hidden_articles: string;
  startup_on_boot: string;
}

export interface DiscoveredFeed {
  url: string;
  title: string;
  description?: string;
  articles?: Article[];
}

export interface Rule {
  id: number;
  name: string;
  enabled: boolean;
  condition: RuleCondition;
  actions: RuleAction[];
}

export interface RuleCondition {
  type: 'always' | 'filter';
  filter?: FilterCondition[];
}

export interface FilterCondition {
  field:
    | 'feed_name'
    | 'feed_category'
    | 'feed_tags'
    | 'article_title'
    | 'is_read'
    | 'is_favorite'
    | 'is_hidden'
    | 'is_read_later';
  operator: 'contains' | 'equals' | 'not_equals';
  value: string;
  logic?: 'and' | 'or' | 'not';
}

export type RuleAction =
  | { type: 'favorite' }
  | { type: 'unfavorite' }
  | { type: 'hide' }
  | { type: 'unhide' }
  | { type: 'mark_read' }
  | { type: 'mark_unread' }
  | { type: 'read_later' }
  | { type: 'remove_read_later' };

export interface KeyboardShortcut {
  action: string;
  key: string;
  defaultKey: string;
}
