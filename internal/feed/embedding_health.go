package feed

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"MavenRSS/internal/interest"
)

const (
	systemMessageKindAIEmbeddingHealthBlocked = "ai_embedding_health_blocked"
	embeddingHealthSampleLimit                = 100
	embeddingHealthBlockRatio                 = 0.70
	embeddingHealthTolerance                  = 1e-3
	embeddingHealthCacheTTL                   = time.Minute

	blockedScopeNewArticleQueue          = "new_article_ai_queue"
	blockedScopeBatchQueue               = "batch_ai_processing"
	blockedScopeClusterPipeline          = "cluster_pipeline"
	blockedScopeDailyRecommendationAuto  = "daily_recommendation_scheduler"
	blockedScopeDailyRecommendationQueue = "daily_recommendation_regenerate"
	blockedScopeDailyRecommendationForce = "daily_recommendation_refresh"
	blockedScopeDailyRecommendationRun   = "daily_recommendation_runtime"
)

type EmbeddingHealthStatus struct {
	SampleSize        int     `json:"sample_size"`
	UnnormalizedCount int     `json:"unnormalized_count"`
	UnnormalizedRatio float64 `json:"unnormalized_ratio"`
	IsHealthy         bool    `json:"is_healthy"`
}

func (m *AIEnhancedManager) getEmbeddingHealthStatus(userID int64, force bool) (EmbeddingHealthStatus, error) {
	if m == nil || m.db == nil || userID <= 0 {
		return EmbeddingHealthStatus{IsHealthy: true}, nil
	}

	if !force {
		m.statusMu.Lock()
		cached, ok := m.embeddingHealthByUser[userID]
		checkedAt := m.embeddingHealthCheckedAt[userID]
		m.statusMu.Unlock()
		if ok && time.Since(checkedAt) < embeddingHealthCacheTTL {
			return cached, nil
		}
	}

	samples, err := m.db.SampleArticleSummaryEmbeddings(userID, embeddingHealthSampleLimit)
	if err != nil {
		return EmbeddingHealthStatus{}, fmt.Errorf("sample article summary embeddings: %w", err)
	}

	status := EmbeddingHealthStatus{
		SampleSize: len(samples),
		IsHealthy:  true,
	}

	for _, blob := range samples {
		vec, err := interest.DeserializeVector(blob)
		if err != nil || !interest.IsNormalized(vec, embeddingHealthTolerance) {
			status.UnnormalizedCount++
		}
	}

	if status.SampleSize > 0 {
		status.UnnormalizedRatio = float64(status.UnnormalizedCount) / float64(status.SampleSize)
	}
	status.IsHealthy = status.UnnormalizedRatio <= embeddingHealthBlockRatio

	m.statusMu.Lock()
	if m.embeddingHealthByUser == nil {
		m.embeddingHealthByUser = make(map[int64]EmbeddingHealthStatus)
	}
	if m.embeddingHealthCheckedAt == nil {
		m.embeddingHealthCheckedAt = make(map[int64]time.Time)
	}
	m.embeddingHealthByUser[userID] = status
	m.embeddingHealthCheckedAt[userID] = time.Now()
	m.statusMu.Unlock()

	return status, nil
}

func (m *AIEnhancedManager) guardEmbeddingHealth(userID int64, triggerScope string) (EmbeddingHealthStatus, bool, error) {
	status, err := m.getEmbeddingHealthStatus(userID, false)
	if err != nil {
		return EmbeddingHealthStatus{}, false, err
	}
	if status.IsHealthy {
		return status, true, nil
	}

	if notifyErr := m.publishEmbeddingHealthBlockedMessage(userID, status, triggerScope); notifyErr != nil {
		log.Printf("Failed to publish embedding health blocked message for user %d: %v", userID, notifyErr)
	}

	return status, false, nil
}

func (m *AIEnhancedManager) publishEmbeddingHealthBlockedMessage(userID int64, status EmbeddingHealthStatus, triggerScope string) error {
	if m == nil || m.db == nil {
		return nil
	}

	metadata := map[string]any{
		"sample_size":        status.SampleSize,
		"unnormalized_count": status.UnnormalizedCount,
		"unnormalized_ratio": status.UnnormalizedRatio,
		"trigger_scope":      triggerScope,
		"blocked_scope_range": []string{
			blockedScopeNewArticleQueue,
			blockedScopeBatchQueue,
			blockedScopeClusterPipeline,
			blockedScopeDailyRecommendationAuto,
			blockedScopeDailyRecommendationQueue,
			blockedScopeDailyRecommendationForce,
			blockedScopeDailyRecommendationRun,
		},
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal embedding health metadata: %w", err)
	}

	title := "AI task flow blocked by unhealthy summary embeddings"
	body := fmt.Sprintf(
		"Embedding health check failed. Sampled %d article summary vectors and found %d unnormalized vectors (%.1f%%). "+
			"The AI-enhanced article queue, batch clustering, cluster fusion, and daily recommendations are blocked until summary embeddings are normalized. "+
			"This release does not backfill historical vectors automatically; a later full rebuild is still required.",
		status.SampleSize,
		status.UnnormalizedCount,
		status.UnnormalizedRatio*100,
	)

	if _, err := m.db.UpsertSystemMessage(userID, systemMessageKindAIEmbeddingHealthBlocked, title, body, string(metadataJSON)); err != nil {
		return fmt.Errorf("upsert embedding health blocked message: %w", err)
	}

	return nil
}
