package dedup

import (
	"testing"
	"time"

	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
)

func TestProcessArticleCreatesPendingMergeClusterForStandaloneArticle(t *testing.T) {
	db := newDedupTestDB(t)

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

	articleResult, err := db.Exec(
		`INSERT INTO articles (user_id, feed_id, title, url, published_at, summary, unique_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, feedID, "Article Title", "https://example.com/article", time.Now(), "这是一个用于触发独立文章簇创建的有效摘要内容", "article-1",
	)
	if err != nil {
		t.Fatalf("insert article error: %v", err)
	}
	articleID, err := articleResult.LastInsertId()
	if err != nil {
		t.Fatalf("article LastInsertId error: %v", err)
	}

	if err := ProcessArticle(db, articleID, userID); err != nil {
		t.Fatalf("ProcessArticle error: %v", err)
	}

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
	if cluster.Status != "pending_merge" {
		t.Fatalf("cluster status = %q, want pending_merge", cluster.Status)
	}
	if cluster.ArticleCount != 1 {
		t.Fatalf("cluster article_count = %d, want 1", cluster.ArticleCount)
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
