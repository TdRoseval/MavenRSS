package sqlite_test

import (
	"testing"
	"time"

	"MavenRSS/internal/models"
	dbpkg "MavenRSS/internal/store/sqlite"
)

func insertCleanupFeed(t *testing.T, db *dbpkg.DB, userID int64, title, url string) int64 {
	t.Helper()
	result, err := db.Exec(
		`INSERT INTO feeds (user_id, title, url, category, is_image_mode, hide_from_timeline) VALUES (?, ?, ?, ?, ?, ?)`,
		userID, title, url, "news", 0, 0,
	)
	if err != nil {
		t.Fatalf("insert feed: %v", err)
	}
	feedID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("feed last insert id: %v", err)
	}
	return feedID
}

func insertCleanupUser(t *testing.T, db *dbpkg.DB, userID int64) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO users (id, username, email, password_hash, role, status) VALUES (?, ?, ?, ?, ?, ?)`,
		userID, "user-cleanup-"+time.Now().Format("150405.000000"), "cleanup-user-"+time.Now().Format("150405.000000")+"@example.com", "hash", "user", "active",
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func createCleanupArticle(t *testing.T, db *dbpkg.DB, article *models.Article, clusterID int64) int64 {
	t.Helper()
	if err := db.SaveArticle(article); err != nil {
		t.Fatalf("save article: %v", err)
	}

	var articleID int64
	if err := db.QueryRow(`SELECT id FROM articles WHERE unique_id = ?`, article.UniqueID).Scan(&articleID); err != nil {
		t.Fatalf("scan article id: %v", err)
	}

	if clusterID > 0 {
		if err := db.UpdateArticleClusterID(articleID, clusterID); err != nil {
			t.Fatalf("update cluster id: %v", err)
		}
	}

	return articleID
}

func TestCleanupExpiredReadClustersSupportsAllUsers(t *testing.T) {
	db := setupTestDB(t)

	insertCleanupUser(t, db, 2)
	feedOne := insertCleanupFeed(t, db, 1, "Cleanup Feed", "https://example.com/cleanup")
	feedTwo := insertCleanupFeed(t, db, 2, "Cleanup Feed 2", "https://example.com/cleanup-2")

	clusterOne, err := db.CreateCluster(1, "complete")
	if err != nil {
		t.Fatalf("create cluster one: %v", err)
	}
	clusterTwo, err := db.CreateCluster(2, "complete")
	if err != nil {
		t.Fatalf("create cluster two: %v", err)
	}
	clusterProtected, err := db.CreateCluster(1, "complete")
	if err != nil {
		t.Fatalf("create protected cluster: %v", err)
	}

	oldTime := time.Now().AddDate(0, 0, -45)
	if _, err := db.Exec(`UPDATE clusters SET updated_at = ?, is_read = 1 WHERE id IN (?, ?)`, oldTime, clusterOne, clusterTwo); err != nil {
		t.Fatalf("update read clusters: %v", err)
	}
	if _, err := db.Exec(`UPDATE clusters SET updated_at = ?, is_read = 1, is_favorite = 1 WHERE id = ?`, oldTime, clusterProtected); err != nil {
		t.Fatalf("update protected cluster: %v", err)
	}

	articleOne := &models.Article{UserID: 1, FeedID: feedOne, Title: "Cluster One", URL: "https://example.com/c1", PublishedAt: oldTime, UniqueID: "cluster-one"}
	articleTwo := &models.Article{UserID: 2, FeedID: feedTwo, Title: "Cluster Two", URL: "https://example.com/c2", PublishedAt: oldTime, UniqueID: "cluster-two"}
	articleThree := &models.Article{UserID: 1, FeedID: feedOne, Title: "Protected", URL: "https://example.com/c3", PublishedAt: oldTime, UniqueID: "cluster-protected"}

	createCleanupArticle(t, db, articleOne, clusterOne)
	createCleanupArticle(t, db, articleTwo, clusterTwo)
	createCleanupArticle(t, db, articleThree, clusterProtected)

	deleted, err := db.CleanupExpiredReadClusters(0, 30)
	if err != nil {
		t.Fatalf("cleanup expired read clusters: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 clusters deleted, got %d", deleted)
	}

	var remainingClusters int
	if err := db.QueryRow(`SELECT COUNT(*) FROM clusters WHERE id IN (?, ?, ?)`, clusterOne, clusterTwo, clusterProtected).Scan(&remainingClusters); err != nil {
		t.Fatalf("count clusters: %v", err)
	}
	if remainingClusters != 1 {
		t.Fatalf("expected only protected cluster to remain, got %d", remainingClusters)
	}

	var protectedExists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM articles WHERE cluster_id = ?`, clusterProtected).Scan(&protectedExists); err != nil {
		t.Fatalf("count protected articles: %v", err)
	}
	if protectedExists != 1 {
		t.Fatalf("expected protected cluster articles to remain, got %d", protectedExists)
	}
}

func TestCleanupAllArticleContentsSkipsClusteredArticles(t *testing.T) {
	db := setupTestDB(t)
	feedID := insertCleanupFeed(t, db, 1, "Cleanup Feed", "https://example.com/cleanup")

	clusterID, err := db.CreateCluster(1, "complete")
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	clustered := &models.Article{UserID: 1, FeedID: feedID, Title: "Clustered", URL: "https://example.com/clustered", PublishedAt: time.Now(), UniqueID: "clustered-article"}
	standalone := &models.Article{UserID: 1, FeedID: feedID, Title: "Standalone", URL: "https://example.com/standalone", PublishedAt: time.Now(), UniqueID: "standalone-article"}

	clusteredID := createCleanupArticle(t, db, clustered, clusterID)
	standaloneID := createCleanupArticle(t, db, standalone, 0)

	if _, err := db.Exec(`INSERT INTO article_contents (article_id, content) VALUES (?, ?), (?, ?)`, clusteredID, "clustered", standaloneID, "standalone"); err != nil {
		t.Fatalf("insert article contents: %v", err)
	}

	deleted, err := db.CleanupAllArticleContents(1)
	if err != nil {
		t.Fatalf("cleanup article contents: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 unclustered article content deleted, got %d", deleted)
	}

	var clusteredContentCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM article_contents WHERE article_id = ?`, clusteredID).Scan(&clusteredContentCount); err != nil {
		t.Fatalf("count clustered content: %v", err)
	}
	if clusteredContentCount != 1 {
		t.Fatalf("expected clustered article content to remain, got %d", clusteredContentCount)
	}
}

func TestDeleteAllArticlesRemovesClustersAndStandaloneArticles(t *testing.T) {
	db := setupTestDB(t)
	feedID := insertCleanupFeed(t, db, 1, "Cleanup Feed", "https://example.com/cleanup")

	clusterID, err := db.CreateCluster(1, "complete")
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	clustered := &models.Article{UserID: 1, FeedID: feedID, Title: "Clustered", URL: "https://example.com/delete-clustered", PublishedAt: time.Now(), UniqueID: "delete-clustered"}
	standalone := &models.Article{UserID: 1, FeedID: feedID, Title: "Standalone", URL: "https://example.com/delete-standalone", PublishedAt: time.Now(), UniqueID: "delete-standalone"}

	clusteredID := createCleanupArticle(t, db, clustered, clusterID)
	standaloneID := createCleanupArticle(t, db, standalone, 0)

	if _, err := db.Exec(`INSERT INTO article_contents (article_id, content) VALUES (?, ?), (?, ?)`, clusteredID, "clustered", standaloneID, "standalone"); err != nil {
		t.Fatalf("insert article contents: %v", err)
	}

	deleted, err := db.DeleteAllArticles(1)
	if err != nil {
		t.Fatalf("delete all articles: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 articles deleted, got %d", deleted)
	}

	var articleCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM articles WHERE user_id = 1`).Scan(&articleCount); err != nil {
		t.Fatalf("count remaining articles: %v", err)
	}
	if articleCount != 0 {
		t.Fatalf("expected no remaining articles, got %d", articleCount)
	}

	var clusterCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM clusters WHERE user_id = 1`).Scan(&clusterCount); err != nil {
		t.Fatalf("count remaining clusters: %v", err)
	}
	if clusterCount != 0 {
		t.Fatalf("expected no remaining clusters, got %d", clusterCount)
	}
}

func TestDeleteArticlesForFeedSkipsClusteredArticles(t *testing.T) {
	db := setupTestDB(t)
	feedID := insertCleanupFeed(t, db, 1, "Cleanup Feed", "https://example.com/cleanup")

	clusterID, err := db.CreateCluster(1, "complete")
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	clustered := &models.Article{UserID: 1, FeedID: feedID, Title: "Clustered", URL: "https://example.com/feed-clustered", PublishedAt: time.Now(), UniqueID: "feed-clustered"}
	standalone := &models.Article{UserID: 1, FeedID: feedID, Title: "Standalone", URL: "https://example.com/feed-standalone", PublishedAt: time.Now(), UniqueID: "feed-standalone"}

	clusteredID := createCleanupArticle(t, db, clustered, clusterID)
	standaloneID := createCleanupArticle(t, db, standalone, 0)

	if _, err := db.Exec(`INSERT INTO article_contents (article_id, content) VALUES (?, ?), (?, ?)`, clusteredID, "clustered", standaloneID, "standalone"); err != nil {
		t.Fatalf("insert article contents: %v", err)
	}

	deleted, err := db.DeleteArticlesForFeed(feedID, 1)
	if err != nil {
		t.Fatalf("delete articles for feed: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected only 1 standalone article deleted, got %d", deleted)
	}

	var clusteredArticleCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM articles WHERE id = ?`, clusteredID).Scan(&clusteredArticleCount); err != nil {
		t.Fatalf("count clustered article: %v", err)
	}
	if clusteredArticleCount != 1 {
		t.Fatalf("expected clustered article to remain, got %d", clusteredArticleCount)
	}

	var standaloneArticleCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM articles WHERE id = ?`, standaloneID).Scan(&standaloneArticleCount); err != nil {
		t.Fatalf("count standalone article: %v", err)
	}
	if standaloneArticleCount != 0 {
		t.Fatalf("expected standalone article to be deleted, got %d", standaloneArticleCount)
	}
}
