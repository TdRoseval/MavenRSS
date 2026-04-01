package feed

import (
	"testing"
	"time"

	"MavenRSS/internal/models"
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
	if err := db.SetSettingForUser(userID, "update_interval", "30"); err != nil {
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
