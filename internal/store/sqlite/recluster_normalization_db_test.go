package sqlite_test

import (
	"database/sql"
	"testing"
	"time"

	"MavenRSS/internal/interest"
	"MavenRSS/internal/models"
	dbpkg "MavenRSS/internal/store/sqlite"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

func TestResetAIClustersForRenormalizationClearsClusteringState(t *testing.T) {
	db := setupTestDB(t)

	userID, err := db.CreateUser(&models.User{
		Username:     "recluster-reset-user",
		Email:        "recluster-reset@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser error = %v", err)
	}

	feedID, err := db.AddFeedForUser(userID, &models.Feed{
		Title:           "Reset Feed",
		URL:             "https://example.com/reset-feed",
		Type:            "rss",
		RefreshInterval: 60,
	})
	if err != nil {
		t.Fatalf("AddFeedForUser error = %v", err)
	}

	result, err := db.Exec(
		`INSERT INTO articles (user_id, feed_id, title, url, published_at, summary, unique_id, cluster_id, simhash_64, simhash_b1, simhash_b2, simhash_b3, simhash_b4)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, feedID, "Reset article", "https://example.com/reset-article", time.Now(), "summary", "reset-article", 1, 77, 1, 2, 3, 4,
	)
	if err != nil {
		t.Fatalf("insert article error = %v", err)
	}
	articleID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId error = %v", err)
	}

	clusterID, err := db.CreateCluster(userID, "complete")
	if err != nil {
		t.Fatalf("CreateCluster error = %v", err)
	}
	if err := db.UpdateArticleClusterID(articleID, clusterID); err != nil {
		t.Fatalf("UpdateArticleClusterID error = %v", err)
	}
	if err := db.UpdateClusterEmbeddings(clusterID, mustSerializeReclusterVector(t, []float32{1}), mustSerializeReclusterVector(t, []float32{1})); err != nil {
		t.Fatalf("UpdateClusterEmbeddings error = %v", err)
	}

	if err := db.SaveDailyRecommendations(userID, "2026-04-02", []models.DailyRecommendation{{
		UserID:             userID,
		ClusterID:          clusterID,
		RecommendationDate: "2026-04-02",
		RecommendationRank: 1,
	}}); err != nil {
		t.Fatalf("SaveDailyRecommendations error = %v", err)
	}

	if err := db.UpdateUserInterestVector(userID, mustSerializeReclusterVector(t, []float32{1})); err != nil {
		t.Fatalf("UpdateUserInterestVector error = %v", err)
	}
	if err := db.SetAIArticleStageSkip(userID, articleID, "summary", "skip"); err != nil {
		t.Fatalf("SetAIArticleStageSkip error = %v", err)
	}

	if err := db.ResetAIClustersForRenormalization(userID); err != nil {
		t.Fatalf("ResetAIClustersForRenormalization error = %v", err)
	}

	var clusterCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM clusters WHERE user_id = ?`, userID).Scan(&clusterCount); err != nil {
		t.Fatalf("cluster count error = %v", err)
	}
	if clusterCount != 0 {
		t.Fatalf("clusterCount = %d, want 0", clusterCount)
	}

	var dailyRecommendationCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM daily_recommendations WHERE user_id = ?`, userID).Scan(&dailyRecommendationCount); err != nil {
		t.Fatalf("daily recommendation count error = %v", err)
	}
	if dailyRecommendationCount != 0 {
		t.Fatalf("dailyRecommendationCount = %d, want 0", dailyRecommendationCount)
	}

	var clusterIDValue sql.NullInt64
	var simhash int64
	if err := db.QueryRow(`SELECT cluster_id, simhash_64 FROM articles WHERE id = ?`, articleID).Scan(&clusterIDValue, &simhash); err != nil {
		t.Fatalf("article cluster state error = %v", err)
	}
	if clusterIDValue.Valid {
		t.Fatalf("cluster_id should be NULL after reset, got %d", clusterIDValue.Int64)
	}
	if simhash != 0 {
		t.Fatalf("simhash_64 = %d, want 0", simhash)
	}

	vecBlob, err := db.GetUserInterestVector(userID)
	if err != nil {
		t.Fatalf("GetUserInterestVector error = %v", err)
	}
	if len(vecBlob) != 0 {
		t.Fatalf("interest vector should be cleared")
	}

	var skipCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ai_article_stage_skips WHERE user_id = ?`, userID).Scan(&skipCount); err != nil {
		t.Fatalf("skip count error = %v", err)
	}
	if skipCount != 0 {
		t.Fatalf("skipCount = %d, want 0", skipCount)
	}
}

func TestNormalizeArticleEmbeddingsForUserNormalizesRows(t *testing.T) {
	db := setupTestDB(t)

	userID, err := db.CreateUser(&models.User{
		Username:     "recluster-normalize-user",
		Email:        "recluster-normalize@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser error = %v", err)
	}

	feedID, err := db.AddFeedForUser(userID, &models.Feed{
		Title:           "Normalize Feed",
		URL:             "https://example.com/normalize-feed",
		Type:            "rss",
		RefreshInterval: 60,
	})
	if err != nil {
		t.Fatalf("AddFeedForUser error = %v", err)
	}

	validArticleID := mustInsertReclusterTestArticle(t, db, userID, feedID, "valid-article", "valid-summary")
	validBlob := mustSerializeRawVector(t, []float32{2, 0})
	if _, err := db.Exec(
		`INSERT OR REPLACE INTO article_embeddings (article_id, title_embedding, summary_embedding) VALUES (?, ?, ?)`,
		validArticleID, validBlob, validBlob,
	); err != nil {
		t.Fatalf("insert valid embedding error = %v", err)
	}

	normalized, cleared, err := db.NormalizeArticleEmbeddingsForUser(userID)
	if err != nil {
		t.Fatalf("NormalizeArticleEmbeddingsForUser error = %v", err)
	}
	if normalized != 1 {
		t.Fatalf("normalized = %d, want 1", normalized)
	}
	if cleared != 0 {
		t.Fatalf("cleared = %d, want 0", cleared)
	}

	var normalizedBlob []byte
	if err := db.QueryRow(`SELECT summary_embedding FROM article_embeddings WHERE article_id = ?`, validArticleID).Scan(&normalizedBlob); err != nil {
		t.Fatalf("query normalized blob error = %v", err)
	}
	vec, err := interest.DeserializeVector(normalizedBlob)
	if err != nil {
		t.Fatalf("DeserializeVector error = %v", err)
	}
	if !interest.IsNormalized(vec, 1e-3) {
		t.Fatalf("normalized article embedding should be unit normalized")
	}

}

func mustInsertReclusterTestArticle(t *testing.T, db *dbpkg.DB, userID, feedID int64, uniqueID, summary string) int64 {
	t.Helper()
	result, err := db.Exec(
		`INSERT INTO articles (user_id, feed_id, title, url, published_at, summary, unique_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, feedID, uniqueID, "https://example.com/articles/"+uniqueID, time.Now(), summary, uniqueID,
	)
	if err != nil {
		t.Fatalf("insert article error = %v", err)
	}
	articleID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId error = %v", err)
	}
	return articleID
}

func mustSerializeRawVector(t *testing.T, vec []float32) []byte {
	t.Helper()
	fullVec := make([]float32, 1024)
	copy(fullVec, vec)
	blob, err := sqlite_vec.SerializeFloat32(fullVec)
	if err != nil {
		t.Fatalf("SerializeFloat32 error = %v", err)
	}
	return blob
}

func mustSerializeReclusterVector(t *testing.T, vec []float32) []byte {
	t.Helper()
	fullVec := make([]float32, 1024)
	copy(fullVec, vec)
	blob, err := interest.SerializeVector(fullVec)
	if err != nil {
		t.Fatalf("SerializeVector error = %v", err)
	}
	return blob
}
