package sqlite

import (
	"database/sql"
	"fmt"
	"strings"

	"MavenRSS/internal/models"
)

// GetClusterSnapshot returns the current cluster baseline used to evaluate
// batch-aware clustering limits before the current batch adds more articles.
func (db *DB) GetClusterSnapshot(clusterID int64) (*models.ClusterBatchSnapshot, error) {
	db.WaitForReady()

	var snapshot models.ClusterBatchSnapshot
	var mergedTitle sql.NullString
	var mergedSummary sql.NullString
	var mergedContent sql.NullString

	err := db.QueryRow(`
		SELECT
			c.id,
			c.article_count,
			COALESCE(SUM(LENGTH(COALESCE(a.summary, '')) + LENGTH(COALESCE(ac.content, ''))), 0),
			c.merged_title,
			c.merged_summary,
			c.merged_content
		FROM clusters c
		LEFT JOIN articles a ON a.cluster_id = c.id
		LEFT JOIN article_contents ac ON ac.article_id = a.id
		WHERE c.id = ?
		GROUP BY c.id, c.article_count, c.merged_title, c.merged_summary, c.merged_content
	`, clusterID).Scan(
		&snapshot.ClusterID,
		&snapshot.ExistingArticleCount,
		&snapshot.ExistingTotalChars,
		&mergedTitle,
		&mergedSummary,
		&mergedContent,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get cluster snapshot %d: %w", clusterID, err)
	}

	snapshot.MergedTitle = strings.TrimSpace(mergedTitle.String)
	snapshot.MergedSummary = strings.TrimSpace(mergedSummary.String)
	snapshot.MergedContent = strings.TrimSpace(mergedContent.String)

	if snapshot.MergedTitle == "" || snapshot.MergedSummary == "" || snapshot.MergedContent == "" {
		cluster, err := db.GetClusterByID(clusterID)
		if err != nil {
			return nil, fmt.Errorf("load cluster fallback snapshot %d: %w", clusterID, err)
		}
		if cluster != nil {
			if snapshot.MergedTitle == "" {
				snapshot.MergedTitle = strings.TrimSpace(cluster.MergedTitle)
			}
			if snapshot.MergedSummary == "" {
				snapshot.MergedSummary = strings.TrimSpace(cluster.MergedSummary)
			}
			if snapshot.MergedContent == "" {
				snapshot.MergedContent = strings.TrimSpace(cluster.MergedContent)
			}
		}
	}

	return &snapshot, nil
}
