package feed

import (
	"bytes"
	"context"
	"strconv"
	"strings"
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
	mustSetUserSetting(t, db, 1, "ai_fusion_enabled", "true")
	mustSetUserSetting(t, db, 1, "ai_recommendation_enabled", "true")
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
	mustSetGlobalSetting(t, db, "ai_fusion_enabled", "true")
	mustSetGlobalSetting(t, db, "ai_recommendation_enabled", "true")
	mustSetGlobalSetting(t, db, "ai_search_enabled", "true")
	mustSetGlobalSetting(t, db, "ai_chat_enabled", "true")
	mustSetGlobalEncryptedSetting(t, db, "ai_api_key", "global-key")
	mustSetGlobalSetting(t, db, "ai_endpoint", "https://global.example.com")
	mustSetGlobalSetting(t, db, "ai_model", "global-model")

	if ShouldProcess(db, 1) {
		t.Fatal("ShouldProcess() = true, want false when only global AI config exists")
	}
}

func TestShouldInspectUserForRecoverySkipsDisabledUsers(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := NewAIEnhancedManager(db)
	t.Cleanup(manager.Stop)

	if manager.shouldInspectUserForRecovery(1) {
		t.Fatal("shouldInspectUserForRecovery() = true, want false when AI enhanced mode is disabled")
	}
}

func TestShouldProcessRequiresFusionAndRecommendationEnabled(t *testing.T) {
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

	if ShouldProcess(db, 1) {
		t.Fatal("ShouldProcess() = true, want false when fusion/recommendation toggles are disabled")
	}

	mustSetUserSetting(t, db, 1, "ai_fusion_enabled", "true")
	if ShouldProcess(db, 1) {
		t.Fatal("ShouldProcess() = true, want false when recommendation toggle is disabled")
	}

	mustSetUserSetting(t, db, 1, "ai_recommendation_enabled", "true")
	if !ShouldProcess(db, 1) {
		t.Fatal("ShouldProcess() = false, want true when fusion/recommendation toggles are enabled")
	}
}

func TestShouldProcessRequiresUsableEmbeddingModelConfig(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	mustEnableAIEnhancedProcessing(t, db, 1)

	mustSetUserSetting(t, db, 1, "ai_embedding_models", `[{"modelname":"","baseurl":"https://embed.example.com","apikey":"embed-key"}]`)
	if ShouldProcess(db, 1) {
		t.Fatal("ShouldProcess() = true, want false when embedding model name is missing")
	}

	mustSetUserSetting(t, db, 1, "ai_embedding_models", `[{"modelname":"embed-v1","baseurl":"","apikey":"embed-key"}]`)
	if ShouldProcess(db, 1) {
		t.Fatal("ShouldProcess() = true, want false when embedding model baseurl is missing")
	}

	mustSetUserSetting(t, db, 1, "ai_embedding_models", `[{"modelname":"embed-v1","baseurl":"https://embed.example.com","apikey":"embed-key"}]`)
	if !ShouldProcess(db, 1) {
		t.Fatal("ShouldProcess() = false, want true when embedding model config is usable")
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

func TestBuildFusionConfigFallsBackToSummaryProfileWhenFusionProfileMissing(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := NewAIEnhancedManager(db)
	defer manager.Stop()

	summaryProfileID, err := db.CreateAIProfile(&models.AIProfile{
		UserID:         1,
		Name:           "summary-only",
		APIKey:         "summary-key",
		Endpoint:       "https://summary.example.com",
		Model:          "summary-model",
		CustomHeaders:  `{"X-Summary":"1"}`,
		UseGlobalProxy: false,
	})
	if err != nil {
		t.Fatalf("CreateAIProfile summary error: %v", err)
	}

	mustSetUserSetting(t, db, 1, "ai_fusion_profile_id", "")
	mustSetUserSetting(t, db, 1, "ai_summary_profile_id", strconv.FormatInt(summaryProfileID, 10))
	mustSetUserSetting(t, db, 1, "target_language", "zh")

	cfg, err := manager.buildFusionConfig(1)
	if err != nil {
		t.Fatalf("buildFusionConfig summary fallback error: %v", err)
	}
	if cfg == nil || cfg.Summarizer == nil {
		t.Fatal("buildFusionConfig() returned nil summarizer, want summary-profile fallback")
	}
	if cfg.Summarizer.APIKey != "summary-key" || cfg.Summarizer.Endpoint != "https://summary.example.com" || cfg.Summarizer.Model != "summary-model" {
		t.Fatalf("buildFusionConfig() summary fallback = %#v", cfg.Summarizer)
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

func TestRequestClusterPipelineWaitsForArticleWorkToDrain(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)

	if _, err := db.CreateCluster(1, "pending_merge"); err != nil {
		t.Fatalf("CreateCluster error: %v", err)
	}

	started := make(chan struct{}, 1)
	manager := &AIEnhancedManager{
		db:                       db,
		taskChan:                 make(chan *AIEnhancedTask, 10),
		stopChan:                 make(chan struct{}),
		queuedTasksByUser:        map[int64]int{1: 1},
		activeWorkerTasksByUser:  make(map[int64]int64),
		activeAsyncWorkByUser:    map[int64]int64{1: 1},
		clusterPipelineRunning:   make(map[int64]bool),
		clusterPipelineQueued:    make(map[int64]bool),
		clusterPipelineRequested: make(map[int64]bool),
		clusterFusionRunning:     make(map[int64]bool),
		clusterEmbeddingRunning:  make(map[int64]bool),
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

	manager.requestClusterPipeline(1)

	select {
	case <-started:
		t.Fatal("cluster pipeline started before article work drained")
	case <-time.After(150 * time.Millisecond):
	}

	manager.statusMu.Lock()
	delete(manager.queuedTasksByUser, 1)
	delete(manager.activeAsyncWorkByUser, 1)
	manager.statusMu.Unlock()

	if !manager.maybeStartRequestedClusterPipeline(1) {
		t.Fatal("maybeStartRequestedClusterPipeline() = false, want true after article work drained")
	}

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("cluster pipeline did not start after article work drained")
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
	if !got[completeFavorite] {
		t.Fatalf("complete favorite article %d should remain in batch candidates for favorite cluster repair", completeFavorite)
	}
	if got[completeRecent] {
		t.Fatalf("complete recent article %d should be skipped", completeRecent)
	}
	if got[oldNonFavorite] {
		t.Fatalf("old non-favorite article %d should be out of scope", oldNonFavorite)
	}
}

func TestGetArticlesForAIBatchProcessingTreatsSkippedTranslationAsResolved(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)

	feedID := mustCreateTestFeed(t, db, 1, true)
	articleID := mustInsertBatchArticle(t, db, 1, feedID, false, time.Now().Add(-2*time.Hour), "skip-translation", true)

	if err := db.UpdateArticleSummary(articleID, "done"); err != nil {
		t.Fatalf("UpdateArticleSummary error: %v", err)
	}
	mustAttachCompleteCluster(t, db, 1, articleID, "complete")
	if err := db.SetAIArticleStageSkip(1, articleID, "translation", "code=1301"); err != nil {
		t.Fatalf("SetAIArticleStageSkip error: %v", err)
	}

	articles, err := db.GetArticlesForAIBatchProcessing(1, "zh")
	if err != nil {
		t.Fatalf("GetArticlesForAIBatchProcessing error: %v", err)
	}

	for _, article := range articles {
		if article.Article.ID == articleID {
			t.Fatalf("article %d should be skipped after non-recoverable translation failure", articleID)
		}
	}
}

func TestGetArticlesForAIBatchProcessingIncludesArticlesWithoutContentForTitleFallback(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)

	feedID := mustCreateTestFeed(t, db, 1, true)
	articleID := mustInsertBatchArticle(t, db, 1, feedID, false, time.Now().Add(-2*time.Hour), "no-content", false)
	if err := db.DeleteArticleContent(articleID); err != nil {
		t.Fatalf("DeleteArticleContent error: %v", err)
	}

	articles, err := db.GetArticlesForAIBatchProcessing(1, "zh")
	if err != nil {
		t.Fatalf("GetArticlesForAIBatchProcessing error: %v", err)
	}

	found := false
	for _, article := range articles {
		if article.Article.ID == articleID {
			found = true
		}
	}
	if !found {
		t.Fatalf("article %d should remain eligible so title fallback can be generated", articleID)
	}
}

func TestGetProcessingStatusCountsEligibleAndPendingArticles(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := &AIEnhancedManager{
		db:                        db,
		taskChan:                  make(chan *AIEnhancedTask, 10),
		queuedTasksByUser:         make(map[int64]int),
		activeWorkerTasksByUser:   make(map[int64]int64),
		activeAsyncWorkByUser:     make(map[int64]int64),
		recoveryInProgress:        make(map[int64]bool),
		lastRecoveryAttemptByUser: make(map[int64]time.Time),
		clusterPipelineRunning:    make(map[int64]bool),
		clusterPipelineQueued:     make(map[int64]bool),
		recommendationRunning:     make(map[int64]bool),
		pendingRecommendationDate: make(map[int64]string),
		pendingRecommendationWait: make(map[int64]bool),
	}

	mustEnableAIEnhancedProcessing(t, db, 1)

	feedID := mustCreateTestFeed(t, db, 1, false)
	completedArticleID := mustInsertBatchArticle(
		t,
		db,
		1,
		feedID,
		true,
		time.Now().Add(-12*time.Hour),
		"completed-article",
		true,
	)
	mustAttachCompleteCluster(t, db, 1, completedArticleID, "complete")

	_ = mustInsertBatchArticle(
		t,
		db,
		1,
		feedID,
		false,
		time.Now().Add(-6*time.Hour),
		"pending-article",
		true,
	)

	status := manager.GetProcessingStatus(1)

	if !status.IsEnabled {
		t.Fatal("GetProcessingStatus().IsEnabled = false, want true")
	}
	if status.EligibleArticles != 2 {
		t.Fatalf("EligibleArticles = %d, want 2", status.EligibleArticles)
	}
	if status.CompletedArticles != 1 {
		t.Fatalf("CompletedArticles = %d, want 1", status.CompletedArticles)
	}
	if status.PendingArticles != 1 {
		t.Fatalf("PendingArticles = %d, want 1", status.PendingArticles)
	}
	if !status.IsConfigFrozen {
		t.Fatal("IsConfigFrozen = false, want true while pending work exists")
	}
	if status.ProgressPercent != 50 {
		t.Fatalf("ProgressPercent = %v, want 50", status.ProgressPercent)
	}
}

func TestGetProcessingStatusDoesNotEnableFreezeFromRawToggleAlone(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := &AIEnhancedManager{
		db:                        db,
		taskChan:                  make(chan *AIEnhancedTask, 10),
		queuedTasksByUser:         make(map[int64]int),
		activeWorkerTasksByUser:   make(map[int64]int64),
		activeAsyncWorkByUser:     make(map[int64]int64),
		recoveryInProgress:        make(map[int64]bool),
		lastRecoveryAttemptByUser: make(map[int64]time.Time),
		clusterPipelineRunning:    make(map[int64]bool),
		clusterPipelineQueued:     make(map[int64]bool),
		recommendationRunning:     make(map[int64]bool),
		pendingRecommendationDate: make(map[int64]string),
		pendingRecommendationWait: make(map[int64]bool),
	}

	mustSetUserSetting(t, db, 1, "ai_enhanced_mode", "true")

	feedID := mustCreateTestFeed(t, db, 1, false)
	_ = mustInsertBatchArticle(
		t,
		db,
		1,
		feedID,
		false,
		time.Now().Add(-30*time.Minute),
		"pending-without-prerequisites",
		false,
	)

	status := manager.GetProcessingStatus(1)
	if status.IsEnabled {
		t.Fatal("GetProcessingStatus().IsEnabled = true, want false when only raw toggle is enabled")
	}
	if status.IsConfigFrozen {
		t.Fatal("GetProcessingStatus().IsConfigFrozen = true, want false when prerequisites are incomplete")
	}
	if status.PendingArticles == 0 {
		t.Fatal("PendingArticles = 0, want pending work to remain observable for diagnostics")
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

func TestQueueExistingArticlesForProcessingRepairsFavoriteClusterForCompletedFavoriteArticle(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)

	feedID := mustCreateTestFeed(t, db, 1, false)
	articleID := mustInsertBatchArticle(t, db, 1, feedID, true, time.Now().Add(-7*24*time.Hour), "favorite-repair", true)
	clusterID := mustAttachCompleteCluster(t, db, 1, articleID, "complete")

	if err := db.SetClusterFavorite(clusterID, false); err != nil {
		t.Fatalf("SetClusterFavorite(false) error: %v", err)
	}

	manager := &AIEnhancedManager{
		db:       db,
		taskChan: make(chan *AIEnhancedTask, 10),
	}

	queued, err := manager.queueExistingArticlesForProcessing(1)
	if err != nil {
		t.Fatalf("queueExistingArticlesForProcessing error: %v", err)
	}
	if queued != 0 {
		t.Fatalf("queued = %d, want 0 for completed favorite article", queued)
	}
	if len(manager.taskChan) != 0 {
		t.Fatalf("unexpected queued article tasks: got %d, want 0", len(manager.taskChan))
	}

	cluster, err := db.GetClusterByID(clusterID)
	if err != nil {
		t.Fatalf("GetClusterByID error: %v", err)
	}
	if cluster == nil || !cluster.IsFavorite {
		t.Fatalf("cluster favorite state = %v, want true after repair", cluster != nil && cluster.IsFavorite)
	}
}

func TestRunClusterPipelineOnceInitializesInterestVectorFromFavoriteClusters(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)

	clusterID, err := db.CreateCluster(1, "complete")
	if err != nil {
		t.Fatalf("CreateCluster error: %v", err)
	}
	if err := db.SetClusterFavorite(clusterID, true); err != nil {
		t.Fatalf("SetClusterFavorite error: %v", err)
	}

	unitBlob := mustUnitEmbeddingBlob(t)
	if err := db.UpdateClusterEmbeddings(clusterID, unitBlob, unitBlob); err != nil {
		t.Fatalf("UpdateClusterEmbeddings error: %v", err)
	}

	manager := &AIEnhancedManager{
		db: db,
		resolveFusionConfig: func(userID int64) (*dedup.FusionConfig, error) {
			return &dedup.FusionConfig{}, nil
		},
		runFusion: func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
			return nil
		},
		runEmbedding: func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
			return nil
		},
	}

	if err := manager.runClusterPipelineOnce(1); err != nil {
		t.Fatalf("runClusterPipelineOnce error: %v", err)
	}

	interestBlob, err := db.GetUserInterestVector(1)
	if err != nil {
		t.Fatalf("GetUserInterestVector error: %v", err)
	}
	if !bytes.Equal(interestBlob, unitBlob) {
		t.Fatalf("interest vector blob mismatch after initialization")
	}
}

func TestRunClusterPipelineOnceOverlapsFusionAndEmbedding(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)

	pendingEmbedClusterID, err := db.CreateCluster(1, "pending_embed")
	if err != nil {
		t.Fatalf("CreateCluster pending_embed error: %v", err)
	}
	if err := db.UpdateClusterMergedContent(pendingEmbedClusterID, "title", "summary", "content"); err != nil {
		t.Fatalf("UpdateClusterMergedContent error: %v", err)
	}

	fusionStarted := make(chan struct{}, 1)
	releaseFusion := make(chan struct{})
	embeddingStarted := make(chan struct{}, 1)

	manager := &AIEnhancedManager{
		db:                      db,
		clusterPipelineRunning:  make(map[int64]bool),
		clusterPipelineQueued:   make(map[int64]bool),
		clusterFusionRunning:    make(map[int64]bool),
		clusterEmbeddingRunning: make(map[int64]bool),
		resolveFusionConfig: func(userID int64) (*dedup.FusionConfig, error) {
			return &dedup.FusionConfig{}, nil
		},
		runFusion: func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
			select {
			case fusionStarted <- struct{}{}:
			default:
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-releaseFusion:
				return nil
			}
		},
		runEmbedding: func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
			select {
			case embeddingStarted <- struct{}{}:
			default:
			}
			if err := db.UpdateClusterStatus(pendingEmbedClusterID, "complete"); err != nil {
				return err
			}
			return nil
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- manager.runClusterPipelineOnce(1)
	}()

	select {
	case <-fusionStarted:
	case <-time.After(time.Second):
		t.Fatal("fusion stage did not start")
	}

	select {
	case <-embeddingStarted:
	case <-time.After(time.Second):
		t.Fatal("embedding stage did not start while fusion was still running")
	}

	close(releaseFusion)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runClusterPipelineOnce error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runClusterPipelineOnce did not finish")
	}
}

func TestGetProcessingStatusDoesNotTriggerRecoveryWhenPendingButIdle(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	mustEnableAIEnhancedProcessing(t, db, 1)

	feedID := mustCreateTestFeed(t, db, 1, false)
	_ = mustInsertBatchArticle(
		t,
		db,
		1,
		feedID,
		false,
		time.Now().Add(-4*time.Hour),
		"recoverable-pending-article",
		false,
	)

	manager := &AIEnhancedManager{
		db:                        db,
		taskChan:                  make(chan *AIEnhancedTask, 10),
		queuedTasksByUser:         make(map[int64]int),
		activeWorkerTasksByUser:   make(map[int64]int64),
		activeAsyncWorkByUser:     make(map[int64]int64),
		recoveryInProgress:        make(map[int64]bool),
		lastRecoveryAttemptByUser: make(map[int64]time.Time),
		clusterPipelineRunning:    make(map[int64]bool),
		clusterPipelineQueued:     make(map[int64]bool),
		recommendationRunning:     make(map[int64]bool),
		pendingRecommendationDate: make(map[int64]string),
		pendingRecommendationWait: make(map[int64]bool),
	}

	status := manager.GetProcessingStatus(1)
	if status.PendingArticles == 0 {
		t.Fatalf("PendingArticles = %d, want > 0 for recovery test", status.PendingArticles)
	}
	if !status.IsConfigFrozen {
		t.Fatal("IsConfigFrozen = false, want true while pending work exists before recovery")
	}

	queued, _, _ := manager.getUserTaskCounts(1)
	if queued != 0 || len(manager.taskChan) != 0 {
		t.Fatalf("expected status read to stay side-effect free, queued=%d len(taskChan)=%d", queued, len(manager.taskChan))
	}
}

func TestRecoverPendingWorkForKnownUsersQueuesRecoveryWhenPendingButIdle(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	mustEnableAIEnhancedProcessing(t, db, 1)

	feedID := mustCreateTestFeed(t, db, 1, false)
	_ = mustInsertBatchArticle(
		t,
		db,
		1,
		feedID,
		false,
		time.Now().Add(-4*time.Hour),
		"recoverable-pending-article",
		false,
	)

	manager := &AIEnhancedManager{
		db:                        db,
		taskChan:                  make(chan *AIEnhancedTask, 10),
		queuedTasksByUser:         make(map[int64]int),
		activeWorkerTasksByUser:   make(map[int64]int64),
		activeAsyncWorkByUser:     make(map[int64]int64),
		recoveryInProgress:        make(map[int64]bool),
		lastRecoveryAttemptByUser: make(map[int64]time.Time),
		clusterPipelineRunning:    make(map[int64]bool),
		clusterPipelineQueued:     make(map[int64]bool),
		clusterFusionRunning:      make(map[int64]bool),
		clusterEmbeddingRunning:   make(map[int64]bool),
		recommendationRunning:     make(map[int64]bool),
		pendingRecommendationDate: make(map[int64]string),
		pendingRecommendationWait: make(map[int64]bool),
	}

	manager.recoverPendingWorkForKnownUsers("test")

	deadline := time.Now().Add(2 * time.Second)
	for {
		queued, _, _ := manager.getUserTaskCounts(1)
		if queued > 0 || len(manager.taskChan) > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected recovery to queue AI work, queued=%d len(taskChan)=%d", queued, len(manager.taskChan))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestRecoverPendingWorkForKnownUsersSchedulesClusterRecoveryWhenOnlyClusterWorkRemains(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	mustEnableAIEnhancedProcessing(t, db, 1)

	feedID := mustCreateTestFeed(t, db, 1, false)
	articleID := mustInsertBatchArticle(
		t,
		db,
		1,
		feedID,
		false,
		time.Now().Add(-4*time.Hour),
		"recoverable-pending-cluster",
		true,
	)
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
		db:                        db,
		taskChan:                  make(chan *AIEnhancedTask, 10),
		queuedTasksByUser:         make(map[int64]int),
		activeWorkerTasksByUser:   make(map[int64]int64),
		activeAsyncWorkByUser:     make(map[int64]int64),
		recoveryInProgress:        make(map[int64]bool),
		lastRecoveryAttemptByUser: make(map[int64]time.Time),
		clusterPipelineRunning:    make(map[int64]bool),
		clusterPipelineQueued:     make(map[int64]bool),
		clusterFusionRunning:      make(map[int64]bool),
		clusterEmbeddingRunning:   make(map[int64]bool),
		recommendationRunning:     make(map[int64]bool),
		pendingRecommendationDate: make(map[int64]string),
		pendingRecommendationWait: make(map[int64]bool),
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

	manager.recoverPendingWorkForKnownUsers("test")

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("expected recovery to schedule cluster pipeline when only cluster work remains")
	}

	if len(manager.taskChan) != 0 {
		t.Fatalf("unexpected queued article tasks: got %d, want 0", len(manager.taskChan))
	}
}

func TestGetProcessingStatusReportsPendingStageBreakdown(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	mustEnableAIEnhancedProcessing(t, db, 1)
	manager := &AIEnhancedManager{
		db:                        db,
		taskChan:                  make(chan *AIEnhancedTask, 10),
		queuedTasksByUser:         make(map[int64]int),
		activeWorkerTasksByUser:   make(map[int64]int64),
		activeAsyncWorkByUser:     make(map[int64]int64),
		recoveryInProgress:        make(map[int64]bool),
		lastRecoveryAttemptByUser: make(map[int64]time.Time),
		clusterPipelineRunning:    make(map[int64]bool),
		clusterPipelineQueued:     make(map[int64]bool),
		recommendationRunning:     make(map[int64]bool),
		pendingRecommendationDate: make(map[int64]string),
		pendingRecommendationWait: make(map[int64]bool),
	}

	plainFeedID := mustCreateTestFeed(t, db, 1, false)
	translatedFeedID := mustCreateTestFeed(t, db, 1, true)

	firstArticleID := mustInsertBatchArticle(
		t,
		db,
		1,
		plainFeedID,
		false,
		time.Now().Add(-3*time.Hour),
		"needs-summary-embedding-cluster",
		false,
	)
	if err := db.SetArticleTranslatedContent(firstArticleID, "already translated", "zh", "ai"); err != nil {
		t.Fatalf("SetArticleTranslatedContent error: %v", err)
	}

	clusteredArticleID := mustInsertBatchArticle(
		t,
		db,
		1,
		translatedFeedID,
		false,
		time.Now().Add(-2*time.Hour),
		"needs-translation-and-cluster-complete",
		true,
	)
	if err := db.UpdateArticleEmbeddings(clusteredArticleID, mustEmbeddingBlob(t), mustEmbeddingBlob(t)); err != nil {
		t.Fatalf("UpdateArticleEmbeddings error: %v", err)
	}
	clusterID, err := db.CreateCluster(1, "pending_merge")
	if err != nil {
		t.Fatalf("CreateCluster error: %v", err)
	}
	if err := db.UpdateArticleClusterID(clusteredArticleID, clusterID); err != nil {
		t.Fatalf("UpdateArticleClusterID error: %v", err)
	}
	if err := db.UpdateClusterArticleCount(clusterID); err != nil {
		t.Fatalf("UpdateClusterArticleCount error: %v", err)
	}

	clusteringArticleID := mustInsertBatchArticle(
		t,
		db,
		1,
		plainFeedID,
		false,
		time.Now().Add(-90*time.Minute),
		"needs-clustering-only",
		true,
	)
	if err := db.SetArticleTranslatedContent(clusteringArticleID, "already translated", "zh", "ai"); err != nil {
		t.Fatalf("SetArticleTranslatedContent error: %v", err)
	}
	if err := db.UpdateArticleEmbeddings(clusteringArticleID, mustEmbeddingBlob(t), mustEmbeddingBlob(t)); err != nil {
		t.Fatalf("UpdateArticleEmbeddings error: %v", err)
	}

	status := manager.GetProcessingStatus(1)

	if status.PendingArticles != 3 {
		t.Fatalf("PendingArticles = %d, want 3", status.PendingArticles)
	}
	if status.PendingSummaryArticles != 1 {
		t.Fatalf("PendingSummaryArticles = %d, want 1", status.PendingSummaryArticles)
	}
	if status.PendingTranslationArticles != 1 {
		t.Fatalf("PendingTranslationArticles = %d, want 1", status.PendingTranslationArticles)
	}
	if status.PendingEmbeddingArticles != 0 {
		t.Fatalf("PendingEmbeddingArticles = %d, want 0", status.PendingEmbeddingArticles)
	}
	if status.PendingClusteringArticles != 1 {
		t.Fatalf("PendingClusteringArticles = %d, want 1", status.PendingClusteringArticles)
	}
	if status.PendingMergeClusters != 1 {
		t.Fatalf("PendingMergeClusters = %d, want 1", status.PendingMergeClusters)
	}
	if status.PendingEmbedClusters != 0 {
		t.Fatalf("PendingEmbedClusters = %d, want 0", status.PendingEmbedClusters)
	}
	if status.PendingRecommendationDays != 1 {
		t.Fatalf("PendingRecommendationDays = %d, want 1", status.PendingRecommendationDays)
	}
	if status.PendingSummaryArticles+status.PendingTranslationArticles+status.PendingEmbeddingArticles+status.PendingClusteringArticles != status.PendingArticles {
		t.Fatalf(
			"pending breakdown sum = %d, want %d",
			status.PendingSummaryArticles+status.PendingTranslationArticles+status.PendingEmbeddingArticles+status.PendingClusteringArticles,
			status.PendingArticles,
		)
	}
}

func TestGetProcessingStatusUsesFullArticleScopeDuringRenormalization(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	mustEnableAIEnhancedProcessing(t, db, 1)

	manager := &AIEnhancedManager{
		db:                        db,
		taskChan:                  make(chan *AIEnhancedTask, 10),
		queuedTasksByUser:         make(map[int64]int),
		activeWorkerTasksByUser:   make(map[int64]int64),
		activeAsyncWorkByUser:     make(map[int64]int64),
		recoveryInProgress:        make(map[int64]bool),
		lastRecoveryAttemptByUser: make(map[int64]time.Time),
		renormalizationRunning:    map[int64]bool{1: true},
		clusterPipelineRunning:    make(map[int64]bool),
		clusterPipelineQueued:     make(map[int64]bool),
		clusterFusionRunning:      make(map[int64]bool),
		clusterEmbeddingRunning:   make(map[int64]bool),
		recommendationRunning:     make(map[int64]bool),
		pendingRecommendationDate: make(map[int64]string),
		pendingRecommendationWait: make(map[int64]bool),
	}

	feedID := mustCreateTestFeed(t, db, 1, false)
	oldArticleID := mustInsertBatchArticle(
		t,
		db,
		1,
		feedID,
		false,
		time.Now().Add(-10*24*time.Hour),
		"old-renorm-article",
		true,
	)
	if err := db.UpdateArticleEmbeddings(oldArticleID, mustEmbeddingBlob(t), mustEmbeddingBlob(t)); err != nil {
		t.Fatalf("UpdateArticleEmbeddings old article error: %v", err)
	}
	recentArticleID := mustInsertBatchArticle(
		t,
		db,
		1,
		feedID,
		false,
		time.Now().Add(-2*time.Hour),
		"recent-renorm-article",
		true,
	)
	_ = mustAttachCompleteCluster(t, db, 1, recentArticleID, "complete")
	noContentArticleID := mustInsertBatchArticle(
		t,
		db,
		1,
		feedID,
		false,
		time.Now().Add(-30*time.Minute),
		"no-content-renorm-article",
		true,
	)
	if err := db.DeleteArticleContent(noContentArticleID); err != nil {
		t.Fatalf("DeleteArticleContent error: %v", err)
	}

	status := manager.GetProcessingStatus(1)

	if !status.IsRenormalizationRunning {
		t.Fatal("IsRenormalizationRunning = false, want true")
	}
	if status.EligibleArticles != 2 {
		t.Fatalf("EligibleArticles = %d, want 2 during renormalization full-scope progress", status.EligibleArticles)
	}
	if status.PendingArticles != 1 {
		t.Fatalf("PendingArticles = %d, want 1 for old article missing clustering state", status.PendingArticles)
	}
	if status.CompletedArticles != 1 {
		t.Fatalf("CompletedArticles = %d, want 1", status.CompletedArticles)
	}
	if status.PendingClusteringArticles != 1 {
		t.Fatalf("PendingClusteringArticles = %d, want 1", status.PendingClusteringArticles)
	}
	if status.RenormalizationTotalArticles != 2 {
		t.Fatalf("RenormalizationTotalArticles = %d, want 2", status.RenormalizationTotalArticles)
	}
	if status.RenormalizationPendingArticles != 1 {
		t.Fatalf("RenormalizationPendingArticles = %d, want 1", status.RenormalizationPendingArticles)
	}
	if status.RenormalizationCompletedArticles != 1 {
		t.Fatalf("RenormalizationCompletedArticles = %d, want 1", status.RenormalizationCompletedArticles)
	}

	_ = oldArticleID
	_ = noContentArticleID
}

func TestClusterPipelineBarrierStateStillWaitsDuringRenormalization(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	mustEnableAIEnhancedProcessing(t, db, 1)

	manager := &AIEnhancedManager{
		db:                      db,
		taskChan:                make(chan *AIEnhancedTask, 10),
		queuedTasksByUser:       map[int64]int{1: 3},
		activeWorkerTasksByUser: map[int64]int64{1: 2},
		activeAsyncWorkByUser:   map[int64]int64{1: 4},
		renormalizationRunning:  map[int64]bool{1: true},
		clusterPipelineRunning:  make(map[int64]bool),
		clusterPipelineQueued:   make(map[int64]bool),
		clusterFusionRunning:    make(map[int64]bool),
		clusterEmbeddingRunning: make(map[int64]bool),
	}

	feedID := mustCreateTestFeed(t, db, 1, true)
	_ = mustInsertBatchArticle(
		t,
		db,
		1,
		feedID,
		false,
		time.Now().Add(-time.Hour),
		"renorm-barrier-pending-summary",
		false,
	)

	if _, err := db.CreateCluster(1, "pending_merge"); err != nil {
		t.Fatalf("CreateCluster error = %v", err)
	}

	hasWork, barrierReached, err := manager.clusterPipelineBarrierState(1)
	if err != nil {
		t.Fatalf("clusterPipelineBarrierState error = %v", err)
	}
	if !hasWork {
		t.Fatal("hasWork = false, want true")
	}
	if barrierReached {
		t.Fatal("barrierReached = true, want false during renormalization while article work exists")
	}
}

func TestClusterPipelineBarrierStateStillWaitsOutsideRenormalization(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	mustEnableAIEnhancedProcessing(t, db, 1)

	manager := &AIEnhancedManager{
		db:                      db,
		taskChan:                make(chan *AIEnhancedTask, 10),
		queuedTasksByUser:       map[int64]int{1: 1},
		activeWorkerTasksByUser: make(map[int64]int64),
		activeAsyncWorkByUser:   make(map[int64]int64),
		clusterPipelineRunning:  make(map[int64]bool),
		clusterPipelineQueued:   make(map[int64]bool),
		clusterFusionRunning:    make(map[int64]bool),
		clusterEmbeddingRunning: make(map[int64]bool),
	}

	if _, err := db.CreateCluster(1, "pending_merge"); err != nil {
		t.Fatalf("CreateCluster error = %v", err)
	}

	hasWork, barrierReached, err := manager.clusterPipelineBarrierState(1)
	if err != nil {
		t.Fatalf("clusterPipelineBarrierState error = %v", err)
	}
	if !hasWork {
		t.Fatal("hasWork = false, want true")
	}
	if barrierReached {
		t.Fatal("barrierReached = true, want false outside renormalization when queued article work exists")
	}
}

func TestRunUserClusterAssignmentSeriallySerializesSameUserWork(t *testing.T) {
	manager := &AIEnhancedManager{
		userClusteringLocks: make(map[int64]*sync.Mutex),
	}

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondEntered := make(chan struct{})
	firstDone := make(chan struct{})

	go func() {
		defer close(firstDone)
		_ = manager.runUserClusterAssignmentSerially(1, func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()

	select {
	case <-firstEntered:
	case <-time.After(time.Second):
		t.Fatal("first serialized section did not start")
	}

	go func() {
		_ = manager.runUserClusterAssignmentSerially(1, func() error {
			close(secondEntered)
			return nil
		})
	}()

	select {
	case <-secondEntered:
		t.Fatal("second serialized section started before first finished")
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseFirst)

	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first serialized section did not finish")
	}

	select {
	case <-secondEntered:
	case <-time.After(time.Second):
		t.Fatal("second serialized section did not start after first finished")
	}
}

func TestArticleSummaryStageSemaphoreLimitsConcurrency(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := NewAIEnhancedManager(db)
	defer manager.Stop()

	feedID := mustCreateTestFeed(t, db, 1, false)
	started := make(chan int64, 12)
	release := make(chan struct{}, 12)
	var running int32
	var maxRunning int32

	manager.runArticleSummary = func(task *AIEnhancedTask, content string) {
		current := atomic.AddInt32(&running, 1)
		for {
			observed := atomic.LoadInt32(&maxRunning)
			if current <= observed || atomic.CompareAndSwapInt32(&maxRunning, observed, current) {
				break
			}
		}
		started <- task.ArticleID
		<-release
		atomic.AddInt32(&running, -1)
	}

	tasks := make([]*AIEnhancedTask, 0, 8)
	for i := 0; i < 8; i++ {
		title := "summary-stage-" + strconv.Itoa(i)
		articleID := mustInsertBatchArticle(t, db, 1, feedID, false, time.Now().Add(time.Duration(-i)*time.Minute), title, false)
		tasks = append(tasks, &AIEnhancedTask{
			ArticleID:    articleID,
			UserID:       1,
			FeedID:       feedID,
			ArticleTitle: title,
			NeedsSummary: true,
		})
	}

	for _, task := range tasks {
		if err := manager.startArticlePipeline(task, -1); err != nil {
			t.Fatalf("startArticlePipeline error: %v", err)
		}
	}

	for i := 0; i < articleSummaryConcurrency; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatalf("summary stage started only %d tasks, want %d", i, articleSummaryConcurrency)
		}
	}

	select {
	case <-started:
		t.Fatal("summary stage exceeded concurrency limit before a slot was released")
	case <-time.After(150 * time.Millisecond):
	}

	release <- struct{}{}

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("summary stage did not admit the next task after a slot was released")
	}

	for i := 0; i < 7; i++ {
		release <- struct{}{}
	}

	waitForCondition(t, 2*time.Second, func() bool {
		queued, activeWorker, activeAsync := manager.getUserTaskCounts(1)
		return queued == 0 && activeWorker == 0 && activeAsync == 0
	})

	if got := atomic.LoadInt32(&maxRunning); got != articleSummaryConcurrency {
		t.Fatalf("max summary concurrency = %d, want %d", got, articleSummaryConcurrency)
	}
}

func TestGetProcessingStatusUnfreezesAfterStaleTimeout(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	mustEnableAIEnhancedProcessing(t, db, 1)
	manager := &AIEnhancedManager{
		db:                        db,
		taskChan:                  make(chan *AIEnhancedTask, 10),
		queuedTasksByUser:         make(map[int64]int),
		activeWorkerTasksByUser:   make(map[int64]int64),
		activeAsyncWorkByUser:     make(map[int64]int64),
		recoveryInProgress:        make(map[int64]bool),
		lastRecoveryAttemptByUser: make(map[int64]time.Time),
		clusterPipelineRunning:    make(map[int64]bool),
		clusterPipelineQueued:     make(map[int64]bool),
		recommendationRunning:     make(map[int64]bool),
		pendingRecommendationDate: make(map[int64]string),
		pendingRecommendationWait: make(map[int64]bool),
	}

	feedID := mustCreateTestFeed(t, db, 1, false)
	_ = mustInsertBatchArticle(
		t,
		db,
		1,
		feedID,
		false,
		time.Now().Add(-1*time.Hour),
		"stale-pending-article",
		false,
	)

	initial := manager.GetProcessingStatus(1)
	if !initial.IsConfigFrozen {
		t.Fatal("initial IsConfigFrozen = false, want true")
	}

	if err := db.SetSettingForUser(1, aiProcessingSnapshotSettingKey, manager.buildProcessingSnapshot(initial)); err != nil {
		t.Fatalf("SetSettingForUser snapshot error: %v", err)
	}
	if err := db.SetSettingForUser(1, aiProcessingLastProgressAtSettingKey, time.Now().Add(-31*time.Minute).Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("SetSettingForUser last progress error: %v", err)
	}
	if err := db.SetSettingForUser(1, aiProcessingFreezeSuspendedSettingKey, "false"); err != nil {
		t.Fatalf("SetSettingForUser freeze suspended error: %v", err)
	}

	status := manager.GetProcessingStatus(1)
	if status.IsConfigFrozen {
		t.Fatal("IsConfigFrozen = true, want false after stale timeout")
	}
	if !status.IsStale {
		t.Fatal("IsStale = false, want true after stale timeout")
	}
	if !status.IsFreezeSuspended {
		t.Fatal("IsFreezeSuspended = false, want true after stale timeout")
	}
	if status.StalledForSeconds < int64((30 * time.Minute).Seconds()) {
		t.Fatalf("StalledForSeconds = %d, want >= %d", status.StalledForSeconds, int64((30 * time.Minute).Seconds()))
	}
}

func TestGetProcessingStatusIncludesRecentFailureDiagnostics(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := &AIEnhancedManager{
		db:                      db,
		taskChan:                make(chan *AIEnhancedTask, 10),
		queuedTasksByUser:       make(map[int64]int),
		activeWorkerTasksByUser: make(map[int64]int64),
		activeAsyncWorkByUser:   make(map[int64]int64),
		recentFailureByUser:     make(map[int64]AIProcessingFailure),
		clusterPipelineRunning:  make(map[int64]bool),
		clusterPipelineQueued:   make(map[int64]bool),
	}

	mustSetUserSetting(t, db, 1, "ai_enhanced_mode", "true")
	feedID := mustCreateTestFeed(t, db, 1, false)
	articleID := mustInsertBatchArticle(
		t,
		db,
		1,
		feedID,
		false,
		time.Now().Add(-30*time.Minute),
		"diagnostic-pending-article",
		false,
	)

	task := &AIEnhancedTask{
		ArticleID:    articleID,
		UserID:       1,
		FeedID:       feedID,
		ArticleTitle: "diagnostic-pending-article",
	}
	manager.recordTaskFailure(1, "summary", task, "gpt-test", "https://api.example.com/v1/chat/completions", &ai.RequestError{
		UserMessage: "AI service unavailable",
		Diagnostics: []string{"OpenAI: bad request - check parameters"},
	})

	status := manager.GetProcessingStatus(1)
	if status.RecentFailureStage != "summary" {
		t.Fatalf("RecentFailureStage = %q, want %q", status.RecentFailureStage, "summary")
	}
	if status.RecentFailureArticleID != articleID {
		t.Fatalf("RecentFailureArticleID = %d, want %d", status.RecentFailureArticleID, articleID)
	}
	if status.RecentFailureArticleTitle != "diagnostic-pending-article" {
		t.Fatalf("RecentFailureArticleTitle = %q", status.RecentFailureArticleTitle)
	}
	if status.RecentFailureModel != "gpt-test" {
		t.Fatalf("RecentFailureModel = %q", status.RecentFailureModel)
	}
	if status.RecentFailureEndpoint != "https://api.example.com/v1/chat/completions" {
		t.Fatalf("RecentFailureEndpoint = %q", status.RecentFailureEndpoint)
	}
	if !strings.Contains(status.RecentFailureMessage, "bad request - check parameters") {
		t.Fatalf("RecentFailureMessage = %q", status.RecentFailureMessage)
	}
	if status.RecentFailureCount != 1 {
		t.Fatalf("RecentFailureCount = %d, want 1", status.RecentFailureCount)
	}
}

func TestGetProcessingStatusTreatsSkippedStagesAsCompleted(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := NewAIEnhancedManager(db)
	defer manager.Stop()

	mustSetUserSetting(t, db, 1, "ai_enhanced_mode", "true")

	feedID := mustCreateTestFeed(t, db, 1, true)
	articleID := mustInsertBatchArticle(t, db, 1, feedID, false, time.Now().Add(-1*time.Hour), "skipped-pipeline", true)

	if err := db.SetAIArticleStageSkip(1, articleID, "summary", "safety"); err != nil {
		t.Fatalf("SetAIArticleStageSkip summary error: %v", err)
	}
	if err := db.SetAIArticleStageSkip(1, articleID, "translation", "safety"); err != nil {
		t.Fatalf("SetAIArticleStageSkip translation error: %v", err)
	}
	if err := db.SetAIArticleStageSkip(1, articleID, "embedding", "timeout"); err != nil {
		t.Fatalf("SetAIArticleStageSkip embedding error: %v", err)
	}
	if err := db.SetAIArticleStageSkip(1, articleID, "clustering", "timeout"); err != nil {
		t.Fatalf("SetAIArticleStageSkip clustering error: %v", err)
	}

	status := manager.GetProcessingStatus(1)
	if status.PendingArticles != 0 {
		t.Fatalf("PendingArticles = %d, want 0 after stage skips", status.PendingArticles)
	}
	if status.CompletedArticles != 1 {
		t.Fatalf("CompletedArticles = %d, want 1", status.CompletedArticles)
	}
}

func TestProcessTaskUsesTitleAsFallbackSummaryAndContent(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := NewAIEnhancedManager(db)
	defer manager.Stop()

	mustSetUserSetting(t, db, 1, "ai_enhanced_mode", "true")

	feedID := mustCreateTestFeed(t, db, 1, true)
	articleID := mustInsertBatchArticle(t, db, 1, feedID, false, time.Now().Add(-1*time.Hour), "missing-content", false)
	if err := db.DeleteArticleContent(articleID); err != nil {
		t.Fatalf("DeleteArticleContent error: %v", err)
	}

	task := &AIEnhancedTask{
		ArticleID:         articleID,
		UserID:            1,
		FeedID:            feedID,
		ArticleTitle:      "missing-content",
		NeedsSummary:      true,
		NeedsTranslation:  false,
		TranslateArticles: false,
		NeedsEmbedding:    false,
		NeedsDedup:        false,
	}
	manager.processTask(task, 0)

	waitForCondition(t, 2*time.Second, func() bool {
		article, err := db.GetArticleByID(articleID)
		if err != nil || article == nil {
			return false
		}
		content, found, err := db.GetArticleContent(articleID)
		if err != nil || !found {
			return false
		}
		return article.Summary == "missing-content" && strings.Contains(content, "missing-content")
	})
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

func mustCreateAIProfile(t *testing.T, db *sqlite.DB, userID int64, name string) int64 {
	t.Helper()

	profileID, err := db.CreateAIProfile(&models.AIProfile{
		UserID:   userID,
		Name:     name,
		APIKey:   name + "-key",
		Endpoint: "https://" + name + ".example.com",
		Model:    name + "-model",
	})
	if err != nil {
		t.Fatalf("CreateAIProfile(%s) error: %v", name, err)
	}

	return profileID
}

func mustEnableAIEnhancedProcessing(t *testing.T, db *sqlite.DB, userID int64) {
	t.Helper()

	profileIDs := []int64{
		mustCreateAIProfile(t, db, userID, "summary"),
		mustCreateAIProfile(t, db, userID, "translation"),
		mustCreateAIProfile(t, db, userID, "search"),
		mustCreateAIProfile(t, db, userID, "chat"),
		mustCreateAIProfile(t, db, userID, "fusion"),
	}

	if err := db.SetDefaultAIProfileForUser(userID, profileIDs[0]); err != nil {
		t.Fatalf("SetDefaultAIProfileForUser error: %v", err)
	}

	mustSetUserSetting(t, db, userID, "ai_enhanced_mode", "true")
	mustSetUserSetting(t, db, userID, "ai_embedding_models", `[{"modelname":"embed","baseurl":"https://embed.example.com","apikey":"embed-key","rpm":0,"tpm":0,"use_global_proxy":true}]`)
	mustSetUserSetting(t, db, userID, "summary_enabled", "true")
	mustSetUserSetting(t, db, userID, "summary_provider", "ai")
	mustSetUserSetting(t, db, userID, "translation_enabled", "true")
	mustSetUserSetting(t, db, userID, "ai_fusion_enabled", "true")
	mustSetUserSetting(t, db, userID, "ai_recommendation_enabled", "true")
	mustSetUserSetting(t, db, userID, "ai_search_enabled", "true")
	mustSetUserSetting(t, db, userID, "ai_chat_enabled", "true")
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
		summary = "summary content for batch processing tests"
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
	if err := db.SetArticleContent(articleID, "article content for batch processing tests"); err != nil {
		t.Fatalf("SetArticleContent error: %v", err)
	}
	return articleID
}

func mustAttachCompleteCluster(t *testing.T, db *sqlite.DB, userID, articleID int64, status string) int64 {
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

	return clusterID
}

func mustEmbeddingBlob(t *testing.T) []byte {
	t.Helper()
	vec := make([]float32, 1024)
	vec[0] = 1
	blob, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		t.Fatalf("SerializeFloat32 error: %v", err)
	}
	return blob
}

func mustUnitEmbeddingBlob(t *testing.T) []byte {
	t.Helper()

	vec := make([]float32, 1024)
	vec[0] = 1
	blob, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		t.Fatalf("SerializeFloat32 unit vector error: %v", err)
	}
	return blob
}

func TestPrepareAITranslationInputConvertsHTMLToStableText(t *testing.T) {
	t.Parallel()

	input := `<p>Hello <strong>world</strong></p><ul><li>First item</li><li>Second item</li></ul>`
	got := prepareAITranslationInput(input)

	if got == "" {
		t.Fatal("prepareAITranslationInput() returned empty string")
	}
	if strings.Contains(got, "<") || strings.Contains(got, ">") {
		t.Fatalf("prepareAITranslationInput() = %q, want HTML tags removed", got)
	}
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "First item") || !strings.Contains(got, "Second item") {
		t.Fatalf("prepareAITranslationInput() = %q, want readable text content preserved", got)
	}
}

func TestPrepareAITranslationInputReturnsEmptyForNonTextHTML(t *testing.T) {
	t.Parallel()

	input := `<div><img src="cover.jpg" /><br/></div>`
	if got := prepareAITranslationInput(input); got != "" {
		t.Fatalf("prepareAITranslationInput() = %q, want empty string", got)
	}
}
