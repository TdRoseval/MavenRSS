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

	articleOne := &models.Article{UserID: 1, FeedID: feedOne, Title: "Cluster One", URL: "https://example.com/c1", PublishedAt: oldTime, UniqueID: "cluster-one", IsRead: true}
	articleTwo := &models.Article{UserID: 2, FeedID: feedTwo, Title: "Cluster Two", URL: "https://example.com/c2", PublishedAt: oldTime, UniqueID: "cluster-two", IsRead: true}
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

func TestCleanupExpiredReadClustersRequiresAllArticlesToMeetCriteria(t *testing.T) {
	db := setupTestDB(t)
	feedID := insertCleanupFeed(t, db, 1, "Cleanup Feed", "https://example.com/cleanup-all-read")

	clusterID, err := db.CreateCluster(1, "complete")
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	oldTime := time.Now().AddDate(0, 0, -45)
	recentTime := time.Now().AddDate(0, 0, -5)
	if _, err := db.Exec(`UPDATE clusters SET updated_at = ?, is_read = 1 WHERE id = ?`, oldTime, clusterID); err != nil {
		t.Fatalf("update cluster state: %v", err)
	}

	oldArticle := &models.Article{UserID: 1, FeedID: feedID, Title: "Old Cluster Article", URL: "https://example.com/old-cluster", PublishedAt: oldTime, UniqueID: "old-cluster-article", IsRead: true}
	recentArticle := &models.Article{UserID: 1, FeedID: feedID, Title: "Recent Cluster Article", URL: "https://example.com/recent-cluster", PublishedAt: recentTime, UniqueID: "recent-cluster-article", IsRead: true}

	createCleanupArticle(t, db, oldArticle, clusterID)
	createCleanupArticle(t, db, recentArticle, clusterID)

	deleted, err := db.CleanupExpiredReadClusters(1, 30)
	if err != nil {
		t.Fatalf("cleanup expired read clusters: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected no clusters deleted when one article is still too new, got %d", deleted)
	}

	var clusterExists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM clusters WHERE id = ?`, clusterID).Scan(&clusterExists); err != nil {
		t.Fatalf("count cluster: %v", err)
	}
	if clusterExists != 1 {
		t.Fatalf("expected cluster to remain, got %d", clusterExists)
	}
}

func TestCleanupOldReadArticlesDeletesEntireEligibleCluster(t *testing.T) {
	db := setupTestDB(t)
	feedID := insertCleanupFeed(t, db, 1, "Cleanup Feed", "https://example.com/cleanup-old-read")

	clusterID, err := db.CreateCluster(1, "complete")
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	oldTime := time.Now().AddDate(0, 0, -45)
	if _, err := db.Exec(`UPDATE clusters SET updated_at = ?, is_read = 1 WHERE id = ?`, oldTime, clusterID); err != nil {
		t.Fatalf("update cluster state: %v", err)
	}

	first := &models.Article{UserID: 1, FeedID: feedID, Title: "Old Read Cluster A", URL: "https://example.com/old-read-a", PublishedAt: oldTime, UniqueID: "old-read-cluster-a", IsRead: true}
	second := &models.Article{UserID: 1, FeedID: feedID, Title: "Old Read Cluster B", URL: "https://example.com/old-read-b", PublishedAt: oldTime.Add(-time.Hour), UniqueID: "old-read-cluster-b", IsRead: true}

	createCleanupArticle(t, db, first, clusterID)
	createCleanupArticle(t, db, second, clusterID)

	deletedArticles, err := db.CleanupOldReadArticles(30, 1)
	if err != nil {
		t.Fatalf("cleanup old read articles: %v", err)
	}
	if deletedArticles != 2 {
		t.Fatalf("expected 2 clustered articles deleted, got %d", deletedArticles)
	}

	var clusterExists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM clusters WHERE id = ?`, clusterID).Scan(&clusterExists); err != nil {
		t.Fatalf("count cluster: %v", err)
	}
	if clusterExists != 0 {
		t.Fatalf("expected eligible cluster to be deleted, got %d", clusterExists)
	}

	var articleCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM articles WHERE user_id = 1`).Scan(&articleCount); err != nil {
		t.Fatalf("count remaining articles: %v", err)
	}
	if articleCount != 0 {
		t.Fatalf("expected clustered articles to be deleted together, got %d remaining", articleCount)
	}
}

func TestCleanupAllArticleContentsRemovesClusteredArticles(t *testing.T) {
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
	if deleted != 2 {
		t.Fatalf("expected 2 article contents deleted, got %d", deleted)
	}

	var clusteredContentCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM article_contents WHERE article_id = ?`, clusteredID).Scan(&clusteredContentCount); err != nil {
		t.Fatalf("count clustered content: %v", err)
	}
	if clusteredContentCount != 0 {
		t.Fatalf("expected clustered article content to be removed, got %d", clusteredContentCount)
	}
}

func TestCleanupArticleCachePreservingFavoritesKeepsFavoriteArticlesAndRelatedClusters(t *testing.T) {
	db := setupTestDB(t)
	feedID := insertCleanupFeed(t, db, 1, "Cleanup Feed", "https://example.com/manual-cleanup")

	favoriteClusterID, err := db.CreateCluster(1, "complete")
	if err != nil {
		t.Fatalf("create favorite cluster: %v", err)
	}
	explicitFavoriteClusterID, err := db.CreateCluster(1, "complete")
	if err != nil {
		t.Fatalf("create explicit favorite cluster: %v", err)
	}
	deletedClusterID, err := db.CreateCluster(1, "complete")
	if err != nil {
		t.Fatalf("create deletable cluster: %v", err)
	}

	if err := db.SetClusterFavorite(explicitFavoriteClusterID, true); err != nil {
		t.Fatalf("set explicit favorite cluster: %v", err)
	}

	favoriteClusterLead := &models.Article{UserID: 1, FeedID: feedID, Title: "Favorite Cluster Lead", URL: "https://example.com/favorite-cluster-lead", PublishedAt: time.Now(), UniqueID: "favorite-cluster-lead", IsFavorite: true}
	favoriteClusterPeer := &models.Article{UserID: 1, FeedID: feedID, Title: "Favorite Cluster Peer", URL: "https://example.com/favorite-cluster-peer", PublishedAt: time.Now(), UniqueID: "favorite-cluster-peer"}
	explicitFavoriteClusterArticle := &models.Article{UserID: 1, FeedID: feedID, Title: "Explicit Favorite Cluster Article", URL: "https://example.com/explicit-favorite-cluster", PublishedAt: time.Now(), UniqueID: "explicit-favorite-cluster"}
	deletedClusterArticle := &models.Article{UserID: 1, FeedID: feedID, Title: "Deleted Cluster Article", URL: "https://example.com/deleted-cluster", PublishedAt: time.Now(), UniqueID: "deleted-cluster"}
	favoriteStandalone := &models.Article{UserID: 1, FeedID: feedID, Title: "Favorite Standalone", URL: "https://example.com/favorite-standalone", PublishedAt: time.Now(), UniqueID: "favorite-standalone", IsFavorite: true}
	deletedStandalone := &models.Article{UserID: 1, FeedID: feedID, Title: "Deleted Standalone", URL: "https://example.com/deleted-standalone", PublishedAt: time.Now(), UniqueID: "deleted-standalone"}

	favoriteClusterLeadID := createCleanupArticle(t, db, favoriteClusterLead, favoriteClusterID)
	favoriteClusterPeerID := createCleanupArticle(t, db, favoriteClusterPeer, favoriteClusterID)
	explicitFavoriteClusterArticleID := createCleanupArticle(t, db, explicitFavoriteClusterArticle, explicitFavoriteClusterID)
	deletedClusterArticleID := createCleanupArticle(t, db, deletedClusterArticle, deletedClusterID)
	favoriteStandaloneID := createCleanupArticle(t, db, favoriteStandalone, 0)
	deletedStandaloneID := createCleanupArticle(t, db, deletedStandalone, 0)

	if _, err := db.Exec(`
		INSERT INTO article_contents (article_id, content) VALUES (?, ?), (?, ?), (?, ?), (?, ?), (?, ?), (?, ?)
	`, favoriteClusterLeadID, "favorite cluster lead", favoriteClusterPeerID, "favorite cluster peer", explicitFavoriteClusterArticleID, "explicit favorite cluster article", deletedClusterArticleID, "deleted cluster article", favoriteStandaloneID, "favorite standalone", deletedStandaloneID, "deleted standalone"); err != nil {
		t.Fatalf("insert article contents: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO daily_recommendations (user_id, cluster_id, recommendation_date, recommendation_score, recommendation_rank, recommendation_profile_id) VALUES (?, ?, ?, ?, ?, ?)`, 1, deletedClusterID, "2026-04-03", 0.9, 1, 123); err != nil {
		t.Fatalf("insert daily recommendation: %v", err)
	}

	stats, err := db.CleanupArticleCachePreservingFavorites(1)
	if err != nil {
		t.Fatalf("manual cleanup preserving favorites: %v", err)
	}

	if stats.DeletedArticles != 2 {
		t.Fatalf("expected 2 deleted articles, got %d", stats.DeletedArticles)
	}
	if stats.DeletedClusters != 1 {
		t.Fatalf("expected 1 deleted cluster, got %d", stats.DeletedClusters)
	}
	if stats.RetainedArticles != 4 {
		t.Fatalf("expected 4 retained articles, got %d", stats.RetainedArticles)
	}
	if stats.RetainedClusters != 2 {
		t.Fatalf("expected 2 retained clusters, got %d", stats.RetainedClusters)
	}

	var remainingTitles []string
	rows, err := db.Query(`SELECT title FROM articles WHERE user_id = 1 ORDER BY title ASC`)
	if err != nil {
		t.Fatalf("query remaining articles: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			t.Fatalf("scan remaining title: %v", err)
		}
		remainingTitles = append(remainingTitles, title)
	}

	expectedTitles := map[string]bool{
		"Favorite Cluster Lead":             true,
		"Favorite Cluster Peer":             true,
		"Explicit Favorite Cluster Article": true,
		"Favorite Standalone":               true,
	}
	if len(remainingTitles) != len(expectedTitles) {
		t.Fatalf("expected %d remaining articles, got %d (%v)", len(expectedTitles), len(remainingTitles), remainingTitles)
	}
	for _, title := range remainingTitles {
		if !expectedTitles[title] {
			t.Fatalf("unexpected remaining article %q", title)
		}
	}

	var deletedClusterExists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM clusters WHERE id = ?`, deletedClusterID).Scan(&deletedClusterExists); err != nil {
		t.Fatalf("count deleted cluster: %v", err)
	}
	if deletedClusterExists != 0 {
		t.Fatalf("expected deleted cluster to be removed, got %d", deletedClusterExists)
	}

	var deletedStandaloneExists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM articles WHERE id = ?`, deletedStandaloneID).Scan(&deletedStandaloneExists); err != nil {
		t.Fatalf("count deleted standalone: %v", err)
	}
	if deletedStandaloneExists != 0 {
		t.Fatalf("expected deleted standalone article to be removed, got %d", deletedStandaloneExists)
	}

	var deletedRecommendationCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM daily_recommendations WHERE cluster_id = ?`, deletedClusterID).Scan(&deletedRecommendationCount); err != nil {
		t.Fatalf("count deleted recommendations: %v", err)
	}
	if deletedRecommendationCount != 0 {
		t.Fatalf("expected deleted cluster recommendations to be removed, got %d", deletedRecommendationCount)
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
