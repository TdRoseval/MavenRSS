package sqlite

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"MavenRSS/internal/models"
)

const defaultClusterTitleTranslationProvider = "google"
const defaultClusterTitleTargetLanguage = "zh"

// ResolveArticleTitleForCluster returns the best title to use for a clustered article.
// Priority:
// 1. Persisted translated_title
// 2. Cached translation for feeds that require translation
// 3. Original title
func (db *DB) ResolveArticleTitleForCluster(userID int64, article models.Article) string {
	title := strings.TrimSpace(article.TranslatedTitle)
	if title != "" {
		return title
	}

	title = strings.TrimSpace(article.Title)
	if title == "" || userID <= 0 || article.ID <= 0 || article.FeedID <= 0 {
		return title
	}

	feed, err := db.GetFeedByIDForUser(userID, article.FeedID)
	if err != nil || feed == nil || !feed.TranslateArticles {
		return title
	}

	targetLang, _ := db.GetSettingWithFallback(userID, "target_language")
	targetLang = strings.TrimSpace(targetLang)
	if targetLang == "" {
		targetLang = defaultClusterTitleTargetLanguage
	}

	provider, _ := db.GetSettingWithFallback(userID, "translation_provider")
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = defaultClusterTitleTranslationProvider
	}

	cachedTitle, found, err := db.GetCachedTranslation(hashClusterTitleTranslation(title), targetLang, provider)
	if err != nil || !found {
		return title
	}

	cachedTitle = strings.TrimSpace(cachedTitle)
	if cachedTitle == "" {
		return title
	}

	if updateErr := db.UpdateArticleTranslation(article.ID, cachedTitle); updateErr == nil {
		article.TranslatedTitle = cachedTitle
	}

	return cachedTitle
}

func hashClusterTitleTranslation(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}
