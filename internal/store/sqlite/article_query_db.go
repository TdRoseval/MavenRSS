package sqlite

import (
	"MavenRSS/internal/models"
	"log"
)

// GetArticlesForAIBatchProcessing returns articles that need AI processing for a user.
// This includes: all favorited articles + unfavorited articles from the last 2 days.
// It returns ALL eligible articles regardless of whether they already have summaries,
// so the caller can decide what processing each article needs.
func (db *DB) GetArticlesForAIBatchProcessing(userID int64) ([]models.Article, error) {
	db.WaitForReady()

	query := `
		SELECT a.id, a.feed_id, a.title, a.url, a.image_url, a.audio_url, a.video_url,
			a.published_at, a.is_read, a.is_favorite, a.is_hidden, a.is_read_later,
			a.translated_title, a.summary, COALESCE(a.freshrss_item_id, ''), COALESCE(f.title, ''), COALESCE(a.author, '')
		FROM articles a
		LEFT JOIN feeds f ON a.feed_id = f.id
		WHERE a.user_id = ?
		AND (
			a.is_favorite = 1
			OR (a.is_favorite = 0 AND a.published_at >= datetime('now', '-2 days'))
		)
		ORDER BY a.published_at DESC
	`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		var a models.Article
		err := rows.Scan(
			&a.ID, &a.FeedID, &a.Title, &a.URL, &a.ImageURL, &a.AudioURL, &a.VideoURL,
			&a.PublishedAt, &a.IsRead, &a.IsFavorite, &a.IsHidden, &a.IsReadLater,
			&a.TranslatedTitle, &a.Summary, &a.FreshRSSItemID, &a.FeedTitle, &a.Author,
		)
		if err != nil {
			log.Printf("Error scanning article for AI batch: %v", err)
			continue
		}
		articles = append(articles, a)
	}

	return articles, rows.Err()
}
