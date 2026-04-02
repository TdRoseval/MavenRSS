package feed

import (
	"context"
	"testing"
	"time"

	"MavenRSS/internal/dedup"
	"MavenRSS/internal/store/sqlite"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

func TestGetEmbeddingHealthStatusBlocksWhenUnnormalizedRatioExceedsThreshold(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := &AIEnhancedManager{
		db:                       db,
		embeddingHealthByUser:    make(map[int64]EmbeddingHealthStatus),
		embeddingHealthCheckedAt: make(map[int64]time.Time),
	}

	feedID := mustCreateTestFeed(t, db, 1, false)
	insertArticleWithSummaryEmbedding(t, db, 1, feedID, "health-a", mustRawEmbeddingBlob(t, vectorWithDimensions(0, 0)))
	insertArticleWithSummaryEmbedding(t, db, 1, feedID, "health-b", mustRawEmbeddingBlob(t, vectorWithDimensions(0, 0)))
	insertArticleWithSummaryEmbedding(t, db, 1, feedID, "health-c", mustRawEmbeddingBlob(t, vectorWithDimensions(0, 0)))
	insertArticleWithSummaryEmbedding(t, db, 1, feedID, "health-d", mustUnitEmbeddingBlob(t))

	status, err := manager.getEmbeddingHealthStatus(1, true)
	if err != nil {
		t.Fatalf("getEmbeddingHealthStatus error: %v", err)
	}

	if status.SampleSize != 4 {
		t.Fatalf("SampleSize = %d, want 4", status.SampleSize)
	}
	if status.UnnormalizedCount != 3 {
		t.Fatalf("UnnormalizedCount = %d, want 3", status.UnnormalizedCount)
	}
	if status.IsHealthy {
		t.Fatal("IsHealthy = true, want false")
	}
}

func TestRunClusterPipelineOnceSkipsWhenEmbeddingHealthBlockedAndMergesSystemMessage(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := &AIEnhancedManager{
		db:                       db,
		embeddingHealthByUser:    make(map[int64]EmbeddingHealthStatus),
		embeddingHealthCheckedAt: make(map[int64]time.Time),
	}

	feedID := mustCreateTestFeed(t, db, 1, false)
	insertArticleWithSummaryEmbedding(t, db, 1, feedID, "blocked-a", mustRawEmbeddingBlob(t, vectorWithDimensions(0, 0)))
	insertArticleWithSummaryEmbedding(t, db, 1, feedID, "blocked-b", mustRawEmbeddingBlob(t, vectorWithDimensions(0, 0)))
	insertArticleWithSummaryEmbedding(t, db, 1, feedID, "blocked-c", mustRawEmbeddingBlob(t, vectorWithDimensions(0, 0)))
	insertArticleWithSummaryEmbedding(t, db, 1, feedID, "blocked-d", mustUnitEmbeddingBlob(t))

	fusionRuns := 0
	manager.resolveFusionConfig = func(userID int64) (*dedup.FusionConfig, error) {
		return &dedup.FusionConfig{}, nil
	}
	manager.runFusion = func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
		fusionRuns++
		return nil
	}
	manager.runEmbedding = func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error {
		t.Fatal("runEmbedding should not be called when health gate blocks")
		return nil
	}

	if err := manager.runClusterPipelineOnce(1); err != nil {
		t.Fatalf("runClusterPipelineOnce error: %v", err)
	}
	if err := manager.runClusterPipelineOnce(1); err != nil {
		t.Fatalf("second runClusterPipelineOnce error: %v", err)
	}
	if fusionRuns != 0 {
		t.Fatalf("fusionRuns = %d, want 0", fusionRuns)
	}

	messages, err := db.ListSystemMessages(1, 100)
	if err != nil {
		t.Fatalf("ListSystemMessages error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].Kind != systemMessageKindAIEmbeddingHealthBlocked {
		t.Fatalf("message kind = %q", messages[0].Kind)
	}

	unreadCount, err := db.CountUnreadSystemMessages(1)
	if err != nil {
		t.Fatalf("CountUnreadSystemMessages error: %v", err)
	}
	if unreadCount != 1 {
		t.Fatalf("unread count = %d, want 1", unreadCount)
	}
}

func TestQueueDailyRecommendationsReturnsFalseWhenEmbeddingHealthBlocked(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := &AIEnhancedManager{
		db:                       db,
		embeddingHealthByUser:    make(map[int64]EmbeddingHealthStatus),
		embeddingHealthCheckedAt: make(map[int64]time.Time),
	}

	feedID := mustCreateTestFeed(t, db, 1, false)
	insertArticleWithSummaryEmbedding(t, db, 1, feedID, "recommend-a", mustRawEmbeddingBlob(t, vectorWithDimensions(0, 0)))
	insertArticleWithSummaryEmbedding(t, db, 1, feedID, "recommend-b", mustRawEmbeddingBlob(t, vectorWithDimensions(0, 0)))
	insertArticleWithSummaryEmbedding(t, db, 1, feedID, "recommend-c", mustRawEmbeddingBlob(t, vectorWithDimensions(0, 0)))
	insertArticleWithSummaryEmbedding(t, db, 1, feedID, "recommend-d", mustUnitEmbeddingBlob(t))

	scheduled, err := manager.QueueDailyRecommendations(1, "2026-04-02", true, true)
	if err != nil {
		t.Fatalf("QueueDailyRecommendations error: %v", err)
	}
	if scheduled {
		t.Fatal("scheduled = true, want false when health gate blocks")
	}
}

func insertArticleWithSummaryEmbedding(t *testing.T, db *sqlite.DB, userID, feedID int64, uniqueID string, blob []byte) int64 {
	t.Helper()

	articleID := mustInsertBatchArticle(t, db, userID, feedID, false, time.Now().Add(-time.Hour), uniqueID, true)
	if err := db.UpdateArticleEmbeddings(articleID, nil, blob); err != nil {
		t.Fatalf("UpdateArticleEmbeddings error: %v", err)
	}
	return articleID
}

func mustRawEmbeddingBlob(t *testing.T, vec []float32) []byte {
	t.Helper()

	blob, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		t.Fatalf("SerializeFloat32 error: %v", err)
	}
	return blob
}

func vectorWithDimensions(values ...float32) []float32 {
	vec := make([]float32, 1024)
	copy(vec, values)
	return vec
}
