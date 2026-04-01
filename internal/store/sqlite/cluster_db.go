package sqlite

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"MavenRSS/internal/models"
)

// CreateCluster creates a new article cluster and returns its ID.
func (db *DB) CreateCluster(userID int64, status string) (int64, error) {
	db.WaitForReady()
	result, err := db.Exec(
		`INSERT INTO clusters (user_id, status) VALUES (?, ?)`,
		userID, status,
	)
	if err != nil {
		return 0, fmt.Errorf("create cluster: %w", err)
	}
	return result.LastInsertId()
}

// GetClusterByID retrieves a cluster by its ID.
func (db *DB) GetClusterByID(clusterID int64) (*models.Cluster, error) {
	db.WaitForReady()
	var c models.Cluster
	err := db.QueryRow(`
		SELECT id, user_id, status, merged_title, merged_summary, merged_content,
recommendation_archive_date, recommendation_score, is_ai_recommended, recommendation_profile_id,
article_count, created_at, updated_at, is_read, is_favorite, is_read_later, is_hidden
FROM clusters WHERE id = ?
`, clusterID).Scan(
		&c.ID, &c.UserID, &c.Status, &c.MergedTitle, &c.MergedSummary, &c.MergedContent,
		&c.RecommendationArchiveDate, &c.RecommendationScore, &c.IsAIRecommended, &c.RecommendationProfileID,
		&c.ArticleCount, &c.CreatedAt, &c.UpdatedAt, &c.IsRead, &c.IsFavorite, &c.IsReadLater, &c.IsHidden,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	db.populateClusterMeta(&c)
	return &c, nil
}

// UpdateClusterStatus updates the status of a cluster.
func (db *DB) UpdateClusterStatus(clusterID int64, status string) error {
	db.WaitForReady()
	_, err := db.Exec(
		`UPDATE clusters SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), clusterID,
	)
	return err
}

// UpdateClusterMergedContent writes the AI-fused content to a cluster.
func (db *DB) UpdateClusterMergedContent(clusterID int64, title, summary, content string) error {
	db.WaitForReady()
	_, err := db.Exec(
		`UPDATE clusters SET merged_title = ?, merged_summary = ?, merged_content = ?, updated_at = ? WHERE id = ?`,
		title, summary, content, time.Now(), clusterID,
	)
	return err
}

// UpdateClusterArticleCount updates the article_count for a cluster.
func (db *DB) UpdateClusterArticleCount(clusterID int64) error {
	db.WaitForReady()
	_, err := db.Exec(
		`UPDATE clusters SET article_count = (SELECT COUNT(*) FROM articles WHERE cluster_id = ?), updated_at = ? WHERE id = ?`,
		clusterID, time.Now(), clusterID,
	)
	return err
}

// GetClustersByStatus retrieves clusters by status for a user.
func (db *DB) GetClustersByStatus(userID int64, status string) ([]models.Cluster, error) {
	db.WaitForReady()
	rows, err := db.Query(`
		SELECT id, user_id, status, merged_title, merged_summary,
recommendation_archive_date, recommendation_score, is_ai_recommended, recommendation_profile_id,
article_count, created_at, updated_at, is_read, is_favorite, is_read_later, is_hidden
FROM clusters WHERE user_id = ? AND status = ?
	`, userID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clusters []models.Cluster
	for rows.Next() {
		var c models.Cluster
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.Status, &c.MergedTitle, &c.MergedSummary,
			&c.RecommendationArchiveDate, &c.RecommendationScore, &c.IsAIRecommended, &c.RecommendationProfileID,
			&c.ArticleCount, &c.CreatedAt, &c.UpdatedAt, &c.IsRead, &c.IsFavorite, &c.IsReadLater, &c.IsHidden,
		); err != nil {
			log.Printf("Error scanning cluster: %v", err)
			continue
		}
		clusters = append(clusters, c)
	}
	return clusters, nil
}

// GetArticlesByClusterID retrieves all articles belonging to a cluster.
func (db *DB) GetArticlesByClusterID(clusterID int64) ([]models.Article, error) {
	db.WaitForReady()
	rows, err := db.Query(`
		SELECT a.id, a.feed_id, a.title, a.url, COALESCE(a.image_url, ''), a.published_at, a.summary, f.title, a.author, COALESCE(a.translated_title, '')
		FROM articles a
		LEFT JOIN feeds f ON a.feed_id = f.id
		WHERE a.cluster_id = ?
		ORDER BY a.published_at DESC
	`, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		var a models.Article
		var publishedAt sql.NullTime
		var summary, feedTitle, author sql.NullString
		if err := rows.Scan(&a.ID, &a.FeedID, &a.Title, &a.URL, &a.ImageURL, &publishedAt, &summary, &feedTitle, &author, &a.TranslatedTitle); err != nil {
			log.Printf("Error scanning cluster article: %v", err)
			continue
		}
		if publishedAt.Valid {
			a.PublishedAt = publishedAt.Time
		}
		a.Summary = summary.String
		a.FeedTitle = feedTitle.String
		a.Author = author.String
		a.ClusterID = clusterID
		articles = append(articles, a)
	}
	return articles, nil
}

// UpdateArticleClusterID assigns an article to a cluster.
func (db *DB) UpdateArticleClusterID(articleID, clusterID int64) error {
	db.WaitForReady()
	_, err := db.Exec(`UPDATE articles SET cluster_id = ? WHERE id = ?`, clusterID, articleID)
	return err
}

// UpdateArticleSimHash stores SimHash data for an article.
func (db *DB) UpdateArticleSimHash(articleID int64, hash64 int64, b1, b2, b3, b4 int16) error {
	db.WaitForReady()
	_, err := db.Exec(
		`UPDATE articles SET simhash_64 = ?, simhash_b1 = ?, simhash_b2 = ?, simhash_b3 = ?, simhash_b4 = ? WHERE id = ?`,
		hash64, b1, b2, b3, b4, articleID,
	)
	return err
}

// FindSimHashCandidates finds articles with matching SimHash bands (pigeonhole principle).
func (db *DB) FindSimHashCandidates(userID int64, b1, b2, b3, b4 int16) ([]struct {
	ArticleID int64
	SimHash64 int64
	ClusterID int64
}, error) {
	db.WaitForReady()
	rows, err := db.Query(`
		SELECT id, simhash_64, cluster_id FROM articles
		WHERE user_id = ? AND cluster_id IS NOT NULL
		AND (simhash_b1 = ? OR simhash_b2 = ? OR simhash_b3 = ? OR simhash_b4 = ?)
	`, userID, b1, b2, b3, b4)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []struct {
		ArticleID int64
		SimHash64 int64
		ClusterID int64
	}
	for rows.Next() {
		var c struct {
			ArticleID int64
			SimHash64 int64
			ClusterID int64
		}
		if err := rows.Scan(&c.ArticleID, &c.SimHash64, &c.ClusterID); err != nil {
			continue
		}
		candidates = append(candidates, c)
	}
	return candidates, nil
}

// FindSemanticCandidates uses sqlite-vec ANN search to find semantically similar articles.
func (db *DB) FindSemanticCandidates(userID int64, summaryEmbBlob []byte, topK int) ([]struct {
	ArticleID int64
	ClusterID int64
	Distance  float64
}, error) {
	db.WaitForReady()
	if topK <= 0 {
		topK = 10
	}
	rows, err := db.Query(`
		SELECT ae.article_id, a.cluster_id, ae.distance
		FROM article_embeddings ae
		JOIN articles a ON ae.article_id = a.id
		WHERE a.user_id = ? AND a.cluster_id IS NOT NULL
		AND ae.summary_embedding MATCH ? AND k = ?
		ORDER BY ae.distance
	`, userID, summaryEmbBlob, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []struct {
		ArticleID int64
		ClusterID int64
		Distance  float64
	}
	for rows.Next() {
		var r struct {
			ArticleID int64
			ClusterID int64
			Distance  float64
		}
		if err := rows.Scan(&r.ArticleID, &r.ClusterID, &r.Distance); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

// UpdateClusterEmbeddings stores embeddings for a cluster.
func (db *DB) UpdateClusterEmbeddings(clusterID int64, titleEmb, summaryEmb []byte) error {
	db.WaitForReady()
	_, _ = db.Exec(`DELETE FROM cluster_embeddings WHERE cluster_id = ?`, clusterID)
	_, err := db.Exec(
		`INSERT INTO cluster_embeddings (cluster_id, title_embedding, summary_embedding) VALUES (?, ?, ?)`,
		clusterID, titleEmb, summaryEmb,
	)
	return err
}

// GetClustersForUser retrieves clusters for a user with filtering and pagination.
func (db *DB) GetClustersForUser(userID int64, filter string, feedID int64, category string, limit, offset int) ([]models.Cluster, error) {
	db.WaitForReady()

	baseQuery := `SELECT DISTINCT c.id, c.user_id, c.status, c.merged_title, c.merged_summary,
recommendation_archive_date, recommendation_score, is_ai_recommended, recommendation_profile_id,
article_count, c.created_at, c.updated_at, c.is_read, c.is_favorite, c.is_read_later, c.is_hidden
FROM clusters c`
	args := []interface{}{}
	conditions := []string{"c.user_id = ?", "c.is_hidden = 0"}
	args = append(args, userID)

	if feedID > 0 || category != "" {
		baseQuery += `
JOIN articles a ON a.cluster_id = c.id
JOIN feeds f ON f.id = a.feed_id`
		if feedID > 0 {
			conditions = append(conditions, "a.feed_id = ?")
			args = append(args, feedID)
		}
		if category != "" {
			conditions = append(conditions, `(COALESCE(f.category, '') = ? OR COALESCE(f.category, '') LIKE ?)`)
			args = append(args, category, category+"/%")
		}
	}

	switch filter {
	case "unread":
		conditions = append(conditions, "c.is_read = 0")
	case "favorites":
		conditions = append(conditions, "c.is_favorite = 1")
	case "readLater":
		conditions = append(conditions, "c.is_read_later = 1")
	}

	baseQuery += " WHERE " + strings.Join(conditions, " AND ")
	baseQuery += " ORDER BY c.updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(baseQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clusters []models.Cluster
	for rows.Next() {
		var c models.Cluster
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.Status, &c.MergedTitle, &c.MergedSummary,
			&c.RecommendationArchiveDate, &c.RecommendationScore, &c.IsAIRecommended, &c.RecommendationProfileID,
			&c.ArticleCount, &c.CreatedAt, &c.UpdatedAt, &c.IsRead, &c.IsFavorite, &c.IsReadLater, &c.IsHidden,
		); err != nil {
			log.Printf("Error scanning cluster: %v", err)
			continue
		}
		db.populateClusterMeta(&c)
		clusters = append(clusters, c)
	}
	return clusters, nil
}

func chooseClusterDisplayTitle(mergedTitle, translatedTitle, articleTitle string) string {
	mergedTitle = strings.TrimSpace(mergedTitle)
	translatedTitle = strings.TrimSpace(translatedTitle)
	articleTitle = strings.TrimSpace(articleTitle)

	if translatedTitle != "" {
		return translatedTitle
	}
	if mergedTitle != "" {
		return mergedTitle
	}
	return articleTitle
}

// populateClusterMeta populates FeedTitles and Authors for a cluster.
func (db *DB) populateClusterMeta(c *models.Cluster) {
	rows, err := db.Query(`
		SELECT COALESCE(f.title, ''), COALESCE(a.author, ''), COALESCE(a.title, ''), COALESCE(a.translated_title, ''), COALESCE(a.image_url, '')
		FROM articles a
		LEFT JOIN feeds f ON a.feed_id = f.id
		WHERE a.cluster_id = ?
		ORDER BY a.published_at DESC, a.id DESC
	`, c.ID)
	if err != nil {
		return
	}
	defer rows.Close()

	feedSet := make(map[string]bool)
	authorSet := make(map[string]bool)
	articleCount := 0
	latestTitle := ""
	latestTranslatedTitle := ""
	c.ImageURL = ""
	for rows.Next() {
		var feedTitle, author, articleTitle, translatedTitle, imageURL string
		if err := rows.Scan(&feedTitle, &author, &articleTitle, &translatedTitle, &imageURL); err != nil {
			continue
		}
		articleCount++
		if articleCount == 1 {
			latestTitle = articleTitle
			latestTranslatedTitle = translatedTitle
		}
		if c.ImageURL == "" && imageURL != "" {
			c.ImageURL = imageURL
		}
		if feedTitle != "" && !feedSet[feedTitle] {
			feedSet[feedTitle] = true
			c.FeedTitles = append(c.FeedTitles, feedTitle)
		}
		if author != "" && !authorSet[author] {
			authorSet[author] = true
			c.Authors = append(c.Authors, author)
		}
	}

	if articleCount <= 1 {
		c.DisplayTitle = chooseClusterDisplayTitle(c.MergedTitle, latestTranslatedTitle, latestTitle)
		return
	}
	c.DisplayTitle = chooseClusterDisplayTitle(c.MergedTitle, "", latestTitle)
}

// MarkClusterRead marks a cluster as read/unread.
func (db *DB) MarkClusterRead(clusterID int64, read bool) error {
	db.WaitForReady()
	_, err := db.Exec(`UPDATE clusters SET is_read = ?, updated_at = ? WHERE id = ?`, read, time.Now(), clusterID)
	return err
}

// MarkAllClustersReadForUser marks all visible clusters as read for a user.
func (db *DB) MarkAllClustersReadForUser(userID int64, filter string, feedID int64, category string) error {
	db.WaitForReady()

	query := `UPDATE clusters SET is_read = 1, updated_at = ? WHERE id IN (
SELECT DISTINCT c.id
FROM clusters c`
	args := []interface{}{time.Now()}
	conditions := []string{"c.user_id = ?", "c.is_hidden = 0"}
	args = append(args, userID)

	if feedID > 0 || category != "" {
		query += `
JOIN articles a ON a.cluster_id = c.id
JOIN feeds f ON f.id = a.feed_id`
		if feedID > 0 {
			conditions = append(conditions, "a.feed_id = ?")
			args = append(args, feedID)
		}
		if category != "" {
			conditions = append(conditions, `(COALESCE(f.category, '') = ? OR COALESCE(f.category, '') LIKE ?)`)
			args = append(args, category, category+"/%")
		}
	}

	switch filter {
	case "unread":
		conditions = append(conditions, "c.is_read = 0")
	case "favorites":
		conditions = append(conditions, "c.is_favorite = 1")
	case "readLater":
		conditions = append(conditions, "c.is_read_later = 1")
	}

	query += " WHERE " + strings.Join(conditions, " AND ") + ")"

	_, err := db.Exec(query, args...)
	return err
}

// ToggleClusterFavorite toggles the favorite status of a cluster.
func (db *DB) ToggleClusterFavorite(clusterID int64) error {
	db.WaitForReady()
	_, err := db.Exec(`UPDATE clusters SET is_favorite = 1 - is_favorite, updated_at = ? WHERE id = ?`, time.Now(), clusterID)
	return err
}

// SetClusterFavorite sets the favorite status of a cluster.
func (db *DB) SetClusterFavorite(clusterID int64, favorite bool) error {
	db.WaitForReady()
	_, err := db.Exec(`UPDATE clusters SET is_favorite = ?, updated_at = ? WHERE id = ?`, favorite, time.Now(), clusterID)
	return err
}

// ToggleClusterReadLater toggles the read-later status of a cluster.
func (db *DB) ToggleClusterReadLater(clusterID int64) error {
	db.WaitForReady()
	_, err := db.Exec(`
		UPDATE clusters
		SET is_read_later = 1 - is_read_later,
			is_read = CASE WHEN is_read_later = 0 THEN 0 ELSE is_read END,
			updated_at = ?
		WHERE id = ?
	`, time.Now(), clusterID)
	return err
}
