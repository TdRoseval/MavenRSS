package feed

import (
	"fmt"
	"log"
	"time"

	"MavenRSS/internal/store/sqlite"
)

type clusterRenormalizeResponse struct {
	Scheduled bool   `json:"scheduled"`
	Reason    string `json:"reason,omitempty"`
	Message   string `json:"message,omitempty"`
}

func (m *AIEnhancedManager) StartClusterRenormalization(userID int64) (bool, string, error) {
	if m == nil || m.db == nil || userID <= 0 {
		return false, "disabled", nil
	}
	if !ShouldProcess(m.db, userID) {
		return false, "disabled", nil
	}
	if m.isUserBusyForRenormalization(userID) {
		return false, "busy", nil
	}
	if !m.beginClusterRenormalization(userID) {
		return false, "busy", nil
	}

	go m.runClusterRenormalization(userID)
	return true, "", nil
}

func (m *AIEnhancedManager) ForceStartClusterRenormalization(userID int64) (bool, string, error) {
	if m == nil || m.db == nil || userID <= 0 {
		return false, "disabled", nil
	}
	if !ShouldProcess(m.db, userID) {
		return false, "disabled", nil
	}
	if m.isRenormalizationRunning(userID) {
		return false, "busy", nil
	}

	removed := m.interruptUserWorkForRenormalization(userID)
	if !m.beginClusterRenormalization(userID) {
		return false, "busy", nil
	}

	log.Printf("Force starting cluster renormalization for user %d after interrupting %d queued AI tasks", userID, removed)
	go m.runClusterRenormalization(userID)
	return true, "", nil
}

func (m *AIEnhancedManager) beginClusterRenormalization(userID int64) bool {
	if userID <= 0 {
		return false
	}

	m.statusMu.Lock()
	defer m.statusMu.Unlock()

	if m.renormalizationRunning == nil {
		m.renormalizationRunning = make(map[int64]bool)
	}
	if m.renormalizationRunning[userID] {
		return false
	}
	m.renormalizationRunning[userID] = true
	return true
}

func (m *AIEnhancedManager) finishClusterRenormalization(userID int64) {
	if userID <= 0 {
		return
	}

	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	delete(m.renormalizationRunning, userID)
}

func (m *AIEnhancedManager) isRenormalizationRunning(userID int64) bool {
	if userID <= 0 {
		return false
	}

	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	return m.renormalizationRunning[userID]
}

func (m *AIEnhancedManager) isUserBusyForRenormalization(userID int64) bool {
	if userID <= 0 {
		return false
	}

	if m.isRenormalizationRunning(userID) {
		return true
	}

	queued, activeWorker, activeAsync := m.getUserTaskCounts(userID)
	if queued > 0 || activeWorker > 0 || activeAsync > 0 {
		return true
	}

	m.statusMu.Lock()
	recoveryBusy := m.recoveryInProgress[userID]
	m.statusMu.Unlock()
	if recoveryBusy {
		return true
	}

	m.clusterMu.Lock()
	clusterBusy := m.clusterPipelineRunning[userID] ||
		m.clusterPipelineQueued[userID] ||
		m.clusterFusionRunning[userID] ||
		m.clusterEmbeddingRunning[userID]
	m.clusterMu.Unlock()
	if !clusterBusy {
		clusterBusy = m.userHasPendingClusterWork(userID)
	}
	if clusterBusy {
		return true
	}

	m.recommendationMu.Lock()
	recommendationBusy := m.recommendationRunning[userID] ||
		m.pendingRecommendationDate[userID] != "" ||
		m.pendingRecommendationWait[userID] ||
		m.pendingRecommendationForce[userID]
	m.recommendationMu.Unlock()

	return recommendationBusy
}

func (m *AIEnhancedManager) runClusterRenormalization(userID int64) {
	defer m.finishClusterRenormalization(userID)

	if err := m.db.ResetAIClustersForRenormalization(userID); err != nil {
		m.recordTaskFailure(userID, "recluster_normalize", nil, "", "", err)
		log.Printf("Failed to reset AI cluster state for user %d: %v", userID, err)
		return
	}

	m.clearUserRuntimeStateForRenormalization(userID)

	if _, _, err := m.db.NormalizeArticleEmbeddingsForUser(userID); err != nil {
		m.recordTaskFailure(userID, "recluster_normalize", nil, "", "", err)
		log.Printf("Failed to normalize article embeddings for user %d: %v", userID, err)
		return
	}

	targetLang, _ := m.db.GetSettingWithFallback(userID, "target_language")
	if targetLang == "" {
		targetLang = "zh"
	}

	articles, err := m.db.GetArticlesForAIReclusterNormalization(userID, targetLang)
	if err != nil {
		m.recordTaskFailure(userID, "recluster_normalize", nil, "", "", err)
		log.Printf("Failed to load articles for cluster renormalization for user %d: %v", userID, err)
		return
	}

	for _, article := range articles {
		task := buildRenormalizationTask(article, userID)
		if task == nil {
			continue
		}
		select {
		case <-m.stopChan:
			log.Printf("Stopping cluster renormalization early for user %d because AI manager is stopping", userID)
			return
		default:
		}
		if !m.tryEnqueueTask(task) {
			log.Printf("AI enhanced task queue full during cluster renormalization for user %d", userID)
			return
		}
		if err := m.waitForRenormalizationArticleTask(userID, task.ArticleID, targetLang); err != nil {
			m.recordTaskFailure(userID, "recluster_normalize", nil, "", "", err)
			log.Printf("Waiting for renormalization article-stage task failed for user %d: %v", userID, err)
			return
		}
	}

	m.scheduleClusterPipeline(userID)
	if err := m.waitForUserArticleProcessingIdle(userID); err != nil {
		m.recordTaskFailure(userID, "recluster_normalize", nil, "", "", err)
		log.Printf("Waiting for final cluster pipeline failed for user %d: %v", userID, err)
		return
	}

	m.requestMissingRecommendationBackfillDuringRenormalization(userID)
}

func (m *AIEnhancedManager) interruptUserWorkForRenormalization(userID int64) int {
	if userID <= 0 {
		return 0
	}

	newVersion := m.nextUserOperationVersion(userID)
	removed := m.purgeQueuedTasksForUser(userID)

	m.clusterMu.Lock()
	delete(m.clusterPipelineRunning, userID)
	delete(m.clusterPipelineQueued, userID)
	delete(m.clusterPipelineVersion, userID)
	delete(m.clusterPipelineQueuedVer, userID)
	delete(m.clusterFusionRunning, userID)
	delete(m.clusterEmbeddingRunning, userID)
	m.clusterMu.Unlock()

	m.recommendationMu.Lock()
	delete(m.recommendationRunning, userID)
	delete(m.recommendationRunningVer, userID)
	delete(m.recommendationStatusByUser, userID)
	delete(m.pendingRecommendationDate, userID)
	delete(m.pendingRecommendationWait, userID)
	delete(m.pendingRecommendationForce, userID)
	delete(m.pendingRecommendationMode, userID)
	delete(m.pendingRecommendationVer, userID)
	m.recommendationMu.Unlock()

	log.Printf("Interrupted AI article/cluster/recommendation work for user %d; switched to operation version %d", userID, newVersion)
	return removed
}

func buildRenormalizationTask(article sqlite.AIBatchProcessingArticle, userID int64) *AIEnhancedTask {
	needsSummary := !article.HasSummary
	needsTranslation := article.TranslateArticles && !article.HasTranslation
	needsEmbedding := !article.HasArticleEmbedding

	return &AIEnhancedTask{
		ArticleID:                 article.Article.ID,
		UserID:                    userID,
		FeedID:                    article.Article.FeedID,
		ArticleTitle:              article.Article.Title,
		NeedsSummary:              needsSummary,
		NeedsTranslation:          needsTranslation,
		TranslateArticles:         article.TranslateArticles,
		NeedsEmbedding:            needsEmbedding,
		NeedsDedup:                true,
		NeedsClusterRun:           false,
		ForceTitleSummaryFallback: true,
	}
}

func (m *AIEnhancedManager) waitForRenormalizationArticleTask(userID, articleID int64, targetLang string) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		done, err := m.isRenormalizationArticleTaskDone(userID, articleID, targetLang)
		if err != nil {
			return err
		}
		if done {
			return nil
		}

		select {
		case <-m.stopChan:
			return fmt.Errorf("ai enhanced manager stopped")
		case <-ticker.C:
		}
	}
}

func (m *AIEnhancedManager) isRenormalizationArticleTaskDone(userID, articleID int64, targetLang string) (bool, error) {
	queued, activeWorker, activeAsync := m.getUserTaskCounts(userID)
	if queued > 0 || activeWorker > 0 || activeAsync > 0 {
		return false, nil
	}

	if userID <= 0 || articleID <= 0 || m.db == nil {
		return true, nil
	}

	if targetLang == "" {
		targetLang = "zh"
	}
	var (
		translateArticles bool
		hasSummary        bool
		hasTranslation    bool
		hasEmbedding      bool
		hasCluster        bool
	)
	err := m.db.QueryRow(
		`SELECT
			COALESCE(f.translate_articles, 0),
			((TRIM(COALESCE(a.summary, '')) <> '' AND COALESCE(a.summary, '') <> '<no content>') OR skip_summary.article_id IS NOT NULL),
			(atc.article_id IS NOT NULL OR skip_translation.article_id IS NOT NULL),
			ae.article_id IS NOT NULL,
			a.cluster_id IS NOT NULL
		FROM articles a
		LEFT JOIN feeds f ON a.feed_id = f.id
		LEFT JOIN article_translated_contents atc ON atc.article_id = a.id AND atc.target_lang = ?
		LEFT JOIN article_embeddings ae ON ae.article_id = a.id
		LEFT JOIN ai_article_stage_skips skip_summary
			ON skip_summary.user_id = a.user_id AND skip_summary.article_id = a.id AND skip_summary.stage = 'summary'
		LEFT JOIN ai_article_stage_skips skip_translation
			ON skip_translation.user_id = a.user_id AND skip_translation.article_id = a.id AND skip_translation.stage = 'translation'
		WHERE a.user_id = ? AND a.id = ?`,
		targetLang,
		userID,
		articleID,
	).Scan(
		&translateArticles,
		&hasSummary,
		&hasTranslation,
		&hasEmbedding,
		&hasCluster,
	)
	if err != nil {
		return false, fmt.Errorf("query renormalization article state: %w", err)
	}

	if !translateArticles {
		hasTranslation = true
	}

	return hasSummary && hasTranslation && hasEmbedding && hasCluster, nil
}

func (m *AIEnhancedManager) waitForUserArticleProcessingIdle(userID int64) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		if !m.hasUserArticleProcessingActivity(userID) {
			return nil
		}

		select {
		case <-m.stopChan:
			return fmt.Errorf("ai enhanced manager stopped")
		case <-ticker.C:
		}
	}
}

func (m *AIEnhancedManager) hasUserArticleProcessingActivity(userID int64) bool {
	queued, activeWorker, activeAsync := m.getUserTaskCounts(userID)
	if queued > 0 || activeWorker > 0 || activeAsync > 0 {
		return true
	}

	m.clusterMu.Lock()
	clusterBusy := m.clusterPipelineRunning[userID] ||
		m.clusterPipelineQueued[userID] ||
		m.clusterFusionRunning[userID] ||
		m.clusterEmbeddingRunning[userID]
	m.clusterMu.Unlock()
	return clusterBusy || m.userHasPendingClusterWork(userID)
}

func (m *AIEnhancedManager) clearUserRuntimeStateForRenormalization(userID int64) {
	if userID <= 0 {
		return
	}

	_ = m.db.SetSettingForUser(userID, aiProcessingSnapshotSettingKey, "")
	_ = m.db.SetSettingForUser(userID, aiProcessingLastProgressAtSettingKey, "")
	_ = m.db.SetSettingForUser(userID, aiProcessingFreezeSuspendedSettingKey, "false")

	m.statusMu.Lock()
	delete(m.embeddingHealthByUser, userID)
	delete(m.embeddingHealthCheckedAt, userID)
	delete(m.recentFailureByUser, userID)
	delete(m.recoveryInProgress, userID)
	delete(m.lastRecoveryAttemptByUser, userID)
	delete(m.queuedTasksByUser, userID)
	delete(m.activeWorkerTasksByUser, userID)
	delete(m.activeAsyncWorkByUser, userID)
	m.statusMu.Unlock()

	m.clusterMu.Lock()
	delete(m.clusterPipelineRunning, userID)
	delete(m.clusterPipelineQueued, userID)
	delete(m.clusterFusionRunning, userID)
	delete(m.clusterEmbeddingRunning, userID)
	m.clusterMu.Unlock()

	m.recommendationMu.Lock()
	delete(m.recommendationRunning, userID)
	delete(m.recommendationStatusByUser, userID)
	delete(m.pendingRecommendationDate, userID)
	delete(m.pendingRecommendationWait, userID)
	delete(m.pendingRecommendationForce, userID)
	delete(m.pendingRecommendationMode, userID)
	m.recommendationMu.Unlock()
}

func (m *AIEnhancedManager) requestMissingRecommendationBackfillDuringRenormalization(userID int64) {
	if userID <= 0 {
		return
	}

	targetDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	shouldQueue, forceRegenerate, err := m.shouldQueueDailyRecommendations(userID, targetDate)
	if err != nil {
		log.Printf("check recommendation backfill after cluster renormalization for user %d error: %v", userID, err)
		return
	}
	if !shouldQueue {
		return
	}

	m.queueRecommendationGeneration(userID, targetDate, true, forceRegenerate, recommendationTriggerAutomatic)
}
