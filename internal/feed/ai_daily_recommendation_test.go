package feed

import (
	"testing"
	"time"

	"MavenRSS/internal/ai"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
)

func TestComputeDailyRecommendationRunTime(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := NewAIEnhancedManager(db)
	defer manager.Stop()

	userID := int64(999)
	if _, err := db.Exec(`INSERT INTO users (id, username, password_hash, email, created_at, updated_at) VALUES (?, 'u999', 'hash', 'u999@test.com', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := db.SetSetting("last_global_refresh", "2026-03-30T23:45:00Z"); err != nil {
		t.Fatalf("set last_global_refresh: %v", err)
	}
	if err := db.SetSettingForUser(userID, "update_interval", "120"); err != nil {
		t.Fatalf("set update_interval: %v", err)
	}

	now := time.Date(2026, 3, 31, 0, 31, 0, 0, time.UTC)
	runAt, ok := manager.computeDailyRecommendationRunTime(userID, now)
	if !ok {
		t.Fatalf("expected schedule time")
	}
	expected := time.Date(2026, 3, 31, 0, 15, 0, 0, time.UTC)
	if !runAt.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, runAt)
	}
}

func TestSaveDailyRecommendationsUpdatesClusterFlags(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	userID := int64(1)
	clusterID, err := db.CreateCluster(userID, "complete")
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	recommendations := []models.DailyRecommendation{{
		UserID:                  userID,
		ClusterID:               clusterID,
		RecommendationDate:      "2026-03-30",
		RecommendationScore:     9.5,
		RecommendationRank:      1,
		RecommendationProfileID: 3,
	}}
	if err := db.SaveDailyRecommendations(userID, "2026-03-30", recommendations); err != nil {
		t.Fatalf("save daily recommendations: %v", err)
	}

	cluster, err := db.GetClusterByID(clusterID)
	if err != nil {
		t.Fatalf("get cluster: %v", err)
	}
	if cluster == nil || !cluster.IsAIRecommended {
		t.Fatalf("expected cluster marked as ai recommended")
	}
	if cluster.RecommendationArchiveDate != "2026-03-30" {
		t.Fatalf("expected archive date saved, got %q", cluster.RecommendationArchiveDate)
	}
}

func TestShouldQueueDailyRecommendationsForIncompleteResults(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := NewAIEnhancedManager(db)
	defer manager.Stop()

	userID := int64(999)
	if _, err := db.Exec(`INSERT INTO users (id, username, password_hash, email, created_at, updated_at) VALUES (?, 'u999', 'hash', 'u999@test.com', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	feedID := mustCreateTestFeed(t, db, userID, false)
	recommendationDate := "2026-03-31"
	publishedAt := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)

	clusterIDs := make([]int64, 0, 3)
	for i := 0; i < 3; i++ {
		clusterID, err := db.CreateCluster(userID, "complete")
		if err != nil {
			t.Fatalf("create cluster %d: %v", i, err)
		}
		clusterIDs = append(clusterIDs, clusterID)
		if _, err := db.Exec(
			`INSERT INTO articles (user_id, feed_id, title, url, published_at, unique_id, cluster_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			userID,
			feedID,
			"article",
			"https://example.com/article/"+time.Now().Format("150405.000000"),
			publishedAt.Add(time.Duration(i)*time.Minute),
			time.Now().Format("150405.000000")+string(rune('a'+i)),
			clusterID,
		); err != nil {
			t.Fatalf("insert article %d: %v", i, err)
		}
	}

	if err := db.SaveDailyRecommendations(userID, recommendationDate, []models.DailyRecommendation{{
		UserID:              userID,
		ClusterID:           clusterIDs[0],
		RecommendationDate:  recommendationDate,
		RecommendationRank:  1,
		RecommendationScore: 9,
	}}); err != nil {
		t.Fatalf("save initial recommendations: %v", err)
	}

	shouldQueue, forceRegenerate, err := manager.shouldQueueDailyRecommendations(userID, recommendationDate)
	if err != nil {
		t.Fatalf("shouldQueueDailyRecommendations error: %v", err)
	}
	if !shouldQueue {
		t.Fatalf("expected incomplete recommendations to be queued for regeneration")
	}
	if !forceRegenerate {
		t.Fatalf("expected incomplete recommendations to force regeneration")
	}
}

func TestCleanupExpiredClustersKeepsRecommendedClusters(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	userID := int64(1)
	clusterID, err := db.CreateCluster(userID, "complete")
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if _, err := db.Exec(`UPDATE clusters SET is_ai_recommended = 1, updated_at = ? WHERE id = ?`, time.Now().AddDate(0, 0, -60), clusterID); err != nil {
		t.Fatalf("mark cluster recommended: %v", err)
	}

	deleted, err := db.CleanupExpiredClusters(userID, 30)
	if err != nil {
		t.Fatalf("cleanup expired clusters: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected no cluster deleted, got %d", deleted)
	}
	cluster, err := db.GetClusterByID(clusterID)
	if err != nil {
		t.Fatalf("get cluster: %v", err)
	}
	if cluster == nil {
		t.Fatalf("expected cluster to remain")
	}
}

func TestForceDailyRecommendationsQueuesManualRefreshEvenWhenResultsExist(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := NewAIEnhancedManager(db)
	defer manager.Stop()

	userID := int64(999)
	if _, err := db.Exec(`INSERT INTO users (id, username, password_hash, email, created_at, updated_at) VALUES (?, 'u999', 'hash', 'u999@test.com', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := db.SetSettingForUser(userID, "ai_recommendation_enabled", "true"); err != nil {
		t.Fatalf("enable recommendation: %v", err)
	}

	recommendationDate := "2026-03-31"
	recommendations := make([]models.DailyRecommendation, 0, 10)
	for i := 0; i < 10; i++ {
		clusterID, err := db.CreateCluster(userID, "complete")
		if err != nil {
			t.Fatalf("create cluster %d: %v", i, err)
		}
		recommendations = append(recommendations, models.DailyRecommendation{
			UserID:              userID,
			ClusterID:           clusterID,
			RecommendationDate:  recommendationDate,
			RecommendationRank:  i + 1,
			RecommendationScore: float64(10 - i),
		})
	}
	if err := db.SaveDailyRecommendations(userID, recommendationDate, recommendations); err != nil {
		t.Fatalf("save recommendations: %v", err)
	}

	manager.incrementActiveAsyncWork(userID)
	status, err := manager.ForceDailyRecommendations(userID, recommendationDate, true)
	if err != nil {
		t.Fatalf("ForceDailyRecommendations error: %v", err)
	}
	if !status.HasTask {
		t.Fatal("expected manual refresh task to be queued")
	}
	if status.Trigger != recommendationTriggerManual {
		t.Fatalf("expected manual trigger, got %q", status.Trigger)
	}
	if !status.IsWaitingForIdle {
		t.Fatal("expected manual refresh to wait for idle background work")
	}
	if !status.Force {
		t.Fatal("expected manual refresh to force regeneration")
	}
}

func TestRankRecommendationCandidatesSkipsTournamentWhenRecallBelowThreshold(t *testing.T) {
	manager := &AIEnhancedManager{}
	profileID := int64(7)
	candidates := make([]rankedRecommendationCandidate, 0, recommendationStageOneMinCandidates-1)
	now := time.Now()

	for i := 0; i < recommendationStageOneMinCandidates-1; i++ {
		candidates = append(candidates, rankedRecommendationCandidate{
			Candidate: sqlite.DailyRecommendationCandidate{
				Cluster: models.Cluster{
					ID:          int64(i + 1),
					MergedTitle: "candidate",
				},
				PublishedAt: now.Add(-time.Duration(i) * time.Minute),
			},
			RecallScore: float64(len(candidates) + 1),
			FinalScore:  float64(len(candidates) + 1),
		})
	}

	ranked := manager.rankRecommendationCandidates(1, "2026-03-31", candidates, &ai.ClientConfig{}, profileID, true)
	if len(ranked) != len(candidates) {
		t.Fatalf("expected %d ranked candidates, got %d", len(candidates), len(ranked))
	}

	for i, candidate := range ranked {
		if candidate.ProfileID != profileID {
			t.Fatalf("candidate %d profile id = %d, want %d", i, candidate.ProfileID, profileID)
		}
		if candidate.StageTwoScore != 0 {
			t.Fatalf("candidate %d stage two score = %f, want 0 for empty content fallback", i, candidate.StageTwoScore)
		}
	}

	status := manager.recommendationStatusByUser[1]
	if status.Stage != recommendationStageScoring {
		t.Fatalf("status stage = %q, want %q", status.Stage, recommendationStageScoring)
	}
	if status.SelectedCount != len(candidates) {
		t.Fatalf("status selected count = %d, want %d", status.SelectedCount, len(candidates))
	}
}

func TestRankRecommendationCandidatesChronologicalUsesPublishedAtDescending(t *testing.T) {
	profileID := int64(11)
	now := time.Now()
	candidates := []rankedRecommendationCandidate{
		{
			Candidate: sqlite.DailyRecommendationCandidate{
				Cluster:     models.Cluster{ID: 1},
				PublishedAt: now.Add(-3 * time.Hour),
			},
			RecallScore: 0.99,
		},
		{
			Candidate: sqlite.DailyRecommendationCandidate{
				Cluster:     models.Cluster{ID: 2},
				PublishedAt: now.Add(-1 * time.Hour),
			},
			RecallScore: 0.10,
		},
		{
			Candidate: sqlite.DailyRecommendationCandidate{
				Cluster:     models.Cluster{ID: 3},
				PublishedAt: now.Add(-2 * time.Hour),
			},
			RecallScore: 0.50,
		},
	}

	ranked := rankRecommendationCandidatesChronological(candidates, profileID)
	if len(ranked) != 3 {
		t.Fatalf("expected 3 ranked candidates, got %d", len(ranked))
	}

	gotOrder := []int64{
		ranked[0].Candidate.Cluster.ID,
		ranked[1].Candidate.Cluster.ID,
		ranked[2].Candidate.Cluster.ID,
	}
	wantOrder := []int64{2, 3, 1}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("ranked order = %v, want %v", gotOrder, wantOrder)
		}
		if ranked[i].ProfileID != profileID {
			t.Fatalf("candidate %d profile id = %d, want %d", i, ranked[i].ProfileID, profileID)
		}
	}
}

func TestRecommendationRequestTimeoutDefaultsAndOverrides(t *testing.T) {
	if got := recommendationRequestTimeout(nil); got != ai.MinimumConfigurableTimeout {
		t.Fatalf("recommendationRequestTimeout(nil) = %v, want %v", got, ai.MinimumConfigurableTimeout)
	}
	if got := recommendationRequestTimeout(&ai.ClientConfig{}); got != ai.MinimumConfigurableTimeout {
		t.Fatalf("recommendationRequestTimeout(empty) = %v, want %v", got, ai.MinimumConfigurableTimeout)
	}
	if got := recommendationRequestTimeout(&ai.ClientConfig{Timeout: 10 * time.Minute}); got != 10*time.Minute {
		t.Fatalf("recommendationRequestTimeout(10m) = %v, want 10m", got)
	}
}
