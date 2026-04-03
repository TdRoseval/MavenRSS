package dedup

import (
	"context"
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
