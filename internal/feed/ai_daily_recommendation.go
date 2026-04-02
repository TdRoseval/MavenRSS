package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"MavenRSS/internal/ai"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
)

type rankedRecommendationCandidate struct {
	Candidate      sqlite.DailyRecommendationCandidate
	RecallScore    float64
	FinalScore     float64
	StageOneScore  float64
	StageTwoScore  float64
	ProfileID      int64
	StageOneReason string
	StageTwoReason string
}

type stageOneResponse struct {
	Analysis string   `json:"analysis"`
	Answer   []string `json:"answer"`
}

type stageTwoScoreResponse struct {
	Analysis           string  `json:"analysis"`
	InformationDensity float64 `json:"information_density"`
	PracticalValue     float64 `json:"practical_value"`
	Interestingness    float64 `json:"interestingness"`
	Timeliness         float64 `json:"timeliness"`
}

type stageTwoReviewResponse struct {
	Analysis           string  `json:"analysis"`
	IsReasonable       bool    `json:"is_reasonable"`
	InformationDensity float64 `json:"information_density"`
	PracticalValue     float64 `json:"practical_value"`
	Interestingness    float64 `json:"interestingness"`
	Timeliness         float64 `json:"timeliness"`
}

type DailyRecommendationTaskStatus struct {
	IsEnabled          bool    `json:"is_enabled"`
	HasTask            bool    `json:"has_task"`
	RecommendationDate string  `json:"recommendation_date,omitempty"`
	Trigger            string  `json:"trigger,omitempty"`
	Stage              string  `json:"stage,omitempty"`
	IsQueued           bool    `json:"is_queued"`
	IsRunning          bool    `json:"is_running"`
	IsWaitingForIdle   bool    `json:"is_waiting_for_idle"`
	Force              bool    `json:"force"`
	ProgressPercent    float64 `json:"progress_percent"`
	CandidateCount     int     `json:"candidate_count"`
	SelectedCount      int     `json:"selected_count"`
	SavedCount         int     `json:"saved_count"`
	StartedAt          string  `json:"started_at,omitempty"`
	UpdatedAt          string  `json:"updated_at,omitempty"`
	LastErrorMessage   string  `json:"last_error_message,omitempty"`
	LastErrorAt        string  `json:"last_error_at,omitempty"`
}

const (
	recommendationTriggerAutomatic = "automatic"
	recommendationTriggerManual    = "manual"
	recommendationStageQueued      = "queued"
	recommendationStageWaiting     = "waiting_for_idle"
	recommendationStagePreparing   = "preparing"
	recommendationStageRecalling   = "recalling"
	recommendationStageRanking     = "ranking"
	recommendationStageScoring     = "scoring"
	recommendationStageSaving      = "saving"
	recommendationStageFailed      = "failed"

	recommendationDirectChronologicalMaxCandidates = 10
	recommendationStageOneMinCandidates            = 40
)

func (m *AIEnhancedManager) startDailyRecommendationScheduler() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.scheduleDailyRecommendationsForAllUsers()
			case <-m.stopChan:
				return
			}
		}
	}()
}

func (m *AIEnhancedManager) scheduleDailyRecommendationsForAllUsers() {
	users, err := m.db.ListUsers()
	if err != nil {
		log.Printf("daily recommendation scheduler list users error: %v", err)
		return
	}
	for _, user := range users {
		if user == nil || user.ID <= 0 {
			continue
		}
		if !ShouldProcess(m.db, user.ID) {
			continue
		}
		m.tryScheduleDailyRecommendation(user.ID, time.Now())
	}
}

func (m *AIEnhancedManager) tryScheduleDailyRecommendation(userID int64, now time.Time) {
	if m.isRenormalizationRunning(userID) {
		log.Printf("Skipping scheduled daily recommendations for user %d because cluster renormalization is running", userID)
		return
	}
	health, allowed, err := m.guardEmbeddingHealth(userID, blockedScopeDailyRecommendationAuto)
	if err != nil {
		log.Printf("embedding health gate failed for daily recommendation scheduler user %d: %v", userID, err)
		return
	}
	if !allowed {
		log.Printf(
			"Skipping scheduled daily recommendations for user %d due to unhealthy summary embeddings (sample=%d, unnormalized=%d, ratio=%.2f)",
			userID,
			health.SampleSize,
			health.UnnormalizedCount,
			health.UnnormalizedRatio,
		)
		return
	}

	targetDate := now.AddDate(0, 0, -1).Format("2006-01-02")
	shouldQueue, forceRegenerate, err := m.shouldQueueDailyRecommendations(userID, targetDate)
	if err != nil {
		log.Printf("check daily recommendations for user %d error: %v", userID, err)
		return
	}
	if !shouldQueue {
		return
	}

	targetTime, ok := m.computeDailyRecommendationRunTime(userID, now)
	if !ok || now.Before(targetTime) {
		return
	}

	m.queueRecommendationGeneration(userID, targetDate, false, forceRegenerate, recommendationTriggerAutomatic)
}

func (m *AIEnhancedManager) computeDailyRecommendationRunTime(userID int64, now time.Time) (time.Time, bool) {
	base := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	buffer := 30 * time.Minute
	interval := m.getRefreshInterval(userID)
	if interval <= 0 {
		return base, true
	}

	lastRefresh := m.getLastGlobalRefresh(now)
	if lastRefresh.IsZero() {
		return base, true
	}

	candidate := base
	for i := 0; i < 1440; i++ {
		nextRefresh := lastRefresh
		for !nextRefresh.After(candidate) {
			nextRefresh = nextRefresh.Add(interval)
		}
		prevRefresh := nextRefresh.Add(-interval)
		if candidate.Sub(prevRefresh) >= buffer && nextRefresh.Sub(candidate) >= buffer {
			return candidate, true
		}
		candidate = candidate.Add(time.Minute)
	}

	return time.Time{}, false
}

func (m *AIEnhancedManager) getRefreshInterval(userID int64) time.Duration {
	intervalStr, err := m.db.GetSettingWithFallback(userID, "update_interval")
	if err != nil {
		return 30 * time.Minute
	}
	intervalMinutes, err := strconv.Atoi(intervalStr)
	if err != nil || intervalMinutes <= 0 {
		return 30 * time.Minute
	}
	return time.Duration(intervalMinutes) * time.Minute
}

func (m *AIEnhancedManager) getLastGlobalRefresh(now time.Time) time.Time {
	lastGlobalRefreshStr, err := m.db.GetSetting("last_global_refresh")
	if err != nil || lastGlobalRefreshStr == "" {
		return now
	}
	lastGlobalRefresh, err := time.Parse(time.RFC3339, lastGlobalRefreshStr)
	if err != nil {
		return now
	}
	return lastGlobalRefresh
}

func (m *AIEnhancedManager) requestMissingRecommendationBackfill(userID int64) {
	if userID <= 0 {
		return
	}
	if m.isRenormalizationRunning(userID) {
		log.Printf("Skipping daily recommendation backfill for user %d because cluster renormalization is running", userID)
		return
	}
	health, allowed, err := m.guardEmbeddingHealth(userID, blockedScopeDailyRecommendationAuto)
	if err != nil {
		log.Printf("embedding health gate failed for daily recommendation backfill user %d: %v", userID, err)
		return
	}
	if !allowed {
		log.Printf(
			"Skipping daily recommendation backfill for user %d due to unhealthy summary embeddings (sample=%d, unnormalized=%d, ratio=%.2f)",
			userID,
			health.SampleSize,
			health.UnnormalizedCount,
			health.UnnormalizedRatio,
		)
		return
	}

	targetDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	shouldQueue, forceRegenerate, err := m.shouldQueueDailyRecommendations(userID, targetDate)
	if err != nil {
		log.Printf("check missing recommendation backfill for user %d error: %v", userID, err)
		return
	}
	if !shouldQueue {
		return
	}
	m.queueRecommendationGeneration(userID, targetDate, true, forceRegenerate, recommendationTriggerAutomatic)
}

func (m *AIEnhancedManager) shouldQueueDailyRecommendations(userID int64, recommendationDate string) (bool, bool, error) {
	if recommendationDate == "" {
		recommendationDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}

	currentCount, err := m.db.CountDailyRecommendations(userID, recommendationDate)
	if err != nil {
		return false, false, err
	}
	if currentCount == 0 {
		return true, false, nil
	}
	if currentCount >= 10 {
		return false, false, nil
	}

	dayStart, err := time.ParseInLocation("2006-01-02", recommendationDate, time.Local)
	if err != nil {
		return false, false, fmt.Errorf("parse recommendation date: %w", err)
	}
	dayEnd := dayStart.Add(24 * time.Hour)

	readyCount, err := m.db.CountDailyRecommendationReadyClusters(userID, dayStart, dayEnd)
	if err != nil {
		return false, false, err
	}

	expectedCount := minInt(10, readyCount)
	if expectedCount <= currentCount {
		return false, false, nil
	}

	return true, true, nil
}

func (m *AIEnhancedManager) QueueDailyRecommendations(userID int64, recommendationDate string, waitForIdle bool, forceIfIncomplete bool) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	if m.isRenormalizationRunning(userID) {
		log.Printf("Skipping queued daily recommendations for user %d because cluster renormalization is running", userID)
		return false, nil
	}
	health, allowed, err := m.guardEmbeddingHealth(userID, blockedScopeDailyRecommendationQueue)
	if err != nil {
		return false, fmt.Errorf("embedding health gate: %w", err)
	}
	if !allowed {
		log.Printf(
			"Skipping queued daily recommendations for user %d due to unhealthy summary embeddings (sample=%d, unnormalized=%d, ratio=%.2f)",
			userID,
			health.SampleSize,
			health.UnnormalizedCount,
			health.UnnormalizedRatio,
		)
		return false, nil
	}
	if recommendationDate == "" {
		recommendationDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}

	shouldQueue, forceRegenerate, err := m.shouldQueueDailyRecommendations(userID, recommendationDate)
	if err != nil {
		return false, err
	}
	if !shouldQueue {
		return false, nil
	}
	if !forceIfIncomplete && forceRegenerate {
		return false, nil
	}

	m.queueRecommendationGeneration(userID, recommendationDate, waitForIdle, forceRegenerate, recommendationTriggerAutomatic)
	return true, nil
}

func (m *AIEnhancedManager) ForceDailyRecommendations(userID int64, recommendationDate string, waitForIdle bool) (DailyRecommendationTaskStatus, error) {
	if userID <= 0 {
		return DailyRecommendationTaskStatus{}, nil
	}
	if m.isRenormalizationRunning(userID) {
		log.Printf("Skipping forced daily recommendations for user %d because cluster renormalization is running", userID)
		return m.GetDailyRecommendationTaskStatus(userID), nil
	}
	health, allowed, err := m.guardEmbeddingHealth(userID, blockedScopeDailyRecommendationForce)
	if err != nil {
		return DailyRecommendationTaskStatus{}, fmt.Errorf("embedding health gate: %w", err)
	}
	if !allowed {
		log.Printf(
			"Skipping forced daily recommendations for user %d due to unhealthy summary embeddings (sample=%d, unnormalized=%d, ratio=%.2f)",
			userID,
			health.SampleSize,
			health.UnnormalizedCount,
			health.UnnormalizedRatio,
		)
		return m.GetDailyRecommendationTaskStatus(userID), nil
	}
	if recommendationDate == "" {
		recommendationDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}

	m.queueRecommendationGeneration(userID, recommendationDate, waitForIdle, true, recommendationTriggerManual)
	return m.GetDailyRecommendationTaskStatus(userID), nil
}

func (m *AIEnhancedManager) queueRecommendationGeneration(userID int64, recommendationDate string, waitForIdle bool, force bool, trigger string) {
	m.recommendationMu.Lock()
	defer m.recommendationMu.Unlock()

	if m.recommendationRunning == nil {
		m.recommendationRunning = make(map[int64]bool)
	}
	if m.pendingRecommendationDate == nil {
		m.pendingRecommendationDate = make(map[int64]string)
	}
	if m.pendingRecommendationWait == nil {
		m.pendingRecommendationWait = make(map[int64]bool)
	}
	if m.pendingRecommendationForce == nil {
		m.pendingRecommendationForce = make(map[int64]bool)
	}
	if m.pendingRecommendationMode == nil {
		m.pendingRecommendationMode = make(map[int64]string)
	}
	if m.recommendationStatusByUser == nil {
		m.recommendationStatusByUser = make(map[int64]DailyRecommendationTaskStatus)
	}

	if m.recommendationRunning[userID] {
		m.pendingRecommendationDate[userID] = recommendationDate
		m.pendingRecommendationWait[userID] = m.pendingRecommendationWait[userID] || waitForIdle
		m.pendingRecommendationForce[userID] = m.pendingRecommendationForce[userID] || force
		if trigger != "" {
			m.pendingRecommendationMode[userID] = trigger
		}
		return
	}
	if waitForIdle && atomic.LoadInt64(&m.activeAsyncWork) > 0 {
		m.pendingRecommendationDate[userID] = recommendationDate
		m.pendingRecommendationWait[userID] = true
		m.pendingRecommendationForce[userID] = m.pendingRecommendationForce[userID] || force
		m.pendingRecommendationMode[userID] = trigger
		m.setRecommendationTaskStatusLocked(userID, func(status *DailyRecommendationTaskStatus) {
			status.HasTask = true
			status.RecommendationDate = recommendationDate
			status.Trigger = trigger
			status.Stage = recommendationStageWaiting
			status.IsQueued = true
			status.IsRunning = false
			status.IsWaitingForIdle = true
			status.Force = force
			status.ProgressPercent = 5
			status.CandidateCount = 0
			status.SelectedCount = 0
			status.SavedCount = 0
			status.StartedAt = ""
			status.LastErrorMessage = ""
			status.LastErrorAt = ""
		})
		return
	}
	m.recommendationRunning[userID] = true
	m.setRecommendationTaskStatusLocked(userID, func(status *DailyRecommendationTaskStatus) {
		status.HasTask = true
		status.RecommendationDate = recommendationDate
		status.Trigger = trigger
		status.Stage = recommendationStageQueued
		status.IsQueued = false
		status.IsRunning = true
		status.IsWaitingForIdle = false
		status.Force = force
		status.ProgressPercent = 10
		status.CandidateCount = 0
		status.SelectedCount = 0
		status.SavedCount = 0
		status.LastErrorMessage = ""
		status.LastErrorAt = ""
	})
	go m.runRecommendationLoop(userID, recommendationDate, force, trigger)
}

func (m *AIEnhancedManager) onAsyncWorkDrained() {
	m.recommendationMu.Lock()
	pending := make([]struct {
		userID int64
		date   string
		force  bool
		mode   string
	}, 0)
	for userID, recommendationDate := range m.pendingRecommendationDate {
		if !m.pendingRecommendationWait[userID] || m.recommendationRunning[userID] {
			continue
		}
		pending = append(pending, struct {
			userID int64
			date   string
			force  bool
			mode   string
		}{
			userID: userID,
			date:   recommendationDate,
			force:  m.pendingRecommendationForce[userID],
			mode:   m.pendingRecommendationMode[userID],
		})
		delete(m.pendingRecommendationDate, userID)
		delete(m.pendingRecommendationWait, userID)
		delete(m.pendingRecommendationForce, userID)
		delete(m.pendingRecommendationMode, userID)
		m.recommendationRunning[userID] = true
		m.setRecommendationTaskStatusLocked(userID, func(status *DailyRecommendationTaskStatus) {
			status.HasTask = true
			status.RecommendationDate = recommendationDate
			status.Trigger = pending[len(pending)-1].mode
			status.Stage = recommendationStageQueued
			status.IsQueued = false
			status.IsRunning = true
			status.IsWaitingForIdle = false
			status.Force = pending[len(pending)-1].force
			status.ProgressPercent = 10
			status.CandidateCount = 0
			status.SelectedCount = 0
			status.SavedCount = 0
			status.StartedAt = ""
			status.LastErrorMessage = ""
			status.LastErrorAt = ""
		})
	}
	m.recommendationMu.Unlock()

	for _, item := range pending {
		go m.runRecommendationLoop(item.userID, item.date, item.force, item.mode)
	}
}

func (m *AIEnhancedManager) runRecommendationLoop(userID int64, recommendationDate string, force bool, trigger string) {
	for {
		m.updateRecommendationTaskStatus(userID, func(status *DailyRecommendationTaskStatus) {
			status.HasTask = true
			status.RecommendationDate = recommendationDate
			status.Trigger = trigger
			status.Stage = recommendationStagePreparing
			status.IsQueued = false
			status.IsRunning = true
			status.IsWaitingForIdle = false
			status.Force = force
			if status.ProgressPercent < 10 {
				status.ProgressPercent = 10
			}
			if status.StartedAt == "" {
				status.StartedAt = time.Now().Format(time.RFC3339)
			}
			status.LastErrorMessage = ""
			status.LastErrorAt = ""
		})

		runErr := m.generateDailyRecommendations(userID, recommendationDate, force)
		if runErr != nil {
			m.recordTaskFailure(userID, "recommendation", nil, "", "", runErr)
			log.Printf("generate daily recommendations for user %d on %s error: %v", userID, recommendationDate, runErr)
		}

		m.recommendationMu.Lock()
		nextDate, hasNext := m.pendingRecommendationDate[userID]
		waitForIdle := m.pendingRecommendationWait[userID]
		nextForce := m.pendingRecommendationForce[userID]
		nextMode := m.pendingRecommendationMode[userID]
		if hasNext {
			if waitForIdle && atomic.LoadInt64(&m.activeAsyncWork) > 0 {
				m.recommendationRunning[userID] = false
				m.setRecommendationTaskStatusLocked(userID, func(status *DailyRecommendationTaskStatus) {
					status.HasTask = true
					status.RecommendationDate = nextDate
					status.Trigger = nextMode
					status.Stage = recommendationStageWaiting
					status.IsQueued = true
					status.IsRunning = false
					status.IsWaitingForIdle = true
					status.Force = nextForce
					status.ProgressPercent = 5
					status.CandidateCount = 0
					status.SelectedCount = 0
					status.SavedCount = 0
					status.StartedAt = ""
					if runErr != nil {
						status.LastErrorMessage = recommendationTaskErrorMessage(runErr)
						status.LastErrorAt = time.Now().Format(time.RFC3339)
					}
				})
				m.recommendationMu.Unlock()
				return
			}
			delete(m.pendingRecommendationDate, userID)
			delete(m.pendingRecommendationWait, userID)
			delete(m.pendingRecommendationForce, userID)
			delete(m.pendingRecommendationMode, userID)
			recommendationDate = nextDate
			force = nextForce
			trigger = nextMode
			m.recommendationMu.Unlock()
			continue
		}
		delete(m.recommendationRunning, userID)
		if runErr != nil {
			m.setRecommendationTaskStatusLocked(userID, func(status *DailyRecommendationTaskStatus) {
				status.HasTask = false
				status.Stage = recommendationStageFailed
				status.IsQueued = false
				status.IsRunning = false
				status.IsWaitingForIdle = false
				status.ProgressPercent = 0
				status.LastErrorMessage = recommendationTaskErrorMessage(runErr)
				status.LastErrorAt = time.Now().Format(time.RFC3339)
			})
		} else {
			delete(m.recommendationStatusByUser, userID)
		}
		m.recommendationMu.Unlock()
		return
	}
}

func (m *AIEnhancedManager) generateDailyRecommendations(userID int64, recommendationDate string, force bool) error {
	if !ShouldProcess(m.db, userID) {
		return nil
	}
	health, allowed, err := m.guardEmbeddingHealth(userID, blockedScopeDailyRecommendationRun)
	if err != nil {
		return fmt.Errorf("embedding health gate: %w", err)
	}
	if !allowed {
		log.Printf(
			"Skipping recommendation generation for user %d due to unhealthy summary embeddings (sample=%d, unnormalized=%d, ratio=%.2f)",
			userID,
			health.SampleSize,
			health.UnnormalizedCount,
			health.UnnormalizedRatio,
		)
		return nil
	}
	if recommendationDate == "" {
		recommendationDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}
	if !force {
		hasRecommendations, err := m.db.HasDailyRecommendations(userID, recommendationDate)
		if err != nil {
			return err
		}
		if hasRecommendations {
			return nil
		}
	}

	dayStart, err := time.ParseInLocation("2006-01-02", recommendationDate, time.Local)
	if err != nil {
		return fmt.Errorf("parse recommendation date: %w", err)
	}
	dayEnd := dayStart.Add(24 * time.Hour)

	m.updateRecommendationTaskStatus(userID, func(status *DailyRecommendationTaskStatus) {
		status.Stage = recommendationStageRecalling
		status.ProgressPercent = 20
		status.CandidateCount = 0
		status.SelectedCount = 0
		status.SavedCount = 0
	})
	candidates, err := m.recallRecommendationCandidates(userID, recommendationDate, dayStart, dayEnd)
	if err != nil {
		return err
	}
	m.updateRecommendationTaskStatus(userID, func(status *DailyRecommendationTaskStatus) {
		status.Stage = recommendationStageRanking
		status.ProgressPercent = 35
		status.CandidateCount = len(candidates)
	})
	if len(candidates) == 0 {
		m.updateRecommendationTaskStatus(userID, func(status *DailyRecommendationTaskStatus) {
			status.Stage = recommendationStageSaving
			status.ProgressPercent = 92
			status.SelectedCount = 0
			status.SavedCount = 0
		})
		return m.db.SaveDailyRecommendations(userID, recommendationDate, nil)
	}

	config, profileID, hasAIConfig, err := m.getRecommendationAIConfig(userID)
	if err != nil {
		return err
	}

	m.updateRecommendationTaskStatus(userID, func(status *DailyRecommendationTaskStatus) {
		status.Stage = recommendationStageRanking
		status.ProgressPercent = 55
	})
	ranked := []rankedRecommendationCandidate(nil)
	if len(candidates) <= recommendationDirectChronologicalMaxCandidates {
		ranked = rankRecommendationCandidatesChronological(candidates, profileID)
	} else {
		ranked = m.rankRecommendationCandidates(userID, recommendationDate, candidates, config, profileID, hasAIConfig)
	}
	m.updateRecommendationTaskStatus(userID, func(status *DailyRecommendationTaskStatus) {
		status.Stage = recommendationStageSaving
		status.ProgressPercent = 92
		status.SelectedCount = minInt(len(ranked), 10)
	})
	recommendations := make([]models.DailyRecommendation, 0, minInt(len(ranked), 10))
	for idx, item := range ranked {
		if idx >= 10 {
			break
		}
		recommendations = append(recommendations, models.DailyRecommendation{
			UserID:                  userID,
			ClusterID:               item.Candidate.Cluster.ID,
			RecommendationDate:      recommendationDate,
			RecommendationScore:     item.FinalScore,
			RecommendationRank:      idx + 1,
			RecommendationProfileID: profileID,
		})
	}
	m.updateRecommendationTaskStatus(userID, func(status *DailyRecommendationTaskStatus) {
		status.SavedCount = len(recommendations)
	})

	return m.db.SaveDailyRecommendations(userID, recommendationDate, recommendations)
}

func rankRecommendationCandidatesChronological(candidates []rankedRecommendationCandidate, profileID int64) []rankedRecommendationCandidate {
	results := make([]rankedRecommendationCandidate, len(candidates))
	copy(results, candidates)
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Candidate.PublishedAt.Equal(results[j].Candidate.PublishedAt) {
			if results[i].RecallScore == results[j].RecallScore {
				return results[i].Candidate.Cluster.ID < results[j].Candidate.Cluster.ID
			}
			return results[i].RecallScore > results[j].RecallScore
		}
		return results[i].Candidate.PublishedAt.After(results[j].Candidate.PublishedAt)
	})
	for i := range results {
		results[i].ProfileID = profileID
		results[i].FinalScore = float64(len(results) - i)
	}
	return results
}

func (m *AIEnhancedManager) recallRecommendationCandidates(userID int64, recommendationDate string, dayStart, dayEnd time.Time) ([]rankedRecommendationCandidate, error) {
	excludeIDs, err := m.db.ListAIRecommendedClusterIDsExcludingDate(userID, recommendationDate)
	if err != nil {
		return nil, err
	}

	interestVecBlob, err := m.db.GetUserInterestVector(userID)
	if err == nil && len(interestVecBlob) > 0 {
		vectorCandidates, err := m.db.GetDailyRecommendationCandidatesByVector(userID, dayStart, dayEnd, interestVecBlob, excludeIDs, 100)
		if err == nil && len(vectorCandidates) > 0 {
			return normalizeRecallScores(vectorCandidates), nil
		}
	}

	candidates, err := m.db.GetDailyRecommendationCandidatesChronological(userID, dayStart, dayEnd, excludeIDs, 100)
	if err != nil {
		return nil, err
	}
	return normalizeRecallScores(candidates), nil
}

func normalizeRecallScores(candidates []sqlite.DailyRecommendationCandidate) []rankedRecommendationCandidate {
	results := make([]rankedRecommendationCandidate, 0, len(candidates))
	for idx, candidate := range candidates {
		recallScore := float64(len(candidates)-idx) / float64(maxInt(len(candidates), 1))
		if candidate.Distance > 0 {
			recallScore = 1 / (1 + candidate.Distance)
		}
		results = append(results, rankedRecommendationCandidate{
			Candidate:   candidate,
			RecallScore: recallScore,
			FinalScore:  recallScore,
		})
	}
	return results
}

func (m *AIEnhancedManager) getRecommendationAIConfig(userID int64) (*ai.ClientConfig, int64, bool, error) {
	config, err := getUserFeatureAIConfig(m.db, userID, ai.FeatureRecommendation)
	if err != nil {
		return nil, 0, false, err
	}
	if !hasConfiguredAPIKey(config) {
		return nil, 0, false, nil
	}
	profileProvider := ai.NewProfileProvider(m.db)
	profile, err := profileProvider.GetProfileForFeatureForUser(userID, ai.FeatureRecommendation)
	if err != nil {
		return config, 0, true, nil
	}
	if profile != nil {
		return config, profile.ID, true, nil
	}
	return config, 0, true, nil
}

func (m *AIEnhancedManager) rankRecommendationCandidates(userID int64, recommendationDate string, candidates []rankedRecommendationCandidate, config *ai.ClientConfig, profileID int64, hasAIConfig bool) []rankedRecommendationCandidate {
	if len(candidates) == 0 {
		return nil
	}
	if !hasAIConfig {
		return m.rankRecommendationCandidatesRuleBased(candidates)
	}

	if len(candidates) < recommendationStageOneMinCandidates {
		m.updateRecommendationTaskStatus(userID, func(status *DailyRecommendationTaskStatus) {
			status.Stage = recommendationStageScoring
			status.ProgressPercent = 70
			status.SelectedCount = len(candidates)
		})

		ranked, ok := m.runRecommendationStageTwo(userID, candidates, config, profileID)
		if !ok || len(ranked) == 0 {
			return m.rankRecommendationCandidatesRuleBased(candidates)
		}

		sort.SliceStable(ranked, func(i, j int) bool {
			if ranked[i].FinalScore == ranked[j].FinalScore {
				return ranked[i].Candidate.PublishedAt.After(ranked[j].Candidate.PublishedAt)
			}
			return ranked[i].FinalScore > ranked[j].FinalScore
		})
		return ranked
	}

	stageOneSelected, ok := m.runRecommendationStageOne(userID, recommendationDate, candidates, config, profileID)
	if !ok || len(stageOneSelected) == 0 {
		return m.rankRecommendationCandidatesRuleBased(candidates)
	}
	m.updateRecommendationTaskStatus(userID, func(status *DailyRecommendationTaskStatus) {
		status.Stage = recommendationStageScoring
		status.ProgressPercent = 70
		status.SelectedCount = len(stageOneSelected)
	})

	ranked, ok := m.runRecommendationStageTwo(userID, stageOneSelected, config, profileID)
	if !ok || len(ranked) == 0 {
		return m.rankRecommendationCandidatesRuleBased(candidates)
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].FinalScore == ranked[j].FinalScore {
			return ranked[i].Candidate.PublishedAt.After(ranked[j].Candidate.PublishedAt)
		}
		return ranked[i].FinalScore > ranked[j].FinalScore
	})
	return ranked
}

func (m *AIEnhancedManager) rankRecommendationCandidatesRuleBased(candidates []rankedRecommendationCandidate) []rankedRecommendationCandidate {
	results := make([]rankedRecommendationCandidate, len(candidates))
	copy(results, candidates)
	for i := range results {
		freshness := 0.0
		if !results[i].Candidate.PublishedAt.IsZero() {
			hours := time.Since(results[i].Candidate.PublishedAt).Hours()
			freshness = math.Max(0, 1-(hours/(24*7)))
		}
		results[i].FinalScore = results[i].RecallScore*0.7 + freshness*0.3
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].FinalScore == results[j].FinalScore {
			return results[i].Candidate.PublishedAt.After(results[j].Candidate.PublishedAt)
		}
		return results[i].FinalScore > results[j].FinalScore
	})
	return results
}

func (m *AIEnhancedManager) runRecommendationStageOne(userID int64, recommendationDate string, candidates []rankedRecommendationCandidate, config *ai.ClientConfig, profileID int64) ([]rankedRecommendationCandidate, bool) {
	shuffled := make([]rankedRecommendationCandidate, len(candidates))
	copy(shuffled, candidates)
	rng := rand.New(rand.NewSource(int64(userID) + int64(len(recommendationDate))*97))
	rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	groupSize := 10
	selected := make([]rankedRecommendationCandidate, 0, minInt(len(shuffled), 30))
	for start := 0; start < len(shuffled); start += groupSize {
		end := start + groupSize
		if end > len(shuffled) {
			end = len(shuffled)
		}
		group := shuffled[start:end]
		if len(group) <= 3 {
			for i := range group {
				group[i].StageOneScore = group[i].RecallScore
				group[i].FinalScore = group[i].RecallScore
				group[i].ProfileID = profileID
			}
			selected = append(selected, group...)
			continue
		}
		groupSelected, ok := m.pickRecommendationTopThree(userID, group, config)
		if !ok {
			return nil, false
		}
		for i := range groupSelected {
			groupSelected[i].ProfileID = profileID
		}
		selected = append(selected, groupSelected...)
	}
	return selected, true
}

func (m *AIEnhancedManager) pickRecommendationTopThree(userID int64, group []rankedRecommendationCandidate, config *ai.ClientConfig) ([]rankedRecommendationCandidate, bool) {
	order1 := make([]int, len(group))
	for i := range group {
		order1[i] = i
	}
	order2 := append([]int(nil), order1...)
	if len(order2) > 1 {
		order2 = append(order2[1:], order2[0])
	}

	firstPick, firstUnion, ok := m.runRecommendationStageOneRound(userID, group, order1, config)
	if !ok {
		return nil, false
	}
	secondPick, secondUnion, ok := m.runRecommendationStageOneRound(userID, group, order2, config)
	if !ok {
		return nil, false
	}

	if sameSelection(firstPick, secondPick) {
		return firstPick, true
	}

	merged := make([]rankedRecommendationCandidate, 0, len(firstUnion)+len(secondUnion))
	seen := make(map[int64]bool)
	for _, item := range append(firstUnion, secondUnion...) {
		if seen[item.Candidate.Cluster.ID] {
			continue
		}
		seen[item.Candidate.Cluster.ID] = true
		merged = append(merged, item)
	}
	if len(merged) <= 3 {
		return merged, true
	}
	order3 := make([]int, len(merged))
	for i := range merged {
		order3[i] = i
	}
	finalPick, _, ok := m.runRecommendationStageOneRound(userID, merged, order3, config)
	if !ok {
		return nil, false
	}
	return finalPick, true
}

func (m *AIEnhancedManager) runRecommendationStageOneRound(userID int64, group []rankedRecommendationCandidate, order []int, config *ai.ClientConfig) ([]rankedRecommendationCandidate, []rankedRecommendationCandidate, bool) {
	letters := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"}
	var prompt strings.Builder
	prompt.WriteString("你是 RSS 每日推荐助手。请根据信息密度、实用价值、有趣程度、时效性四个维度，从候选摘要中选出最值得推荐的 3 个选项。")
	prompt.WriteString(" 只返回 JSON，格式为 {\"analysis\":\"...\",\"answer\":[\"A\",\"B\",\"C\"]}。answer 必须恰好包含 3 个不同选项。\n")
	for idx, originalIndex := range order {
		candidate := group[originalIndex]
		prompt.WriteString(letters[idx])
		prompt.WriteString(": 标题=")
		prompt.WriteString(strings.TrimSpace(candidate.Candidate.Cluster.MergedTitle))
		prompt.WriteString("；摘要=")
		prompt.WriteString(strings.TrimSpace(candidate.Candidate.Cluster.MergedSummary))
		prompt.WriteString("\n")
	}

	responseText, ok := m.requestRecommendationJSON(userID, config, prompt.String())
	if !ok {
		return nil, nil, false
	}

	var response stageOneResponse
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		return nil, nil, false
	}
	if len(response.Answer) != 3 {
		return nil, nil, false
	}

	mapped := make([]rankedRecommendationCandidate, 0, 3)
	union := make([]rankedRecommendationCandidate, 0, 3)
	seen := make(map[int64]bool)
	for _, answer := range response.Answer {
		answer = strings.TrimSpace(strings.ToUpper(answer))
		idx := indexOfString(letters, answer)
		if idx < 0 || idx >= len(order) {
			return nil, nil, false
		}
		item := group[order[idx]]
		item.StageOneScore = item.RecallScore
		item.FinalScore = item.RecallScore
		item.StageOneReason = response.Analysis
		mapped = append(mapped, item)
		if !seen[item.Candidate.Cluster.ID] {
			seen[item.Candidate.Cluster.ID] = true
			union = append(union, item)
		}
	}
	return mapped, union, true
}

func (m *AIEnhancedManager) runRecommendationStageTwo(userID int64, candidates []rankedRecommendationCandidate, config *ai.ClientConfig, profileID int64) ([]rankedRecommendationCandidate, bool) {
	results := make([]rankedRecommendationCandidate, 0, len(candidates))
	total := maxInt(len(candidates), 1)
	for idx, candidate := range candidates {
		scored, ok := m.scoreRecommendationCandidate(userID, candidate, config, profileID)
		if !ok {
			return nil, false
		}
		results = append(results, scored)
		progress := 70 + (20 * float64(idx+1) / float64(total))
		m.updateRecommendationTaskStatus(userID, func(status *DailyRecommendationTaskStatus) {
			status.Stage = recommendationStageScoring
			status.ProgressPercent = progress
			status.SelectedCount = len(candidates)
		})
	}
	return results, true
}

func (m *AIEnhancedManager) scoreRecommendationCandidate(userID int64, candidate rankedRecommendationCandidate, config *ai.ClientConfig, profileID int64) (rankedRecommendationCandidate, bool) {
	content := strings.TrimSpace(candidate.Candidate.Cluster.MergedContent)
	if content == "" {
		content = strings.TrimSpace(candidate.Candidate.Cluster.MergedSummary)
	}
	if content == "" {
		candidate.FinalScore = candidate.RecallScore
		candidate.ProfileID = profileID
		return candidate, true
	}

	scorePrompt := fmt.Sprintf("你是 RSS 每日推荐评分助手。请阅读文章内容，并按 0 到 5 分给出 information_density、practical_value、interestingness、timeliness 四个分数。只返回 JSON，格式为 {\"analysis\":\"...\",\"information_density\":1,\"practical_value\":1,\"interestingness\":1,\"timeliness\":1}。\n标题：%s\n内容：%s", strings.TrimSpace(candidate.Candidate.Cluster.MergedTitle), content)
	scoreText, ok := m.requestRecommendationJSON(userID, config, scorePrompt)
	if !ok {
		return rankedRecommendationCandidate{}, false
	}
	var scoreResponse stageTwoScoreResponse
	if err := json.Unmarshal([]byte(scoreText), &scoreResponse); err != nil {
		return rankedRecommendationCandidate{}, false
	}
	if !validScoreResponse(scoreResponse.InformationDensity, scoreResponse.PracticalValue, scoreResponse.Interestingness, scoreResponse.Timeliness) {
		return rankedRecommendationCandidate{}, false
	}

	reviewPrompt := fmt.Sprintf("你是 RSS 每日推荐评分复核助手。请根据文章全文与首轮分析判断评分是否合理。若合理，只返回 JSON：{\"analysis\":\"...\",\"is_reasonable\":true}；若不合理，返回 JSON：{\"analysis\":\"...\",\"is_reasonable\":false,\"information_density\":1,\"practical_value\":1,\"interestingness\":1,\"timeliness\":1}。\n标题：%s\n内容：%s\n首轮分析：%s", strings.TrimSpace(candidate.Candidate.Cluster.MergedTitle), content, scoreResponse.Analysis)
	reviewText, ok := m.requestRecommendationJSON(userID, config, reviewPrompt)
	if !ok {
		return rankedRecommendationCandidate{}, false
	}
	var reviewResponse stageTwoReviewResponse
	if err := json.Unmarshal([]byte(reviewText), &reviewResponse); err != nil {
		return rankedRecommendationCandidate{}, false
	}

	informationDensity := scoreResponse.InformationDensity
	practicalValue := scoreResponse.PracticalValue
	interestingness := scoreResponse.Interestingness
	timeliness := scoreResponse.Timeliness
	if !reviewResponse.IsReasonable {
		if !validScoreResponse(reviewResponse.InformationDensity, reviewResponse.PracticalValue, reviewResponse.Interestingness, reviewResponse.Timeliness) {
			return rankedRecommendationCandidate{}, false
		}
		informationDensity = (informationDensity + reviewResponse.InformationDensity) / 2
		practicalValue = (practicalValue + reviewResponse.PracticalValue) / 2
		interestingness = (interestingness + reviewResponse.Interestingness) / 2
		timeliness = (timeliness + reviewResponse.Timeliness) / 2
	}

	stageTwoScore := informationDensity + practicalValue + interestingness + timeliness
	candidate.StageTwoScore = stageTwoScore
	candidate.FinalScore = stageTwoScore + candidate.RecallScore
	candidate.StageTwoReason = scoreResponse.Analysis
	candidate.ProfileID = profileID

	inputTokens := ai.EstimateTokens(content) + ai.EstimateTokens(scorePrompt) + ai.EstimateTokens(reviewPrompt)
	outputTokens := ai.EstimateTokens(scoreText) + ai.EstimateTokens(reviewText)
	m.addAIUsage(userID, inputTokens+outputTokens)

	return candidate, true
}

func (m *AIEnhancedManager) requestRecommendationJSON(userID int64, config *ai.ClientConfig, prompt string) (string, bool) {
	client := ai.NewClient(ai.ClientConfig{
		APIKey:        config.APIKey,
		Endpoint:      config.Endpoint,
		Model:         config.Model,
		CustomHeaders: config.CustomHeaders,
		ProxyURL:      config.ProxyURL,
		Timeout:       90 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	result, err := client.RequestWithConfig(ai.RequestConfig{
		Model:       config.Model,
		Messages:    []map[string]string{{"role": "user", "content": prompt}},
		Temperature: 0.2,
		MaxTokens:   2048,
		Context:     ctx,
	})
	if err != nil {
		m.recordTaskFailure(userID, "recommendation", nil, config.Model, config.Endpoint, err)
		log.Printf("recommendation ai request error: %v", err)
		return "", false
	}
	content := strings.TrimSpace(result.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	return content, content != ""
}

func (m *AIEnhancedManager) GetDailyRecommendationTaskStatus(userID int64) DailyRecommendationTaskStatus {
	status := DailyRecommendationTaskStatus{}
	if userID <= 0 {
		return status
	}

	recommendationEnabled, _ := m.db.GetSettingWithFallback(userID, "ai_recommendation_enabled")
	status.IsEnabled = recommendationEnabled == "true"

	m.recommendationMu.Lock()
	if existing, ok := m.recommendationStatusByUser[userID]; ok {
		status = existing
		status.IsEnabled = recommendationEnabled == "true"
	} else if m.recommendationRunning[userID] {
		status.HasTask = true
		status.IsRunning = true
		status.Stage = recommendationStagePreparing
		status.ProgressPercent = 10
		status.IsEnabled = recommendationEnabled == "true"
	} else if date := m.pendingRecommendationDate[userID]; date != "" {
		status.HasTask = true
		status.RecommendationDate = date
		status.Trigger = m.pendingRecommendationMode[userID]
		status.Stage = recommendationStageWaiting
		status.IsQueued = true
		status.IsWaitingForIdle = m.pendingRecommendationWait[userID]
		status.Force = m.pendingRecommendationForce[userID]
		status.ProgressPercent = 5
		status.IsEnabled = recommendationEnabled == "true"
	}
	m.recommendationMu.Unlock()

	if status.ProgressPercent < 0 {
		status.ProgressPercent = 0
	}
	if status.ProgressPercent > 100 {
		status.ProgressPercent = 100
	}
	return status
}

func (m *AIEnhancedManager) updateRecommendationTaskStatus(userID int64, update func(*DailyRecommendationTaskStatus)) {
	m.recommendationMu.Lock()
	defer m.recommendationMu.Unlock()
	m.setRecommendationTaskStatusLocked(userID, update)
}

func (m *AIEnhancedManager) setRecommendationTaskStatusLocked(userID int64, update func(*DailyRecommendationTaskStatus)) {
	if userID <= 0 || update == nil {
		return
	}
	if m.recommendationStatusByUser == nil {
		m.recommendationStatusByUser = make(map[int64]DailyRecommendationTaskStatus)
	}

	status := m.recommendationStatusByUser[userID]
	update(&status)
	status.UpdatedAt = time.Now().Format(time.RFC3339)
	if status.HasTask && status.IsRunning && status.StartedAt == "" {
		status.StartedAt = status.UpdatedAt
	}
	if !status.HasTask {
		status.IsQueued = false
		status.IsRunning = false
		status.IsWaitingForIdle = false
	}
	if status.ProgressPercent < 0 {
		status.ProgressPercent = 0
	}
	if status.ProgressPercent > 100 {
		status.ProgressPercent = 100
	}
	m.recommendationStatusByUser[userID] = status
}

func recommendationTaskErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(ai.DiagnosticMessage(err))
	if message == "" {
		message = strings.TrimSpace(err.Error())
	}
	if len(message) > 600 {
		message = message[:600]
	}
	return message
}

func validScoreResponse(scores ...float64) bool {
	for _, score := range scores {
		if score < 0 || score > 5 {
			return false
		}
	}
	return true
}

func sameSelection(a, b []rankedRecommendationCandidate) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Candidate.Cluster.ID != b[i].Candidate.Cluster.ID {
			return false
		}
	}
	return true
}

func indexOfString(items []string, target string) int {
	for idx, item := range items {
		if item == target {
			return idx
		}
	}
	return -1
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
