package feed

import (
	"encoding/json"
	"log"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"MavenRSS/internal/store/sqlite"
)

const (
	aiPipelineDiagnosticInterval           = 5 * time.Minute
	aiPipelineDiagnosticBlockingSampleSize = 5
)

type aiPipelineDiagnosticSnapshot struct {
	UserID                    int64                                    `json:"user_id"`
	CollectedAt               string                                   `json:"collected_at"`
	Reason                    string                                   `json:"reason,omitempty"`
	IsEnabled                 bool                                     `json:"is_enabled"`
	LifecycleActive           bool                                     `json:"lifecycle_active"`
	TargetLanguage            string                                   `json:"target_language"`
	HasInterestVector         bool                                     `json:"has_interest_vector"`
	IsRenormalizationRunning  bool                                     `json:"is_renormalization_running"`
	ArticleProgress           sqlite.AIProcessingProgress              `json:"article_progress"`
	ClusterProgress           sqlite.ClusterProcessingProgress         `json:"cluster_progress"`
	PendingRecommendationDays int                                      `json:"pending_recommendation_days"`
	Runtime                   aiPipelineRuntimeDiagnostic              `json:"runtime"`
	ClusterRuntime            aiPipelineClusterRuntimeDiagnostic       `json:"cluster_runtime"`
	Recommendation            DailyRecommendationTaskStatus            `json:"recommendation"`
	EmbeddingHealth           EmbeddingHealthStatus                    `json:"embedding_health"`
	Stale                     aiPipelineStaleDiagnostic                `json:"stale"`
	RecentFailure             aiPipelineRecentFailureDiagnostic        `json:"recent_failure,omitempty"`
	StageSkipCounts           []sqlite.AIStageCount                    `json:"stage_skip_counts,omitempty"`
	StageTimeoutFailures      []sqlite.AIStageTimeoutFailureSummary    `json:"stage_timeout_failures,omitempty"`
	ClusterStatusCounts       []sqlite.ClusterStatusCount              `json:"cluster_status_counts,omitempty"`
	BlockingArticleSamples    []sqlite.AIPipelineBlockingArticleSample `json:"blocking_article_samples,omitempty"`
	ClusterBarrier            aiPipelineClusterBarrierDiagnostic       `json:"cluster_barrier"`
	Blockers                  []string                                 `json:"blockers,omitempty"`
	Errors                    []string                                 `json:"errors,omitempty"`
}

type aiPipelineRuntimeDiagnostic struct {
	QueuedTasks             int    `json:"queued_tasks"`
	ActiveWorkerTasks       int64  `json:"active_worker_tasks"`
	ActiveAsyncWork         int64  `json:"active_async_work"`
	TaskChannelLength       int    `json:"task_channel_length"`
	TaskChannelCapacity     int    `json:"task_channel_capacity"`
	GlobalActiveWorkerTasks int64  `json:"global_active_worker_tasks"`
	GlobalActiveAsyncWork   int64  `json:"global_active_async_work"`
	OperationVersion        int64  `json:"operation_version"`
	RecoveryInProgress      bool   `json:"recovery_in_progress"`
	LastRecoveryAttemptAt   string `json:"last_recovery_attempt_at,omitempty"`
}

type aiPipelineClusterRuntimeDiagnostic struct {
	Requested        bool  `json:"requested"`
	Running          bool  `json:"running"`
	Queued           bool  `json:"queued"`
	FusionRunning    bool  `json:"fusion_running"`
	EmbeddingRunning bool  `json:"embedding_running"`
	PipelineVersion  int64 `json:"pipeline_version"`
	QueuedVersion    int64 `json:"queued_version"`
	HasBatchContext  bool  `json:"has_batch_context"`
}

type aiPipelineClusterBarrierDiagnostic struct {
	HasPendingClusterWork bool `json:"has_pending_cluster_work"`
	Reached               bool `json:"reached"`
}

type aiPipelineStaleDiagnostic struct {
	Snapshot          string `json:"snapshot,omitempty"`
	StoredSnapshot    string `json:"stored_snapshot,omitempty"`
	LastProgressAt    string `json:"last_progress_at,omitempty"`
	StalledForSeconds int64  `json:"stalled_for_seconds,omitempty"`
	IsStale           bool   `json:"is_stale"`
	FreezeSuspended   bool   `json:"freeze_suspended"`
}

type aiPipelineRecentFailureDiagnostic struct {
	Stage        string `json:"stage,omitempty"`
	Message      string `json:"message,omitempty"`
	ArticleID    int64  `json:"article_id,omitempty"`
	ArticleTitle string `json:"article_title,omitempty"`
	Model        string `json:"model,omitempty"`
	Endpoint     string `json:"endpoint,omitempty"`
	OccurredAt   string `json:"occurred_at,omitempty"`
	Count        int    `json:"count,omitempty"`
}

func (m *AIEnhancedManager) startAIPipelineDiagnosticMonitor() {
	m.startAIPipelineDiagnosticMonitorWithInterval(aiPipelineDiagnosticInterval)
}

func (m *AIEnhancedManager) startAIPipelineDiagnosticMonitorWithInterval(interval time.Duration) {
	if interval <= 0 {
		interval = aiPipelineDiagnosticInterval
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.logAIPipelineDiagnostics("periodic")
			case <-m.stopChan:
				return
			}
		}
	}()
}

func (m *AIEnhancedManager) logAIPipelineDiagnostics(reason string) {
	if m == nil || m.db == nil {
		return
	}

	userIDs, err := m.db.ListKnownUserIDs()
	if err != nil {
		log.Printf("AI_PIPELINE_DIAG list users error: %v", err)
		return
	}

	for _, userID := range userIDs {
		snapshot := m.buildAIPipelineDiagnosticSnapshot(userID, reason)
		if !snapshot.LifecycleActive {
			continue
		}

		payload, err := json.Marshal(snapshot)
		if err != nil {
			log.Printf("AI_PIPELINE_DIAG user_id=%d lifecycle_active=%t marshal_error=%v", userID, snapshot.LifecycleActive, err)
			continue
		}
		log.Printf("AI_PIPELINE_DIAG user_id=%d lifecycle_active=%t snapshot=%s", userID, snapshot.LifecycleActive, string(payload))
	}
}

func (m *AIEnhancedManager) buildAIPipelineDiagnosticSnapshot(userID int64, reason string) aiPipelineDiagnosticSnapshot {
	snapshot := aiPipelineDiagnosticSnapshot{
		UserID:      userID,
		CollectedAt: time.Now().Format(time.RFC3339),
		Reason:      reason,
	}
	if m == nil || m.db == nil || userID <= 0 {
		snapshot.Errors = append(snapshot.Errors, "invalid manager/db/user")
		return snapshot
	}

	snapshot.IsEnabled = ShouldProcess(m.db, userID)
	if targetLang, err := m.db.GetSettingWithFallback(userID, "target_language"); err != nil {
		snapshot.Errors = append(snapshot.Errors, "target_language: "+err.Error())
	} else {
		snapshot.TargetLanguage = targetLang
	}
	if snapshot.TargetLanguage == "" {
		snapshot.TargetLanguage = "zh"
	}

	if interestVecBlob, err := m.db.GetUserInterestVector(userID); err != nil {
		snapshot.Errors = append(snapshot.Errors, "interest_vector: "+err.Error())
	} else if len(interestVecBlob) > 0 {
		snapshot.HasInterestVector = true
	}

	snapshot.IsRenormalizationRunning = m.isRenormalizationRunning(userID)
	if snapshot.IsRenormalizationRunning {
		if progress, err := m.db.GetAIReclusterNormalizationProgress(userID, snapshot.TargetLanguage); err != nil {
			snapshot.Errors = append(snapshot.Errors, "article_progress: "+err.Error())
		} else {
			snapshot.ArticleProgress = progress
		}
	} else if progress, err := m.db.GetAIProcessingProgress(userID, snapshot.TargetLanguage); err != nil {
		snapshot.Errors = append(snapshot.Errors, "article_progress: "+err.Error())
	} else {
		snapshot.ArticleProgress = progress
	}

	if progress, err := m.db.GetClusterProcessingProgress(userID); err != nil {
		snapshot.Errors = append(snapshot.Errors, "cluster_progress: "+err.Error())
	} else {
		snapshot.ClusterProgress = progress
	}

	snapshot.PendingRecommendationDays = m.getPendingRecommendationDays(userID)
	snapshot.Runtime = m.collectAIPipelineRuntimeDiagnostic(userID)
	snapshot.ClusterRuntime = m.collectAIPipelineClusterRuntimeDiagnostic(userID)
	snapshot.Recommendation = m.GetDailyRecommendationTaskStatus(userID)

	if health, err := m.getEmbeddingHealthStatus(userID, false); err != nil {
		snapshot.Errors = append(snapshot.Errors, "embedding_health: "+err.Error())
	} else {
		snapshot.EmbeddingHealth = health
	}

	statusForSnapshot := AIProcessingStatus{
		PendingArticles:            snapshot.ArticleProgress.PendingArticles,
		CompletedArticles:          snapshot.ArticleProgress.CompletedArticles,
		PendingSummaryArticles:     snapshot.ArticleProgress.PendingSummaryArticles,
		PendingTranslationArticles: snapshot.ArticleProgress.PendingTranslationArticles,
		PendingEmbeddingArticles:   snapshot.ArticleProgress.PendingEmbeddingArticles,
		PendingClusteringArticles:  snapshot.ArticleProgress.PendingClusteringArticles,
		PendingRecommendationDays:  snapshot.PendingRecommendationDays,
		PendingMergeClusters:       snapshot.ClusterProgress.PendingMergeClusters,
		PendingEmbedClusters:       snapshot.ClusterProgress.PendingEmbedClusters,
	}
	currentSnapshot := m.buildProcessingSnapshot(statusForSnapshot)
	snapshot.Stale = m.readAIPipelineStaleDiagnostic(userID, currentSnapshot, &snapshot.Errors)

	snapshot.RecentFailure = m.collectAIPipelineRecentFailureDiagnostic(userID)
	snapshot.ClusterBarrier = m.collectAIPipelineClusterBarrierDiagnostic(snapshot)

	if counts, err := m.db.GetAIArticleStageSkipCounts(userID); err != nil {
		snapshot.Errors = append(snapshot.Errors, "stage_skip_counts: "+err.Error())
	} else {
		snapshot.StageSkipCounts = counts
	}
	if failures, err := m.db.GetAIArticleStageTimeoutFailureSummaries(userID); err != nil {
		snapshot.Errors = append(snapshot.Errors, "stage_timeout_failures: "+err.Error())
	} else {
		snapshot.StageTimeoutFailures = failures
	}
	if counts, err := m.db.GetClusterStatusCounts(userID); err != nil {
		snapshot.Errors = append(snapshot.Errors, "cluster_status_counts: "+err.Error())
	} else {
		snapshot.ClusterStatusCounts = counts
	}
	if samples, err := m.db.GetAIPipelineBlockingArticleSamples(userID, snapshot.TargetLanguage, aiPipelineDiagnosticBlockingSampleSize); err != nil {
		snapshot.Errors = append(snapshot.Errors, "blocking_article_samples: "+err.Error())
	} else {
		snapshot.BlockingArticleSamples = sanitizeBlockingArticleSamples(samples)
	}

	snapshot.Blockers = m.detectAIPipelineBlockers(snapshot)
	snapshot.LifecycleActive = m.isAIPipelineLifecycleActive(snapshot)
	return snapshot
}

func (m *AIEnhancedManager) collectAIPipelineRuntimeDiagnostic(userID int64) aiPipelineRuntimeDiagnostic {
	queued, activeWorker, activeAsync := m.getUserTaskCounts(userID)

	m.taskQueueMu.Lock()
	taskChannelLength := len(m.taskChan)
	taskChannelCapacity := cap(m.taskChan)
	m.taskQueueMu.Unlock()

	diagnostic := aiPipelineRuntimeDiagnostic{
		QueuedTasks:             queued,
		ActiveWorkerTasks:       activeWorker,
		ActiveAsyncWork:         activeAsync,
		TaskChannelLength:       taskChannelLength,
		TaskChannelCapacity:     taskChannelCapacity,
		GlobalActiveWorkerTasks: atomic.LoadInt64(&m.activeWorkerTasks),
		GlobalActiveAsyncWork:   atomic.LoadInt64(&m.activeAsyncWork),
		OperationVersion:        m.currentUserOperationVersion(userID),
	}

	m.statusMu.Lock()
	diagnostic.RecoveryInProgress = m.recoveryInProgress[userID]
	if lastAttempt := m.lastRecoveryAttemptByUser[userID]; !lastAttempt.IsZero() {
		diagnostic.LastRecoveryAttemptAt = lastAttempt.Format(time.RFC3339)
	}
	m.statusMu.Unlock()

	return diagnostic
}

func (m *AIEnhancedManager) collectAIPipelineClusterRuntimeDiagnostic(userID int64) aiPipelineClusterRuntimeDiagnostic {
	m.clusterMu.Lock()
	defer m.clusterMu.Unlock()

	return aiPipelineClusterRuntimeDiagnostic{
		Requested:        m.clusterPipelineRequested[userID],
		Running:          m.clusterPipelineRunning[userID],
		Queued:           m.clusterPipelineQueued[userID],
		FusionRunning:    m.clusterFusionRunning[userID],
		EmbeddingRunning: m.clusterEmbeddingRunning[userID],
		PipelineVersion:  m.clusterPipelineVersion[userID],
		QueuedVersion:    m.clusterPipelineQueuedVer[userID],
		HasBatchContext:  m.clusterBatchContext[userID] != nil,
	}
}

func (m *AIEnhancedManager) collectAIPipelineRecentFailureDiagnostic(userID int64) aiPipelineRecentFailureDiagnostic {
	failure := m.getRecentFailure(userID)
	if failure.OccurredAt.IsZero() {
		return aiPipelineRecentFailureDiagnostic{}
	}

	return aiPipelineRecentFailureDiagnostic{
		Stage:        failure.Stage,
		Message:      failure.Message,
		ArticleID:    failure.ArticleID,
		ArticleTitle: sanitizeDiagnosticText(failure.ArticleTitle, 160),
		Model:        failure.Model,
		Endpoint:     sanitizeDiagnosticEndpoint(failure.Endpoint),
		OccurredAt:   failure.OccurredAt.Format(time.RFC3339),
		Count:        failure.Count,
	}
}

func (m *AIEnhancedManager) collectAIPipelineClusterBarrierDiagnostic(snapshot aiPipelineDiagnosticSnapshot) aiPipelineClusterBarrierDiagnostic {
	hasPendingClusterWork := snapshot.ClusterProgress.PendingMergeClusters > 0 || snapshot.ClusterProgress.PendingEmbedClusters > 0
	return aiPipelineClusterBarrierDiagnostic{
		HasPendingClusterWork: hasPendingClusterWork,
		Reached: hasPendingClusterWork &&
			snapshot.Runtime.QueuedTasks == 0 &&
			snapshot.Runtime.ActiveWorkerTasks == 0 &&
			snapshot.Runtime.ActiveAsyncWork == 0 &&
			snapshot.ArticleProgress.PendingSummaryArticles == 0 &&
			snapshot.ArticleProgress.PendingTranslationArticles == 0 &&
			snapshot.ArticleProgress.PendingEmbeddingArticles == 0 &&
			snapshot.ArticleProgress.PendingClusteringArticles == 0,
	}
}

func (m *AIEnhancedManager) readAIPipelineStaleDiagnostic(
	userID int64,
	currentSnapshot string,
	errors *[]string,
) aiPipelineStaleDiagnostic {
	diagnostic := aiPipelineStaleDiagnostic{Snapshot: currentSnapshot}

	freezeSuspended, err := m.db.GetSettingForUser(userID, aiProcessingFreezeSuspendedSettingKey)
	if err != nil {
		*errors = append(*errors, "freeze_suspended: "+err.Error())
	}
	diagnostic.FreezeSuspended = freezeSuspended == "true"

	storedSnapshot, err := m.db.GetSettingForUser(userID, aiProcessingSnapshotSettingKey)
	if err != nil {
		*errors = append(*errors, "stored_snapshot: "+err.Error())
	}
	diagnostic.StoredSnapshot = storedSnapshot

	lastProgressAtStr, err := m.db.GetSettingForUser(userID, aiProcessingLastProgressAtSettingKey)
	if err != nil {
		*errors = append(*errors, "last_progress_at: "+err.Error())
	}
	if lastProgressAtStr == "" {
		diagnostic.IsStale = diagnostic.FreezeSuspended
		return diagnostic
	}

	lastProgressAt, err := time.Parse(time.RFC3339Nano, lastProgressAtStr)
	if err != nil {
		*errors = append(*errors, "parse_last_progress_at: "+err.Error())
		diagnostic.IsStale = diagnostic.FreezeSuspended
		return diagnostic
	}

	diagnostic.LastProgressAt = lastProgressAt.Format(time.RFC3339)
	stalledFor := time.Since(lastProgressAt)
	if stalledFor < 0 {
		stalledFor = 0
	}
	diagnostic.StalledForSeconds = int64(stalledFor.Seconds())
	diagnostic.IsStale = diagnostic.FreezeSuspended ||
		(storedSnapshot == currentSnapshot && stalledFor >= aiProcessingStaleTimeout)
	return diagnostic
}

func (m *AIEnhancedManager) isAIPipelineLifecycleActive(snapshot aiPipelineDiagnosticSnapshot) bool {
	if !snapshot.IsEnabled {
		return false
	}

	return snapshot.IsRenormalizationRunning ||
		snapshot.ArticleProgress.PendingArticles > 0 ||
		snapshot.ClusterProgress.PendingMergeClusters > 0 ||
		snapshot.ClusterProgress.PendingEmbedClusters > 0 ||
		snapshot.PendingRecommendationDays > 0 ||
		snapshot.Runtime.QueuedTasks > 0 ||
		snapshot.Runtime.ActiveWorkerTasks > 0 ||
		snapshot.Runtime.ActiveAsyncWork > 0 ||
		snapshot.ClusterRuntime.Requested ||
		snapshot.ClusterRuntime.Running ||
		snapshot.ClusterRuntime.Queued ||
		snapshot.ClusterRuntime.FusionRunning ||
		snapshot.ClusterRuntime.EmbeddingRunning ||
		snapshot.Recommendation.HasTask ||
		snapshot.Recommendation.IsQueued ||
		snapshot.Recommendation.IsRunning ||
		snapshot.Recommendation.IsWaitingForIdle
}

func (m *AIEnhancedManager) detectAIPipelineBlockers(snapshot aiPipelineDiagnosticSnapshot) []string {
	blockers := make([]string, 0)

	if snapshot.Stale.IsStale {
		blockers = append(blockers, "stale_progress")
	}
	if !snapshot.EmbeddingHealth.IsHealthy && snapshot.EmbeddingHealth.SampleSize > 0 {
		blockers = append(blockers, "embedding_health_blocked")
	}
	if snapshot.ClusterBarrier.HasPendingClusterWork && !snapshot.ClusterBarrier.Reached {
		blockers = append(blockers, "cluster_barrier_waiting_for_article_work")
	}
	if snapshot.Recommendation.IsWaitingForIdle {
		blockers = append(blockers, "recommendation_waiting_for_idle")
	}
	if snapshot.ArticleProgress.PendingArticles > 0 &&
		snapshot.Runtime.QueuedTasks == 0 &&
		snapshot.Runtime.ActiveWorkerTasks == 0 &&
		snapshot.Runtime.ActiveAsyncWork == 0 &&
		!snapshot.ClusterRuntime.Requested &&
		!snapshot.ClusterRuntime.Running &&
		!snapshot.ClusterRuntime.Queued {
		blockers = append(blockers, "pending_article_work_without_runtime_activity")
	}
	if snapshot.Runtime.QueuedTasks > 0 && snapshot.Runtime.ActiveWorkerTasks == 0 && snapshot.Runtime.GlobalActiveWorkerTasks == 0 {
		blockers = append(blockers, "queued_tasks_without_active_worker")
	}
	if snapshot.ClusterProgress.PendingMergeClusters > 0 && !snapshot.ClusterRuntime.Running && !snapshot.ClusterRuntime.Requested {
		blockers = append(blockers, "pending_merge_without_cluster_runtime")
	}
	if snapshot.ClusterProgress.PendingEmbedClusters > 0 && !snapshot.ClusterRuntime.Running && !snapshot.ClusterRuntime.Requested {
		blockers = append(blockers, "pending_embed_without_cluster_runtime")
	}

	return blockers
}

func sanitizeBlockingArticleSamples(samples []sqlite.AIPipelineBlockingArticleSample) []sqlite.AIPipelineBlockingArticleSample {
	for i := range samples {
		samples[i].Title = sanitizeDiagnosticText(samples[i].Title, 160)
	}
	return samples
}

func sanitizeDiagnosticText(value string, maxLen int) string {
	value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " "))
	if maxLen > 0 && len(value) > maxLen {
		value = value[:maxLen]
	}
	return value
}

func sanitizeDiagnosticEndpoint(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if idx := strings.Index(raw, "?"); idx >= 0 {
			return raw[:idx] + "?<redacted>"
		}
		return raw
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
