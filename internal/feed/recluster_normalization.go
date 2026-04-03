package feed

import (
	"fmt"
	"log"
	"time"

	"MavenRSS/internal/dedup"
	"MavenRSS/internal/store/sqlite"
)

type clusterRenormalizeResponse struct {
	Scheduled bool   `json:"scheduled"`
	Reason    string `json:"reason,omitempty"`
	Message   string `json:"message,omitempty"`
}

var (
	renormalizationWaitPollInterval   = 200 * time.Millisecond
	renormalizationArticleWaitTimeout = 60 * time.Second
	renormalizationClusterWaitTimeout = 60 * time.Second
	renormalizationPreclusterWindow   = articlePipelineWindow
)

type renormalizationArticleState struct {
	translateArticles bool
	hasSummary        bool
	hasTranslation    bool
	hasEmbedding      bool
	hasCluster        bool
}

type renormalizationWorkItem struct {
	article        sqlite.AIBatchProcessingArticle
	preclusterTask *AIEnhancedTask
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

func (m *AIEnhancedManager) InterruptUserWork(userID int64) int {
	if m == nil || userID <= 0 {
		return 0
	}
	return m.interruptUserWorkForRenormalization(userID)
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

	workItems := make([]renormalizationWorkItem, 0, len(articles))
	for _, article := range articles {
		workItems = append(workItems, renormalizationWorkItem{
			article:        article,
			preclusterTask: buildRenormalizationPreclusterTask(article, userID),
		})
	}

	nextToEnqueue := 0
	for nextToEnqueue < len(workItems) && nextToEnqueue < renormalizationPreclusterWindow {
		if err := m.enqueueRenormalizationPreclusterTask(&workItems[nextToEnqueue]); err != nil {
			m.recordTaskFailure(userID, "recluster_normalize", nil, "", "", err)
			log.Printf("Failed to enqueue renormalization precluster task for user %d: %v", userID, err)
			return
		}
		nextToEnqueue++
	}

	for idx := range workItems {
		item := &workItems[idx]

		if err := m.waitForRenormalizationArticleTask(userID, item.article.Article.ID, targetLang); err != nil {
			m.recordTaskFailure(userID, "recluster_normalize", nil, "", "", err)
			log.Printf("Waiting for renormalization article precluster stages failed for user %d: %v", userID, err)
			return
		}

		readyForClustering, err := m.isRenormalizationArticleReadyForClustering(userID, item.article.Article.ID, targetLang)
		if err != nil {
			m.recordTaskFailure(userID, "recluster_normalize", nil, "", "", err)
			log.Printf("Checking renormalization article readiness failed for user %d article %d: %v", userID, item.article.Article.ID, err)
			return
		}
		if readyForClustering {
			if err := m.clusterRenormalizationArticle(item.article, userID); err != nil {
				task := &AIEnhancedTask{
					ArticleID:    item.article.Article.ID,
					UserID:       userID,
					FeedID:       item.article.Article.FeedID,
					ArticleTitle: item.article.Article.Title,
				}
				m.recordTaskFailure(userID, "clustering", task, "", "", err)
				log.Printf("Serial renormalization clustering failed for user %d article %d: %v", userID, item.article.Article.ID, err)
			} else {
				log.Printf("Serial renormalization clustering completed for article %d", item.article.Article.ID)
			}
		}

		if nextToEnqueue < len(workItems) {
			if err := m.enqueueRenormalizationPreclusterTask(&workItems[nextToEnqueue]); err != nil {
				m.recordTaskFailure(userID, "recluster_normalize", nil, "", "", err)
				log.Printf("Failed to enqueue renormalization precluster task for user %d: %v", userID, err)
				return
			}
			nextToEnqueue++
		}
	}

	reconciled, err := m.reconcileRenormalizationPendingClustering(userID, targetLang)
	if err != nil {
		m.recordTaskFailure(userID, "recluster_normalize", nil, "", "", err)
		log.Printf("Reconciling residual renormalization clustering failed for user %d: %v", userID, err)
		return
	}
	if reconciled > 0 {
		log.Printf("Reconciled %d residual ready-to-cluster renormalization articles for user %d", reconciled, userID)
	}

	m.requestClusterPipeline(userID)
	if err := m.waitForUserArticleProcessingIdle(userID); err != nil {
		m.recordTaskFailure(userID, "recluster_normalize", nil, "", "", err)
		log.Printf("Waiting for final cluster pipeline failed for user %d: %v", userID, err)
		return
	}
	if _, err := m.db.BackfillEmptyClusterMergedContent(userID); err != nil {
		m.recordTaskFailure(userID, "recluster_normalize", nil, "", "", err)
		log.Printf("Backfilling empty merged cluster content failed for user %d: %v", userID, err)
		return
	}
	if err := m.db.SyncClusterFavoriteStatesFromArticles(userID); err != nil {
		m.recordTaskFailure(userID, "recluster_normalize", nil, "", "", err)
		log.Printf("Syncing cluster favorite states failed for user %d: %v", userID, err)
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
	delete(m.clusterPipelineRequested, userID)
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

func buildRenormalizationPreclusterTask(article sqlite.AIBatchProcessingArticle, userID int64) *AIEnhancedTask {
	needsSummary := !article.HasSummary
	needsTranslation := article.TranslateArticles && !article.HasTranslation
	needsEmbedding := !article.HasArticleEmbedding
	if !needsSummary && !needsTranslation && !needsEmbedding {
		return nil
	}

	return &AIEnhancedTask{
		ArticleID:                 article.Article.ID,
		UserID:                    userID,
		FeedID:                    article.Article.FeedID,
		ArticleTitle:              article.Article.Title,
		NeedsSummary:              needsSummary,
		NeedsTranslation:          needsTranslation,
		TranslateArticles:         article.TranslateArticles,
		NeedsEmbedding:            needsEmbedding,
		NeedsDedup:                false,
		NeedsClusterRun:           false,
		ForceTitleSummaryFallback: true,
	}
}

func (m *AIEnhancedManager) enqueueRenormalizationPreclusterTask(item *renormalizationWorkItem) error {
	if item == nil || item.preclusterTask == nil {
		return nil
	}

	for {
		select {
		case <-m.stopChan:
			return fmt.Errorf("ai enhanced manager stopped")
		default:
		}

		if m.tryEnqueueTask(item.preclusterTask) {
			return nil
		}

		select {
		case <-m.stopChan:
			return fmt.Errorf("ai enhanced manager stopped")
		case <-time.After(renormalizationWaitPollInterval):
		}
	}
}

func (m *AIEnhancedManager) waitForRenormalizationArticleTask(userID, articleID int64, targetLang string) error {
	ticker := time.NewTicker(renormalizationWaitPollInterval)
	defer ticker.Stop()
	startedAt := time.Now()

	for {
		done, err := m.isRenormalizationArticleTaskDone(userID, articleID, targetLang)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if time.Since(startedAt) >= renormalizationArticleWaitTimeout {
			reason := fmt.Sprintf("article-stage wait exceeded %s during cluster renormalization", renormalizationArticleWaitTimeout)
			log.Printf(
				"Timed out waiting for renormalization article %d for user %d after %s; marking remaining stages skipped",
				articleID,
				userID,
				renormalizationArticleWaitTimeout,
			)
			if err := m.skipTimedOutRenormalizationArticle(userID, articleID, targetLang, reason); err != nil {
				return err
			}
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
	if userID <= 0 || articleID <= 0 || m.db == nil {
		return true, nil
	}

	state, err := m.getRenormalizationArticleState(userID, articleID, targetLang)
	if err != nil {
		return false, err
	}
	return state.hasSummary && state.hasTranslation && state.hasEmbedding, nil
}

func (m *AIEnhancedManager) isRenormalizationArticleReadyForClustering(userID, articleID int64, targetLang string) (bool, error) {
	state, err := m.getRenormalizationArticleState(userID, articleID, targetLang)
	if err != nil {
		return false, err
	}
	return state.hasSummary && state.hasTranslation && state.hasEmbedding && !state.hasCluster, nil
}

func (m *AIEnhancedManager) getRenormalizationArticleState(userID, articleID int64, targetLang string) (renormalizationArticleState, error) {
	state := renormalizationArticleState{}
	if userID <= 0 || articleID <= 0 || m.db == nil {
		return state, nil
	}

	if targetLang == "" {
		targetLang = "zh"
	}
	err := m.db.QueryRow(
		`SELECT
			COALESCE(f.translate_articles, 0),
			((TRIM(COALESCE(a.summary, '')) <> '' AND COALESCE(a.summary, '') <> '<no content>') OR skip_summary.article_id IS NOT NULL),
			(atc.article_id IS NOT NULL OR skip_translation.article_id IS NOT NULL),
			(ae.article_id IS NOT NULL OR skip_embedding.article_id IS NOT NULL),
			(a.cluster_id IS NOT NULL OR skip_clustering.article_id IS NOT NULL)
		FROM articles a
		LEFT JOIN feeds f ON a.feed_id = f.id
		LEFT JOIN article_translated_contents atc ON atc.article_id = a.id AND atc.target_lang = ?
		LEFT JOIN article_embeddings ae ON ae.article_id = a.id
		LEFT JOIN ai_article_stage_skips skip_summary
			ON skip_summary.user_id = a.user_id AND skip_summary.article_id = a.id AND skip_summary.stage = 'summary'
		LEFT JOIN ai_article_stage_skips skip_translation
			ON skip_translation.user_id = a.user_id AND skip_translation.article_id = a.id AND skip_translation.stage = 'translation'
		LEFT JOIN ai_article_stage_skips skip_embedding
			ON skip_embedding.user_id = a.user_id AND skip_embedding.article_id = a.id AND skip_embedding.stage = 'embedding'
		LEFT JOIN ai_article_stage_skips skip_clustering
			ON skip_clustering.user_id = a.user_id AND skip_clustering.article_id = a.id AND skip_clustering.stage = 'clustering'
		WHERE a.user_id = ? AND a.id = ?`,
		targetLang,
		userID,
		articleID,
	).Scan(
		&state.translateArticles,
		&state.hasSummary,
		&state.hasTranslation,
		&state.hasEmbedding,
		&state.hasCluster,
	)
	if err != nil {
		return state, fmt.Errorf("query renormalization article state: %w", err)
	}

	if !state.translateArticles {
		state.hasTranslation = true
	}

	return state, nil
}

func (m *AIEnhancedManager) waitForUserArticleProcessingIdle(userID int64) error {
	ticker := time.NewTicker(renormalizationWaitPollInterval)
	defer ticker.Stop()
	startedAt := time.Now()

	for {
		if !m.hasUserArticleProcessingActivity(userID) {
			return nil
		}
		if time.Since(startedAt) >= renormalizationClusterWaitTimeout {
			log.Printf(
				"Timed out waiting for remaining cluster work for user %d after %s; force-completing pending clusters",
				userID,
				renormalizationClusterWaitTimeout,
			)
			if _, err := m.forceCompleteTimedOutClusters(userID, "cluster-stage wait exceeded during renormalization"); err != nil {
				return err
			}
			m.abandonTimedOutRenormalizationActivity(userID, "final cluster-stage wait timed out during renormalization")
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
	delete(m.clusterPipelineRequested, userID)
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

func (m *AIEnhancedManager) skipTimedOutRenormalizationArticle(userID, articleID int64, targetLang, reason string) error {
	if userID <= 0 || articleID <= 0 || m.db == nil {
		return nil
	}

	state, err := m.getRenormalizationArticleState(userID, articleID, targetLang)
	if err != nil {
		return err
	}

	if !state.hasSummary {
		if err := m.fallbackArticleSummaryByID(articleID); err != nil {
			log.Printf("Failed to apply title summary fallback for timed-out article %d: %v", articleID, err)
		}
		if err := m.db.SetAIArticleStageSkip(userID, articleID, "summary", reason); err != nil {
			return fmt.Errorf("mark timed-out summary skip for article %d: %w", articleID, err)
		}
	}
	if state.translateArticles && !state.hasTranslation {
		if err := m.db.SetAIArticleStageSkip(userID, articleID, "translation", reason); err != nil {
			return fmt.Errorf("mark timed-out translation skip for article %d: %w", articleID, err)
		}
	}
	if !state.hasEmbedding {
		if err := m.db.SetAIArticleStageSkip(userID, articleID, "embedding", reason); err != nil {
			return fmt.Errorf("mark timed-out embedding skip for article %d: %w", articleID, err)
		}
	}
	if !state.hasCluster {
		if err := m.db.SetAIArticleStageSkip(userID, articleID, "clustering", reason); err != nil {
			return fmt.Errorf("mark timed-out clustering skip for article %d: %w", articleID, err)
		}
	}

	return nil
}

func (m *AIEnhancedManager) clusterRenormalizationArticle(article sqlite.AIBatchProcessingArticle, userID int64) error {
	if m == nil || m.db == nil || userID <= 0 || article.Article.ID <= 0 {
		return nil
	}

	select {
	case <-m.stopChan:
		return fmt.Errorf("ai enhanced manager stopped")
	default:
	}

	if err := m.runUserClusterAssignmentSerially(userID, func() error {
		return dedup.ProcessArticle(m.db, article.Article.ID, userID)
	}); err != nil {
		reason := fmt.Sprintf("serial renormalization clustering failed: %v", err)
		if len(reason) > 600 {
			reason = reason[:600]
		}
		if skipErr := m.db.SetAIArticleStageSkip(userID, article.Article.ID, "clustering", reason); skipErr != nil {
			log.Printf("Failed to persist clustering skip marker for article %d after serial renormalization clustering failure: %v", article.Article.ID, skipErr)
		}
		return err
	}
	return nil
}

func (m *AIEnhancedManager) reconcileRenormalizationPendingClustering(userID int64, targetLang string) (int, error) {
	if m == nil || m.db == nil || userID <= 0 {
		return 0, nil
	}

	articles, err := m.db.GetReadyArticlesForAIReclusterClustering(userID, targetLang)
	if err != nil {
		return 0, err
	}

	reconciled := 0
	for _, article := range articles {
		if err := m.clusterRenormalizationArticle(article, userID); err != nil {
			task := &AIEnhancedTask{
				ArticleID:    article.Article.ID,
				UserID:       userID,
				FeedID:       article.Article.FeedID,
				ArticleTitle: article.Article.Title,
			}
			m.recordTaskFailure(userID, "clustering", task, "", "", err)
			log.Printf("Residual renormalization clustering failed for user %d article %d: %v", userID, article.Article.ID, err)
			continue
		}
		reconciled++
	}
	if reconciled > 0 {
		m.requestClusterPipeline(userID)
	}

	return reconciled, nil
}

func (m *AIEnhancedManager) fallbackArticleSummaryByID(articleID int64) error {
	if m == nil || m.db == nil || articleID <= 0 {
		return nil
	}

	article, err := m.db.GetArticleByID(articleID)
	if err != nil {
		return fmt.Errorf("load article for summary fallback: %w", err)
	}
	if article == nil {
		return nil
	}

	title := article.Title
	if article.TranslatedTitle != "" {
		title = article.TranslatedTitle
	}
	if title == "" {
		return nil
	}

	if err := m.db.UpdateArticleSummary(articleID, title); err != nil {
		return fmt.Errorf("persist title summary fallback: %w", err)
	}
	return nil
}

func (m *AIEnhancedManager) abandonTimedOutRenormalizationActivity(userID int64, reason string) {
	if userID <= 0 {
		return
	}

	newVersion := m.nextUserOperationVersion(userID)
	removed := m.purgeQueuedTasksForUser(userID)

	m.statusMu.Lock()
	delete(m.queuedTasksByUser, userID)
	delete(m.activeWorkerTasksByUser, userID)
	delete(m.activeAsyncWorkByUser, userID)
	m.statusMu.Unlock()

	m.clusterMu.Lock()
	delete(m.clusterPipelineRunning, userID)
	delete(m.clusterPipelineQueued, userID)
	delete(m.clusterPipelineRequested, userID)
	delete(m.clusterPipelineVersion, userID)
	delete(m.clusterPipelineQueuedVer, userID)
	delete(m.clusterFusionRunning, userID)
	delete(m.clusterEmbeddingRunning, userID)
	m.clusterMu.Unlock()

	log.Printf(
		"Abandoned timed-out renormalization activity for user %d (%s); purged %d queued tasks and advanced to operation version %d",
		userID,
		reason,
		removed,
		newVersion,
	)
}

func (m *AIEnhancedManager) forceCompleteTimedOutClusters(userID int64, reason string) (int, error) {
	if userID <= 0 || m == nil || m.db == nil {
		return 0, nil
	}

	completed := 0
	pendingMerge, err := m.db.GetClustersByStatus(userID, "pending_merge")
	if err != nil {
		return completed, fmt.Errorf("load timed-out pending_merge clusters: %w", err)
	}
	for _, cluster := range pendingMerge {
		if err := m.db.UpdateClusterStatus(cluster.ID, "complete"); err != nil {
			return completed, fmt.Errorf("complete timed-out merge cluster %d: %w", cluster.ID, err)
		}
		completed++
	}

	pendingEmbed, err := m.db.GetClustersByStatus(userID, "pending_embed")
	if err != nil {
		return completed, fmt.Errorf("load timed-out pending_embed clusters: %w", err)
	}
	for _, cluster := range pendingEmbed {
		if err := m.db.UpdateClusterStatus(cluster.ID, "complete"); err != nil {
			return completed, fmt.Errorf("complete timed-out embedding cluster %d: %w", cluster.ID, err)
		}
		completed++
	}

	if completed > 0 {
		if _, err := m.db.BackfillEmptyClusterMergedContent(userID); err != nil {
			return completed, fmt.Errorf("backfill timed-out cluster content: %w", err)
		}
		if err := m.db.SyncClusterFavoriteStatesFromArticles(userID); err != nil {
			return completed, fmt.Errorf("sync timed-out cluster favorites: %w", err)
		}
		log.Printf("Force-completed %d timed-out clusters for user %d (%s)", completed, userID, reason)
	}

	return completed, nil
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
