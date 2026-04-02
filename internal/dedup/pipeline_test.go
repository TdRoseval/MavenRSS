package dedup

import (
	"testing"
	"time"

	"MavenRSS/internal/interest"
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
		"article-standalone",
		false,
		"this standalone article should create its own cluster",
		vector1024(1, 0),
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
		"this favorite article should create a favorite cluster",
		vector1024(1, 0),
	)

	if err := ProcessArticle(db, articleID, userID); err != nil {
		t.Fatalf("ProcessArticle error: %v", err)
	}

	cluster := mustGetArticleCluster(t, db, articleID)
	if !cluster.IsFavorite {
		t.Fatal("cluster is_favorite = false, want true")
	}
}

func TestProcessArticleChoosesNearestSemanticMatchAmongHammingCandidates(t *testing.T) {
	db := newDedupTestDB(t)
	userID, feedID := createDedupTestUserAndFeed(t, db)

	summary := "shared summary text for hamming candidate matching"
	clusterOne := createSeedClusterArticle(t, db, userID, feedID, "seed-cluster-one", summary, vector1024(1, 0), true)
	clusterTwo := createSeedClusterArticle(t, db, userID, feedID, "seed-cluster-two", summary, vector1024(0.8, 0.6), true)

	targetArticleID := createDedupTestArticle(t, db, userID, feedID, "target-hamming", false, summary, vector1024(0.8, 0.6))
	if err := ProcessArticle(db, targetArticleID, userID); err != nil {
		t.Fatalf("ProcessArticle error: %v", err)
	}

	cluster := mustGetArticleCluster(t, db, targetArticleID)
	if cluster.ID != clusterTwo {
		t.Fatalf("joined cluster %d, want %d", cluster.ID, clusterTwo)
	}
	if cluster.ID == clusterOne {
		t.Fatal("article joined the first match instead of the nearest semantic match")
	}
}

func TestProcessArticleSkipsHammingCandidatesWithoutSummaryEmbedding(t *testing.T) {
	db := newDedupTestDB(t)
	userID, feedID := createDedupTestUserAndFeed(t, db)

	summary := "shared summary text for missing embedding candidate"
	createSeedClusterArticle(t, db, userID, feedID, "missing-embedding", summary, nil, true)
	validCluster := createSeedClusterArticle(t, db, userID, feedID, "valid-embedding", summary, vector1024(1, 0), true)

	targetArticleID := createDedupTestArticle(t, db, userID, feedID, "target-skip-missing", false, summary, vector1024(1, 0))
	if err := ProcessArticle(db, targetArticleID, userID); err != nil {
		t.Fatalf("ProcessArticle error: %v", err)
	}

	cluster := mustGetArticleCluster(t, db, targetArticleID)
	if cluster.ID != validCluster {
		t.Fatalf("joined cluster %d, want %d", cluster.ID, validCluster)
	}
}

func TestProcessArticleUsesClusterCentroidWhenNoHammingMatch(t *testing.T) {
	db := newDedupTestDB(t)
	userID, feedID := createDedupTestUserAndFeed(t, db)

	createSeedClusterArticle(t, db, userID, feedID, "semantic-one", "long text that will not share simhash bands A", vector1024(1, 0), true)
	clusterTwo := createSeedClusterArticle(t, db, userID, feedID, "semantic-two", "long text that will not share simhash bands B", vector1024(0.8, 0.6), true)

	targetArticleID := createDedupTestArticle(t, db, userID, feedID, "semantic-target", false, "short", vector1024(0.8, 0.6))
	if err := ProcessArticle(db, targetArticleID, userID); err != nil {
		t.Fatalf("ProcessArticle error: %v", err)
	}

	cluster := mustGetArticleCluster(t, db, targetArticleID)
	if cluster.ID != clusterTwo {
		t.Fatalf("joined cluster %d, want %d", cluster.ID, clusterTwo)
	}
}

func TestProcessArticleBuildsCentroidFromAllClusterArticles(t *testing.T) {
	db := newDedupTestDB(t)
	userID, feedID := createDedupTestUserAndFeed(t, db)

	clusterOne := createSeedClusterArticle(t, db, userID, feedID, "cluster-one-hit", "cluster one candidate one", vector1024(1, 0), true)
	createArticleInExistingCluster(t, db, userID, feedID, clusterOne, "cluster-one-opposite", "cluster one candidate two", vector1024(-1, 0), true)

	clusterTwo := createSeedClusterArticle(t, db, userID, feedID, "cluster-two-a", "cluster two candidate one", vector1024(0.9, 0.43), true)
	createArticleInExistingCluster(t, db, userID, feedID, clusterTwo, "cluster-two-b", "cluster two candidate two", vector1024(0.92, 0.39), true)

	targetArticleID := createDedupTestArticle(t, db, userID, feedID, "centroid-target", false, "short", vector1024(1, 0))
	if err := ProcessArticle(db, targetArticleID, userID); err != nil {
		t.Fatalf("ProcessArticle error: %v", err)
	}

	cluster := mustGetArticleCluster(t, db, targetArticleID)
	if cluster.ID != clusterTwo {
		t.Fatalf("joined cluster %d, want %d", cluster.ID, clusterTwo)
	}
}

func TestProcessArticleCreatesStandaloneClusterWhenNoMatch(t *testing.T) {
	db := newDedupTestDB(t)
	userID, feedID := createDedupTestUserAndFeed(t, db)

	createSeedClusterArticle(t, db, userID, feedID, "distant-candidate", "distant summary", vector1024(0, 1), true)
	targetArticleID := createDedupTestArticle(t, db, userID, feedID, "no-match-target", false, "short", vector1024(1, 0))

	if err := ProcessArticle(db, targetArticleID, userID); err != nil {
		t.Fatalf("ProcessArticle error: %v", err)
	}

	cluster := mustGetArticleCluster(t, db, targetArticleID)
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

func createDedupTestArticle(
	t *testing.T,
	db *sqlite.DB,
	userID, feedID int64,
	uniqueID string,
	isFavorite bool,
	summary string,
	summaryVector []float32,
) int64 {
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

	if len(summaryVector) > 0 {
		blob, err := interest.SerializeVector(summaryVector)
		if err != nil {
			t.Fatalf("SerializeVector error: %v", err)
		}
		if err := db.UpdateArticleEmbeddings(articleID, nil, blob); err != nil {
			t.Fatalf("UpdateArticleEmbeddings error: %v", err)
		}
	}

	return articleID
}

func createSeedClusterArticle(
	t *testing.T,
	db *sqlite.DB,
	userID, feedID int64,
	uniqueID string,
	summary string,
	summaryVector []float32,
	withSimHash bool,
) int64 {
	t.Helper()

	clusterID, err := db.CreateCluster(userID, "complete")
	if err != nil {
		t.Fatalf("CreateCluster error: %v", err)
	}

	createArticleInExistingCluster(t, db, userID, feedID, clusterID, uniqueID, summary, summaryVector, withSimHash)
	return clusterID
}

func createArticleInExistingCluster(
	t *testing.T,
	db *sqlite.DB,
	userID, feedID, clusterID int64,
	uniqueID string,
	summary string,
	summaryVector []float32,
	withSimHash bool,
) int64 {
	t.Helper()

	articleID := createDedupTestArticle(t, db, userID, feedID, uniqueID, false, summary, summaryVector)
	if err := db.UpdateArticleClusterID(articleID, clusterID); err != nil {
		t.Fatalf("UpdateArticleClusterID error: %v", err)
	}
	if err := db.UpdateClusterArticleCount(clusterID); err != nil {
		t.Fatalf("UpdateClusterArticleCount error: %v", err)
	}
	if withSimHash && IsValidForSimHash(summary) {
		hash := ComputeSimHash64(summary)
		b1, b2, b3, b4 := SplitBands(hash)
		if err := db.UpdateArticleSimHash(articleID, hash, b1, b2, b3, b4); err != nil {
			t.Fatalf("UpdateArticleSimHash error: %v", err)
		}
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

func vector1024(values ...float32) []float32 {
	vec := make([]float32, 1024)
	copy(vec, values)
	return vec
}
