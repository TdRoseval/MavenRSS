package models

import "time"

type Feed struct {
	ID                  int64      `json:"id"`
	UserID              int64      `json:"user_id"`
	Title               string     `json:"title"`
	URL                 string     `json:"url"`
	Link                string     `json:"link"`
	Description         string     `json:"description"`
	Category            string     `json:"category"`
	ImageURL            string     `json:"image_url"`
	Position            int        `json:"position"`
	LastUpdated         time.Time  `json:"last_updated"`
	LastError           string     `json:"last_error,omitempty"`
	DiscoveryCompleted  bool       `json:"discovery_completed"`
	ScriptPath          string     `json:"script_path,omitempty"`
	HideFromTimeline    bool       `json:"hide_from_timeline"`
	ProxyURL            string     `json:"proxy_url,omitempty"`
	ProxyEnabled        bool       `json:"proxy_enabled"`
	RefreshInterval     int        `json:"refresh_interval"`
	IsImageMode         bool       `json:"is_image_mode"`
	Type                string     `json:"type"`
	XPathItem           string     `json:"xpath_item"`
	XPathItemTitle      string     `json:"xpath_item_title"`
	XPathItemContent    string     `json:"xpath_item_content"`
	XPathItemUri        string     `json:"xpath_item_uri"`
	XPathItemAuthor     string     `json:"xpath_item_author"`
	XPathItemTimestamp  string     `json:"xpath_item_timestamp"`
	XPathItemTimeFormat string     `json:"xpath_item_time_format"`
	XPathItemThumbnail  string     `json:"xpath_item_thumbnail"`
	XPathItemCategories string     `json:"xpath_item_categories"`
	XPathItemUid        string     `json:"xpath_item_uid"`
	ArticleViewMode     string     `json:"article_view_mode"`
	AutoExpandContent   string     `json:"auto_expand_content"`
	EmailAddress        string     `json:"email_address,omitempty"`
	EmailIMAPServer     string     `json:"email_imap_server,omitempty"`
	EmailIMAPPort       int        `json:"email_imap_port"`
	EmailUsername       string     `json:"email_username,omitempty"`
	EmailPassword       string     `json:"email_password,omitempty"`
	EmailFolder         string     `json:"email_folder"`
	EmailLastUID        int        `json:"email_last_uid"`
	IsFreshRSSSource    bool       `json:"is_freshrss_source"`
	FreshRSSStreamID    string     `json:"freshrss_stream_id"`
	TranslateArticles   bool       `json:"translate_articles"`
	ETag                string     `json:"etag,omitempty"`
	LastModified        string     `json:"last_modified,omitempty"`
	LatestArticleTime   *time.Time `json:"latest_article_time,omitempty"`
	ArticlesPerMonth    float64    `json:"articles_per_month,omitempty"`
	LastUpdateStatus    string     `json:"last_update_status,omitempty"`
	Tags                []Tag      `json:"tags,omitempty"`
}

type Article struct {
	ID                    int64     `json:"id"`
	UserID                int64     `json:"user_id"`
	FeedID                int64     `json:"feed_id"`
	Title                 string    `json:"title"`
	URL                   string    `json:"url"`
	ImageURL              string    `json:"image_url"`
	AudioURL              string    `json:"audio_url"`
	VideoURL              string    `json:"video_url"`
	PublishedAt           time.Time `json:"published_at"`
	HasValidPublishedTime bool      `json:"-"`
	IsRead                bool      `json:"is_read"`
	IsFavorite            bool      `json:"is_favorite"`
	IsHidden              bool      `json:"is_hidden"`
	IsReadLater           bool      `json:"is_read_later"`
	FeedTitle             string    `json:"feed_title,omitempty"`
	Author                string    `json:"author,omitempty"`
	TranslatedTitle       string    `json:"translated_title"`
	Summary               string    `json:"summary"`
	UniqueID              string    `json:"unique_id"`
	FreshRSSItemID        string    `json:"freshrss_item_id"`
	ClusterID             int64     `json:"cluster_id,omitempty"`
	SimHash64             int64     `json:"-"`
}

type SavedFilter struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	Name       string    `json:"name"`
	Conditions string    `json:"conditions"`
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Tag struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Position int    `json:"position"`
}

type AIProfile struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	Name           string    `json:"name"`
	APIKey         string    `json:"api_key,omitempty"`
	Endpoint       string    `json:"endpoint"`
	Model          string    `json:"model"`
	CustomHeaders  string    `json:"custom_headers"`
	IsDefault      bool      `json:"is_default"`
	UseGlobalProxy bool      `json:"use_global_proxy"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type EmbeddingModelConfig struct {
	ModelName      string `json:"modelname"`
	BaseURL        string `json:"baseurl"`
	APIKey         string `json:"apikey"`
	RPM            int    `json:"rpm"`
	TPM            int    `json:"tpm"`
	UseGlobalProxy bool   `json:"use_global_proxy"`
}

type DailyRecommendation struct {
	ID                      int64     `json:"id"`
	UserID                  int64     `json:"user_id"`
	ClusterID               int64     `json:"cluster_id"`
	RecommendationDate      string    `json:"recommendation_date"`
	RecommendationScore     float64   `json:"recommendation_score"`
	RecommendationRank      int       `json:"recommendation_rank"`
	RecommendationProfileID int64     `json:"recommendation_profile_id"`
	CreatedAt               time.Time `json:"created_at"`
}

type Cluster struct {
	ID                        int64     `json:"id"`
	UserID                    int64     `json:"user_id"`
	Status                    string    `json:"status"`
	MergedTitle               string    `json:"merged_title"`
	DisplayTitle              string    `json:"display_title,omitempty"`
	MergedSummary             string    `json:"merged_summary"`
	MergedContent             string    `json:"merged_content"`
	ArticleCount              int       `json:"article_count"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
	IsRead                    bool      `json:"is_read"`
	IsFavorite                bool      `json:"is_favorite"`
	IsReadLater               bool      `json:"is_read_later"`
	IsHidden                  bool      `json:"is_hidden"`
	RecommendationArchiveDate string    `json:"recommendation_archive_date,omitempty"`
	RecommendationScore       float64   `json:"recommendation_score,omitempty"`
	IsAIRecommended           bool      `json:"is_ai_recommended"`
	RecommendationProfileID   int64     `json:"recommendation_profile_id,omitempty"`
	FeedTitles                []string  `json:"feed_titles,omitempty"`
	Authors                   []string  `json:"authors,omitempty"`
	Articles                  []Article `json:"articles,omitempty"`
}
