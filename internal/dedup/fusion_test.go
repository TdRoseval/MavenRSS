package dedup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync/atomic"
	"testing"

	"MavenRSS/internal/models"
)

func TestRunFusionCopiesSingleArticleWithoutSummarizer(t *testing.T) {
	db := newDedupTestDB(t)
	userID, feedID := createDedupTestUserAndFeed(t, db)

	articleID := createDedupTestArticle(
		t,
		db,
		userID,
		feedID,
		"single-fusion-fallback",
		false,
		"single article summary",
		nil,
	)
	if err := db.SetArticleContent(articleID, "single article body content"); err != nil {
		t.Fatalf("SetArticleContent error: %v", err)
	}

	clusterID, err := db.CreateCluster(userID, "pending_merge")
	if err != nil {
		t.Fatalf("CreateCluster error: %v", err)
	}
	if err := db.UpdateArticleClusterID(articleID, clusterID); err != nil {
		t.Fatalf("UpdateArticleClusterID error: %v", err)
	}
	if err := db.UpdateClusterArticleCount(clusterID); err != nil {
		t.Fatalf("UpdateClusterArticleCount error: %v", err)
	}

	if err := RunFusion(context.Background(), db, userID, &FusionConfig{}); err != nil {
		t.Fatalf("RunFusion error: %v", err)
	}

	cluster, err := db.GetClusterByID(clusterID)
	if err != nil {
		t.Fatalf("GetClusterByID error: %v", err)
	}
	if cluster == nil {
		t.Fatal("GetClusterByID returned nil cluster")
	}
	if cluster.Status != "pending_embed" {
		t.Fatalf("cluster status = %q, want pending_embed", cluster.Status)
	}
	if cluster.MergedTitle == "" {
		t.Fatal("MergedTitle = empty, want article title")
	}
	if cluster.MergedSummary != "single article summary" {
		t.Fatalf("MergedSummary = %q, want source summary", cluster.MergedSummary)
	}
	if !strings.Contains(cluster.MergedContent, "single article body content") {
		t.Fatalf("MergedContent = %q, want source content", cluster.MergedContent)
	}
}

func TestRunFusionCopiesSingleArticleUsesCachedTranslatedTitle(t *testing.T) {
	db := newDedupTestDB(t)
	userID, err := db.CreateUser(&models.User{
		Username:     "dedup-cached-title-user",
		Email:        "dedup-cached-title@example.com",
		PasswordHash: "hash",
		Role:         "user",
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser error: %v", err)
	}
	feedID, err := db.AddFeedForUser(userID, &models.Feed{
		Title:             "Translated Feed",
		URL:               "https://example.com/dedup-translated-feed.xml",
		Type:              "rss",
		RefreshInterval:   60,
		TranslateArticles: true,
	})
	if err != nil {
		t.Fatalf("AddFeedForUser error: %v", err)
	}
	if err := db.SetSettingForUser(userID, "target_language", "zh"); err != nil {
		t.Fatalf("SetSettingForUser target_language error: %v", err)
	}
	if err := db.SetSettingForUser(userID, "translation_provider", "google"); err != nil {
		t.Fatalf("SetSettingForUser translation_provider error: %v", err)
	}

	articleID := createDedupTestArticle(
		t,
		db,
		userID,
		feedID,
		"single-fusion-cached-title",
		false,
		"single article summary",
		nil,
	)
	if err := db.SetArticleContent(articleID, "single article body content"); err != nil {
		t.Fatalf("SetArticleContent error: %v", err)
	}

	article, err := db.GetArticleByID(articleID)
	if err != nil {
		t.Fatalf("GetArticleByID error: %v", err)
	}
	if article == nil {
		t.Fatal("GetArticleByID returned nil article")
	}

	const cachedTitle = "单篇文章缓存标题"
	if err := db.SetCachedTranslation(hashDedupTestTranslation(article.Title), article.Title, "zh", cachedTitle, "google"); err != nil {
		t.Fatalf("SetCachedTranslation error: %v", err)
	}

	clusterID, err := db.CreateCluster(userID, "pending_merge")
	if err != nil {
		t.Fatalf("CreateCluster error: %v", err)
	}
	if err := db.UpdateArticleClusterID(articleID, clusterID); err != nil {
		t.Fatalf("UpdateArticleClusterID error: %v", err)
	}
	if err := db.UpdateClusterArticleCount(clusterID); err != nil {
		t.Fatalf("UpdateClusterArticleCount error: %v", err)
	}

	if err := RunFusion(context.Background(), db, userID, &FusionConfig{}); err != nil {
		t.Fatalf("RunFusion error: %v", err)
	}

	cluster, err := db.GetClusterByID(clusterID)
	if err != nil {
		t.Fatalf("GetClusterByID error: %v", err)
	}
	if cluster == nil {
		t.Fatal("GetClusterByID returned nil cluster")
	}
	if cluster.MergedTitle != cachedTitle {
		t.Fatalf("MergedTitle = %q, want cached translated title %q", cluster.MergedTitle, cachedTitle)
	}

	updatedArticle, err := db.GetArticleByID(articleID)
	if err != nil {
		t.Fatalf("GetArticleByID after fusion error: %v", err)
	}
	if updatedArticle == nil {
		t.Fatal("GetArticleByID after fusion returned nil article")
	}
	if updatedArticle.TranslatedTitle != cachedTitle {
		t.Fatalf("TranslatedTitle = %q, want cached translated title %q", updatedArticle.TranslatedTitle, cachedTitle)
	}
}

func TestRunClusterWorkersProcessesClustersConcurrently(t *testing.T) {
	clusters := []models.Cluster{
		{ID: 1},
		{ID: 2},
		{ID: 3},
	}

	var active int32
	var maxActive int32
	started := make(chan struct{}, len(clusters))
	release := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		done <- runClusterWorkers(context.Background(), clusters, 2, func(cluster models.Cluster) {
			current := atomic.AddInt32(&active, 1)
			for {
				previous := atomic.LoadInt32(&maxActive)
				if current <= previous || atomic.CompareAndSwapInt32(&maxActive, previous, current) {
					break
				}
			}
			started <- struct{}{}
			<-release
			atomic.AddInt32(&active, -1)
		})
	}()

	<-started
	<-started

	if got := atomic.LoadInt32(&maxActive); got < 2 {
		t.Fatalf("maxActive = %d, want >= 2", got)
	}

	close(release)

	if err := <-done; err != nil {
		t.Fatalf("runClusterWorkers error = %v", err)
	}
}

func TestBuildFusionInputUsesMergedClusterContextForOversizedExistingCluster(t *testing.T) {
	db := newDedupTestDB(t)
	userID, feedID := createDedupTestUserAndFeed(t, db)
	batch := NewBatchContext()

	clusterID := createSeedClusterArticle(t, db, userID, feedID, "existing-cluster-seed", "existing cluster summary", nil, false)
	if err := db.UpdateClusterMergedContent(clusterID, "Existing Merged Title", "Existing merged summary", "Existing merged content"); err != nil {
		t.Fatalf("UpdateClusterMergedContent error: %v", err)
	}
	if _, err := db.Exec(`UPDATE clusters SET article_count = ? WHERE id = ?`, 101, clusterID); err != nil {
		t.Fatalf("update cluster article_count error: %v", err)
	}
	if _, err := batch.EnsureClusterSnapshot(clusterID, db.GetClusterSnapshot); err != nil {
		t.Fatalf("EnsureClusterSnapshot error: %v", err)
	}

	oldArticles, err := db.GetArticlesByClusterID(clusterID)
	if err != nil {
		t.Fatalf("GetArticlesByClusterID error: %v", err)
	}
	if len(oldArticles) != 1 {
		t.Fatalf("old article count = %d, want 1", len(oldArticles))
	}
	if err := db.SetArticleContent(oldArticles[0].ID, "OLD RAW ARTICLE CONTENT SHOULD NOT APPEAR"); err != nil {
		t.Fatalf("SetArticleContent old article error: %v", err)
	}

	newArticleID := createDedupTestArticle(t, db, userID, feedID, "new-batch-article", false, "new article summary", nil)
	if err := db.SetArticleContent(newArticleID, "NEW ARTICLE CONTENT SHOULD APPEAR"); err != nil {
		t.Fatalf("SetArticleContent new article error: %v", err)
	}
	if err := db.UpdateArticleClusterID(newArticleID, clusterID); err != nil {
		t.Fatalf("UpdateArticleClusterID error: %v", err)
	}
	if err := db.UpdateClusterArticleCount(clusterID); err != nil {
		t.Fatalf("UpdateClusterArticleCount error: %v", err)
	}
	batch.RecordNewArticle(clusterID, newArticleID)

	cluster, err := db.GetClusterByID(clusterID)
	if err != nil {
		t.Fatalf("GetClusterByID error: %v", err)
	}
	if cluster == nil {
		t.Fatal("cluster = nil, want cluster")
	}

	articles, err := db.GetArticlesByClusterID(clusterID)
	if err != nil {
		t.Fatalf("GetArticlesByClusterID after new article error: %v", err)
	}

	input, err := buildFusionInput(*cluster, articles, db, &FusionConfig{Batch: batch})
	if err != nil {
		t.Fatalf("buildFusionInput error: %v", err)
	}
	if !strings.Contains(input, "Existing merged summary") || !strings.Contains(input, "Existing merged content") {
		t.Fatalf("compact fusion input missing existing merged cluster context: %q", input)
	}
	if !strings.Contains(input, "NEW ARTICLE CONTENT SHOULD APPEAR") {
		t.Fatalf("compact fusion input missing new article content: %q", input)
	}
	if strings.Contains(input, "OLD RAW ARTICLE CONTENT SHOULD NOT APPEAR") {
		t.Fatalf("compact fusion input should not include old raw article content: %q", input)
	}
}

func hashDedupTestTranslation(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
