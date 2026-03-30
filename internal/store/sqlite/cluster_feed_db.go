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

// GetClustersByVectorSimilarity retrieves clusters ranked by cosine similarity
// between the user's interest vector and each cluster's summary_embedding.
// It filters by user, excludes already-seen IDs, filters by recency (maxAgeDays),
// and returns at most topK results.
func (db *DB) GetClustersByVectorSimilarity(
	userID int64,
	interestVecBlob []byte,
	excludeIDs []int64,
	maxAgeDays int,
	topK int,
) ([]ClusterWithScore, error) {
	db.WaitForReady()

	if topK <= 0 {
		topK = 100
	}

	args := make([]interface{}, 0, len(excludeIDs)+4)
	args = append(args, userID, maxAgeDays)

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
			c.article_count, c.created_at, c.updated_at, c.is_read, c.is_favorite, c.is_read_later, c.is_hidden,
			ce.distance
		FROM cluster_embeddings ce
		JOIN clusters c ON ce.cluster_id = c.id
		WHERE c.user_id = ?
		  AND c.is_hidden = 0
		  AND c.status = 'complete'
		  AND c.updated_at >= datetime('now', '-' || ? || ' days')%s
		  AND ce.summary_embedding MATCH ? AND k = ?
		ORDER BY ce.distance
	`, excludeClause)

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
			&c.ArticleCount, &c.CreatedAt, &c.UpdatedAt, &c.IsRead, &c.IsFavorite, &c.IsReadLater, &c.IsHidden,
			&distance,
		); err != nil {
			log.Printf("Error scanning cluster in vector query: %v", err)
			continue
		}
		db.populateClusterMeta(&c)
		results = append(results, ClusterWithScore{
			Cluster:  c,
			Distance: distance,
		})
	}

	return results, nil
}

// GetRecentClustersChronological retrieves clusters in reverse chronological order,
// excluding already-seen IDs. Used as fallback when no interest vector exists.
func (db *DB) GetRecentClustersChronological(
	userID int64,
	excludeIDs []int64,
	limit int,
) ([]models.Cluster, error) {
	db.WaitForReady()

	if limit <= 0 {
		limit = 30
	}

	args := []interface{}{userID}
	excludeClause := ""
	if len(excludeIDs) > 0 {
		placeholders := make([]string, len(excludeIDs))
		for i, id := range excludeIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		excludeClause = fmt.Sprintf(" AND id NOT IN (%s)", strings.Join(placeholders, ","))
	}
	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT id, user_id, status, merged_title, merged_summary,
			article_count, created_at, updated_at, is_read, is_favorite, is_read_later, is_hidden
		FROM clusters
		WHERE user_id = ? AND is_hidden = 0 AND status = 'complete'%s
		ORDER BY updated_at DESC
		LIMIT ?
	`, excludeClause)

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
