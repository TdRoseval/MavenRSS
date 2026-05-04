package sqlite_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"MavenRSS/internal/models"
)

func TestGetClusterByIDUsesCachedTranslatedTitleForSingleArticleDisplay(t *testing.T) {
	db := setupTestDB(t)

	userID, err := db.CreateUser(&models.User{
		Username:     "cluster-title-cache-user",
		Email:        "cluster-title-cache@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser error = %v", err)
	}

	feedID, err := db.AddFeedForUser(userID, &models.Feed{
		Title:             "Translated Feed",
		URL:               "https://example.com/translated-feed",
		Type:              "rss",
		RefreshInterval:   60,
		TranslateArticles: true,
	})
	if err != nil {
		t.Fatalf("AddFeedForUser error = %v", err)
	}

	if err := db.SetSettingForUser(userID, "target_language", "zh"); err != nil {
		t.Fatalf("SetSettingForUser target_language error = %v", err)
	}
	if err := db.SetSettingForUser(userID, "translation_provider", "google"); err != nil {
		t.Fatalf("SetSettingForUser translation_provider error = %v", err)
	}

	const originalTitle = "Single article cluster title"
	const cachedTitle = "单篇聚类标题缓存"

	articleResult, err := db.Exec(
		`INSERT INTO articles (user_id, feed_id, title, url, published_at, summary, unique_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, feedID, originalTitle, "https://example.com/article", time.Now(), "summary", "cluster-title-cache-1",
	)
	if err != nil {
		t.Fatalf("insert article error = %v", err)
	}
	articleID, err := articleResult.LastInsertId()
	if err != nil {
		t.Fatalf("article LastInsertId error = %v", err)
	}

	if err := db.SetCachedTranslation(hashClusterTitleCacheTest(originalTitle), originalTitle, "zh", cachedTitle, "google"); err != nil {
		t.Fatalf("SetCachedTranslation error = %v", err)
	}

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
	if err := db.UpdateClusterMergedContent(clusterID, originalTitle, "summary", "content"); err != nil {
		t.Fatalf("UpdateClusterMergedContent error = %v", err)
	}

	cluster, err := db.GetClusterByID(clusterID)
	if err != nil {
		t.Fatalf("GetClusterByID error = %v", err)
	}
	if cluster == nil {
		t.Fatal("GetClusterByID returned nil cluster")
	}
	if cluster.DisplayTitle != cachedTitle {
		t.Fatalf("DisplayTitle = %q, want cached translated title %q", cluster.DisplayTitle, cachedTitle)
	}

	article, err := db.GetArticleByID(articleID)
	if err != nil {
		t.Fatalf("GetArticleByID error = %v", err)
	}
	if article == nil {
		t.Fatal("GetArticleByID returned nil article")
	}
	if article.TranslatedTitle != cachedTitle {
		t.Fatalf("TranslatedTitle = %q, want cached translated title %q", article.TranslatedTitle, cachedTitle)
	}
}

func hashClusterTitleCacheTest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
