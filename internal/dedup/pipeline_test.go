package dedup

import (
	"testing"
	"time"

	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
)

func TestProcessArticleCreatesPendingMergeClusterForStandaloneArticle(t *testing.T) {
	db := newDedupTestDB(t)
	userID, feedID := createDedupTestUserAndFeed(t, db)

	articleID := createDedupTestArticle(
		t,
		db,
		userID,
		feedID,
		"article-1",
		false,
		"这是一个用于触发独立文章成簇的有效摘要内容。",
	)

	if err := ProcessArticle(db, articleID, userID); err != nil {
		t.Fatalf("ProcessArticle error: %v", err)
	}

	cluster := mustGetArticleCluster(t, db, articleID)
	if cluster.Status != "pending_merge" {
		t.Fatalf("cluster status = %q, want pending_merge", cluster.Status)
	}
	if cluster.ArticleCount != 1 {
		t.Fatalf("cluster article_count = %d, want 1", cluster.ArticleCount)
	}
	if cluster.IsFavorite {
		t.Fatal("cluster is_favorite = true, want false")
	}
}

func TestProcessArticleMarksStandaloneClusterFavoriteWhenArticleFavorited(t *testing.T) {
	db := newDedupTestDB(t)
	userID, feedID := createDedupTestUserAndFeed(t, db)

	articleID := createDedupTestArticle(
		t,
		db,
		userID,
		feedID,
		"favorited-standalone",
		true,
		"这是一个用于测试收藏文章独立成簇的有效摘要内容。",
	)

	if err := ProcessArticle(db, articleID, userID); err != nil {
		t.Fatalf("ProcessArticle error: %v", err)
	}

	cluster := mustGetArticleCluster(t, db, articleID)
	if !cluster.IsFavorite {
		t.Fatal("cluster is_favorite = false, want true")
	}
}

func TestProcessArticleMarksExistingClusterFavoriteWhenFavoritedArticleJoins(t *testing.T) {
	db := newDedupTestDB(t)
	userID, feedID := createDedupTestUserAndFeed(t, db)

	summaryText := "这是一个用于测试文章加入现有簇的相同摘要内容。"
	seedArticleID := createDedupTestArticle(t, db, userID, feedID, "seed-article", false, summaryText)
	if err := ProcessArticle(db, seedArticleID, userID); err != nil {
		t.Fatalf("ProcessArticle seed error: %v", err)
	}

	seedCluster := mustGetArticleCluster(t, db, seedArticleID)
	if seedCluster.IsFavorite {
		t.Fatal("seed cluster is_favorite = true, want false")
	}

	favoritedArticleID := createDedupTestArticle(t, db, userID, feedID, "favorited-joiner", true, summaryText)
	if err := ProcessArticle(db, favoritedArticleID, userID); err != nil {
		t.Fatalf("ProcessArticle favorited joiner error: %v", err)
	}

	updatedCluster := mustGetArticleCluster(t, db, favoritedArticleID)
	if updatedCluster.ID != seedCluster.ID {
		t.Fatalf("cluster id = %d, want %d", updatedCluster.ID, seedCluster.ID)
	}
	if !updatedCluster.IsFavorite {
		t.Fatal("cluster is_favorite = false, want true after favorited article joined")
	}
	if updatedCluster.ArticleCount != 2 {
		t.Fatalf("cluster article_count = %d, want 2", updatedCluster.ArticleCount)
	}
}

func newDedupTestDB(t *testing.T) *sqlite.DB {
	t.Helper()

	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("db Init error: %v", err)
	}

	return db
}

func createDedupTestUserAndFeed(t *testing.T, db *sqlite.DB) (int64, int64) {
	t.Helper()

	userID, err := db.CreateUser(&models.User{
		Username:     "dedup-user",
		Email:        "dedup@example.com",
		PasswordHash: "hash",
		Role:         "user",
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser error: %v", err)
	}

	feedResult, err := db.Exec(
		`INSERT INTO feeds (user_id, title, url, last_updated) VALUES (?, ?, ?, ?)`,
		userID, "Feed", "https://example.com/feed.xml", time.Now(),
	)
	if err != nil {
		t.Fatalf("insert feed error: %v", err)
	}
	feedID, err := feedResult.LastInsertId()
	if err != nil {
		t.Fatalf("feed LastInsertId error: %v", err)
	}

	return userID, feedID
}

func createDedupTestArticle(t *testing.T, db *sqlite.DB, userID, feedID int64, uniqueID string, isFavorite bool, summary string) int64 {
	t.Helper()

	articleResult, err := db.Exec(
		`INSERT INTO articles (user_id, feed_id, title, url, published_at, summary, unique_id, is_favorite) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, feedID, "Article Title "+uniqueID, "https://example.com/"+uniqueID, time.Now(), summary, uniqueID, isFavorite,
	)
	if err != nil {
		t.Fatalf("insert article error: %v", err)
	}
	articleID, err := articleResult.LastInsertId()
	if err != nil {
		t.Fatalf("article LastInsertId error: %v", err)
	}

	return articleID
}

func mustGetArticleCluster(t *testing.T, db *sqlite.DB, articleID int64) *models.Cluster {
	t.Helper()

	var clusterID int64
	if err := db.QueryRow(`SELECT cluster_id FROM articles WHERE id = ?`, articleID).Scan(&clusterID); err != nil {
		t.Fatalf("query article cluster_id error: %v", err)
	}
	if clusterID == 0 {
		t.Fatal("cluster_id = 0, want non-zero")
	}

	cluster, err := db.GetClusterByID(clusterID)
	if err != nil {
		t.Fatalf("GetClusterByID error: %v", err)
	}
	if cluster == nil {
		t.Fatal("cluster = nil, want cluster")
	}

	return cluster
}
