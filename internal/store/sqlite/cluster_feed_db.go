package sqlite

import (
	"fmt"
	"log"
	"strings"

	"MavenRSS/internal/models"
)

// ClusterWithScore holds a cluster and its similarity score for ranking.
type ClusterWithScore struct {
	Cluster  models.Cluster
	Distance float64
}

// GetClustersByVectorSimilarity retrieves clusters ranked by sqlite-vec's default
// squared L2 distance between normalized user interest vectors and cluster summary embeddings.
// It filters by user, excludes already-seen IDs, filters by recency (maxAgeDays),
// and returns at most topK results.
func (db *DB) GetClustersByVectorSimilarity(
	userID int64,
	interestVecBlob []byte,
	excludeIDs []int64,
	filter string,
	feedID int64,
	category string,
	maxAgeDays int,
	topK int,
) ([]ClusterWithScore, error) {
	db.WaitForReady()

	if topK <= 0 {
		topK = 100
	}

	args := make([]interface{}, 0, len(excludeIDs)+8)
	args = append(args, userID, maxAgeDays)

	joinClause := ""
	if feedID > 0 || category != "" {
		joinClause = `
		JOIN articles a ON a.cluster_id = c.id
		JOIN feeds f ON f.id = a.feed_id`
	}

	conditions := []string{
		"c.user_id = ?",
		"c.is_hidden = 0",
		"c.status = 'complete'",
		"c.updated_at >= datetime('now', '-' || ? || ' days')",
	}

	switch filter {
	case "unread":
		conditions = append(conditions, "c.is_read = 0")
	case "favorites":
		conditions = append(conditions, "c.is_favorite = 1")
	case "readLater":
		conditions = append(conditions, "c.is_read_later = 1")
	}

	if feedID > 0 {
		conditions = append(conditions, "a.feed_id = ?")
		args = append(args, feedID)
	}
	if category != "" {
		conditions = append(conditions, "(COALESCE(f.category, '') = ? OR COALESCE(f.category, '') LIKE ?)")
		args = append(args, category, category+"/%")
	}

	excludeClause := ""
	if len(excludeIDs) > 0 {
		placeholders := make([]string, len(excludeIDs))
		for i, id := range excludeIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		excludeClause = fmt.Sprintf(" AND c.id NOT IN (%s)", strings.Join(placeholders, ","))
	}

	args = append(args, interestVecBlob, topK)

	query := fmt.Sprintf(`
		SELECT c.id, c.user_id, c.status, c.merged_title, c.merged_summary,
c.recommendation_archive_date, c.recommendation_score, c.is_ai_recommended, c.recommendation_profile_id,
c.article_count, c.created_at, c.updated_at, c.is_read, c.is_favorite, c.is_read_later, c.is_hidden,
ce.distance
		FROM cluster_embeddings ce
		JOIN clusters c ON ce.cluster_id = c.id%s
		WHERE %s%s
		  AND ce.summary_embedding MATCH ? AND k = ?
		ORDER BY ce.distance
	`, joinClause, strings.Join(conditions, " AND "), excludeClause)

	clusterRows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("vector similarity query: %w", err)
	}
	defer clusterRows.Close()

	var results []ClusterWithScore
	for clusterRows.Next() {
		var c models.Cluster
		var distance float64
		if err := clusterRows.Scan(
			&c.ID, &c.UserID, &c.Status, &c.MergedTitle, &c.MergedSummary,
			&c.RecommendationArchiveDate, &c.RecommendationScore, &c.IsAIRecommended, &c.RecommendationProfileID,
			&c.ArticleCount, &c.CreatedAt, &c.UpdatedAt, &c.IsRead, &c.IsFavorite, &c.IsReadLater, &c.IsHidden,
			&distance,
		); err != nil {
			log.Printf("Error scanning cluster in vector query: %v", err)
			continue
		}
		results = append(results, ClusterWithScore{
			Cluster:  c,
			Distance: distance,
		})
	}

	// Note: meta population (feed titles, authors, display title) is deferred
	// to the caller so that only the final selected clusters (after pruning,
	// scoring, and trimming) incur the batch JOIN cost — not every recalled
	// candidate (which can be up to 500).
	return results, nil
}

// GetRecentClustersChronological retrieves clusters in reverse chronological order,
// excluding already-seen IDs. Used as fallback when no interest vector exists.
func (db *DB) GetRecentClustersChronological(
	userID int64,
	excludeIDs []int64,
	filter string,
	feedID int64,
	category string,
	limit int,
) ([]models.Cluster, error) {
	db.WaitForReady()

	if limit <= 0 {
		limit = 30
	}

	args := []interface{}{userID}
	baseQuery := `FROM clusters c`
	conditions := []string{"c.user_id = ?", "c.is_hidden = 0", "c.status = 'complete'"}

	if feedID > 0 || category != "" {
		baseQuery += `
		JOIN articles a ON a.cluster_id = c.id
		JOIN feeds f ON f.id = a.feed_id`
	}

	switch filter {
	case "unread":
		conditions = append(conditions, "c.is_read = 0")
	case "favorites":
		conditions = append(conditions, "c.is_favorite = 1")
	case "readLater":
		conditions = append(conditions, "c.is_read_later = 1")
	}

	if feedID > 0 {
		conditions = append(conditions, "a.feed_id = ?")
		args = append(args, feedID)
	}
	if category != "" {
		conditions = append(conditions, `(COALESCE(f.category, '') = ? OR COALESCE(f.category, '') LIKE ?)`)
		args = append(args, category, category+"/%")
	}

	excludeClause := ""
	if len(excludeIDs) > 0 {
		placeholders := make([]string, len(excludeIDs))
		for i, id := range excludeIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		excludeClause = fmt.Sprintf(" AND c.id NOT IN (%s)", strings.Join(placeholders, ","))
	}
	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT DISTINCT c.id, c.user_id, c.status, c.merged_title, c.merged_summary,
c.recommendation_archive_date, c.recommendation_score, c.is_ai_recommended, c.recommendation_profile_id,
c.article_count, c.created_at, c.updated_at, c.is_read, c.is_favorite, c.is_read_later, c.is_hidden
		%s
		WHERE %s%s
		ORDER BY c.updated_at DESC
		LIMIT ?
	`, baseQuery, strings.Join(conditions, " AND "), excludeClause)

	rows, err := db.Query(query, args...)
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
	db.PopulateClustersMeta(clusters)
	return clusters, nil
}

// GetClusterEmbedding retrieves a specific embedding column for a cluster.
// column should be "title_embedding" or "summary_embedding".
func (db *DB) GetClusterEmbedding(clusterID int64, column string) ([]byte, error) {
	db.WaitForReady()
	// Validate column name to prevent SQL injection
	if column != "title_embedding" && column != "summary_embedding" {
		return nil, fmt.Errorf("invalid embedding column: %s", column)
	}
	var blob []byte
	err := db.QueryRow(
		fmt.Sprintf(`SELECT %s FROM cluster_embeddings WHERE cluster_id = ?`, column),
		clusterID,
	).Scan(&blob)
	if err != nil {
		return nil, err
	}
	return blob, nil
}
