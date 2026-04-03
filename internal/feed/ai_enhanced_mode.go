package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"MavenRSS/internal/ai"
	"MavenRSS/internal/dedup"
	"MavenRSS/internal/interest"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
	"MavenRSS/internal/summary"
	"MavenRSS/internal/translation"
	"MavenRSS/internal/utils/httputil"

	md "github.com/JohannesKaufmann/html-to-markdown"
)

// AIEnhancedTask represents a task for AI enhanced mode processing
type AIEnhancedTask struct {
	ArticleID                 int64
	UserID                    int64
	FeedID                    int64
	ArticleTitle              string
	OperationVersion          int64
	NeedsSummary              bool
	NeedsTranslation          bool
	TranslateArticles         bool
	NeedsEmbedding            bool
	NeedsDedup                bool
	NeedsClusterRun           bool
	ForceTitleSummaryFallback bool
}

type AIProcessingFailure struct {
	Stage        string
	Message      string
	ArticleID    int64
	ArticleTitle string
	Model        string
	Endpoint     string
	OccurredAt   time.Time
	Count        int
}

// AIEnhancedManager manages the AI enhanced mode task queue and workers
type AIEnhancedManager struct {
	db                         *sqlite.DB
	taskChan                   chan *AIEnhancedTask
	workerWg                   sync.WaitGroup
	workerCount                int
	activeWorkerTasks          int64
	statusMu                   sync.Mutex
	queuedTasksByUser          map[int64]int
	activeWorkerTasksByUser    map[int64]int64
	activeAsyncWorkByUser      map[int64]int64
	userOperationVersion       map[int64]int64
	embeddingHealthByUser      map[int64]EmbeddingHealthStatus
	embeddingHealthCheckedAt   map[int64]time.Time
	recentFailureByUser        map[int64]AIProcessingFailure
	recoveryInProgress         map[int64]bool
	lastRecoveryAttemptByUser  map[int64]time.Time
	renormalizationRunning     map[int64]bool
	taskQueueMu                sync.Mutex
	clusterMu                  sync.Mutex
	clusterPipelineRunning     map[int64]bool
	clusterPipelineQueued      map[int64]bool
	clusterPipelineVersion     map[int64]int64
	clusterPipelineQueuedVer   map[int64]int64
	clusterFusionRunning       map[int64]bool
	clusterEmbeddingRunning    map[int64]bool
	recommendationMu           sync.Mutex
	recommendationRunning      map[int64]bool
	recommendationRunningVer   map[int64]int64
	recommendationStatusByUser map[int64]DailyRecommendationTaskStatus
	pendingRecommendationDate  map[int64]string
	pendingRecommendationWait  map[int64]bool
	pendingRecommendationForce map[int64]bool
	pendingRecommendationMode  map[int64]string
	pendingRecommendationVer   map[int64]int64
	activeAsyncWork            int64
	stopChan                   chan struct{}
	resolveFusionConfig        func(userID int64) (*dedup.FusionConfig, error)
	runFusion                  func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error
	runEmbedding               func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error
}

const (
	aiProcessingSnapshotSettingKey        = "_internal_ai_processing_snapshot"
	aiProcessingLastProgressAtSettingKey  = "_internal_ai_processing_last_progress_at"
	aiProcessingFreezeSuspendedSettingKey = "_internal_ai_processing_freeze_suspended"
	aiProcessingStaleTimeout              = 30 * time.Minute
)

type AIProcessingStatus struct {
	IsEnabled                        bool    `json:"is_enabled"`
	HasInterestVector                bool    `json:"has_interest_vector"`
	IsRenormalizationRunning         bool    `json:"is_renormalization_running"`
	IsConfigFrozen                   bool    `json:"is_config_frozen"`
	IsStale                          bool    `json:"is_stale"`
	IsFreezeSuspended                bool    `json:"is_freeze_suspended"`
	EligibleArticles                 int     `json:"eligible_articles"`
	PendingArticles                  int     `json:"pending_articles"`
	CompletedArticles                int     `json:"completed_articles"`
	PendingSummaryArticles           int     `json:"pending_summary_articles"`
	PendingTranslationArticles       int     `json:"pending_translation_articles"`
	PendingEmbeddingArticles         int     `json:"pending_embedding_articles"`
	PendingClusteringArticles        int     `json:"pending_clustering_articles"`
	PendingRecommendationDays        int     `json:"pending_recommendation_days"`
	ProgressPercent                  float64 `json:"progress_percent"`
	QueuedTasks                      int     `json:"queued_tasks"`
	ActiveWorkerTasks                int64   `json:"active_worker_tasks"`
	ActiveAsyncWork                  int64   `json:"active_async_work"`
	IsClusterPipelineBusy            bool    `json:"is_cluster_pipeline_busy"`
	IsClusterFusionRunning           bool    `json:"is_cluster_fusion_running"`
	IsClusterEmbeddingRunning        bool    `json:"is_cluster_embedding_running"`
	PendingMergeClusters             int     `json:"pending_merge_clusters"`
	PendingEmbedClusters             int     `json:"pending_embed_clusters"`
	ClusterPhase                     string  `json:"cluster_phase,omitempty"`
	RenormalizationTotalArticles     int     `json:"renormalization_total_articles,omitempty"`
	RenormalizationPendingArticles   int     `json:"renormalization_pending_articles,omitempty"`
	RenormalizationCompletedArticles int     `json:"renormalization_completed_articles,omitempty"`
	LastProgressAt                   string  `json:"last_progress_at,omitempty"`
	StalledForSeconds                int64   `json:"stalled_for_seconds,omitempty"`
	RecentFailureStage               string  `json:"recent_failure_stage,omitempty"`
	RecentFailureMessage             string  `json:"recent_failure_message,omitempty"`
	RecentFailureArticleID           int64   `json:"recent_failure_article_id,omitempty"`
	RecentFailureArticleTitle        string  `json:"recent_failure_article_title,omitempty"`
	RecentFailureModel               string  `json:"recent_failure_model,omitempty"`
	RecentFailureEndpoint            string  `json:"recent_failure_endpoint,omitempty"`
	RecentFailureAt                  string  `json:"recent_failure_at,omitempty"`
	RecentFailureCount               int     `json:"recent_failure_count,omitempty"`
	EmbeddingHealthBlocked           bool    `json:"embedding_health_blocked"`
	EmbeddingHealthSampleSize        int     `json:"embedding_health_sample_size"`
	EmbeddingHealthUnnormalizedCount int     `json:"embedding_health_unnormalized_count"`
	EmbeddingHealthUnnormalizedRatio float64 `json:"embedding_health_unnormalized_ratio"`
}

// NewAIEnhancedManager creates a new AI enhanced mode manager
func NewAIEnhancedManager(db *sqlite.DB) *AIEnhancedManager {
	// Calculate optimal worker count
	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}
	if numWorkers > 2 {
		numWorkers = 2 // Cap at 2 workers for AI tasks to avoid overload
	}

	taskChan := make(chan *AIEnhancedTask, 100)

	manager := &AIEnhancedManager{
		db:                         db,
		taskChan:                   taskChan,
		workerCount:                numWorkers,
		queuedTasksByUser:          make(map[int64]int),
		activeWorkerTasksByUser:    make(map[int64]int64),
		activeAsyncWorkByUser:      make(map[int64]int64),
		userOperationVersion:       make(map[int64]int64),
		embeddingHealthByUser:      make(map[int64]EmbeddingHealthStatus),
		embeddingHealthCheckedAt:   make(map[int64]time.Time),
		recentFailureByUser:        make(map[int64]AIProcessingFailure),
		recoveryInProgress:         make(map[int64]bool),
		lastRecoveryAttemptByUser:  make(map[int64]time.Time),
		renormalizationRunning:     make(map[int64]bool),
		clusterPipelineRunning:     make(map[int64]bool),
		clusterPipelineQueued:      make(map[int64]bool),
		clusterPipelineVersion:     make(map[int64]int64),
		clusterPipelineQueuedVer:   make(map[int64]int64),
		clusterFusionRunning:       make(map[int64]bool),
		clusterEmbeddingRunning:    make(map[int64]bool),
		recommendationRunning:      make(map[int64]bool),
		recommendationRunningVer:   make(map[int64]int64),
		recommendationStatusByUser: make(map[int64]DailyRecommendationTaskStatus),
		pendingRecommendationDate:  make(map[int64]string),
		pendingRecommendationWait:  make(map[int64]bool),
		pendingRecommendationForce: make(map[int64]bool),
		pendingRecommendationMode:  make(map[int64]string),
		pendingRecommendationVer:   make(map[int64]int64),
		stopChan:                   make(chan struct{}),
	}
	manager.resolveFusionConfig = manager.buildFusionConfig
	manager.runFusion = dedup.RunFusion
	manager.runEmbedding = dedup.RunEmbedding

	log.Printf("Starting %d AI enhanced mode workers", numWorkers)
	manager.startWorkers()
	manager.startDailyRecommendationScheduler()
	manager.startPendingWorkRecoveryMonitor()

	return manager
}

// startWorkers starts the AI enhanced mode worker pool
func (m *AIEnhancedManager) startWorkers() {
	for i := 0; i < m.workerCount; i++ {
		m.workerWg.Add(1)
		go m.worker(i)
	}
}

// worker processes AI enhanced tasks
func (m *AIEnhancedManager) worker(workerID int) {
	defer m.workerWg.Done()
	log.Printf("AI enhanced mode worker %d started", workerID)

	for task := range m.taskChan {
		m.processTask(task, workerID)
	}

	log.Printf("AI enhanced mode worker %d stopped (channel closed)", workerID)
}

// processTask processes a single AI enhanced task
func (m *AIEnhancedManager) processTask(task *AIEnhancedTask, workerID int) {
	m.decrementQueuedTask(task.UserID)
	atomic.AddInt64(&m.activeWorkerTasks, 1)
	m.incrementActiveWorkerTask(task.UserID)
	defer atomic.AddInt64(&m.activeWorkerTasks, -1)
	defer m.decrementActiveWorkerTask(task.UserID)

	if !m.isTaskOperationCurrent(task) {
		log.Printf("Skipping stale AI enhanced task for article %d user %d", task.ArticleID, task.UserID)
		return
	}

	log.Printf("AI enhanced worker %d processing article %d for user %d", workerID, task.ArticleID, task.UserID)

	needsContent := task.NeedsSummary || (task.NeedsTranslation && task.TranslateArticles) || task.NeedsEmbedding
	content := ""
	if needsContent {
		var hasContent bool
		var err error
		content, hasContent, err = m.db.GetArticleContent(task.ArticleID)
		if err != nil {
			log.Printf("Error getting article content for AI enhanced task: %v", err)
			return
		}
		if !hasContent || content == "" {
			content, err = m.ensureArticleContentFallbackFromTitle(task)
			if err != nil {
				log.Printf("No content available for article %d and failed to build title fallback: %v", task.ArticleID, err)
				if task.NeedsClusterRun && !task.NeedsSummary && !task.NeedsEmbedding && !task.NeedsDedup {
					m.scheduleClusterPipeline(task.UserID)
				}
				return
			}
		}
	}

	if !m.isTaskOperationCurrent(task) {
		log.Printf("Stopping stale AI enhanced task for article %d user %d after content load", task.ArticleID, task.UserID)
		return
	}

	if task.NeedsSummary {
		m.generateAISummary(task, content)
	}

	if task.NeedsTranslation && task.TranslateArticles {
		m.generateAITranslation(task, content)
	}

	if task.NeedsEmbedding || task.NeedsDedup || task.NeedsClusterRun {
		m.advanceArticlePipelineAsync(task, content)
	}
}

func (m *AIEnhancedManager) currentUserOperationVersion(userID int64) int64 {
	if userID <= 0 {
		return 0
	}

	m.statusMu.Lock()
	defer m.statusMu.Unlock()

	return m.userOperationVersion[userID]
}

func (m *AIEnhancedManager) nextUserOperationVersion(userID int64) int64 {
	if userID <= 0 {
		return 0
	}

	m.statusMu.Lock()
	defer m.statusMu.Unlock()

	if m.userOperationVersion == nil {
		m.userOperationVersion = make(map[int64]int64)
	}
	next := m.userOperationVersion[userID] + 1
	m.userOperationVersion[userID] = next
	return next
}

func (m *AIEnhancedManager) stampTaskOperationVersion(task *AIEnhancedTask) int64 {
	if task == nil || task.UserID <= 0 {
		return 0
	}
	if task.OperationVersion == 0 {
		task.OperationVersion = m.currentUserOperationVersion(task.UserID)
	}
	return task.OperationVersion
}

func (m *AIEnhancedManager) isUserOperationCurrent(userID, version int64) bool {
	if userID <= 0 {
		return false
	}
	if version == 0 {
		return true
	}
	return m.currentUserOperationVersion(userID) == version
}

func (m *AIEnhancedManager) isTaskOperationCurrent(task *AIEnhancedTask) bool {
	if task == nil {
		return false
	}
	version := task.OperationVersion
	if version == 0 {
		version = m.stampTaskOperationVersion(task)
	}
	return m.isUserOperationCurrent(task.UserID, version)
}

func (m *AIEnhancedManager) ensureArticleContentFallbackFromTitle(task *AIEnhancedTask) (string, error) {
	if task == nil || task.ArticleID <= 0 {
		return "", fmt.Errorf("invalid AI enhanced task")
	}

	title := strings.TrimSpace(task.ArticleTitle)
	if title == "" {
		article, err := m.db.GetArticleByID(task.ArticleID)
		if err != nil {
			return "", fmt.Errorf("load article title: %w", err)
		}
		if article != nil {
			title = strings.TrimSpace(article.Title)
		}
	}
	if title == "" {
		return "", fmt.Errorf("article title is empty")
	}

	fallbackContent := "<p>" + html.EscapeString(title) + "</p>"
	if err := m.db.SetArticleContent(task.ArticleID, fallbackContent); err != nil {
		return "", fmt.Errorf("cache title fallback content: %w", err)
	}

	if task.NeedsSummary {
		if err := m.db.UpdateArticleSummary(task.ArticleID, title); err != nil {
			log.Printf("Failed to cache title fallback summary for article %d: %v", task.ArticleID, err)
		} else {
			task.NeedsSummary = false
			log.Printf("Using article title as fallback summary for article %d", task.ArticleID)
		}
	}

	log.Printf("Using article title as fallback content for article %d", task.ArticleID)
	return fallbackContent, nil
}

func (m *AIEnhancedManager) advanceArticlePipelineAsync(task *AIEnhancedTask, content string) {
	atomic.AddInt64(&m.activeAsyncWork, 1)
	m.incrementActiveAsyncWork(task.UserID)
	version := m.stampTaskOperationVersion(task)
	go func() {
		defer func() {
			m.decrementActiveAsyncWork(task.UserID)
			if atomic.AddInt64(&m.activeAsyncWork, -1) == 0 {
				m.onAsyncWorkDrained()
			}
		}()
		time.Sleep(2 * time.Second)
		if !m.isUserOperationCurrent(task.UserID, version) {
			log.Printf("Skipping stale async article pipeline for article %d user %d", task.ArticleID, task.UserID)
			return
		}

		article, err := m.db.GetArticleByID(task.ArticleID)
		if err != nil || article == nil {
			log.Printf("Failed to fetch article %d for AI enhanced pipeline: %v", task.ArticleID, err)
			return
		}
		if !m.isUserOperationCurrent(task.UserID, version) {
			log.Printf("Stopping stale async article pipeline for article %d user %d", task.ArticleID, task.UserID)
			return
		}

		if task.NeedsEmbedding {
			summaryText := article.Summary
			if summaryText == "" {
				summaryText = content
			}

			configsJSON, err := m.db.GetSettingWithFallback(task.UserID, "ai_embedding_models")
			if err != nil || configsJSON == "" || configsJSON == "[]" {
				log.Printf("No embedding models configured for user %d, skipping embedding", task.UserID)
			} else {
				globalProxyURL, _ := buildGlobalProxyURL(m.db, task.UserID)
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				var titleEmbBlob, summaryEmbBlob []byte

				if article.Title != "" {
					if titleEmb, err := ai.GenerateEmbeddings(ctx, article.Title, configsJSON, globalProxyURL); err == nil {
						if len(titleEmb) > 0 {
							titleEmbBlob, _ = interest.NormalizeAndSerialize(titleEmb)
						}
					} else {
						log.Printf("Failed to generate title embedding for article %d: %v", task.ArticleID, err)
					}
				}

				if summaryText != "" {
					if sumEmb, err := ai.GenerateEmbeddings(ctx, summaryText, configsJSON, globalProxyURL); err == nil {
						if len(sumEmb) > 0 {
							summaryEmbBlob, _ = interest.NormalizeAndSerialize(sumEmb)
						}
					} else {
						log.Printf("Failed to generate summary embedding for article %d: %v", task.ArticleID, err)
					}
				}

				if len(titleEmbBlob) > 0 || len(summaryEmbBlob) > 0 {
					if !m.isUserOperationCurrent(task.UserID, version) {
						log.Printf("Discarding stale embeddings for article %d user %d", task.ArticleID, task.UserID)
						return
					}
					if err := m.db.UpdateArticleEmbeddings(task.ArticleID, titleEmbBlob, summaryEmbBlob); err != nil {
						log.Printf("Failed to update article %d embeddings: %v", task.ArticleID, err)
					} else {
						log.Printf("Successfully saved embeddings for article %d", task.ArticleID)
					}
				}
			}
		}

		clusterScheduled := false
		if task.NeedsDedup {
			if !m.isUserOperationCurrent(task.UserID, version) {
				log.Printf("Skipping stale dedup for article %d user %d", task.ArticleID, task.UserID)
				return
			}
			if err := dedup.ProcessArticle(m.db, task.ArticleID, task.UserID); err != nil {
				log.Printf("Dedup pipeline failed for article %d: %v", task.ArticleID, err)
			} else {
				log.Printf("Dedup pipeline completed for article %d", task.ArticleID)
				clusterScheduled = true
			}
		}

		if task.NeedsClusterRun && !clusterScheduled {
			if m.isUserOperationCurrent(task.UserID, version) {
				m.scheduleClusterPipeline(task.UserID)
			}
			return
		}

		if clusterScheduled && m.isUserOperationCurrent(task.UserID, version) {
			m.scheduleClusterPipeline(task.UserID)
		}
	}()
}

func (m *AIEnhancedManager) scheduleClusterPipeline(userID int64) {
	m.scheduleClusterPipelineForVersion(userID, m.currentUserOperationVersion(userID))
}

func (m *AIEnhancedManager) scheduleClusterPipelineForVersion(userID, version int64) {
	if userID <= 0 {
		return
	}

	m.clusterMu.Lock()
	if m.clusterPipelineRunning == nil {
		m.clusterPipelineRunning = make(map[int64]bool)
	}
	if m.clusterPipelineQueued == nil {
		m.clusterPipelineQueued = make(map[int64]bool)
	}
	if m.clusterPipelineVersion == nil {
		m.clusterPipelineVersion = make(map[int64]int64)
	}
	if m.clusterPipelineQueuedVer == nil {
		m.clusterPipelineQueuedVer = make(map[int64]int64)
	}
	if m.clusterPipelineRunning[userID] {
		m.clusterPipelineQueued[userID] = true
		if version > m.clusterPipelineQueuedVer[userID] {
			m.clusterPipelineQueuedVer[userID] = version
		}
		m.clusterMu.Unlock()
		return
	}
	m.clusterPipelineRunning[userID] = true
	m.clusterPipelineVersion[userID] = version
	m.clusterMu.Unlock()

	go m.runClusterPipeline(userID, version)
}

func (m *AIEnhancedManager) runClusterPipeline(userID, version int64) {
	for {
		shouldRun := false
		if m.isUserOperationCurrent(userID, version) {
			var err error
			shouldRun, err = m.waitForClusterPipelineBarrier(userID)
			if err != nil {
				m.recordTaskFailure(userID, "clustering", nil, "", "", err)
				log.Printf("Cluster pipeline wait failed for user %d: %v", userID, err)
				shouldRun = false
			}
		} else {
			log.Printf("Stopping stale cluster pipeline for user %d", userID)
		}

		if shouldRun && m.isUserOperationCurrent(userID, version) {
			if err := m.runClusterPipelineOnce(userID); err != nil {
				m.recordTaskFailure(userID, "clustering", nil, "", "", err)
				log.Printf("Cluster pipeline failed for user %d: %v", userID, err)
			}
		}

		m.clusterMu.Lock()
		nextVersion := m.clusterPipelineQueuedVer[userID]
		if m.clusterPipelineQueued[userID] {
			if nextVersion < version {
				nextVersion = version
			}
			m.clusterPipelineQueued[userID] = false
			delete(m.clusterPipelineQueuedVer, userID)
			m.clusterPipelineVersion[userID] = nextVersion
			m.clusterMu.Unlock()
			version = nextVersion
			continue
		}
		delete(m.clusterPipelineRunning, userID)
		delete(m.clusterPipelineQueued, userID)
		delete(m.clusterPipelineVersion, userID)
		delete(m.clusterPipelineQueuedVer, userID)
		delete(m.clusterFusionRunning, userID)
		delete(m.clusterEmbeddingRunning, userID)
		m.clusterMu.Unlock()
		if m.db != nil {
			if clusterProgress, err := m.db.GetClusterProcessingProgress(userID); err == nil &&
				clusterProgress.PendingMergeClusters == 0 &&
				clusterProgress.PendingEmbedClusters == 0 {
				m.requestMissingRecommendationBackfill(userID)
			}
		}
		m.onAsyncWorkDrained()
		return
	}
}

func (m *AIEnhancedManager) waitForClusterPipelineBarrier(userID int64) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	if m.db == nil {
		return true, nil
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	startedAt := time.Now()

	for {
		hasWork, barrierReached, err := m.clusterPipelineBarrierState(userID)
		if err != nil {
			return false, err
		}
		if !hasWork {
			return false, nil
		}
		if barrierReached {
			return true, nil
		}
		if m.isRenormalizationRunning(userID) && time.Since(startedAt) >= renormalizationClusterWaitTimeout {
			log.Printf(
				"Cluster pipeline barrier timed out for renormalization user %d after %s; proceeding with remaining cluster work",
				userID,
				renormalizationClusterWaitTimeout,
			)
			m.abandonTimedOutRenormalizationActivity(userID, "cluster pipeline barrier timed out during renormalization")
			return true, nil
		}

		select {
		case <-m.stopChan:
			return false, fmt.Errorf("ai enhanced manager stopped")
		case <-ticker.C:
		}
	}
}

func (m *AIEnhancedManager) clusterPipelineBarrierState(userID int64) (bool, bool, error) {
	if userID <= 0 {
		return false, false, nil
	}
	if m.db == nil {
		return true, true, nil
	}

	clusterProgress, err := m.db.GetClusterProcessingProgress(userID)
	if err != nil {
		return false, false, fmt.Errorf("get cluster processing progress: %w", err)
	}

	hasPendingClusterWork := clusterProgress.PendingMergeClusters > 0 || clusterProgress.PendingEmbedClusters > 0
	if !hasPendingClusterWork {
		return false, false, nil
	}

	targetLang, _ := m.db.GetSettingWithFallback(userID, "target_language")
	if targetLang == "" {
		targetLang = "zh"
	}
	progress, err := m.db.GetAIProcessingProgress(userID, targetLang)
	if err != nil {
		return false, false, fmt.Errorf("get ai processing progress: %w", err)
	}

	queued, activeWorker, activeAsync := m.getUserTaskCounts(userID)
	barrierReached := queued == 0 &&
		activeWorker == 0 &&
		activeAsync == 0 &&
		progress.PendingSummaryArticles == 0 &&
		progress.PendingTranslationArticles == 0 &&
		progress.PendingEmbeddingArticles == 0 &&
		progress.PendingClusteringArticles == 0

	return true, barrierReached, nil
}

func (m *AIEnhancedManager) setClusterStageRunning(userID int64, stage string, running bool) {
	if userID <= 0 {
		return
	}

	m.clusterMu.Lock()
	defer m.clusterMu.Unlock()

	if m.clusterFusionRunning == nil {
		m.clusterFusionRunning = make(map[int64]bool)
	}
	if m.clusterEmbeddingRunning == nil {
		m.clusterEmbeddingRunning = make(map[int64]bool)
	}

	switch stage {
	case "fusion":
		if running {
			m.clusterFusionRunning[userID] = true
			return
		}
		delete(m.clusterFusionRunning, userID)
	case "embedding":
		if running {
			m.clusterEmbeddingRunning[userID] = true
			return
		}
		delete(m.clusterEmbeddingRunning, userID)
	}
}

func (m *AIEnhancedManager) getClusterStageState(userID int64) (bool, bool, bool, string) {
	if userID <= 0 {
		return false, false, false, ""
	}

	m.clusterMu.Lock()
	defer m.clusterMu.Unlock()

	pipelineRunning := m.clusterPipelineRunning[userID] || m.clusterPipelineQueued[userID]
	fusionRunning := m.clusterFusionRunning[userID]
	embeddingRunning := m.clusterEmbeddingRunning[userID]

	phase := ""
	switch {
	case fusionRunning:
		phase = "fusion"
	case embeddingRunning:
		phase = "embedding"
	case pipelineRunning:
		phase = "waiting"
	}

	return pipelineRunning, fusionRunning, embeddingRunning, phase
}

func (m *AIEnhancedManager) hasUserRecommendationBlockingWork(userID int64) bool {
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
	if clusterBusy {
		return true
	}

	return m.userHasPendingClusterWork(userID)
}

func (m *AIEnhancedManager) userHasPendingClusterWork(userID int64) bool {
	if userID <= 0 || m.db == nil {
		return false
	}

	clusterProgress, err := m.db.GetClusterProcessingProgress(userID)
	if err != nil {
		return false
	}
	return clusterProgress.PendingMergeClusters > 0 || clusterProgress.PendingEmbedClusters > 0
}

func (m *AIEnhancedManager) determineClusterPhase(status *AIProcessingStatus) {
	if status == nil {
		return
	}
	if status.IsClusterFusionRunning {
		status.ClusterPhase = "fusion"
		return
	}
	if status.IsClusterEmbeddingRunning {
		status.ClusterPhase = "embedding"
		return
	}
	if status.PendingMergeClusters > 0 {
		status.ClusterPhase = "pending_merge"
		return
	}
	if status.PendingEmbedClusters > 0 {
		status.ClusterPhase = "pending_embed"
		return
	}
	if status.IsClusterPipelineBusy {
		status.ClusterPhase = "waiting"
	}
}

func (m *AIEnhancedManager) runClusterPipelineOnce(userID int64) error {
	health, allowed, err := m.guardEmbeddingHealth(userID, blockedScopeClusterPipeline)
	if err != nil {
		return fmt.Errorf("embedding health gate: %w", err)
	}
	if !allowed {
		log.Printf(
			"Skipping cluster pipeline for user %d due to unhealthy summary embeddings (sample=%d, unnormalized=%d, ratio=%.2f)",
			userID,
			health.SampleSize,
			health.UnnormalizedCount,
			health.UnnormalizedRatio,
		)
		return nil
	}

	cfg, err := m.resolveFusionConfig(userID)
	if err != nil {
		return err
	}
	if cfg == nil || cfg.Summarizer == nil {
		log.Printf(
			"Cluster pipeline fusion summarizer unavailable for user %d; pending_merge clusters will use source-article fallback content until a fusion model is resolved",
			userID,
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	m.setClusterStageRunning(userID, "fusion", true)
	if err := m.runFusion(ctx, m.db, userID, cfg); err != nil {
		m.setClusterStageRunning(userID, "fusion", false)
		return fmt.Errorf("run fusion: %w", err)
	}
	m.setClusterStageRunning(userID, "fusion", false)

	m.setClusterStageRunning(userID, "embedding", true)
	if err := m.runEmbedding(ctx, m.db, userID, cfg); err != nil {
		m.setClusterStageRunning(userID, "embedding", false)
		return fmt.Errorf("run embedding: %w", err)
	}
	m.setClusterStageRunning(userID, "embedding", false)
	if m.db == nil {
		return nil
	}
	if err := m.initializeInterestVectorFromFavoriteClustersIfMissing(userID); err != nil {
		return fmt.Errorf("initialize interest vector from favorite clusters: %w", err)
	}

	return nil
}

func (m *AIEnhancedManager) initializeInterestVectorFromFavoriteClustersIfMissing(userID int64) error {
	if userID <= 0 {
		return nil
	}

	vecBlob, err := m.db.GetUserInterestVector(userID)
	if err != nil {
		return fmt.Errorf("get user interest vector: %w", err)
	}
	if len(vecBlob) > 0 {
		return nil
	}

	favoriteBlobs, err := m.db.GetFavoriteClusterSummaryEmbeddings(userID)
	if err != nil {
		return fmt.Errorf("get favorite cluster summary embeddings: %w", err)
	}
	if len(favoriteBlobs) == 0 {
		return nil
	}

	initVec, err := interest.InitFromFavorites(favoriteBlobs)
	if err != nil {
		return fmt.Errorf("build interest vector from favorite clusters: %w", err)
	}
	if len(initVec) == 0 {
		return nil
	}

	serialized, err := interest.SerializeVector(initVec)
	if err != nil {
		return fmt.Errorf("serialize interest vector: %w", err)
	}

	if err := m.db.UpdateUserInterestVector(userID, serialized); err != nil {
		return fmt.Errorf("persist interest vector: %w", err)
	}

	return nil
}

func (m *AIEnhancedManager) buildFusionConfig(userID int64) (*dedup.FusionConfig, error) {
	embeddingConfigsJSON, err := m.db.GetSettingWithFallback(userID, "ai_embedding_models")
	if err != nil {
		return nil, fmt.Errorf("load embedding models: %w", err)
	}

	globalProxyURL, err := buildGlobalProxyURL(m.db, userID)
	if err != nil {
		return nil, fmt.Errorf("load proxy settings: %w", err)
	}
	targetLanguage, _ := m.db.GetSettingWithFallback(userID, "target_language")
	if targetLanguage == "" {
		targetLanguage = "zh"
	}

	config, proxyFeature, err := m.resolveFusionAIClientConfig(userID)
	if err != nil {
		return nil, err
	}

	var aiSummarizer *summary.AISummarizer
	if hasConfiguredAPIKey(config) {
		useGlobalProxy := ai.NewProfileProvider(m.db).UseGlobalProxyForFeatureForUser(userID, proxyFeature)
		aiSummarizer = summary.NewAISummarizerWithDB(config.APIKey, config.Endpoint, config.Model, m.db, useGlobalProxy)
		if config.CustomHeaders != "" {
			aiSummarizer.SetCustomHeaders(config.CustomHeaders)
		}
		aiSummarizer.SetLanguage(targetLanguage)
	}

	log.Printf(
		"Built cluster fusion config for user %d: summarizer_enabled=%t proxy_feature=%q target_language=%q embedding_models_configured=%t",
		userID,
		aiSummarizer != nil,
		string(proxyFeature),
		targetLanguage,
		strings.TrimSpace(embeddingConfigsJSON) != "" && strings.TrimSpace(embeddingConfigsJSON) != "[]",
	)

	return &dedup.FusionConfig{
		Summarizer:     aiSummarizer,
		EmbConfigsJSON: embeddingConfigsJSON,
		GlobalProxyURL: globalProxyURL,
		TargetLanguage: targetLanguage,
	}, nil
}

func (m *AIEnhancedManager) resolveFusionAIClientConfig(userID int64) (*ai.ClientConfig, ai.FeatureType, error) {
	provider := ai.NewProfileProvider(m.db)
	selectedFusionProfileID, _ := m.db.GetSettingForUser(userID, "ai_fusion_profile_id")
	selectedSummaryProfileID, _ := m.db.GetSettingForUser(userID, "ai_summary_profile_id")

	fusionProfile, profileErr := provider.GetProfileForFeatureForUser(userID, ai.FeatureFusion)
	if profileErr != nil {
		log.Printf("Failed to inspect fusion profile resolution for user %d: %v", userID, profileErr)
	}
	config, err := getUserFeatureAIConfig(m.db, userID, ai.FeatureFusion)
	if err != nil {
		return nil, ai.FeatureFusion, fmt.Errorf("resolve fusion ai config: %w", err)
	}
	if hasConfiguredAPIKey(config) {
		source := "legacy_user_ai_settings"
		if fusionProfile != nil {
			source = "fusion_profile_or_default"
		}
		m.logFusionConfigResolution(userID, source, selectedFusionProfileID, selectedSummaryProfileID, fusionProfile, config)
		return config, ai.FeatureFusion, nil
	}

	// Backward compatibility: older versions reused the summary model for cluster fusion.
	summaryProfile, profileErr := provider.GetProfileForFeatureForUser(userID, ai.FeatureSummary)
	if profileErr != nil {
		log.Printf("Failed to inspect summary profile fallback for user %d: %v", userID, profileErr)
	}
	config, err = getUserFeatureAIConfig(m.db, userID, ai.FeatureSummary)
	if err != nil {
		return nil, ai.FeatureSummary, fmt.Errorf("resolve summary ai config fallback for fusion: %w", err)
	}
	if hasConfiguredAPIKey(config) {
		source := "legacy_user_ai_settings_via_summary_fallback"
		if summaryProfile != nil {
			source = "summary_profile_fallback"
		}
		m.logFusionConfigResolution(userID, source, selectedFusionProfileID, selectedSummaryProfileID, summaryProfile, config)
		return config, ai.FeatureSummary, nil
	}

	m.logFusionConfigResolution(userID, "missing", selectedFusionProfileID, selectedSummaryProfileID, nil, config)
	return config, ai.FeatureFusion, nil
}

func (m *AIEnhancedManager) logFusionConfigResolution(
	userID int64,
	source string,
	selectedFusionProfileID string,
	selectedSummaryProfileID string,
	resolvedProfile *models.AIProfile,
	config *ai.ClientConfig,
) {
	if m == nil || m.db == nil || userID <= 0 {
		return
	}

	defaultProfile, defaultErr := m.db.GetDefaultAIProfileForUser(userID)
	if defaultErr != nil {
		log.Printf("Failed to inspect default AI profile during fusion config logging for user %d: %v", userID, defaultErr)
	}
	legacyEndpoint, _ := m.db.GetSettingForUser(userID, "ai_endpoint")
	legacyModel, _ := m.db.GetSettingForUser(userID, "ai_model")

	log.Printf(
		"AI fusion config resolution for user %d: source=%s selected_fusion_profile_setting=%q selected_summary_profile_setting=%q resolved_profile=%s default_profile=%s resolved_model=%q resolved_endpoint=%q has_api_key=%t legacy_model=%q legacy_endpoint=%q",
		userID,
		source,
		strings.TrimSpace(selectedFusionProfileID),
		strings.TrimSpace(selectedSummaryProfileID),
		formatFusionProfileLogValue(resolvedProfile),
		formatFusionProfileLogValue(defaultProfile),
		strings.TrimSpace(fusionConfigValue(config, "model")),
		strings.TrimSpace(fusionConfigValue(config, "endpoint")),
		hasConfiguredAPIKey(config),
		strings.TrimSpace(legacyModel),
		strings.TrimSpace(legacyEndpoint),
	)
}

func formatFusionProfileLogValue(profile *models.AIProfile) string {
	if profile == nil {
		return "none"
	}

	name := strings.TrimSpace(profile.Name)
	if name == "" {
		return fmt.Sprintf("%d", profile.ID)
	}
	return fmt.Sprintf("%d(%s)", profile.ID, name)
}

func fusionConfigValue(config *ai.ClientConfig, field string) string {
	if config == nil {
		return ""
	}

	switch field {
	case "model":
		return config.Model
	case "endpoint":
		return config.Endpoint
	default:
		return ""
	}
}

// generateAISummary generates and saves AI summary for an article
func (m *AIEnhancedManager) generateAISummary(task *AIEnhancedTask, content string) {
	if !m.isTaskOperationCurrent(task) {
		return
	}
	// Check if AI usage limit is reached
	if m.isAILimitReached(task.UserID) {
		log.Printf("AI usage limit reached for user %d, skipping summary", task.UserID)
		m.fallbackArticleSummaryToTitle(task, "ai usage limit reached")
		return
	}

	config, err := getUserFeatureAIConfig(m.db, task.UserID, ai.FeatureSummary)
	if err != nil {
		log.Printf("Error resolving AI summary config for user %d: %v", task.UserID, err)
		m.fallbackArticleSummaryToTitle(task, "summary config unavailable")
		return
	}
	if !hasConfiguredAPIKey(config) {
		log.Printf("No user-level AI summary config available for user %d, skipping summary", task.UserID)
		m.fallbackArticleSummaryToTitle(task, "summary config missing api key")
		return
	}

	systemPrompt, _ := m.db.GetSettingForUser(task.UserID, "ai_summary_prompt")
	summaryLengthStr, _ := m.db.GetSettingWithFallback(task.UserID, "summary_length")

	// Set summary length
	summaryLength := summary.Medium
	switch summaryLengthStr {
	case "short":
		summaryLength = summary.Short
	case "long":
		summaryLength = summary.Long
	}

	// Create AI summarizer with Chinese language
	aiSummarizer := summary.NewAISummarizerWithDB(config.APIKey, config.Endpoint, config.Model, m.db, true)
	if systemPrompt != "" {
		aiSummarizer.SetSystemPrompt(systemPrompt)
	}
	if config.CustomHeaders != "" {
		aiSummarizer.SetCustomHeaders(config.CustomHeaders)
	}
	aiSummarizer.SetLanguage("zh") // Force Chinese for AI enhanced mode

	// Generate summary
	result, err := aiSummarizer.Summarize(content, summaryLength)
	if err != nil {
		m.recordTaskFailure(task.UserID, "summary", task, config.Model, config.Endpoint, err)
		if m.skipArticleStageIfNonRecoverable(task, "summary", err) {
			m.fallbackArticleSummaryToTitle(task, "non-recoverable summary failure")
			return
		}
		log.Printf("Error generating AI summary for article %d using model %q endpoint %q: %v", task.ArticleID, config.Model, config.Endpoint, err)
		m.fallbackArticleSummaryToTitle(task, "summary generation failed")
		return
	}

	if err := m.db.DeleteAIArticleStageSkip(task.ArticleID, "summary"); err != nil {
		log.Printf("Failed to clear AI summary skip marker for article %d: %v", task.ArticleID, err)
	}

	if result.IsTooShort && utf8.RuneCountInString(strings.TrimSpace(result.Summary)) < 10 {
		article, articleErr := m.db.GetArticleByIDForUser(task.UserID, task.ArticleID)
		if articleErr != nil {
			log.Printf("Failed to load article title for short summary fallback, article %d: %v", task.ArticleID, articleErr)
		} else if article != nil {
			title := strings.TrimSpace(article.Title)
			if title != "" {
				result.Summary = title
			}
		}
	}

	// Track AI usage
	inputTokens := ai.EstimateTokens(content)
	outputTokens := ai.EstimateTokens(result.Summary)
	totalTokens := inputTokens + outputTokens
	m.addAIUsage(task.UserID, totalTokens)

	if !m.isTaskOperationCurrent(task) {
		log.Printf("Discarding stale AI summary for article %d user %d", task.ArticleID, task.UserID)
		return
	}

	// Cache the summary in the database
	if err := m.db.UpdateArticleSummary(task.ArticleID, result.Summary); err != nil {
		log.Printf("Failed to cache summary for article %d: %v", task.ArticleID, err)
	} else {
		log.Printf("Successfully cached AI summary for article %d", task.ArticleID)
	}
}

func (m *AIEnhancedManager) fallbackArticleSummaryToTitle(task *AIEnhancedTask, reason string) {
	if task == nil || !task.ForceTitleSummaryFallback || task.ArticleID <= 0 {
		return
	}

	title := strings.TrimSpace(task.ArticleTitle)
	if title == "" {
		article, err := m.db.GetArticleByID(task.ArticleID)
		if err != nil {
			log.Printf("Failed to load article title for summary fallback on article %d: %v", task.ArticleID, err)
			return
		}
		if article != nil {
			title = strings.TrimSpace(article.Title)
		}
	}
	if title == "" {
		return
	}

	if err := m.db.UpdateArticleSummary(task.ArticleID, title); err != nil {
		log.Printf("Failed to fallback summary to title for article %d: %v", task.ArticleID, err)
		return
	}
	task.NeedsSummary = false
	log.Printf("Using article title as summary fallback for article %d (%s)", task.ArticleID, reason)
}

// generateAITranslation generates and saves AI translation for an article
func (m *AIEnhancedManager) generateAITranslation(task *AIEnhancedTask, content string) {
	if !m.isTaskOperationCurrent(task) {
		return
	}
	// Check if AI usage limit is reached
	if m.isAILimitReached(task.UserID) {
		log.Printf("AI usage limit reached for user %d, skipping translation", task.UserID)
		return
	}

	config, err := getUserFeatureAIConfig(m.db, task.UserID, ai.FeatureTranslation)
	if err != nil {
		log.Printf("Error resolving AI translation config for user %d: %v", task.UserID, err)
		return
	}
	if !hasConfiguredAPIKey(config) {
		log.Printf("No user-level AI translation config available for user %d, skipping translation", task.UserID)
		return
	}

	systemPrompt, _ := m.db.GetSettingForUser(task.UserID, "ai_translation_prompt")
	targetLang, _ := m.db.GetSettingWithFallback(task.UserID, "target_language")
	if targetLang == "" {
		targetLang = "zh"
	}

	// Create AI translator
	aiTranslator := translation.NewAITranslatorWithDB(config.APIKey, config.Endpoint, config.Model, m.db, true)
	if systemPrompt != "" {
		aiTranslator.SetSystemPrompt(systemPrompt)
	}
	if config.CustomHeaders != "" {
		aiTranslator.SetCustomHeaders(config.CustomHeaders)
	}

	translationInput := prepareAITranslationInput(content)
	if translationInput == "" {
		log.Printf("No translatable text extracted for article %d, caching original content", task.ArticleID)
		if err := m.db.SetArticleTranslatedContent(task.ArticleID, content, targetLang, "ai"); err != nil {
			log.Printf("Failed to cache original content as translation for article %d: %v", task.ArticleID, err)
		}
		return
	}

	// Generate translation
	translatedContent, err := translation.TranslateMarkdownAIPrompt(translationInput, aiTranslator, targetLang)
	if err != nil {
		m.recordTaskFailure(task.UserID, "translation", task, config.Model, config.Endpoint, err)
		if m.skipArticleStageIfNonRecoverable(task, "translation", err) {
			return
		}
		log.Printf("Error generating AI translation for article %d using model %q endpoint %q: %v", task.ArticleID, config.Model, config.Endpoint, err)
		return
	}

	if err := m.db.DeleteAIArticleStageSkip(task.ArticleID, "translation"); err != nil {
		log.Printf("Failed to clear AI translation skip marker for article %d: %v", task.ArticleID, err)
	}

	// Track AI usage
	inputTokens := ai.EstimateTokens(translationInput)
	outputTokens := ai.EstimateTokens(translatedContent)
	totalTokens := inputTokens + outputTokens
	m.addAIUsage(task.UserID, totalTokens)

	if !m.isTaskOperationCurrent(task) {
		log.Printf("Discarding stale AI translation for article %d user %d", task.ArticleID, task.UserID)
		return
	}

	// Cache the translation in the database
	if err := m.db.SetArticleTranslatedContent(task.ArticleID, translatedContent, targetLang, "ai"); err != nil {
		log.Printf("Failed to cache translation for article %d: %v", task.ArticleID, err)
	} else {
		log.Printf("Successfully cached AI translation for article %d", task.ArticleID)
	}
}

func prepareAITranslationInput(content string) string {
	prepared := strings.TrimSpace(content)
	if prepared == "" {
		return ""
	}

	if looksLikeHTMLContent(prepared) {
		converter := md.NewConverter("", true, nil)
		if markdown, err := converter.ConvertString(prepared); err == nil {
			prepared = markdown
		} else {
			prepared = stripHTMLForTranslation(prepared)
		}
		prepared = html.UnescapeString(prepared)
	}

	return normalizeTranslationInput(prepared)
}

func looksLikeHTMLContent(content string) bool {
	return strings.Contains(content, "<") && strings.Contains(content, ">")
}

func stripHTMLForTranslation(content string) string {
	replacer := strings.NewReplacer(
		"<br>", "\n", "<br/>", "\n", "<br />", "\n",
		"</p>", "\n", "</div>", "\n", "</li>", "\n",
	)
	content = replacer.Replace(content)

	inTag := false
	var result strings.Builder
	for _, r := range content {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				result.WriteRune(r)
			}
		}
	}

	return result.String()
}

func normalizeTranslationInput(content string) string {
	content = strings.ReplaceAll(content, "\u00a0", " ")
	lines := strings.Split(content, "\n")
	normalized := make([]string, 0, len(lines))
	lastBlank := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if isNonTranslatableMarkdownLine(line) {
			line = ""
		}
		if line == "" {
			if !lastBlank {
				normalized = append(normalized, "")
				lastBlank = true
			}
			continue
		}

		line = strings.Join(strings.Fields(line), " ")
		normalized = append(normalized, line)
		lastBlank = false
	}

	return strings.TrimSpace(strings.Join(normalized, "\n"))
}

func isNonTranslatableMarkdownLine(line string) bool {
	if line == "" {
		return false
	}
	return strings.HasPrefix(line, "![") && strings.Contains(line, "](") && strings.HasSuffix(line, ")")
}

// isAILimitReached checks if the AI usage limit is reached for a specific user
func (m *AIEnhancedManager) isAILimitReached(userID int64) bool {
	usageStr, err := m.db.GetSettingWithFallback(userID, "ai_usage_tokens")
	if err != nil {
		return false
	}
	usage, _ := strconv.ParseInt(usageStr, 10, 64)

	userLimitStr, _ := m.db.GetSettingWithFallback(userID, "ai_usage_limit")
	hardLimitStr, _ := m.db.GetSettingWithFallback(userID, "ai_usage_hard_limit")

	userLimit, _ := strconv.ParseInt(userLimitStr, 10, 64)
	hardLimit, _ := strconv.ParseInt(hardLimitStr, 10, 64)

	effectiveLimit := int64(0)
	if userLimit > 0 && hardLimit > 0 {
		effectiveLimit = min(userLimit, hardLimit)
	} else if userLimit > 0 {
		effectiveLimit = userLimit
	} else if hardLimit > 0 {
		effectiveLimit = hardLimit
	}

	if effectiveLimit == 0 {
		return false
	}

	return usage >= effectiveLimit
}

func getUserFeatureAIConfig(db *sqlite.DB, userID int64, feature ai.FeatureType) (*ai.ClientConfig, error) {
	if userID <= 0 {
		return nil, nil
	}
	return ai.NewProfileProvider(db).GetConfigForFeatureForUser(userID, feature)
}

func hasConfiguredAPIKey(config *ai.ClientConfig) bool {
	return config != nil && config.APIKey != ""
}

// addAIUsage adds tokens to the AI usage counter for a specific user
func (m *AIEnhancedManager) addAIUsage(userID int64, tokens int64) {
	usageStr, _ := m.db.GetSettingWithFallback(userID, "ai_usage_tokens")
	currentUsage, _ := strconv.ParseInt(usageStr, 10, 64)
	newUsage := currentUsage + tokens
	if userID > 0 {
		m.db.SetSettingForUser(userID, "ai_usage_tokens", strconv.FormatInt(newUsage, 10))
	} else {
		m.db.SetSetting("ai_usage_tokens", strconv.FormatInt(newUsage, 10))
	}
}

func (m *AIEnhancedManager) tryEnqueueTask(task *AIEnhancedTask) bool {
	if task == nil || task.UserID <= 0 {
		return false
	}

	m.stampTaskOperationVersion(task)

	m.taskQueueMu.Lock()
	defer m.taskQueueMu.Unlock()

	select {
	case m.taskChan <- task:
		m.incrementQueuedTask(task.UserID)
		return true
	default:
		return false
	}
}

func (m *AIEnhancedManager) purgeQueuedTasksForUser(userID int64) int {
	if userID <= 0 {
		return 0
	}

	m.taskQueueMu.Lock()
	defer m.taskQueueMu.Unlock()

	buffered := len(m.taskChan)
	if buffered == 0 {
		return 0
	}

	kept := make([]*AIEnhancedTask, 0, buffered)
	removed := 0
	for i := 0; i < buffered; i++ {
		select {
		case task := <-m.taskChan:
			if task == nil {
				continue
			}
			if task.UserID == userID {
				m.decrementQueuedTask(userID)
				removed++
				continue
			}
			kept = append(kept, task)
		default:
			i = buffered
		}
	}

	for _, task := range kept {
		m.taskChan <- task
	}

	return removed
}

// BatchProcessExistingArticles scans existing articles for a user and queues them for AI processing.
// This is triggered when AI Enhanced Mode is first activated.
// It processes: all favorited articles + unfavorited articles from the last 2 days.
func (m *AIEnhancedManager) BatchProcessExistingArticles(userID int64) {
	go func() {
		queued, err := m.queueExistingArticlesForProcessing(userID)
		if err != nil {
			log.Printf("Error fetching articles for AI batch processing (user %d): %v", userID, err)
			return
		}

		log.Printf("Batch AI processing: queued %d tasks for user %d", queued, userID)
	}()
}

func (m *AIEnhancedManager) queueExistingArticlesForProcessing(userID int64) (int, error) {
	log.Printf("Starting batch AI processing for user %d...", userID)
	if m.isRenormalizationRunning(userID) {
		log.Printf("Skipping batch AI processing for user %d because cluster renormalization is running", userID)
		return 0, nil
	}

	health, allowed, err := m.guardEmbeddingHealth(userID, blockedScopeBatchQueue)
	if err != nil {
		return 0, fmt.Errorf("embedding health gate: %w", err)
	}
	if !allowed {
		log.Printf(
			"Skipping batch AI processing for user %d due to unhealthy summary embeddings (sample=%d, unnormalized=%d, ratio=%.2f)",
			userID,
			health.SampleSize,
			health.UnnormalizedCount,
			health.UnnormalizedRatio,
		)
		return 0, nil
	}

	targetLang, _ := m.db.GetSettingWithFallback(userID, "target_language")
	if targetLang == "" {
		targetLang = "zh"
	}

	articles, err := m.db.GetArticlesForAIBatchProcessing(userID, targetLang)
	if err != nil {
		return 0, err
	}
	clusterProgress, err := m.db.GetClusterProcessingProgress(userID)
	if err != nil {
		return 0, err
	}
	hasPendingClusterWork := clusterProgress.PendingMergeClusters > 0 || clusterProgress.PendingEmbedClusters > 0

	if len(articles) == 0 {
		if hasPendingClusterWork {
			log.Printf("No article-stage work for user %d, scheduling pending cluster post-processing", userID)
			m.scheduleClusterPipeline(userID)
			return 0, nil
		}
		log.Printf("No articles to batch process for user %d", userID)
		return 0, nil
	}

	log.Printf("Found %d articles for AI batch processing (user %d)", len(articles), userID)

	queued := 0
	clusterRunNeeded := false
	for _, article := range articles {
		needsSummary := !article.HasSummary
		needsTranslation := article.TranslateArticles && !article.HasTranslation
		needsEmbedding := !article.HasArticleEmbedding
		needsDedup := !article.HasCluster
		needsClusterRun := article.HasCluster && article.ClusterNeedsPostProcess

		if article.ClusterNeedsEmbeddingRepair && article.Article.ClusterID > 0 {
			if err := m.db.UpdateClusterStatus(article.Article.ClusterID, "pending_embed"); err != nil {
				log.Printf("Failed to mark cluster %d for embedding repair: %v", article.Article.ClusterID, err)
			} else {
				needsClusterRun = true
			}
		}

		if !needsSummary && !needsTranslation && !needsEmbedding && !needsDedup {
			if needsClusterRun {
				clusterRunNeeded = true
			} else if article.Article.IsFavorite && article.HasCluster && article.Article.ClusterID > 0 {
				if err := m.db.SetClusterFavorite(article.Article.ClusterID, true); err != nil {
					log.Printf(
						"Failed to resync favorite cluster %d from completed favorite article %d: %v",
						article.Article.ClusterID,
						article.Article.ID,
						err,
					)
				}
			}
			continue
		}

		task := &AIEnhancedTask{
			ArticleID:         article.Article.ID,
			UserID:            userID,
			FeedID:            article.Article.FeedID,
			ArticleTitle:      article.Article.Title,
			NeedsSummary:      needsSummary,
			NeedsTranslation:  needsTranslation,
			TranslateArticles: article.TranslateArticles,
			NeedsEmbedding:    needsEmbedding,
			NeedsDedup:        needsDedup,
			NeedsClusterRun:   needsClusterRun,
		}

		if !m.tryEnqueueTask(task) {
			log.Printf("AI enhanced task queue full during batch processing, queued %d/%d articles", queued, len(articles))
			return queued, nil
		}
		queued++
	}

	if clusterRunNeeded {
		m.scheduleClusterPipeline(userID)
	} else if hasPendingClusterWork {
		m.scheduleClusterPipeline(userID)
	}

	return queued, nil
}

// AddTask adds a task to the AI enhanced mode queue
func (m *AIEnhancedManager) AddTask(task *AIEnhancedTask) {
	if task == nil || task.UserID <= 0 {
		return
	}
	if m.isRenormalizationRunning(task.UserID) {
		log.Printf("Skipping AI enhanced task for article %d because cluster renormalization is running", task.ArticleID)
		return
	}

	health, allowed, err := m.guardEmbeddingHealth(task.UserID, blockedScopeNewArticleQueue)
	if err != nil {
		log.Printf("Embedding health gate failed for article %d: %v", task.ArticleID, err)
		return
	}
	if !allowed {
		log.Printf(
			"Skipping AI enhanced task for article %d due to unhealthy summary embeddings (sample=%d, unnormalized=%d, ratio=%.2f)",
			task.ArticleID,
			health.SampleSize,
			health.UnnormalizedCount,
			health.UnnormalizedRatio,
		)
		return
	}

	select {
	case <-m.stopChan:
		return
	default:
	}

	if m.tryEnqueueTask(task) {
		log.Printf("Added AI enhanced task for article %d", task.ArticleID)
		return
	}
	log.Printf("AI enhanced task queue full, dropped task for article %d", task.ArticleID)
}

// Stop stops the AI enhanced mode manager and cleans up resources
func (m *AIEnhancedManager) Stop() {
	log.Println("Stopping AI enhanced mode manager...")
	close(m.stopChan)
	close(m.taskChan)
	m.workerWg.Wait()
	log.Println("AI enhanced mode manager stopped")
}

// ShouldProcess checks if AI enhanced mode should process an article for a user
func ShouldProcess(db *sqlite.DB, userID int64) bool {
	// Check if AI enhanced mode is enabled
	enhancedModeStr, _ := db.GetSettingWithFallback(userID, "ai_enhanced_mode")
	if enhancedModeStr != "true" {
		return false
	}

	// Check if at least one embedding model is usable.
	embeddingsConfig, _ := db.GetSettingWithFallback(userID, "ai_embedding_models")
	if !hasUsableEmbeddingModelConfig(embeddingsConfig) {
		return false
	}

	summaryConfig, err := getUserFeatureAIConfig(db, userID, ai.FeatureSummary)
	if err != nil || !hasConfiguredAPIKey(summaryConfig) {
		return false
	}

	translationConfig, err := getUserFeatureAIConfig(db, userID, ai.FeatureTranslation)
	if err != nil || !hasConfiguredAPIKey(translationConfig) {
		return false
	}

	searchConfig, err := getUserFeatureAIConfig(db, userID, ai.FeatureSearch)
	if err != nil || !hasConfiguredAPIKey(searchConfig) {
		return false
	}

	chatConfig, err := getUserFeatureAIConfig(db, userID, ai.FeatureChat)
	if err != nil || !hasConfiguredAPIKey(chatConfig) {
		return false
	}

	fusionConfig, err := getUserFeatureAIConfig(db, userID, ai.FeatureFusion)
	if err != nil || !hasConfiguredAPIKey(fusionConfig) {
		return false
	}

	// Check if summary is enabled and using AI
	summaryEnabled, _ := db.GetSettingWithFallback(userID, "summary_enabled")
	summaryProvider, _ := db.GetSettingWithFallback(userID, "summary_provider")
	if summaryEnabled != "true" || summaryProvider != "ai" {
		return false
	}

	// Check if translation is enabled
	translationEnabled, _ := db.GetSettingWithFallback(userID, "translation_enabled")
	if translationEnabled != "true" {
		return false
	}

	// Check if AI search is enabled
	aiSearchEnabled, _ := db.GetSettingWithFallback(userID, "ai_search_enabled")
	if aiSearchEnabled != "true" {
		return false
	}

	// Check if AI chat is enabled
	aiChatEnabled, _ := db.GetSettingWithFallback(userID, "ai_chat_enabled")
	if aiChatEnabled != "true" {
		return false
	}

	// Check if AI fusion is enabled
	aiFusionEnabled, _ := db.GetSettingWithFallback(userID, "ai_fusion_enabled")
	if aiFusionEnabled != "true" {
		return false
	}

	// Check if AI daily recommendations are enabled
	aiRecommendationEnabled, _ := db.GetSettingWithFallback(userID, "ai_recommendation_enabled")
	if aiRecommendationEnabled != "true" {
		return false
	}

	return true
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func hasUsableEmbeddingModelConfig(configsJSON string) bool {
	if strings.TrimSpace(configsJSON) == "" {
		return false
	}

	var configs []models.EmbeddingModelConfig
	if err := json.Unmarshal([]byte(configsJSON), &configs); err != nil {
		return false
	}

	for _, config := range configs {
		if strings.TrimSpace(config.ModelName) != "" && strings.TrimSpace(config.BaseURL) != "" {
			return true
		}
	}

	return false
}

func buildGlobalProxyURL(db *sqlite.DB, userID int64) (string, error) {
	proxyEnabled, err := db.GetSettingWithFallback(userID, "proxy_enabled")
	if err != nil || proxyEnabled != "true" {
		return "", err
	}

	proxyType, err := db.GetSettingWithFallback(userID, "proxy_type")
	if err != nil {
		return "", err
	}
	proxyHost, err := db.GetSettingWithFallback(userID, "proxy_host")
	if err != nil {
		return "", err
	}
	proxyPort, err := db.GetSettingWithFallback(userID, "proxy_port")
	if err != nil {
		return "", err
	}
	proxyUsername, err := db.GetEncryptedSettingWithFallback(userID, "proxy_username")
	if err != nil {
		return "", err
	}
	proxyPassword, err := db.GetEncryptedSettingWithFallback(userID, "proxy_password")
	if err != nil {
		return "", err
	}

	return httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword), nil
}

func (m *AIEnhancedManager) startPendingWorkRecoveryMonitor() {
	go func() {
		startupTimer := time.NewTimer(5 * time.Second)
		defer startupTimer.Stop()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-startupTimer.C:
				m.recoverPendingWorkForKnownUsers("startup")
			case <-ticker.C:
				m.recoverPendingWorkForKnownUsers("periodic")
			case <-m.stopChan:
				return
			}
		}
	}()
}

func (m *AIEnhancedManager) recoverPendingWorkForKnownUsers(reason string) {
	userIDs, err := m.db.ListKnownUserIDs()
	if err != nil {
		log.Printf("pending AI work recovery list users error: %v", err)
		return
	}

	candidateUserIDs := make([]int64, 0, len(userIDs))
	for _, userID := range userIDs {
		if !m.shouldInspectUserForRecovery(userID) {
			continue
		}
		candidateUserIDs = append(candidateUserIDs, userID)
	}

	if len(candidateUserIDs) > 0 {
		log.Printf("Scanning %d AI-enabled users for recoverable AI processing work (%s)", len(candidateUserIDs), reason)
	}

	for _, userID := range candidateUserIDs {
		status := m.GetProcessingStatus(userID)
		m.recoverPendingWorkIfIdle(userID, status, reason)
	}
}

func (m *AIEnhancedManager) shouldInspectUserForRecovery(userID int64) bool {
	if m == nil || m.db == nil || userID <= 0 {
		return false
	}
	if m.isRenormalizationRunning(userID) {
		return false
	}
	return ShouldProcess(m.db, userID)
}

func (m *AIEnhancedManager) incrementQueuedTask(userID int64) {
	if userID <= 0 {
		return
	}

	m.statusMu.Lock()
	if m.queuedTasksByUser == nil {
		m.queuedTasksByUser = make(map[int64]int)
	}
	m.queuedTasksByUser[userID]++
	m.statusMu.Unlock()
}

func (m *AIEnhancedManager) decrementQueuedTask(userID int64) {
	if userID <= 0 {
		return
	}

	m.statusMu.Lock()
	defer m.statusMu.Unlock()

	next := m.queuedTasksByUser[userID] - 1
	if next > 0 {
		m.queuedTasksByUser[userID] = next
		return
	}
	delete(m.queuedTasksByUser, userID)
}

func (m *AIEnhancedManager) incrementActiveWorkerTask(userID int64) {
	if userID <= 0 {
		return
	}

	m.statusMu.Lock()
	if m.activeWorkerTasksByUser == nil {
		m.activeWorkerTasksByUser = make(map[int64]int64)
	}
	m.activeWorkerTasksByUser[userID]++
	m.statusMu.Unlock()
}

func (m *AIEnhancedManager) decrementActiveWorkerTask(userID int64) {
	if userID <= 0 {
		return
	}

	m.statusMu.Lock()
	defer m.statusMu.Unlock()

	next := m.activeWorkerTasksByUser[userID] - 1
	if next > 0 {
		m.activeWorkerTasksByUser[userID] = next
		return
	}
	delete(m.activeWorkerTasksByUser, userID)
}

func (m *AIEnhancedManager) incrementActiveAsyncWork(userID int64) {
	if userID <= 0 {
		return
	}

	m.statusMu.Lock()
	if m.activeAsyncWorkByUser == nil {
		m.activeAsyncWorkByUser = make(map[int64]int64)
	}
	m.activeAsyncWorkByUser[userID]++
	m.statusMu.Unlock()
}

func (m *AIEnhancedManager) decrementActiveAsyncWork(userID int64) {
	if userID <= 0 {
		return
	}

	m.statusMu.Lock()
	defer m.statusMu.Unlock()

	next := m.activeAsyncWorkByUser[userID] - 1
	if next > 0 {
		m.activeAsyncWorkByUser[userID] = next
		return
	}
	delete(m.activeAsyncWorkByUser, userID)
}

func (m *AIEnhancedManager) getUserTaskCounts(userID int64) (int, int64, int64) {
	if userID <= 0 {
		return 0, 0, 0
	}

	m.statusMu.Lock()
	defer m.statusMu.Unlock()

	return m.queuedTasksByUser[userID], m.activeWorkerTasksByUser[userID], m.activeAsyncWorkByUser[userID]
}

func (m *AIEnhancedManager) recordTaskFailure(userID int64, stage string, task *AIEnhancedTask, model, endpoint string, err error) {
	if userID <= 0 || err == nil {
		return
	}

	message := strings.TrimSpace(ai.DiagnosticMessage(err))
	if message == "" {
		message = strings.TrimSpace(err.Error())
	}
	if len(message) > 600 {
		message = message[:600]
	}

	failure := AIProcessingFailure{
		Stage:      stage,
		Message:    message,
		Model:      strings.TrimSpace(model),
		Endpoint:   strings.TrimSpace(endpoint),
		OccurredAt: time.Now(),
		Count:      1,
	}
	if task != nil {
		failure.ArticleID = task.ArticleID
		failure.ArticleTitle = strings.TrimSpace(task.ArticleTitle)
	}

	m.statusMu.Lock()
	defer m.statusMu.Unlock()

	if m.recentFailureByUser == nil {
		m.recentFailureByUser = make(map[int64]AIProcessingFailure)
	}

	if previous, ok := m.recentFailureByUser[userID]; ok &&
		previous.Stage == failure.Stage &&
		previous.Message == failure.Message &&
		previous.ArticleID == failure.ArticleID {
		failure.Count = previous.Count + 1
	}

	m.recentFailureByUser[userID] = failure
}

func (m *AIEnhancedManager) skipArticleStageIfNonRecoverable(task *AIEnhancedTask, stage string, err error) bool {
	if task == nil || task.ArticleID <= 0 || task.UserID <= 0 || !ai.ShouldSkipArticleRetry(err) {
		return false
	}

	reason := strings.TrimSpace(ai.DiagnosticMessage(err))
	if reason == "" {
		reason = strings.TrimSpace(err.Error())
	}
	if len(reason) > 600 {
		reason = reason[:600]
	}

	if dbErr := m.db.SetAIArticleStageSkip(task.UserID, task.ArticleID, stage, reason); dbErr != nil {
		log.Printf("Failed to persist AI %s skip marker for article %d: %v", stage, task.ArticleID, dbErr)
		return false
	}

	log.Printf("Skipping future AI %s retries for article %d due to non-recoverable article-specific failure: %s", stage, task.ArticleID, reason)
	return true
}

func (m *AIEnhancedManager) getRecentFailure(userID int64) AIProcessingFailure {
	if userID <= 0 {
		return AIProcessingFailure{}
	}

	m.statusMu.Lock()
	defer m.statusMu.Unlock()

	return m.recentFailureByUser[userID]
}

func (m *AIEnhancedManager) beginRecoveryAttempt(userID int64, cooldown time.Duration) bool {
	if userID <= 0 {
		return false
	}

	now := time.Now()

	m.statusMu.Lock()
	defer m.statusMu.Unlock()

	if m.recoveryInProgress == nil {
		m.recoveryInProgress = make(map[int64]bool)
	}
	if m.lastRecoveryAttemptByUser == nil {
		m.lastRecoveryAttemptByUser = make(map[int64]time.Time)
	}

	if m.recoveryInProgress[userID] {
		return false
	}
	if lastAttempt := m.lastRecoveryAttemptByUser[userID]; !lastAttempt.IsZero() && now.Sub(lastAttempt) < cooldown {
		return false
	}

	m.recoveryInProgress[userID] = true
	m.lastRecoveryAttemptByUser[userID] = now
	return true
}

func (m *AIEnhancedManager) finishRecoveryAttempt(userID int64) {
	if userID <= 0 {
		return
	}

	m.statusMu.Lock()
	defer m.statusMu.Unlock()

	delete(m.recoveryInProgress, userID)
}

func (m *AIEnhancedManager) recoverPendingWorkIfIdle(userID int64, status AIProcessingStatus, reason string) {
	hasPendingClusterWork := status.PendingMergeClusters > 0 || status.PendingEmbedClusters > 0
	if userID <= 0 || !status.IsEnabled || (status.PendingArticles == 0 && !hasPendingClusterWork) {
		return
	}
	if m.isRenormalizationRunning(userID) {
		return
	}
	if status.EmbeddingHealthBlocked {
		return
	}
	pipelineRunning, _, _, _ := m.getClusterStageState(userID)
	if status.QueuedTasks > 0 || status.ActiveWorkerTasks > 0 || status.ActiveAsyncWork > 0 || pipelineRunning {
		return
	}
	if !ShouldProcess(m.db, userID) {
		return
	}
	if !m.beginRecoveryAttempt(userID, 15*time.Second) {
		return
	}

	go func() {
		defer m.finishRecoveryAttempt(userID)

		log.Printf("Detected idle-but-incomplete AI processing state for user %d, attempting recovery (%s)", userID, reason)
		queued, err := m.queueExistingArticlesForProcessing(userID)
		if err != nil {
			log.Printf("AI processing recovery failed for user %d: %v", userID, err)
			return
		}

		log.Printf("AI processing recovery finished for user %d, queued %d tasks", userID, queued)
	}()
}

func (m *AIEnhancedManager) GetProcessingStatus(userID int64) AIProcessingStatus {
	status := AIProcessingStatus{}

	if userID <= 0 {
		return status
	}

	status.IsEnabled = ShouldProcess(m.db, userID)
	if interestVecBlob, err := m.db.GetUserInterestVector(userID); err == nil && len(interestVecBlob) > 0 {
		status.HasInterestVector = true
	}
	if health, err := m.getEmbeddingHealthStatus(userID, false); err != nil {
		log.Printf("failed to get embedding health status for user %d: %v", userID, err)
	} else {
		status.EmbeddingHealthBlocked = !health.IsHealthy
		status.EmbeddingHealthSampleSize = health.SampleSize
		status.EmbeddingHealthUnnormalizedCount = health.UnnormalizedCount
		status.EmbeddingHealthUnnormalizedRatio = health.UnnormalizedRatio
	}

	targetLang, _ := m.db.GetSettingWithFallback(userID, "target_language")
	isRenormalizing := m.isRenormalizationRunning(userID)
	status.IsRenormalizationRunning = isRenormalizing

	var progress sqlite.AIProcessingProgress
	var err error
	if isRenormalizing {
		progress, err = m.db.GetAIReclusterNormalizationProgress(userID, targetLang)
	} else {
		progress, err = m.db.GetAIProcessingProgress(userID, targetLang)
	}
	if err != nil {
		log.Printf("failed to get AI processing progress for user %d: %v", userID, err)
	} else {
		status.EligibleArticles = progress.EligibleArticles
		status.PendingArticles = progress.PendingArticles
		status.CompletedArticles = progress.CompletedArticles
		status.PendingSummaryArticles = progress.PendingSummaryArticles
		status.PendingTranslationArticles = progress.PendingTranslationArticles
		status.PendingEmbeddingArticles = progress.PendingEmbeddingArticles
		status.PendingClusteringArticles = progress.PendingClusteringArticles
		if isRenormalizing {
			status.RenormalizationTotalArticles = progress.EligibleArticles
			status.RenormalizationPendingArticles = progress.PendingArticles
			status.RenormalizationCompletedArticles = progress.CompletedArticles
		}
	}
	clusterProgress, err := m.db.GetClusterProcessingProgress(userID)
	if err != nil {
		log.Printf("failed to get cluster processing progress for user %d: %v", userID, err)
	} else {
		status.PendingMergeClusters = clusterProgress.PendingMergeClusters
		status.PendingEmbedClusters = clusterProgress.PendingEmbedClusters
	}
	status.PendingRecommendationDays = m.getPendingRecommendationDays(userID)

	status.QueuedTasks, status.ActiveWorkerTasks, status.ActiveAsyncWork = m.getUserTaskCounts(userID)

	pipelineBusy, fusionRunning, embeddingRunning, phase := m.getClusterStageState(userID)
	status.IsClusterPipelineBusy = pipelineBusy || status.PendingMergeClusters > 0 || status.PendingEmbedClusters > 0
	status.IsClusterFusionRunning = fusionRunning
	status.IsClusterEmbeddingRunning = embeddingRunning
	if phase != "" {
		status.ClusterPhase = phase
	}

	if status.EligibleArticles > 0 {
		status.ProgressPercent = (float64(status.CompletedArticles) / float64(status.EligibleArticles)) * 100
		if status.ProgressPercent < 0 {
			status.ProgressPercent = 0
		}
		if status.ProgressPercent > 100 {
			status.ProgressPercent = 100
		}
		if status.ProgressPercent == 100 && (status.PendingMergeClusters > 0 || status.PendingEmbedClusters > 0) {
			status.ProgressPercent = 99
		}
	} else if status.PendingArticles == 0 && status.PendingMergeClusters == 0 && status.PendingEmbedClusters == 0 {
		status.ProgressPercent = 100
	}

	status.IsConfigFrozen = status.IsEnabled && (status.PendingArticles > 0 ||
		status.QueuedTasks > 0 ||
		status.ActiveWorkerTasks > 0 ||
		status.ActiveAsyncWork > 0 ||
		status.PendingMergeClusters > 0 ||
		status.PendingEmbedClusters > 0 ||
		status.IsClusterPipelineBusy ||
		isRenormalizing)

	recentFailure := m.getRecentFailure(userID)
	if !recentFailure.OccurredAt.IsZero() {
		status.RecentFailureStage = recentFailure.Stage
		status.RecentFailureMessage = recentFailure.Message
		status.RecentFailureArticleID = recentFailure.ArticleID
		status.RecentFailureArticleTitle = recentFailure.ArticleTitle
		status.RecentFailureModel = recentFailure.Model
		status.RecentFailureEndpoint = recentFailure.Endpoint
		status.RecentFailureAt = recentFailure.OccurredAt.Format(time.RFC3339)
		status.RecentFailureCount = recentFailure.Count
	}

	m.determineClusterPhase(&status)
	m.updateFreezeSuspensionState(userID, &status)

	return status
}

func (m *AIEnhancedManager) IsConfigFrozen(userID int64) bool {
	return m.GetProcessingStatus(userID).IsConfigFrozen
}

func (m *AIEnhancedManager) getPendingRecommendationDays(userID int64) int {
	if userID <= 0 {
		return 0
	}

	recommendationEnabled, _ := m.db.GetSettingWithFallback(userID, "ai_recommendation_enabled")
	if recommendationEnabled != "true" {
		return 0
	}

	targetDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	hasRecommendations, err := m.db.HasDailyRecommendations(userID, targetDate)
	if err != nil || hasRecommendations {
		return 0
	}

	return 1
}

func (m *AIEnhancedManager) buildProcessingSnapshot(status AIProcessingStatus) string {
	parts := []string{
		strconv.Itoa(status.PendingArticles),
		strconv.Itoa(status.CompletedArticles),
		strconv.Itoa(status.PendingSummaryArticles),
		strconv.Itoa(status.PendingTranslationArticles),
		strconv.Itoa(status.PendingEmbeddingArticles),
		strconv.Itoa(status.PendingClusteringArticles),
		strconv.Itoa(status.PendingMergeClusters),
		strconv.Itoa(status.PendingEmbedClusters),
		strconv.Itoa(status.PendingRecommendationDays),
	}
	return strings.Join(parts, "|")
}

func (m *AIEnhancedManager) updateFreezeSuspensionState(userID int64, status *AIProcessingStatus) {
	if userID <= 0 || status == nil {
		return
	}

	if status.PendingArticles == 0 && status.PendingMergeClusters == 0 && status.PendingEmbedClusters == 0 {
		_ = m.db.SetSettingForUser(userID, aiProcessingSnapshotSettingKey, "")
		_ = m.db.SetSettingForUser(userID, aiProcessingLastProgressAtSettingKey, "")
		_ = m.db.SetSettingForUser(userID, aiProcessingFreezeSuspendedSettingKey, "false")
		return
	}

	snapshot := m.buildProcessingSnapshot(*status)
	freezeSuspended, _ := m.db.GetSettingForUser(userID, aiProcessingFreezeSuspendedSettingKey)
	lastSnapshot, _ := m.db.GetSettingForUser(userID, aiProcessingSnapshotSettingKey)
	lastProgressAtStr, _ := m.db.GetSettingForUser(userID, aiProcessingLastProgressAtSettingKey)

	if lastSnapshot == "" || lastSnapshot != snapshot {
		now := time.Now()
		_ = m.db.SetSettingForUser(userID, aiProcessingSnapshotSettingKey, snapshot)
		_ = m.db.SetSettingForUser(userID, aiProcessingLastProgressAtSettingKey, now.Format(time.RFC3339Nano))
		if freezeSuspended != "true" {
			_ = m.db.SetSettingForUser(userID, aiProcessingFreezeSuspendedSettingKey, "false")
		}
		status.LastProgressAt = now.Format(time.RFC3339)
		return
	}

	if lastProgressAtStr == "" {
		now := time.Now()
		_ = m.db.SetSettingForUser(userID, aiProcessingLastProgressAtSettingKey, now.Format(time.RFC3339Nano))
		status.LastProgressAt = now.Format(time.RFC3339)
		return
	}

	lastProgressAt, err := time.Parse(time.RFC3339Nano, lastProgressAtStr)
	if err != nil {
		now := time.Now()
		_ = m.db.SetSettingForUser(userID, aiProcessingLastProgressAtSettingKey, now.Format(time.RFC3339Nano))
		status.LastProgressAt = now.Format(time.RFC3339)
		return
	}

	status.LastProgressAt = lastProgressAt.Format(time.RFC3339)
	stalledFor := time.Since(lastProgressAt)
	if stalledFor < 0 {
		stalledFor = 0
	}
	status.StalledForSeconds = int64(stalledFor.Seconds())

	if freezeSuspended == "true" || stalledFor >= aiProcessingStaleTimeout {
		if freezeSuspended != "true" && stalledFor >= aiProcessingStaleTimeout {
			_ = m.db.SetSettingForUser(userID, aiProcessingFreezeSuspendedSettingKey, "true")
			log.Printf("AI processing freeze suspended for user %d after %s without progress", userID, stalledFor.Round(time.Second))
		}
		status.IsStale = true
		status.IsFreezeSuspended = true
		status.IsConfigFrozen = false
	}
}
