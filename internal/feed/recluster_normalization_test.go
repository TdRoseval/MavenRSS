package feed

import (
	"context"
	"strings"
	"testing"
	"time"

	"MavenRSS/internal/dedup"
	"MavenRSS/internal/interest"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

func TestStartClusterRenormalizationReturnsBusyWhenUserHasQueuedWork(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	mustEnableAIEnhancedProcessing(t, db, 1)

	manager := NewAIEnhancedManager(db)
	defer manager.Stop()

	manager.incrementQueuedTask(1)

	scheduled, reason, err := manager.StartClusterRenormalization(1)
	if err != nil {
		t.Fatalf("StartClusterRenormalization error = %v", err)
	}
	if scheduled {
		t.Fatal("scheduled = true, want false when user already has queued AI work")
	}
	if reason != "busy" {
		t.Fatalf("reason = %q, want busy", reason)
	}
}

func TestInterruptUserWorkForRenormalizationPurgesQueuedTasksAndRecommendations(t *testing.T) {
	manager := &AIEnhancedManager{
		taskChan:                 make(chan *AIEnhancedTask, 8),
		queuedTasksByUser:        make(map[int64]int),
		activeWorkerTasksByUser:  make(map[int64]int64),
		activeAsyncWorkByUser:    make(map[int64]int64),
		userOperationVersion:     make(map[int64]int64),
		clusterPipelineRunning:   make(map[int64]bool),
		clusterPipelineQueued:    make(map[int64]bool),
		clusterPipelineVersion:   make(map[int64]int64),
		clusterPipelineQueuedVer: make(map[int64]int64),
		clusterFusionRunning:     make(map[int64]bool),
		clusterEmbeddingRunning:  make(map[int64]bool),
		recommendationRunning:    make(map[int64]bool),
		recommendationRunningVer: make(map[int64]int64),
		recommendationStatusByUser: map[int64]DailyRecommendationTaskStatus{
			1: {HasTask: true, Stage: "queued"},
		},
		pendingRecommendationDate:  map[int64]string{1: "2026-04-02"},
		pendingRecommendationWait:  map[int64]bool{1: true},
		pendingRecommendationForce: map[int64]bool{1: true},
		pendingRecommendationMode:  map[int64]string{1: "manual"},
		pendingRecommendationVer:   map[int64]int64{1: 3},
	}
	manager.taskChan <- &AIEnhancedTask{ArticleID: 1, UserID: 1}
	manager.incrementQueuedTask(1)
	manager.taskChan <- &AIEnhancedTask{ArticleID: 2, UserID: 2}
	manager.incrementQueuedTask(2)

	removed := manager.interruptUserWorkForRenormalization(1)
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if got, _, _ := manager.getUserTaskCounts(1); got != 0 {
		t.Fatalf("queued count for user 1 = %d, want 0", got)
	}
	if got, _, _ := manager.getUserTaskCounts(2); got != 1 {
		t.Fatalf("queued count for user 2 = %d, want 1", got)
	}
	if len(manager.taskChan) != 1 {
		t.Fatalf("remaining queued tasks = %d, want 1", len(manager.taskChan))
	}
	remaining := <-manager.taskChan
	if remaining == nil || remaining.UserID != 2 {
		t.Fatalf("remaining task user = %v, want 2", remaining)
	}
	if manager.currentUserOperationVersion(1) != 1 {
		t.Fatalf("operation version = %d, want 1", manager.currentUserOperationVersion(1))
	}
	if manager.pendingRecommendationDate[1] != "" {
		t.Fatalf("pending recommendation date = %q, want empty", manager.pendingRecommendationDate[1])
	}
	if _, ok := manager.recommendationStatusByUser[1]; ok {
		t.Fatal("recommendation status should be cleared for interrupted user")
	}
}

func TestStartClusterRenormalizationResetsStateAndRequeuesArticles(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	mustEnableAIEnhancedProcessing(t, db, 1)

	feedID := mustCreateTestFeed(t, db, 1, false)
	firstArticleID := mustInsertBatchArticle(t, db, 1, feedID, false, time.Now().Add(-2*time.Hour), "renorm-1", true)
	secondArticleID := mustInsertBatchArticle(t, db, 1, feedID, false, time.Now().Add(-time.Hour), "renorm-2", true)

	oldClusterID, err := db.CreateCluster(1, "complete")
	if err != nil {
		t.Fatalf("CreateCluster error = %v", err)
	}
	if err := db.UpdateArticleClusterID(firstArticleID, oldClusterID); err != nil {
		t.Fatalf("UpdateArticleClusterID first error = %v", err)
	}
	if err := db.UpdateArticleClusterID(secondArticleID, oldClusterID); err != nil {
		t.Fatalf("UpdateArticleClusterID second error = %v", err)
	}
	if err := db.UpdateClusterArticleCount(oldClusterID); err != nil {
		t.Fatalf("UpdateClusterArticleCount error = %v", err)
	}
	if err := db.UpdateClusterMergedContent(oldClusterID, "old title", "old summary", "old content"); err != nil {
		t.Fatalf("UpdateClusterMergedContent error = %v", err)
	}

	rawBlob := mustSerializeRawEmbeddingBlob(t, 2)
	if _, err := db.Exec(
		`INSERT OR REPLACE INTO article_embeddings (article_id, title_embedding, summary_embedding) VALUES (?, ?, ?)`,
		firstArticleID, rawBlob, rawBlob,
	); err != nil {
		t.Fatalf("insert first embedding error = %v", err)
	}
	if _, err := db.Exec(
		`INSERT OR REPLACE INTO article_embeddings (article_id, title_embedding, summary_embedding) VALUES (?, ?, ?)`,
		secondArticleID, rawBlob, rawBlob,
	); err != nil {
		t.Fatalf("insert second embedding error = %v", err)
	}
	if err := db.UpdateClusterEmbeddings(oldClusterID, mustUnitEmbeddingBlob(t), mustUnitEmbeddingBlob(t)); err != nil {
		t.Fatalf("UpdateClusterEmbeddings error = %v", err)
	}
	if err := db.SaveDailyRecommendations(1, "2026-04-02", []models.DailyRecommendation{{
		UserID:             1,
		ClusterID:          oldClusterID,
		RecommendationDate: "2026-04-02",
		RecommendationRank: 1,
	}}); err != nil {
		t.Fatalf("SaveDailyRecommendations error = %v", err)
	}
	if err := db.UpdateUserInterestVector(1, mustUnitEmbeddingBlob(t)); err != nil {
		t.Fatalf("UpdateUserInterestVector error = %v", err)
	}
	if err := db.SetAIArticleStageSkip(1, firstArticleID, "summary", "old skip"); err != nil {
		t.Fatalf("SetAIArticleStageSkip error = %v", err)
	}

	manager := NewAIEnhancedManager(db)
	defer manager.Stop()
	manager.resolveFusionConfig = func(userID int64) (*dedup.FusionConfig, error) {
		return &dedup.FusionConfig{}, nil
	}
	manager.runFusion = func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
		clusters, err := db.GetClustersByStatus(userID, "pending_merge")
		if err != nil {
			return err
		}
		for _, cluster := range clusters {
			if err := db.UpdateClusterMergedContent(cluster.ID, "merged title", "merged summary", "merged content"); err != nil {
				return err
			}
			if err := db.UpdateClusterStatus(cluster.ID, "pending_embed"); err != nil {
				return err
			}
		}
		return nil
	}
	manager.runEmbedding = func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
		clusters, err := db.GetClustersByStatus(userID, "pending_embed")
		if err != nil {
			return err
		}
		for _, cluster := range clusters {
			if err := db.UpdateClusterEmbeddings(cluster.ID, mustUnitEmbeddingBlob(t), mustUnitEmbeddingBlob(t)); err != nil {
				return err
			}
			if err := db.UpdateClusterStatus(cluster.ID, "complete"); err != nil {
				return err
			}
		}
		return nil
	}

	scheduled, reason, err := manager.StartClusterRenormalization(1)
	if err != nil {
		t.Fatalf("StartClusterRenormalization error = %v", err)
	}
	if !scheduled {
		t.Fatalf("scheduled = false, want true (reason=%q)", reason)
	}

	waitForCondition(t, 12*time.Second, func() bool {
		return !manager.isRenormalizationRunning(1)
	})

	var oldClusterCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM clusters WHERE id = ?`, oldClusterID).Scan(&oldClusterCount); err != nil {
		t.Fatalf("old cluster count error = %v", err)
	}
	if oldClusterCount != 0 {
		t.Fatalf("old cluster should be deleted, got %d rows", oldClusterCount)
	}

	var newClusteredArticles int
	if err := db.QueryRow(`SELECT COUNT(*) FROM articles WHERE id IN (?, ?) AND cluster_id IS NOT NULL`, firstArticleID, secondArticleID).Scan(&newClusteredArticles); err != nil {
		t.Fatalf("clustered article count error = %v", err)
	}
	if newClusteredArticles != 2 {
		t.Fatalf("clustered article count = %d, want 2", newClusteredArticles)
	}

	var skipCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ai_article_stage_skips WHERE user_id = ?`, 1).Scan(&skipCount); err != nil {
		t.Fatalf("skip count error = %v", err)
	}
	if skipCount != 0 {
		t.Fatalf("skipCount = %d, want 0", skipCount)
	}

	vecBlob, err := db.GetUserInterestVector(1)
	if err != nil {
		t.Fatalf("GetUserInterestVector error = %v", err)
	}
	if len(vecBlob) != 0 {
		t.Fatal("interest vector should be cleared during renormalization")
	}

	var recommendationCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM daily_recommendations WHERE user_id = ?`, 1).Scan(&recommendationCount); err != nil {
		t.Fatalf("recommendation count error = %v", err)
	}
	if recommendationCount != 0 {
		t.Fatalf("recommendationCount = %d, want 0 after reset with no rebuilt recommendations", recommendationCount)
	}

	var normalizedBlob []byte
	if err := db.QueryRow(`SELECT summary_embedding FROM article_embeddings WHERE article_id = ?`, firstArticleID).Scan(&normalizedBlob); err != nil {
		t.Fatalf("query normalized summary embedding error = %v", err)
	}
	vec, err := interest.DeserializeVector(normalizedBlob)
	if err != nil {
		t.Fatalf("DeserializeVector error = %v", err)
	}
	if !interest.IsNormalized(vec, 1e-3) {
		t.Fatal("summary embedding should be normalized after renormalization")
	}
}

func TestStartClusterRenormalizationRepairsEmptyMergedContentAndFavoriteFlags(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	mustEnableAIEnhancedProcessing(t, db, 1)

	feedID := mustCreateTestFeed(t, db, 1, false)
	articleID := mustInsertBatchArticle(t, db, 1, feedID, true, time.Now().Add(-time.Hour), "renorm-repair", true)

	rawBlob := mustSerializeRawEmbeddingBlob(t, 3)
	if _, err := db.Exec(
		`INSERT OR REPLACE INTO article_embeddings (article_id, title_embedding, summary_embedding) VALUES (?, ?, ?)`,
		articleID, rawBlob, rawBlob,
	); err != nil {
		t.Fatalf("insert embedding error = %v", err)
	}

	manager := NewAIEnhancedManager(db)
	defer manager.Stop()
	manager.resolveFusionConfig = func(userID int64) (*dedup.FusionConfig, error) {
		return &dedup.FusionConfig{}, nil
	}
	manager.runFusion = func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
		clusters, err := db.GetClustersByStatus(userID, "pending_merge")
		if err != nil {
			return err
		}
		for _, cluster := range clusters {
			if err := db.UpdateClusterStatus(cluster.ID, "pending_embed"); err != nil {
				return err
			}
		}
		return nil
	}
	manager.runEmbedding = func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
		clusters, err := db.GetClustersByStatus(userID, "pending_embed")
		if err != nil {
			return err
		}
		for _, cluster := range clusters {
			if err := db.SetClusterFavorite(cluster.ID, false); err != nil {
				return err
			}
			if err := db.UpdateClusterStatus(cluster.ID, "complete"); err != nil {
				return err
			}
		}
		return nil
	}

	scheduled, reason, err := manager.StartClusterRenormalization(1)
	if err != nil {
		t.Fatalf("StartClusterRenormalization error = %v", err)
	}
	if !scheduled {
		t.Fatalf("scheduled = false, want true (reason=%q)", reason)
	}

	waitForCondition(t, 12*time.Second, func() bool {
		return !manager.isRenormalizationRunning(1)
	})

	var clusterID int64
	if err := db.QueryRow(`SELECT cluster_id FROM articles WHERE id = ?`, articleID).Scan(&clusterID); err != nil {
		t.Fatalf("query cluster_id error = %v", err)
	}
	if clusterID <= 0 {
		t.Fatalf("clusterID = %d, want > 0", clusterID)
	}

	cluster, err := db.GetClusterByID(clusterID)
	if err != nil {
		t.Fatalf("GetClusterByID error = %v", err)
	}
	if cluster == nil {
		t.Fatal("GetClusterByID returned nil cluster")
	}
	if !cluster.IsFavorite {
		t.Fatal("cluster should be re-synced to favorite after renormalization")
	}
	if strings.TrimSpace(cluster.MergedSummary) == "" {
		t.Fatal("MergedSummary should be backfilled when fusion leaves it empty")
	}
	if strings.TrimSpace(cluster.MergedContent) == "" {
		t.Fatal("MergedContent should be backfilled when fusion leaves it empty")
	}
}

func TestWaitForRenormalizationArticleTaskTimesOutAndSkipsRemainingStages(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := NewAIEnhancedManager(db)
	defer manager.Stop()

	originalTimeout := renormalizationArticleWaitTimeout
	originalPollInterval := renormalizationWaitPollInterval
	renormalizationArticleWaitTimeout = 20 * time.Millisecond
	renormalizationWaitPollInterval = 5 * time.Millisecond
	t.Cleanup(func() {
		renormalizationArticleWaitTimeout = originalTimeout
		renormalizationWaitPollInterval = originalPollInterval
	})

	feedID := mustCreateTestFeed(t, db, 1, true)
	articleID := mustInsertBatchArticle(t, db, 1, feedID, false, time.Now().Add(-time.Hour), "timed-out-article", false)

	if err := manager.waitForRenormalizationArticleTask(1, articleID, "zh"); err != nil {
		t.Fatalf("waitForRenormalizationArticleTask error = %v", err)
	}

	for _, stage := range []string{"summary", "translation", "embedding", "clustering"} {
		reason, found, err := db.GetAIArticleStageSkipReason(articleID, stage)
		if err != nil {
			t.Fatalf("GetAIArticleStageSkipReason(%s) error = %v", stage, err)
		}
		if !found {
			t.Fatalf("expected %s skip marker for timed-out article", stage)
		}
		if !strings.Contains(reason, "article-stage wait exceeded") {
			t.Fatalf("skip reason for %s = %q, want timeout marker", stage, reason)
		}
	}
}

func TestForceCompleteTimedOutClustersBackfillsAndLeavesRepairableState(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := NewAIEnhancedManager(db)
	defer manager.Stop()

	feedID := mustCreateTestFeed(t, db, 1, false)
	articleID := mustInsertBatchArticle(t, db, 1, feedID, false, time.Now().Add(-time.Hour), "timed-out-cluster", true)
	if err := db.UpdateArticleEmbeddings(articleID, mustEmbeddingBlob(t), mustEmbeddingBlob(t)); err != nil {
		t.Fatalf("UpdateArticleEmbeddings error = %v", err)
	}

	clusterID, err := db.CreateCluster(1, "pending_merge")
	if err != nil {
		t.Fatalf("CreateCluster error = %v", err)
	}
	if err := db.UpdateArticleClusterID(articleID, clusterID); err != nil {
		t.Fatalf("UpdateArticleClusterID error = %v", err)
	}
	if err := db.UpdateClusterArticleCount(clusterID); err != nil {
		t.Fatalf("UpdateClusterArticleCount error = %v", err)
	}

	completed, err := manager.forceCompleteTimedOutClusters(1, "test timeout")
	if err != nil {
		t.Fatalf("forceCompleteTimedOutClusters error = %v", err)
	}
	if completed != 1 {
		t.Fatalf("completed = %d, want 1", completed)
	}

	cluster, err := db.GetClusterByID(clusterID)
	if err != nil {
		t.Fatalf("GetClusterByID error = %v", err)
	}
	if cluster == nil {
		t.Fatal("GetClusterByID returned nil cluster")
	}
	if cluster.Status != "complete" {
		t.Fatalf("cluster status = %q, want complete", cluster.Status)
	}
	if strings.TrimSpace(cluster.MergedSummary) == "" {
		t.Fatal("MergedSummary should be backfilled for force-completed cluster")
	}
	if strings.TrimSpace(cluster.MergedContent) == "" {
		t.Fatal("MergedContent should be backfilled for force-completed cluster")
	}

	articles, err := db.GetArticlesForAIBatchProcessing(1, "zh")
	if err != nil {
		t.Fatalf("GetArticlesForAIBatchProcessing error = %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("GetArticlesForAIBatchProcessing returned %d articles, want 1 repair candidate", len(articles))
	}
	if !articles[0].ClusterNeedsEmbeddingRepair {
		t.Fatal("ClusterNeedsEmbeddingRepair = false, want true for complete cluster without embeddings")
	}
}

func TestReconcileRenormalizationPendingClusteringClustersResidualReadyArticles(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := NewAIEnhancedManager(db)
	defer manager.Stop()

	feedID := mustCreateTestFeed(t, db, 1, false)
	firstArticleID := mustInsertBatchArticle(t, db, 1, feedID, false, time.Now().Add(-2*time.Hour), "residual-ready-1", true)
	secondArticleID := mustInsertBatchArticle(t, db, 1, feedID, false, time.Now().Add(-time.Hour), "residual-ready-2", true)

	if err := db.UpdateArticleEmbeddings(firstArticleID, mustEmbeddingBlob(t), mustEmbeddingBlob(t)); err != nil {
		t.Fatalf("UpdateArticleEmbeddings first error = %v", err)
	}
	if err := db.UpdateArticleEmbeddings(secondArticleID, mustEmbeddingBlob(t), mustEmbeddingBlob(t)); err != nil {
		t.Fatalf("UpdateArticleEmbeddings second error = %v", err)
	}

	reconciled, err := manager.reconcileRenormalizationPendingClustering(1, "zh")
	if err != nil {
		t.Fatalf("reconcileRenormalizationPendingClustering error = %v", err)
	}
	if reconciled != 2 {
		t.Fatalf("reconciled = %d, want 2", reconciled)
	}

	var clusteredCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM articles WHERE id IN (?, ?) AND cluster_id IS NOT NULL`,
		firstArticleID,
		secondArticleID,
	).Scan(&clusteredCount); err != nil {
		t.Fatalf("query clusteredCount error = %v", err)
	}
	if clusteredCount != 2 {
		t.Fatalf("clusteredCount = %d, want 2", clusteredCount)
	}

	progress, err := db.GetAIReclusterNormalizationProgress(1, "zh")
	if err != nil {
		t.Fatalf("GetAIReclusterNormalizationProgress error = %v", err)
	}
	if progress.PendingClusteringArticles != 0 {
		t.Fatalf("PendingClusteringArticles = %d, want 0", progress.PendingClusteringArticles)
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func mustSerializeRawEmbeddingBlob(t *testing.T, leadingValue float32) []byte {
	t.Helper()

	vec := make([]float32, 1024)
	vec[0] = leadingValue
	blob, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		t.Fatalf("SerializeFloat32 raw embedding error = %v", err)
	}
	return blob
}
