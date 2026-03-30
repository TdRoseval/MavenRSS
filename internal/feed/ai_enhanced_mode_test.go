package feed

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"MavenRSS/internal/ai"
	"MavenRSS/internal/dedup"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

func TestShouldProcessUsesUserProfilesWithoutGlobalAIKey(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)

	if _, err := db.CreateAIProfile(&models.AIProfile{
		UserID:   1,
		Name:     "summary",
		APIKey:   "summary-key",
		Endpoint: "https://summary.example.com",
		Model:    "summary-model",
	}); err != nil {
		t.Fatalf("CreateAIProfile summary error: %v", err)
	}

	if _, err := db.CreateAIProfile(&models.AIProfile{
		UserID:   1,
		Name:     "translation",
		APIKey:   "translation-key",
		Endpoint: "https://translation.example.com",
		Model:    "translation-model",
	}); err != nil {
		t.Fatalf("CreateAIProfile translation error: %v", err)
	}

	if _, err := db.CreateAIProfile(&models.AIProfile{
		UserID:   1,
		Name:     "search",
		APIKey:   "search-key",
		Endpoint: "https://search.example.com",
		Model:    "search-model",
	}); err != nil {
		t.Fatalf("CreateAIProfile search error: %v", err)
	}

	if _, err := db.CreateAIProfile(&models.AIProfile{
		UserID:   1,
		Name:     "chat",
		APIKey:   "chat-key",
		Endpoint: "https://chat.example.com",
		Model:    "chat-model",
	}); err != nil {
		t.Fatalf("CreateAIProfile chat error: %v", err)
	}

	if _, err := db.CreateAIProfile(&models.AIProfile{
		UserID:   1,
		Name:     "fusion",
		APIKey:   "fusion-key",
		Endpoint: "https://fusion.example.com",
		Model:    "fusion-model",
	}); err != nil {
		t.Fatalf("CreateAIProfile fusion error: %v", err)
	}

	mustSetUserSetting(t, db, 1, "ai_enhanced_mode", "true")
	mustSetUserSetting(t, db, 1, "ai_embedding_models", `[{"modelname":"embed","baseurl":"https://embed.example.com","apikey":"embed-key","rpm":0,"tpm":0,"use_global_proxy":true}]`)
	mustSetUserSetting(t, db, 1, "summary_enabled", "true")
	mustSetUserSetting(t, db, 1, "summary_provider", "ai")
	mustSetUserSetting(t, db, 1, "translation_enabled", "true")
	mustSetUserSetting(t, db, 1, "ai_search_enabled", "true")
	mustSetUserSetting(t, db, 1, "ai_chat_enabled", "true")

	if !ShouldProcess(db, 1) {
		t.Fatal("ShouldProcess() = false, want true when user profiles are configured")
	}
}

func TestShouldProcessIgnoresGlobalAIKeyWithoutUserConfig(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)

	mustSetGlobalSetting(t, db, "ai_enhanced_mode", "true")
	mustSetGlobalSetting(t, db, "summary_enabled", "true")
	mustSetGlobalSetting(t, db, "summary_provider", "ai")
	mustSetGlobalSetting(t, db, "translation_enabled", "true")
	mustSetGlobalSetting(t, db, "ai_search_enabled", "true")
	mustSetGlobalSetting(t, db, "ai_chat_enabled", "true")
	mustSetGlobalEncryptedSetting(t, db, "ai_api_key", "global-key")
	mustSetGlobalSetting(t, db, "ai_endpoint", "https://global.example.com")
	mustSetGlobalSetting(t, db, "ai_model", "global-model")

	if ShouldProcess(db, 1) {
		t.Fatal("ShouldProcess() = true, want false when only global AI config exists")
	}
}

func TestGetUserFeatureAIConfigPrefersUserProfileAndFallsBackToUserLegacy(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)

	profileID, err := db.CreateAIProfile(&models.AIProfile{
		UserID:   1,
		Name:     "summary",
		APIKey:   "profile-key",
		Endpoint: "https://profile.example.com",
		Model:    "profile-model",
	})
	if err != nil {
		t.Fatalf("CreateAIProfile error: %v", err)
	}

	mustSetUserSetting(t, db, 1, "ai_summary_profile_id", "999999")

	cfg, err := getUserFeatureAIConfig(db, 1, ai.FeatureSummary)
	if err != nil {
		t.Fatalf("getUserFeatureAIConfig invalid selected profile error: %v", err)
	}
	if cfg == nil || cfg.APIKey != "profile-key" || cfg.Endpoint != "https://profile.example.com" || cfg.Model != "profile-model" {
		t.Fatalf("getUserFeatureAIConfig() with default profile = %#v, want default user profile config", cfg)
	}

	mustSetUserSetting(t, db, 1, "ai_summary_profile_id", "")
	if err := db.DeleteAIProfileForUser(1, profileID); err != nil {
		t.Fatalf("DeleteAIProfileForUser error: %v", err)
	}
	mustSetUserEncryptedSetting(t, db, 1, "ai_api_key", "legacy-key")
	mustSetUserSetting(t, db, 1, "ai_endpoint", "https://legacy.example.com")
	mustSetUserSetting(t, db, 1, "ai_model", "legacy-model")
	mustSetUserSetting(t, db, 1, "ai_custom_headers", `{"X-Test":"1"}`)

	cfg, err = getUserFeatureAIConfig(db, 1, ai.FeatureSummary)
	if err != nil {
		t.Fatalf("getUserFeatureAIConfig legacy error: %v", err)
	}
	if cfg == nil || cfg.APIKey != "legacy-key" || cfg.Endpoint != "https://legacy.example.com" || cfg.Model != "legacy-model" || cfg.CustomHeaders != `{"X-Test":"1"}` {
		t.Fatalf("getUserFeatureAIConfig() legacy = %#v, want user legacy config", cfg)
	}
}

func TestBuildFusionConfigUsesFusionProfile(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := NewAIEnhancedManager(db)
	defer manager.Stop()

	if _, err := db.CreateAIProfile(&models.AIProfile{
		UserID:         1,
		Name:           "summary",
		APIKey:         "summary-key",
		Endpoint:       "https://summary.example.com",
		Model:          "summary-model",
		CustomHeaders:  `{"X-Summary":"1"}`,
		UseGlobalProxy: false,
	}); err != nil {
		t.Fatalf("CreateAIProfile summary error: %v", err)
	}

	fusionProfileID, err := db.CreateAIProfile(&models.AIProfile{
		UserID:         1,
		Name:           "fusion",
		APIKey:         "fusion-key",
		Endpoint:       "https://fusion.example.com",
		Model:          "fusion-model",
		CustomHeaders:  `{"X-Fusion":"1"}`,
		UseGlobalProxy: true,
	})
	if err != nil {
		t.Fatalf("CreateAIProfile fusion error: %v", err)
	}

	mustSetUserSetting(t, db, 1, "ai_fusion_profile_id", "999999")
	mustSetUserSetting(t, db, 1, "ai_embedding_models", `[{"modelname":"embed","baseurl":"https://embed.example.com","apikey":"embed-key","rpm":0,"tpm":0,"use_global_proxy":true}]`)
	mustSetUserSetting(t, db, 1, "proxy_enabled", "true")
	mustSetUserSetting(t, db, 1, "proxy_type", "http")
	mustSetUserSetting(t, db, 1, "proxy_host", "127.0.0.1")
	mustSetUserSetting(t, db, 1, "proxy_port", "8080")
	mustSetUserSetting(t, db, 1, "ai_summary_profile_id", "123456")

	cfg, err := manager.buildFusionConfig(1)
	if err != nil {
		t.Fatalf("buildFusionConfig fallback error: %v", err)
	}
	if cfg == nil || cfg.Summarizer == nil {
		t.Fatal("buildFusionConfig() returned nil config or summarizer")
	}
	if cfg.Summarizer.APIKey != "summary-key" || cfg.Summarizer.Endpoint != "https://summary.example.com" || cfg.Summarizer.Model != "summary-model" {
		t.Fatalf("buildFusionConfig() fallback = %#v, want default user profile config", cfg.Summarizer)
	}

	mustSetUserSetting(t, db, 1, "ai_fusion_profile_id", "9999999")
	cfg, err = manager.buildFusionConfig(1)
	if err != nil {
		t.Fatalf("buildFusionConfig invalid selected profile error: %v", err)
	}
	if cfg.Summarizer.APIKey != "summary-key" || cfg.Summarizer.Endpoint != "https://summary.example.com" || cfg.Summarizer.Model != "summary-model" {
		t.Fatalf("buildFusionConfig() invalid selected profile = %#v, want fallback user profile config", cfg.Summarizer)
	}

	mustSetUserSetting(t, db, 1, "ai_fusion_profile_id", "0")
	cfg, err = manager.buildFusionConfig(1)
	if err != nil {
		t.Fatalf("buildFusionConfig zero profile error: %v", err)
	}
	if cfg.Summarizer.APIKey != "summary-key" || cfg.Summarizer.Endpoint != "https://summary.example.com" || cfg.Summarizer.Model != "summary-model" {
		t.Fatalf("buildFusionConfig() zero profile = %#v, want fallback user profile config", cfg.Summarizer)
	}

	if err := db.SetDefaultAIProfileForUser(1, fusionProfileID); err != nil {
		t.Fatalf("SetDefaultAIProfileForUser error: %v", err)
	}
	mustSetUserSetting(t, db, 1, "ai_fusion_profile_id", strconv.FormatInt(fusionProfileID, 10))

	cfg, err = manager.buildFusionConfig(1)
	if err != nil {
		t.Fatalf("buildFusionConfig selected fusion profile error: %v", err)
	}
	if cfg.Summarizer.APIKey != "fusion-key" || cfg.Summarizer.Endpoint != "https://fusion.example.com" || cfg.Summarizer.Model != "fusion-model" {
		t.Fatalf("buildFusionConfig() = %#v, want fusion profile config", cfg.Summarizer)
	}
	if cfg.Summarizer.CustomHeaders != `{"X-Fusion":"1"}` {
		t.Fatalf("buildFusionConfig() custom headers = %q, want fusion headers", cfg.Summarizer.CustomHeaders)
	}
	if cfg.EmbConfigsJSON != `[{"modelname":"embed","baseurl":"https://embed.example.com","apikey":"embed-key","rpm":0,"tpm":0,"use_global_proxy":true}]` {
		t.Fatalf("buildFusionConfig() embedding config = %q", cfg.EmbConfigsJSON)
	}
	if cfg.GlobalProxyURL != "http://127.0.0.1:8080" {
		t.Fatalf("buildFusionConfig() global proxy URL = %q, want %q", cfg.GlobalProxyURL, "http://127.0.0.1:8080")
	}
}

func TestScheduleClusterPipelineCoalescesRepeatedRequests(t *testing.T) {
	started := make(chan int, 2)
	releaseFirst := make(chan struct{})
	done := make(chan struct{})

	manager := &AIEnhancedManager{
		clusterPipelineRunning: make(map[int64]bool),
		clusterPipelineQueued:  make(map[int64]bool),
		resolveFusionConfig: func(userID int64) (*dedup.FusionConfig, error) {
			return &dedup.FusionConfig{}, nil
		},
	}

	var fusionRuns int32
	var embeddingRuns int32

	manager.runFusion = func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
		run := int(atomic.AddInt32(&fusionRuns, 1))
		started <- run
		if run == 1 {
			<-releaseFirst
		}
		return nil
	}
	manager.runEmbedding = func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
		if atomic.AddInt32(&embeddingRuns, 1) == 2 {
			close(done)
		}
		return nil
	}

	manager.scheduleClusterPipeline(1)
	if got := <-started; got != 1 {
		t.Fatalf("first fusion run = %d, want 1", got)
	}

	manager.scheduleClusterPipeline(1)
	close(releaseFirst)

	if got := <-started; got != 2 {
		t.Fatalf("second fusion run = %d, want 2", got)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cluster pipeline did not finish queued rerun")
	}

	if atomic.LoadInt32(&fusionRuns) != 2 {
		t.Fatalf("fusion runs = %d, want 2", atomic.LoadInt32(&fusionRuns))
	}
	if atomic.LoadInt32(&embeddingRuns) != 2 {
		t.Fatalf("embedding runs = %d, want 2", atomic.LoadInt32(&embeddingRuns))
	}
}

func TestRunClusterPipelineOnceRunsFusionThenEmbedding(t *testing.T) {
	manager := &AIEnhancedManager{
		clusterPipelineRunning: make(map[int64]bool),
		clusterPipelineQueued:  make(map[int64]bool),
		resolveFusionConfig: func(userID int64) (*dedup.FusionConfig, error) {
			return &dedup.FusionConfig{}, nil
		},
	}

	var mu sync.Mutex
	var steps []string

	manager.runFusion = func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
		mu.Lock()
		steps = append(steps, "fusion")
		mu.Unlock()
		return nil
	}
	manager.runEmbedding = func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
		mu.Lock()
		steps = append(steps, "embedding")
		mu.Unlock()
		return nil
	}

	if err := manager.runClusterPipelineOnce(7); err != nil {
		t.Fatalf("runClusterPipelineOnce error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(steps) != 2 || steps[0] != "fusion" || steps[1] != "embedding" {
		t.Fatalf("run order = %v, want [fusion embedding]", steps)
	}
}

func TestGetArticlesForAIBatchProcessingFiltersCompletedAndScope(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)

	feedID := mustCreateTestFeed(t, db, 1, false)
	completeFavorite := mustInsertBatchArticle(t, db, 1, feedID, true, time.Now().Add(-7*24*time.Hour), "complete-favorite", true)
	mustAttachCompleteCluster(t, db, 1, completeFavorite, "complete")

	incompleteFavorite := mustInsertBatchArticle(t, db, 1, feedID, true, time.Now().Add(-10*24*time.Hour), "incomplete-favorite", true)
	incompleteRecent := mustInsertBatchArticle(t, db, 1, feedID, false, time.Now().Add(-12*time.Hour), "incomplete-recent", true)
	oldNonFavorite := mustInsertBatchArticle(t, db, 1, feedID, false, time.Now().Add(-5*24*time.Hour), "old-non-favorite", true)
	completeRecent := mustInsertBatchArticle(t, db, 1, feedID, false, time.Now().Add(-6*time.Hour), "complete-recent", true)
	mustAttachCompleteCluster(t, db, 1, completeRecent, "complete")

	articles, err := db.GetArticlesForAIBatchProcessing(1, "zh")
	if err != nil {
		t.Fatalf("GetArticlesForAIBatchProcessing error: %v", err)
	}

	got := make(map[int64]bool, len(articles))
	for _, article := range articles {
		got[article.Article.ID] = true
	}

	if !got[incompleteFavorite] {
		t.Fatalf("missing incomplete favorite article %d in batch candidates", incompleteFavorite)
	}
	if !got[incompleteRecent] {
		t.Fatalf("missing incomplete recent article %d in batch candidates", incompleteRecent)
	}
	if got[completeFavorite] {
		t.Fatalf("complete favorite article %d should be skipped", completeFavorite)
	}
	if got[completeRecent] {
		t.Fatalf("complete recent article %d should be skipped", completeRecent)
	}
	if got[oldNonFavorite] {
		t.Fatalf("old non-favorite article %d should be out of scope", oldNonFavorite)
	}
}

func TestBatchProcessExistingArticlesSchedulesClusterPipelineWithoutRequeueingCompleteArticle(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)

	feedID := mustCreateTestFeed(t, db, 1, false)
	articleID := mustInsertBatchArticle(t, db, 1, feedID, true, time.Now().Add(-3*24*time.Hour), "cluster-only", true)
	clusterID, err := db.CreateCluster(1, "pending_embed")
	if err != nil {
		t.Fatalf("CreateCluster error: %v", err)
	}
	if err := db.UpdateArticleClusterID(articleID, clusterID); err != nil {
		t.Fatalf("UpdateArticleClusterID error: %v", err)
	}
	if err := db.UpdateClusterArticleCount(clusterID); err != nil {
		t.Fatalf("UpdateClusterArticleCount error: %v", err)
	}
	if err := db.UpdateClusterMergedContent(clusterID, "merged title", "merged summary", "merged content"); err != nil {
		t.Fatalf("UpdateClusterMergedContent error: %v", err)
	}
	if err := db.UpdateArticleEmbeddings(articleID, mustEmbeddingBlob(t), mustEmbeddingBlob(t)); err != nil {
		t.Fatalf("UpdateArticleEmbeddings error: %v", err)
	}

	started := make(chan struct{}, 1)
	manager := &AIEnhancedManager{
		db:                     db,
		taskChan:               make(chan *AIEnhancedTask, 10),
		clusterPipelineRunning: make(map[int64]bool),
		clusterPipelineQueued:  make(map[int64]bool),
		resolveFusionConfig: func(userID int64) (*dedup.FusionConfig, error) {
			return &dedup.FusionConfig{}, nil
		},
		runFusion: func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
			select {
			case started <- struct{}{}:
			default:
			}
			return nil
		},
		runEmbedding: func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
			return nil
		},
	}

	manager.BatchProcessExistingArticles(1)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("cluster pipeline was not scheduled for cluster-only backfill")
	}

	if len(manager.taskChan) != 0 {
		t.Fatalf("unexpected queued article tasks: got %d, want 0", len(manager.taskChan))
	}
}

func newAIEnhancedModeTestDB(t *testing.T) *sqlite.DB {
	t.Helper()

	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("db Init error: %v", err)
	}

	return db
}

func mustSetGlobalSetting(t *testing.T, db *sqlite.DB, key, value string) {
	t.Helper()
	if err := db.SetSetting(key, value); err != nil {
		t.Fatalf("SetSetting(%s) error: %v", key, err)
	}
}

func mustSetUserSetting(t *testing.T, db *sqlite.DB, userID int64, key, value string) {
	t.Helper()
	if err := db.SetSettingForUser(userID, key, value); err != nil {
		t.Fatalf("SetSettingForUser(%s) error: %v", key, err)
	}
}

func mustSetGlobalEncryptedSetting(t *testing.T, db *sqlite.DB, key, value string) {
	t.Helper()
	if err := db.SetEncryptedSetting(key, value); err != nil {
		t.Fatalf("SetEncryptedSetting(%s) error: %v", key, err)
	}
}

func mustSetUserEncryptedSetting(t *testing.T, db *sqlite.DB, userID int64, key, value string) {
	t.Helper()
	if err := db.SetEncryptedSettingForUser(userID, key, value); err != nil {
		t.Fatalf("SetEncryptedSettingForUser(%s) error: %v", key, err)
	}
}

func mustCreateTestFeed(t *testing.T, db *sqlite.DB, userID int64, translateArticles bool) int64 {
	t.Helper()
	feedID, err := db.AddFeedForUser(userID, &models.Feed{
		Title:             "Feed",
		URL:               "https://example.com/feed/" + strconv.FormatInt(time.Now().UnixNano(), 10),
		Type:              "rss",
		RefreshInterval:   60,
		TranslateArticles: translateArticles,
	})
	if err != nil {
		t.Fatalf("SaveFeedForUser error: %v", err)
	}
	return feedID
}

func mustInsertBatchArticle(t *testing.T, db *sqlite.DB, userID, feedID int64, isFavorite bool, publishedAt time.Time, uniqueID string, withSummary bool) int64 {
	t.Helper()
	summary := ""
	if withSummary {
		summary = "用于测试的摘要内容，长度足以通过有效性检查"
	}
	result, err := db.Exec(
		`INSERT INTO articles (user_id, feed_id, title, url, published_at, is_favorite, summary, unique_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, feedID, uniqueID, "https://example.com/articles/"+uniqueID, publishedAt, isFavorite, summary, uniqueID,
	)
	if err != nil {
		t.Fatalf("insert article error: %v", err)
	}
	articleID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId error: %v", err)
	}
	if err := db.SetArticleContent(articleID, "用于测试的正文内容"); err != nil {
		t.Fatalf("SetArticleContent error: %v", err)
	}
	return articleID
}

func mustAttachCompleteCluster(t *testing.T, db *sqlite.DB, userID, articleID int64, status string) {
	t.Helper()
	clusterID, err := db.CreateCluster(userID, status)
	if err != nil {
		t.Fatalf("CreateCluster error: %v", err)
	}
	if err := db.UpdateArticleClusterID(articleID, clusterID); err != nil {
		t.Fatalf("UpdateArticleClusterID error: %v", err)
	}
	if err := db.UpdateClusterArticleCount(clusterID); err != nil {
		t.Fatalf("UpdateClusterArticleCount error: %v", err)
	}
	if err := db.UpdateClusterMergedContent(clusterID, "merged title", "merged summary", "merged content"); err != nil {
		t.Fatalf("UpdateClusterMergedContent error: %v", err)
	}
	if err := db.UpdateArticleEmbeddings(articleID, mustEmbeddingBlob(t), mustEmbeddingBlob(t)); err != nil {
		t.Fatalf("UpdateArticleEmbeddings error: %v", err)
	}
	if err := db.UpdateClusterEmbeddings(clusterID, mustEmbeddingBlob(t), mustEmbeddingBlob(t)); err != nil {
		t.Fatalf("UpdateClusterEmbeddings error: %v", err)
	}
}

func mustEmbeddingBlob(t *testing.T) []byte {
	t.Helper()
	blob, err := sqlite_vec.SerializeFloat32(make([]float32, 1024))
	if err != nil {
		t.Fatalf("SerializeFloat32 error: %v", err)
	}
	return blob
}
