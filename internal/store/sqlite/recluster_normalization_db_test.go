package sqlite_test

import (
	"database/sql"
	"strings"
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
	uncachedArticleID := mustInsertReclusterTestArticle(t, db, userID, feedID, "uncached-article", "uncached-summary")
	if err := db.DeleteArticleContent(uncachedArticleID); err != nil {
		t.Fatalf("DeleteArticleContent error = %v", err)
	}
	uncachedBlob := mustSerializeRawVector(t, []float32{0, 3})
	if _, err := db.Exec(
		`INSERT OR REPLACE INTO article_embeddings (article_id, title_embedding, summary_embedding) VALUES (?, ?, ?)`,
		uncachedArticleID, uncachedBlob, uncachedBlob,
	); err != nil {
		t.Fatalf("insert uncached embedding error = %v", err)
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
	var untouchedBlob []byte
	if err := db.QueryRow(`SELECT summary_embedding FROM article_embeddings WHERE article_id = ?`, uncachedArticleID).Scan(&untouchedBlob); err != nil {
		t.Fatalf("query uncached blob error = %v", err)
	}
	if string(untouchedBlob) != string(uncachedBlob) {
		t.Fatalf("embedding for uncached article should remain untouched")
	}

}

func TestGetArticlesForAIReclusterNormalizationOnlyReturnsCachedContentArticles(t *testing.T) {
	db := setupTestDB(t)

	userID, err := db.CreateUser(&models.User{
		Username:     "recluster-filter-user",
		Email:        "recluster-filter@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser error = %v", err)
	}

	feedID, err := db.AddFeedForUser(userID, &models.Feed{
		Title:           "Filter Feed",
		URL:             "https://example.com/filter-feed",
		Type:            "rss",
		RefreshInterval: 60,
	})
	if err != nil {
		t.Fatalf("AddFeedForUser error = %v", err)
	}

	cachedArticleID := mustInsertReclusterTestArticle(t, db, userID, feedID, "cached-article", "cached-summary")
	uncachedArticleID := mustInsertReclusterTestArticle(t, db, userID, feedID, "uncached-article-list", "uncached-summary")
	if err := db.DeleteArticleContent(uncachedArticleID); err != nil {
		t.Fatalf("DeleteArticleContent error = %v", err)
	}

	articles, err := db.GetArticlesForAIReclusterNormalization(userID, "zh")
	if err != nil {
		t.Fatalf("GetArticlesForAIReclusterNormalization error = %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("len(articles) = %d, want 1", len(articles))
	}
	if articles[0].Article.ID != cachedArticleID {
		t.Fatalf("article ID = %d, want %d", articles[0].Article.ID, cachedArticleID)
	}
	if !articles[0].HasContent {
		t.Fatal("cached article should report HasContent = true")
	}
}

func TestGetAIReclusterNormalizationProgressOnlyCountsCachedContentArticles(t *testing.T) {
	db := setupTestDB(t)

	userID, err := db.CreateUser(&models.User{
		Username:     "recluster-progress-user",
		Email:        "recluster-progress@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser error = %v", err)
	}

	feedID, err := db.AddFeedForUser(userID, &models.Feed{
		Title:           "Progress Feed",
		URL:             "https://example.com/progress-feed",
		Type:            "rss",
		RefreshInterval: 60,
	})
	if err != nil {
		t.Fatalf("AddFeedForUser error = %v", err)
	}

	_ = mustInsertReclusterTestArticle(t, db, userID, feedID, "cached-progress-article", "summary")
	uncachedArticleID := mustInsertReclusterTestArticle(t, db, userID, feedID, "uncached-progress-article", "summary")
	if err := db.DeleteArticleContent(uncachedArticleID); err != nil {
		t.Fatalf("DeleteArticleContent error = %v", err)
	}

	progress, err := db.GetAIReclusterNormalizationProgress(userID, "zh")
	if err != nil {
		t.Fatalf("GetAIReclusterNormalizationProgress error = %v", err)
	}
	if progress.EligibleArticles != 1 {
		t.Fatalf("EligibleArticles = %d, want 1", progress.EligibleArticles)
	}
	if progress.PendingArticles != 1 {
		t.Fatalf("PendingArticles = %d, want 1", progress.PendingArticles)
	}
	if progress.PendingEmbeddingArticles != 1 {
		t.Fatalf("PendingEmbeddingArticles = %d, want 1", progress.PendingEmbeddingArticles)
	}
}

func TestBackfillEmptyClusterMergedContentPersistsFallbackFields(t *testing.T) {
	db := setupTestDB(t)

	userID, err := db.CreateUser(&models.User{
		Username:     "recluster-backfill-user",
		Email:        "recluster-backfill@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser error = %v", err)
	}

	feedID, err := db.AddFeedForUser(userID, &models.Feed{
		Title:           "Backfill Feed",
		URL:             "https://example.com/backfill-feed",
		Type:            "rss",
		RefreshInterval: 60,
	})
	if err != nil {
		t.Fatalf("AddFeedForUser error = %v", err)
	}

	articleID := mustInsertReclusterTestArticle(t, db, userID, feedID, "backfill-article", "fallback summary")
	clusterID, err := db.CreateCluster(userID, "complete")
	if err != nil {
		t.Fatalf("CreateCluster error = %v", err)
	}
	if err := db.UpdateArticleClusterID(articleID, clusterID); err != nil {
		t.Fatalf("UpdateArticleClusterID error = %v", err)
	}
	if err := db.UpdateClusterArticleCount(clusterID); err != nil {
		t.Fatalf("UpdateClusterArticleCount error = %v", err)
	}

	updated, err := db.BackfillEmptyClusterMergedContent(userID)
	if err != nil {
		t.Fatalf("BackfillEmptyClusterMergedContent error = %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated = %d, want 1", updated)
	}

	cluster, err := db.GetClusterByID(clusterID)
	if err != nil {
		t.Fatalf("GetClusterByID error = %v", err)
	}
	if cluster == nil {
		t.Fatal("GetClusterByID returned nil cluster")
	}
	if cluster.MergedTitle != "backfill-article" {
		t.Fatalf("MergedTitle = %q, want source article title", cluster.MergedTitle)
	}
	if cluster.MergedSummary != "fallback summary" {
		t.Fatalf("MergedSummary = %q, want source article summary", cluster.MergedSummary)
	}
	if !strings.Contains(cluster.MergedContent, "article content for recluster normalization tests") {
		t.Fatalf("MergedContent = %q, want cached article content", cluster.MergedContent)
	}
}

func TestSyncClusterFavoriteStatesFromArticlesRepairsFavoriteFlag(t *testing.T) {
	db := setupTestDB(t)

	userID, err := db.CreateUser(&models.User{
		Username:     "recluster-favorite-user",
		Email:        "recluster-favorite@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser error = %v", err)
	}

	feedID, err := db.AddFeedForUser(userID, &models.Feed{
		Title:           "Favorite Feed",
		URL:             "https://example.com/favorite-feed",
		Type:            "rss",
		RefreshInterval: 60,
	})
	if err != nil {
		t.Fatalf("AddFeedForUser error = %v", err)
	}

	result, err := db.Exec(
		`INSERT INTO articles (user_id, feed_id, title, url, published_at, is_favorite, summary, unique_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, feedID, "favorite-article", "https://example.com/favorite-article", time.Now(), true, "favorite summary", "favorite-article",
	)
	if err != nil {
		t.Fatalf("insert favorite article error = %v", err)
	}
	articleID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId error = %v", err)
	}
	if err := db.SetArticleContent(articleID, "favorite article content"); err != nil {
		t.Fatalf("SetArticleContent error = %v", err)
	}

	clusterID, err := db.CreateCluster(userID, "complete")
	if err != nil {
		t.Fatalf("CreateCluster error = %v", err)
	}
	if err := db.UpdateArticleClusterID(articleID, clusterID); err != nil {
		t.Fatalf("UpdateArticleClusterID error = %v", err)
	}
	if err := db.SetClusterFavorite(clusterID, false); err != nil {
		t.Fatalf("SetClusterFavorite(false) error = %v", err)
	}

	if err := db.SyncClusterFavoriteStatesFromArticles(userID); err != nil {
		t.Fatalf("SyncClusterFavoriteStatesFromArticles error = %v", err)
	}

	cluster, err := db.GetClusterByID(clusterID)
	if err != nil {
		t.Fatalf("GetClusterByID error = %v", err)
	}
	if cluster == nil || !cluster.IsFavorite {
		t.Fatalf("cluster favorite = %v, want true", cluster != nil && cluster.IsFavorite)
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
	if err := db.SetArticleContent(articleID, "article content for recluster normalization tests"); err != nil {
		t.Fatalf("SetArticleContent error = %v", err)
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
