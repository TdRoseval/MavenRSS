package sqlite

import (
	"fmt"
	"strings"
)

// BackfillEmptyClusterMergedContent populates empty cluster title/summary/content fields
// from the latest source article so the cluster remains readable even if fusion output
// has not been persisted yet.
func (db *DB) BackfillEmptyClusterMergedContent(userID int64) (int, error) {
	db.WaitForReady()
	if userID <= 0 {
		return 0, nil
	}

	rows, err := db.Query(`
		SELECT id, merged_title, merged_summary, merged_content
		FROM clusters
		WHERE user_id = ?
		  AND (
			TRIM(COALESCE(merged_title, '')) = ''
			OR TRIM(COALESCE(merged_summary, '')) = ''
			OR TRIM(COALESCE(merged_content, '')) = ''
		  )
		ORDER BY id ASC
	`, userID)
	if err != nil {
		return 0, fmt.Errorf("query clusters needing merged-content backfill: %w", err)
	}
	defer rows.Close()

	type clusterRow struct {
		id      int64
		title   string
		summary string
		content string
	}

	pending := make([]clusterRow, 0)
	for rows.Next() {
		var row clusterRow
		if err := rows.Scan(&row.id, &row.title, &row.summary, &row.content); err != nil {
			return 0, fmt.Errorf("scan cluster needing merged-content backfill: %w", err)
		}
		pending = append(pending, row)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate clusters needing merged-content backfill: %w", err)
	}

	updated := 0
	for _, row := range pending {
		title, summary, content, ok, err := db.buildClusterFallbackFields(userID, row.id)
		if err != nil {
			return updated, err
		}
		if !ok {
			continue
		}

		nextTitle := strings.TrimSpace(row.title)
		if nextTitle == "" {
			nextTitle = title
		}
		nextSummary := strings.TrimSpace(row.summary)
		if nextSummary == "" {
			nextSummary = summary
		}
		nextContent := strings.TrimSpace(row.content)
		if nextContent == "" {
			nextContent = content
		}

		if nextTitle == strings.TrimSpace(row.title) &&
			nextSummary == strings.TrimSpace(row.summary) &&
			nextContent == strings.TrimSpace(row.content) {
			continue
		}

		if err := db.UpdateClusterMergedContent(row.id, nextTitle, nextSummary, nextContent); err != nil {
			return updated, fmt.Errorf("update fallback merged content for cluster %d: %w", row.id, err)
		}
		updated++
	}

	return updated, nil
}

// SyncClusterFavoriteStatesFromArticles repairs cluster favorite flags from member
// articles without clearing clusters the user explicitly favorited.
//
// Deprecated: This full-table repair is kept only for periodic maintenance.
// Per-article favorite changes should call SyncClusterFavoriteByArticleID instead
// to avoid running a write transaction on every cluster list request.
func (db *DB) SyncClusterFavoriteStatesFromArticles(userID int64) error {
	db.WaitForReady()
	if userID <= 0 {
		return nil
	}

	_, err := db.Exec(`
		UPDATE clusters
		SET is_favorite = CASE
			WHEN EXISTS (
				SELECT 1
				FROM articles a
				WHERE a.cluster_id = clusters.id AND a.is_favorite = 1
			) THEN 1
			ELSE is_favorite
		END
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return fmt.Errorf("sync cluster favorite states: %w", err)
	}

	return nil
}

// SyncClusterFavoriteByArticleID propagates an article favorite change to its
// cluster in an event-driven manner. When an article is favorited, the owning
// cluster is marked favorite. Unfavoriting an article does not clear the
// cluster flag because other member articles (or explicit user action) may
// still keep it favorited — the full repair remains available via
// SyncClusterFavoriteStatesFromArticles for periodic reconciliation.
//
// This avoids the per-list-request full-table UPDATE that previously ran on
// every cluster feed fetch.
func (db *DB) SyncClusterFavoriteByArticleID(articleID int64, favorite bool) error {
	db.WaitForReady()
	if articleID <= 0 {
		return nil
	}
	if !favorite {
		// Only favoriting propagates upward; unfavorite requires checking
		// sibling articles which is handled by the periodic full repair.
		return nil
	}

	_, err := db.Exec(`
		UPDATE clusters
		SET is_favorite = 1
		WHERE id = (SELECT cluster_id FROM articles WHERE id = ?)
		  AND cluster_id IS NOT NULL
	`, articleID)
	if err != nil {
		return fmt.Errorf("sync cluster favorite by article %d: %w", articleID, err)
	}
	return nil
}

func (db *DB) buildClusterFallbackFields(userID, clusterID int64) (string, string, string, bool, error) {
	if userID <= 0 || clusterID <= 0 {
		return "", "", "", false, nil
	}

	articles, err := db.GetArticlesByClusterID(clusterID)
	if err != nil {
		return "", "", "", false, fmt.Errorf("get articles for cluster %d fallback: %w", clusterID, err)
	}
	if len(articles) == 0 {
		return "", "", "", false, nil
	}

	article := articles[0]
	title := db.ResolveArticleTitleForCluster(userID, article)

	summary := strings.TrimSpace(article.Summary)
	content, _, err := db.GetArticleContent(article.ID)
	if err != nil {
		return "", "", "", false, fmt.Errorf("get article %d content for cluster fallback: %w", article.ID, err)
	}
	content = strings.TrimSpace(content)

	if content == "" {
		content = summary
	}
	if summary == "" {
		summary = title
	}
	if content == "" {
		content = summary
	}
	if title == "" && summary == "" && content == "" {
		return "", "", "", false, nil
	}

	return title, summary, content, true, nil
}
