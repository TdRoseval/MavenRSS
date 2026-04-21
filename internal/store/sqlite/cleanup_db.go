package sqlite

import (
	"database/sql"
	"log"
	"strconv"
	"time"
)

// CleanupOldArticles removes articles based on age and status.
// - Articles older than configured days: delete except favorited or read later
// - Also checks database size against max_cache_size_mb setting
// If userID > 0, only clean up for that user; otherwise clean up for all users
func (db *DB) CleanupOldArticles(userID int64) (int64, error) {
	db.WaitForReady()

	totalDeleted := int64(0)

	// Step 1: Clean up by age (existing logic)
	maxAgeDaysStr, err := db.GetSetting("max_article_age_days")
	maxAgeDays := 30
	if err == nil {
		if days, err := strconv.Atoi(maxAgeDaysStr); err == nil && days > 0 {
			maxAgeDays = days
		}
	}

	cutoffDate := time.Now().AddDate(0, 0, -maxAgeDays)

	// Delete articles older than configured age that are not favorited or in read later
	var result sql.Result
	if userID > 0 {
		result, err = db.Exec(`
			DELETE FROM articles
			WHERE published_at < ?
			AND is_favorite = 0
			AND is_read_later = 0
			AND cluster_id IS NULL
			AND user_id = ?
		`, cutoffDate, userID)
	} else {
		result, err = db.Exec(`
			DELETE FROM articles
			WHERE published_at < ?
			AND is_favorite = 0
			AND is_read_later = 0
			AND cluster_id IS NULL
		`, cutoffDate)
	}
	if err != nil {
		return 0, err
	}

	count, _ := result.RowsAffected()
	totalDeleted += count

	clusterDeleted, err := db.CleanupExpiredClusters(userID, maxAgeDays)
	if err != nil {
		log.Printf("Error during cluster cleanup: %v", err)
	} else {
		totalDeleted += clusterDeleted
	}

	// Step 2: Check database size and clean up if over limit
	sizeDeleted, err := db.CleanupBySize(userID)
	if err != nil {
		log.Printf("Error during size-based cleanup: %v", err)
	} else {
		totalDeleted += sizeDeleted
	}

	// Also cleanup related caches with the same age limit
	_, _ = db.CleanupTranslationCache(maxAgeDays, userID)
	_, _ = db.CleanupOldArticleContents(maxAgeDays, userID)

	// Run VACUUM to reclaim space
	_, _ = db.Exec("VACUUM")

	return totalDeleted, nil
}

// CleanupAllArticleContents removes all cached article contents for a specific user.
// Manual cache clearing should remove both clustered and unclustered article content.
// If userID is 0, removes all (legacy behavior).
func (db *DB) CleanupAllArticleContents(userID int64) (int64, error) {
	db.WaitForReady()
	var result sql.Result
	var err error
	if userID > 0 {
		result, err = db.Exec(`
			DELETE FROM article_contents
			WHERE article_id IN (
				SELECT id FROM articles
				WHERE user_id = ?
			)
		`, userID)
	} else {
		result, err = db.Exec(`
			DELETE FROM article_contents
			WHERE article_id IN (
				SELECT id FROM articles
			)
		`)
	}
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

type ManualArticleCleanupStats struct {
	DeletedArticles  int64
	DeletedClusters  int64
	RetainedArticles int64
	RetainedClusters int64
}

// CleanupArticleCachePreservingFavorites performs manual article cleanup while keeping
// favorited standalone articles and any clusters that are either explicitly favorited
// or contain favorited articles. All other standalone articles and clusters are removed.
func (db *DB) CleanupArticleCachePreservingFavorites(userID int64) (ManualArticleCleanupStats, error) {
	db.WaitForReady()

	stats := ManualArticleCleanupStats{}

	retainedArticles, err := db.countRetainedArticlesForManualCleanup(userID)
	if err != nil {
		return stats, err
	}
	stats.RetainedArticles = retainedArticles

	retainedClusters, err := db.countRetainedClustersForManualCleanup(userID)
	if err != nil {
		return stats, err
	}
	stats.RetainedClusters = retainedClusters

	deletedStandaloneArticles, err := db.deleteUnclusteredNonFavoriteArticlesForManualCleanup(userID)
	if err != nil {
		return stats, err
	}
	stats.DeletedArticles += deletedStandaloneArticles

	clusterIDs, err := db.listDeletableClusterIDsForManualCleanup(userID)
	if err != nil {
		return stats, err
	}

	for _, clusterID := range clusterIDs {
		var clusterArticleCount int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM articles WHERE cluster_id = ?`, clusterID).Scan(&clusterArticleCount); err != nil {
			return stats, err
		}
		if err := db.DeleteClusterAndArticles(clusterID); err != nil {
			return stats, err
		}
		stats.DeletedClusters++
		stats.DeletedArticles += clusterArticleCount
	}

	_, _ = db.Exec("VACUUM")
	return stats, nil
}

func (db *DB) countRetainedArticlesForManualCleanup(userID int64) (int64, error) {
	db.WaitForReady()

	var count int64
	if userID > 0 {
		err := db.QueryRow(`
			SELECT COUNT(*)
			FROM articles a
			WHERE a.user_id = ?
			  AND (
				a.is_favorite = 1
				OR EXISTS (
					SELECT 1
					FROM clusters c
					WHERE c.id = a.cluster_id
					  AND c.user_id = a.user_id
					  AND (
						c.is_favorite = 1
						OR EXISTS (
							SELECT 1
							FROM articles fa
							WHERE fa.cluster_id = c.id
							  AND fa.is_favorite = 1
						)
					  )
				)
			  )
		`, userID).Scan(&count)
		return count, err
	}

	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM articles a
		WHERE a.is_favorite = 1
		   OR EXISTS (
				SELECT 1
				FROM clusters c
				WHERE c.id = a.cluster_id
				  AND (
					c.is_favorite = 1
					OR EXISTS (
						SELECT 1
						FROM articles fa
						WHERE fa.cluster_id = c.id
						  AND fa.is_favorite = 1
					)
				  )
			)
	`).Scan(&count)
	return count, err
}

func (db *DB) countRetainedClustersForManualCleanup(userID int64) (int64, error) {
	db.WaitForReady()

	var count int64
	if userID > 0 {
		err := db.QueryRow(`
			SELECT COUNT(*)
			FROM clusters c
			WHERE c.user_id = ?
			  AND (
				c.is_favorite = 1
				OR EXISTS (
					SELECT 1
					FROM articles a
					WHERE a.cluster_id = c.id
					  AND a.is_favorite = 1
				)
			  )
		`, userID).Scan(&count)
		return count, err
	}

	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM clusters c
		WHERE c.is_favorite = 1
		   OR EXISTS (
				SELECT 1
				FROM articles a
				WHERE a.cluster_id = c.id
				  AND a.is_favorite = 1
			)
	`).Scan(&count)
	return count, err
}

func (db *DB) listDeletableClusterIDsForManualCleanup(userID int64) ([]int64, error) {
	db.WaitForReady()

	var (
		rows *sql.Rows
		err  error
	)

	if userID > 0 {
		rows, err = db.Query(`
			SELECT c.id
			FROM clusters c
			WHERE c.user_id = ?
			  AND NOT (
				c.is_favorite = 1
				OR EXISTS (
					SELECT 1
					FROM articles a
					WHERE a.cluster_id = c.id
					  AND a.is_favorite = 1
				)
			  )
			ORDER BY c.id ASC
		`, userID)
	} else {
		rows, err = db.Query(`
			SELECT c.id
			FROM clusters c
			WHERE NOT (
				c.is_favorite = 1
				OR EXISTS (
					SELECT 1
					FROM articles a
					WHERE a.cluster_id = c.id
					  AND a.is_favorite = 1
				)
			)
			ORDER BY c.id ASC
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	clusterIDs := make([]int64, 0)
	for rows.Next() {
		var clusterID int64
		if err := rows.Scan(&clusterID); err != nil {
			return nil, err
		}
		clusterIDs = append(clusterIDs, clusterID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return clusterIDs, nil
}

func (db *DB) deleteUnclusteredNonFavoriteArticlesForManualCleanup(userID int64) (int64, error) {
	db.WaitForReady()

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if userID > 0 {
		if _, err := tx.Exec(`
			DELETE FROM article_embeddings
			WHERE article_id IN (
				SELECT id
				FROM articles
				WHERE user_id = ?
				  AND cluster_id IS NULL
				  AND is_favorite = 0
			)
		`, userID); err != nil {
			return 0, err
		}
	} else {
		if _, err := tx.Exec(`
			DELETE FROM article_embeddings
			WHERE article_id IN (
				SELECT id
				FROM articles
				WHERE cluster_id IS NULL
				  AND is_favorite = 0
			)
		`); err != nil {
			return 0, err
		}
	}

	var result sql.Result
	if userID > 0 {
		result, err = tx.Exec(`
			DELETE FROM articles
			WHERE user_id = ?
			  AND cluster_id IS NULL
			  AND is_favorite = 0
		`, userID)
	} else {
		result, err = tx.Exec(`
			DELETE FROM articles
			WHERE cluster_id IS NULL
			  AND is_favorite = 0
		`)
	}
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// DeleteAllArticles removes ALL articles from the database for a specific user
// This keeps feeds, settings, and other metadata intact
// If userID is 0, removes all (legacy behavior)
func (db *DB) DeleteAllArticles(userID int64) (int64, error) {
	db.WaitForReady()

	clusterQuery := `SELECT id FROM clusters WHERE is_ai_recommended = 0`
	clusterArgs := []interface{}{}
	if userID > 0 {
		clusterQuery += ` AND user_id = ?`
		clusterArgs = append(clusterArgs, userID)
	}

	rows, err := db.Query(clusterQuery, clusterArgs...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var clusterIDs []int64
	for rows.Next() {
		var clusterID int64
		if err := rows.Scan(&clusterID); err != nil {
			return 0, err
		}
		clusterIDs = append(clusterIDs, clusterID)
	}

	deletedArticles := int64(0)
	for _, clusterID := range clusterIDs {
		var clusterArticleCount int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM articles WHERE cluster_id = ?`, clusterID).Scan(&clusterArticleCount); err != nil {
			return deletedArticles, err
		}
		if err := db.DeleteClusterAndArticles(clusterID); err != nil {
			return deletedArticles, err
		}
		deletedArticles += clusterArticleCount
	}

	var unclusteredIDsQuery string
	var unclusteredArgs []interface{}
	if userID > 0 {
		unclusteredIDsQuery = `SELECT id FROM articles WHERE user_id = ? AND cluster_id IS NULL`
		unclusteredArgs = append(unclusteredArgs, userID)
	} else {
		unclusteredIDsQuery = `SELECT id FROM articles WHERE cluster_id IS NULL`
	}

	unclusteredRows, err := db.Query(unclusteredIDsQuery, unclusteredArgs...)
	if err != nil {
		return deletedArticles, err
	}
	defer unclusteredRows.Close()

	var unclusteredIDs []int64
	for unclusteredRows.Next() {
		var articleID int64
		if err := unclusteredRows.Scan(&articleID); err != nil {
			return deletedArticles, err
		}
		unclusteredIDs = append(unclusteredIDs, articleID)
	}

	if len(unclusteredIDs) == 0 {
		return deletedArticles, nil
	}

	for _, articleID := range unclusteredIDs {
		if _, err := db.Exec(`DELETE FROM article_embeddings WHERE article_id = ?`, articleID); err != nil {
			return deletedArticles, err
		}
	}

	var result sql.Result
	if userID > 0 {
		result, err = db.Exec(`DELETE FROM articles WHERE user_id = ? AND cluster_id IS NULL`, userID)
	} else {
		result, err = db.Exec(`DELETE FROM articles WHERE cluster_id IS NULL`)
	}
	if err != nil {
		return deletedArticles, err
	}

	count, _ := result.RowsAffected()
	deletedArticles += count

	return deletedArticles, nil
}

// DeleteArticlesForFeed removes all articles for a specific feed
// This keeps the feed itself intact
// If userID is 0, ignores user filtering (legacy behavior)
func (db *DB) DeleteArticlesForFeed(feedID int64, userID int64) (int64, error) {
	db.WaitForReady()
	var result sql.Result
	var err error

	// First delete article contents for the feed's articles
	if userID > 0 {
		_, err = db.Exec(`
			DELETE FROM article_contents
			WHERE article_id IN (
				SELECT id FROM articles WHERE feed_id = ? AND user_id = ? AND cluster_id IS NULL
			)
		`, feedID, userID)
	} else {
		_, err = db.Exec(`
			DELETE FROM article_contents
			WHERE article_id IN (
				SELECT id FROM articles WHERE feed_id = ? AND cluster_id IS NULL
			)
		`, feedID)
	}

	if err != nil {
		return 0, err
	}

	// Delete standalone embeddings for the feed's articles
	if userID > 0 {
		_, err = db.Exec(`
			DELETE FROM article_embeddings
			WHERE article_id IN (
				SELECT id FROM articles WHERE feed_id = ? AND user_id = ? AND cluster_id IS NULL
			)
		`, feedID, userID)
	} else {
		_, err = db.Exec(`
			DELETE FROM article_embeddings
			WHERE article_id IN (
				SELECT id FROM articles WHERE feed_id = ? AND cluster_id IS NULL
			)
		`, feedID)
	}
	if err != nil {
		return 0, err
	}

	// Then delete the articles themselves
	if userID > 0 {
		result, err = db.Exec(`DELETE FROM articles WHERE feed_id = ? AND user_id = ? AND cluster_id IS NULL`, feedID, userID)
	} else {
		result, err = db.Exec(`DELETE FROM articles WHERE feed_id = ? AND cluster_id IS NULL`, feedID)
	}

	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// CleanupUnimportantArticles removes all articles except read, favorited, and read later ones for a specific user
// If userID is 0, removes all (legacy behavior)
func (db *DB) CleanupUnimportantArticles(userID int64) (int64, error) {
	db.WaitForReady()

	var result sql.Result
	var err error
	if userID > 0 {
		result, err = db.Exec(`
			DELETE FROM articles
			WHERE user_id = ?
			AND is_read = 0
			AND is_favorite = 0
			AND is_read_later = 0
			AND cluster_id IS NULL
		`, userID)
	} else {
		result, err = db.Exec(`
			DELETE FROM articles
			WHERE is_read = 0
			AND is_favorite = 0
			AND is_read_later = 0
			AND cluster_id IS NULL
		`)
	}
	if err != nil {
		return 0, err
	}

	count, _ := result.RowsAffected()

	// Also cleanup related caches (remove entries older than 7 days)
	_, _ = db.CleanupTranslationCache(7, userID)
	_, _ = db.CleanupOldArticleContents(7, userID)

	// Run VACUUM to reclaim space
	_, _ = db.Exec("VACUUM")

	return count, nil
}

// GetDatabaseSizeMB returns the current database size in megabytes.
func (db *DB) GetDatabaseSizeMB() (float64, error) {
	db.WaitForReady()

	pageCount, pageSize, _, err := db.getDatabasePageStats()
	if err != nil {
		return 0, err
	}

	sizeBytes := pageCount * pageSize
	sizeMB := float64(sizeBytes) / (1024 * 1024)

	return sizeMB, nil
}

// GetDatabaseUsedSizeMB returns the estimated live database size in megabytes.
// Free pages that have not yet been returned to the filesystem are excluded.
func (db *DB) GetDatabaseUsedSizeMB() (float64, error) {
	db.WaitForReady()

	pageCount, pageSize, freePages, err := db.getDatabasePageStats()
	if err != nil {
		return 0, err
	}

	usedPages := pageCount - freePages
	if usedPages < 0 {
		usedPages = 0
	}

	sizeBytes := usedPages * pageSize
	sizeMB := float64(sizeBytes) / (1024 * 1024)

	return sizeMB, nil
}

func (db *DB) getDatabasePageStats() (pageCount, pageSize, freePages int64, err error) {
	if err = db.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return 0, 0, 0, err
	}
	if err = db.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return 0, 0, 0, err
	}
	if err = db.QueryRow("PRAGMA freelist_count").Scan(&freePages); err != nil {
		return 0, 0, 0, err
	}
	return pageCount, pageSize, freePages, nil
}

func (db *DB) getEffectiveCleanupLimitMB(userID int64) int {
	if userID > 0 {
		if limit := db.GetEffectiveMaxCacheSizeMB(userID); limit > 0 {
			return limit
		}
	}

	maxSizeMB := 500
	maxSizeMBStr, err := db.GetSetting("max_cache_size_mb")
	if err == nil {
		if size, convErr := strconv.Atoi(maxSizeMBStr); convErr == nil && size > 0 {
			maxSizeMB = size
		}
	}

	return maxSizeMB
}

// GetStorageUsageMB returns the storage usage relevant to cleanup decisions.
// Global cleanup uses SQLite live page usage, while per-user cleanup uses an
// estimated size derived from the user's own article-related records.
func (db *DB) GetStorageUsageMB(userID int64) (float64, error) {
	if userID <= 0 {
		return db.GetDatabaseUsedSizeMB()
	}

	totalBytes, err := db.getEstimatedUserStorageBytes(userID)
	if err != nil {
		return 0, err
	}

	return float64(totalBytes) / (1024 * 1024), nil
}

func (db *DB) getEstimatedUserStorageBytes(userID int64) (int64, error) {
	db.WaitForReady()

	queries := []string{
		`SELECT IFNULL(SUM(
			LENGTH(COALESCE(title, '')) +
			LENGTH(COALESCE(url, '')) +
			LENGTH(COALESCE(image_url, '')) +
			LENGTH(COALESCE(audio_url, '')) +
			LENGTH(COALESCE(video_url, '')) +
			LENGTH(COALESCE(translated_title, '')) +
			LENGTH(COALESCE(summary, '')) +
			LENGTH(COALESCE(unique_id, '')) +
			LENGTH(COALESCE(author, '')) +
			128
		), 0)
		FROM articles
		WHERE user_id = ?`,
		`SELECT IFNULL(SUM(LENGTH(COALESCE(ac.content, ''))), 0)
		FROM article_contents ac
		INNER JOIN articles a ON a.id = ac.article_id
		WHERE a.user_id = ?`,
		`SELECT IFNULL(SUM(LENGTH(COALESCE(atc.content, ''))), 0)
		FROM article_translated_contents atc
		INNER JOIN articles a ON a.id = atc.article_id
		WHERE a.user_id = ?`,
		`SELECT IFNULL(SUM(
			LENGTH(COALESCE(merged_title, '')) +
			LENGTH(COALESCE(merged_summary, '')) +
			LENGTH(COALESCE(merged_content, '')) +
			128
		), 0)
		FROM clusters
		WHERE user_id = ?`,
	}

	var totalBytes int64
	for _, query := range queries {
		var part int64
		if err := db.QueryRow(query, userID).Scan(&part); err != nil {
			return 0, err
		}
		totalBytes += part
	}

	return totalBytes, nil
}

// ShouldCleanupBeforeSave checks if database is approaching the size limit.
// Returns true if database size is over 80% of max_cache_size_mb.
// Admin quota takes precedence over user setting.
func (db *DB) ShouldCleanupBeforeSave(userID int64) (bool, error) {
	db.WaitForReady()

	maxSizeMB := db.getEffectiveCleanupLimitMB(userID)

	// Use live size here, otherwise deleted free pages keep page_count flat and
	// make cleanup appear ineffective.
	currentSizeMB, err := db.GetStorageUsageMB(userID)
	if err != nil {
		return false, err
	}

	// Trigger cleanup if over 80% of limit
	threshold := float64(maxSizeMB) * 0.8
	return currentSizeMB >= threshold, nil
}

// CleanupBySize removes oldest articles to keep database under max_cache_size_mb limit.
// Protects favorited and read later articles.
// Uses priority order: oldest read articles first, then older unread articles.
// Admin quota takes precedence over user setting.
// If userID > 0, only clean up for that user; otherwise clean up for all users
func (db *DB) CleanupBySize(userID int64) (int64, error) {
	db.WaitForReady()

	maxSizeMB := db.getEffectiveCleanupLimitMB(userID)

	// Use live size during cleanup progress tracking.
	currentSizeMB, err := db.GetStorageUsageMB(userID)
	if err != nil {
		return 0, err
	}

	// If under limit, no cleanup needed
	if currentSizeMB <= float64(maxSizeMB) {
		return 0, nil
	}

	log.Printf("Storage usage (%.2f MB) exceeds limit (%d MB) for user %d, starting cleanup...", currentSizeMB, maxSizeMB, userID)

	totalDeleted := int64(0)
	targetSizeMB := float64(maxSizeMB) * 0.95 // Aim for 95% of limit

	// Step 1: Delete oldest read articles (not favorited, not read later)
	for currentSizeMB > targetSizeMB {
		var result sql.Result
		if userID > 0 {
			result, err = db.Exec(`
				DELETE FROM articles
				WHERE id IN (
					SELECT id FROM articles
					WHERE is_read = 1
					AND is_favorite = 0
					AND is_read_later = 0
					AND cluster_id IS NULL
					AND user_id = ?
					ORDER BY published_at ASC
					LIMIT 100
				)
			`, userID)
		} else {
			result, err = db.Exec(`
				DELETE FROM articles
				WHERE id IN (
					SELECT id FROM articles
					WHERE is_read = 1
					AND is_favorite = 0
					AND is_read_later = 0
					AND cluster_id IS NULL
					ORDER BY published_at ASC
					LIMIT 100
				)
			`)
		}
		if err != nil {
			break
		}

		count, _ := result.RowsAffected()
		if count == 0 {
			break // No more read articles to delete
		}

		totalDeleted += count
		currentSizeMB, _ = db.GetStorageUsageMB(userID)
		log.Printf("Deleted %d read articles, current used size: %.2f MB", count, currentSizeMB)
	}

	// Step 1b: If still over limit, delete oldest fully-eligible read clusters.
	readState := true
	for currentSizeMB > targetSizeMB {
		_, deletedArticles, cleanupErr := db.cleanupEligibleClusters(userID, nil, &readState, 0, 100)
		if cleanupErr != nil {
			break
		}
		if deletedArticles == 0 {
			break
		}

		totalDeleted += deletedArticles
		currentSizeMB, _ = db.GetStorageUsageMB(userID)
		log.Printf("Deleted %d read clustered articles, current used size: %.2f MB", deletedArticles, currentSizeMB)
	}

	// Step 2: If still over limit, delete oldest unread articles (not favorited, not read later)
	for currentSizeMB > targetSizeMB {
		var result sql.Result
		if userID > 0 {
			result, err = db.Exec(`
				DELETE FROM articles
				WHERE id IN (
					SELECT id FROM articles
					WHERE is_favorite = 0
					AND is_read_later = 0
					AND cluster_id IS NULL
					AND user_id = ?
					ORDER BY published_at ASC
					LIMIT 100
				)
			`, userID)
		} else {
			result, err = db.Exec(`
				DELETE FROM articles
				WHERE id IN (
					SELECT id FROM articles
					WHERE is_favorite = 0
					AND is_read_later = 0
					AND cluster_id IS NULL
					ORDER BY published_at ASC
					LIMIT 100
				)
			`)
		}
		if err != nil {
			break
		}

		count, _ := result.RowsAffected()
		if count == 0 {
			break // No more articles to delete
		}

		totalDeleted += count
		currentSizeMB, _ = db.GetStorageUsageMB(userID)
		log.Printf("Deleted %d unread articles, current used size: %.2f MB", count, currentSizeMB)
	}

	// Step 2b: If still over limit, delete oldest fully-eligible unread clusters.
	readState = false
	for currentSizeMB > targetSizeMB {
		_, deletedArticles, cleanupErr := db.cleanupEligibleClusters(userID, nil, &readState, 0, 100)
		if cleanupErr != nil {
			break
		}
		if deletedArticles == 0 {
			break
		}

		totalDeleted += deletedArticles
		currentSizeMB, _ = db.GetStorageUsageMB(userID)
		log.Printf("Deleted %d unread clustered articles, current used size: %.2f MB", deletedArticles, currentSizeMB)
	}

	if totalDeleted > 0 {
		_, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		_, _ = db.Exec("VACUUM")
		finalUsedMB, _ := db.GetStorageUsageMB(userID)
		finalAllocatedMB, _ := db.GetDatabaseSizeMB()
		log.Printf("Size-based cleanup completed: removed %d articles, final used size: %.2f MB, allocated size after VACUUM: %.2f MB", totalDeleted, finalUsedMB, finalAllocatedMB)
	}

	return totalDeleted, nil
}

// CleanupArticleContentsByAge removes article content cache entries older than maxAgeDays
// This only deletes content, not article metadata
// If userID > 0, only clean up for that user; otherwise clean up for all users
func (db *DB) CleanupArticleContentsByAge(maxAgeDays int, userID int64) (int64, error) {
	db.WaitForReady()
	var result sql.Result
	var err error
	if userID > 0 {
		result, err = db.Exec(
			`DELETE FROM article_contents 
			 WHERE fetched_at < datetime('now', '-' || ? || ' days')
			 AND article_id IN (
			 	SELECT id FROM articles
			 	WHERE user_id = ?
			 	AND cluster_id IS NULL
			 )`,
			maxAgeDays, userID,
		)
	} else {
		result, err = db.Exec(
			`DELETE FROM article_contents
			 WHERE fetched_at < datetime('now', '-' || ? || ' days')
			 AND article_id IN (
			 	SELECT id FROM articles
			 	WHERE cluster_id IS NULL
			 )`,
			maxAgeDays,
		)
	}
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CleanupArticleContentsBySize removes oldest article contents to reduce database size
// This only deletes content, not article metadata
// If userID > 0, only clean up for that user; otherwise clean up for all users
func (db *DB) CleanupArticleContentsBySize(userID int64) (int64, error) {
	db.WaitForReady()

	maxSizeMB := db.getEffectiveCleanupLimitMB(userID)

	// Get current database size
	currentSizeMB, err := db.GetStorageUsageMB(userID)
	if err != nil {
		return 0, err
	}

	// If under limit, no cleanup needed
	if currentSizeMB <= float64(maxSizeMB)*0.9 {
		return 0, nil
	}

	totalDeleted := int64(0)
	targetSizeMB := float64(maxSizeMB) * 0.85

	// Delete oldest contents in batches
	for currentSizeMB > targetSizeMB {
		var result sql.Result
		var err error
		if userID > 0 {
			result, err = db.Exec(`
				DELETE FROM article_contents
				WHERE article_id IN (
					SELECT ac.article_id FROM article_contents ac
					INNER JOIN articles a ON ac.article_id = a.id
					WHERE a.user_id = ?
					AND a.cluster_id IS NULL
					ORDER BY ac.fetched_at ASC
					LIMIT 100
				)
			`, userID)
		} else {
			result, err = db.Exec(`
				DELETE FROM article_contents
				WHERE article_id IN (
					SELECT ac.article_id FROM article_contents ac
					INNER JOIN articles a ON ac.article_id = a.id
					WHERE a.cluster_id IS NULL
					ORDER BY fetched_at ASC
					LIMIT 100
				)
			`)
		}
		if err != nil {
			break
		}

		count, _ := result.RowsAffected()
		if count == 0 {
			break
		}

		totalDeleted += count
		currentSizeMB, _ = db.GetStorageUsageMB(userID)
	}

	return totalDeleted, nil
}

// CleanupOldArticlesLayered removes articles in layers:
// Layer 1: Read articles older than 30 days (not favorited/read later)
// Layer 2: Read articles older than 14 days (not favorited/read later)
// Layer 3: Unread articles older than 90 days (not favorited/read later)
// Layer 4: Unread articles older than 60 days (not favorited/read later)
func (db *DB) CleanupOldArticlesLayered() (int64, error) {
	db.WaitForReady()

	totalDeleted := int64(0)

	// Get max article age from settings
	maxAgeDaysStr, err := db.GetSetting("max_article_age_days")
	maxAgeDays := 30
	if err == nil {
		if days, err := strconv.Atoi(maxAgeDaysStr); err == nil && days > 0 {
			maxAgeDays = days
		}
	}

	// Layer 1: Delete very old read articles (maxAgeDays)
	cutoffDate := time.Now().AddDate(0, 0, -maxAgeDays)
	result, err := db.Exec(`
		DELETE FROM articles
		WHERE published_at < ?
		AND is_read = 1
		AND is_favorite = 0
		AND is_read_later = 0
		AND cluster_id IS NULL
	`, cutoffDate)
	if err == nil {
		count, _ := result.RowsAffected()
		totalDeleted += count
		if count > 0 {
			log.Printf("Layer 1: Deleted %d read articles older than %d days", count, maxAgeDays)
		}
	}

	// Layer 2: Delete old read articles (14 days)
	cutoffDate = time.Now().AddDate(0, 0, -14)
	result, err = db.Exec(`
		DELETE FROM articles
		WHERE published_at < ?
		AND is_read = 1
		AND is_favorite = 0
		AND is_read_later = 0
		AND cluster_id IS NULL
	`, cutoffDate)
	if err == nil {
		count, _ := result.RowsAffected()
		totalDeleted += count
		if count > 0 {
			log.Printf("Layer 2: Deleted %d read articles older than 14 days", count)
		}
	}

	// Layer 3: Delete very old unread articles (90 days)
	cutoffDate = time.Now().AddDate(0, 0, -90)
	result, err = db.Exec(`
		DELETE FROM articles
		WHERE published_at < ?
		AND is_read = 0
		AND is_favorite = 0
		AND is_read_later = 0
		AND cluster_id IS NULL
	`, cutoffDate)
	if err == nil {
		count, _ := result.RowsAffected()
		totalDeleted += count
		if count > 0 {
			log.Printf("Layer 3: Deleted %d unread articles older than 90 days", count)
		}
	}

	// Layer 4: Delete old unread articles (60 days)
	cutoffDate = time.Now().AddDate(0, 0, -60)
	result, err = db.Exec(`
		DELETE FROM articles
		WHERE published_at < ?
		AND is_read = 0
		AND is_favorite = 0
		AND is_read_later = 0
		AND cluster_id IS NULL
	`, cutoffDate)
	if err == nil {
		count, _ := result.RowsAffected()
		totalDeleted += count
		if count > 0 {
			log.Printf("Layer 4: Deleted %d unread articles older than 60 days", count)
		}
	}

	// Run VACUUM to reclaim space if we deleted anything
	if totalDeleted > 0 {
		_, _ = db.Exec("VACUUM")
	}

	return totalDeleted, nil
}

// CleanupOldReadArticles removes read articles older than specified days
// Protects favorited and read later articles
// If userID > 0, only clean up for that user; otherwise clean up for all users
func (db *DB) CleanupOldReadArticles(maxAgeDays int, userID int64) (int64, error) {
	db.WaitForReady()

	cutoffDate := time.Now().AddDate(0, 0, -maxAgeDays)
	var result sql.Result
	var err error
	if userID > 0 {
		result, err = db.Exec(`
			DELETE FROM articles
			WHERE published_at < ?
			AND is_read = 1
			AND is_favorite = 0
			AND is_read_later = 0
			AND cluster_id IS NULL
			AND user_id = ?
		`, cutoffDate, userID)
	} else {
		result, err = db.Exec(`
			DELETE FROM articles
			WHERE published_at < ?
			AND is_read = 1
			AND is_favorite = 0
			AND is_read_later = 0
			AND cluster_id IS NULL
		`, cutoffDate)
	}
	if err != nil {
		return 0, err
	}

	count, _ := result.RowsAffected()

	readState := true
	_, clusterArticleCount, clusterErr := db.cleanupEligibleClusters(userID, &cutoffDate, &readState, 0, 0)
	if clusterErr != nil {
		return count, clusterErr
	}

	return count + clusterArticleCount, nil
}

// CleanupOldUnreadArticles removes unread articles older than specified days
// Protects favorited and read later articles
// If userID > 0, only clean up for that user; otherwise clean up for all users
func (db *DB) CleanupOldUnreadArticles(maxAgeDays int, userID int64) (int64, error) {
	db.WaitForReady()

	cutoffDate := time.Now().AddDate(0, 0, -maxAgeDays)
	var result sql.Result
	var err error
	if userID > 0 {
		result, err = db.Exec(`
			DELETE FROM articles
			WHERE published_at < ?
			AND is_read = 0
			AND is_favorite = 0
			AND is_read_later = 0
			AND cluster_id IS NULL
			AND user_id = ?
		`, cutoffDate, userID)
	} else {
		result, err = db.Exec(`
			DELETE FROM articles
			WHERE published_at < ?
			AND is_read = 0
			AND is_favorite = 0
			AND is_read_later = 0
			AND cluster_id IS NULL
		`, cutoffDate)
	}
	if err != nil {
		return 0, err
	}

	count, _ := result.RowsAffected()

	readState := false
	_, clusterArticleCount, clusterErr := db.cleanupEligibleClusters(userID, &cutoffDate, &readState, 0, 0)
	if clusterErr != nil {
		return count, clusterErr
	}

	return count + clusterArticleCount, nil
}

// CleanupExpiredClusters removes clusters older than maxAgeDays that are not AI recommended.
// Returns the number of deleted clusters (and their articles).
// If userID > 0, only clean up for that user; otherwise clean up for all users
func (db *DB) CleanupExpiredClusters(userID int64, maxAgeDays int) (int64, error) {
	db.WaitForReady()

	cutoffDate := time.Now().AddDate(0, 0, -maxAgeDays)
	deletedClusters, _, err := db.cleanupEligibleClusters(userID, &cutoffDate, nil, 0, 0)
	return deletedClusters, err
}

// DeleteClusterAndArticles deletes a cluster and all its associated articles and related data.
func (db *DB) DeleteClusterAndArticles(clusterID int64) error {
	db.WaitForReady()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Get all article IDs in this cluster
	articleRows, err := tx.Query(`SELECT id FROM articles WHERE cluster_id = ?`, clusterID)
	if err != nil {
		return err
	}
	var articleIDs []int64
	for articleRows.Next() {
		var articleID int64
		if err := articleRows.Scan(&articleID); err != nil {
			_ = articleRows.Close()
			return err
		}
		articleIDs = append(articleIDs, articleID)
	}
	_ = articleRows.Close()

	// Delete article embeddings for these articles
	for _, articleID := range articleIDs {
		if _, err := tx.Exec(`DELETE FROM article_embeddings WHERE article_id = ?`, articleID); err != nil {
			return err
		}
	}

	// Delete article contents for these articles
	for _, articleID := range articleIDs {
		if _, err := tx.Exec(`DELETE FROM article_contents WHERE article_id = ?`, articleID); err != nil {
			return err
		}
	}

	// Delete translated contents for these articles
	for _, articleID := range articleIDs {
		if _, err := tx.Exec(`DELETE FROM article_translated_contents WHERE article_id = ?`, articleID); err != nil {
			return err
		}
	}

	// Delete chat sessions and messages for these articles
	for _, articleID := range articleIDs {
		// First delete chat messages
		if _, err := tx.Exec(`DELETE FROM chat_messages WHERE session_id IN (SELECT id FROM chat_sessions WHERE article_id = ?)`, articleID); err != nil {
			return err
		}
		// Then delete chat sessions
		if _, err := tx.Exec(`DELETE FROM chat_sessions WHERE article_id = ?`, articleID); err != nil {
			return err
		}
	}

	// Delete the articles themselves
	if _, err := tx.Exec(`DELETE FROM articles WHERE cluster_id = ?`, clusterID); err != nil {
		return err
	}

	// Delete cluster embeddings
	if _, err := tx.Exec(`DELETE FROM cluster_embeddings WHERE cluster_id = ?`, clusterID); err != nil {
		return err
	}

	// Delete from daily_recommendations if present
	if _, err := tx.Exec(`DELETE FROM daily_recommendations WHERE cluster_id = ?`, clusterID); err != nil {
		return err
	}

	// Finally delete the cluster
	if _, err := tx.Exec(`DELETE FROM clusters WHERE id = ?`, clusterID); err != nil {
		return err
	}

	return tx.Commit()
}

// CleanupExpiredReadClusters removes read clusters older than maxAgeDays that are not favorited or read later.
// Returns the number of deleted clusters.
// If userID > 0, only clean up for that user; otherwise clean up for all users
func (db *DB) CleanupExpiredReadClusters(userID int64, maxAgeDays int) (int64, error) {
	db.WaitForReady()

	cutoffDate := time.Now().AddDate(0, 0, -maxAgeDays)

	readState := true
	deletedClusters, _, err := db.cleanupEligibleClusters(userID, &cutoffDate, &readState, 0, 0)
	return deletedClusters, err
}

// CleanupExpiredUnreadClusters removes unread clusters older than maxAgeDays that are not favorited or read later.
// Returns the number of deleted clusters.
// If userID > 0, only clean up for that user; otherwise clean up for all users
func (db *DB) CleanupExpiredUnreadClusters(userID int64, maxAgeDays int) (int64, error) {
	db.WaitForReady()

	cutoffDate := time.Now().AddDate(0, 0, -maxAgeDays)

	readState := false
	deletedClusters, _, err := db.cleanupEligibleClusters(userID, &cutoffDate, &readState, 0, 0)
	return deletedClusters, err
}

type cleanupClusterCandidate struct {
	ClusterID    int64
	ArticleCount int64
}

func (db *DB) cleanupEligibleClusters(
	userID int64,
	cutoffDate *time.Time,
	requireRead *bool,
	limitClusters int,
	maxArticleDeletes int64,
) (int64, int64, error) {
	db.WaitForReady()

	candidates, err := db.listEligibleCleanupClusters(userID, cutoffDate, requireRead, limitClusters)
	if err != nil {
		return 0, 0, err
	}

	var deletedClusters int64
	var deletedArticles int64
	for _, candidate := range candidates {
		if maxArticleDeletes > 0 && deletedArticles >= maxArticleDeletes {
			break
		}
		if err := db.DeleteClusterAndArticles(candidate.ClusterID); err != nil {
			return deletedClusters, deletedArticles, err
		}
		deletedClusters++
		deletedArticles += candidate.ArticleCount
	}

	return deletedClusters, deletedArticles, nil
}

func (db *DB) listEligibleCleanupClusters(
	userID int64,
	cutoffDate *time.Time,
	requireRead *bool,
	limit int,
) ([]cleanupClusterCandidate, error) {
	db.WaitForReady()

	query := `
		SELECT
			c.id,
			COUNT(a.id) AS article_count
		FROM clusters c
		INNER JOIN articles a ON a.cluster_id = c.id AND a.user_id = c.user_id
		WHERE c.is_favorite = 0
		  AND c.is_read_later = 0
		  AND c.is_ai_recommended = 0
	`
	args := make([]interface{}, 0, 4)

	if userID > 0 {
		query += ` AND c.user_id = ?`
		args = append(args, userID)
	}

	if requireRead != nil {
		if *requireRead {
			query += ` AND c.is_read = 1`
		} else {
			query += ` AND c.is_read = 0`
		}
	}

	query += `
		  AND NOT EXISTS (
			SELECT 1
			FROM articles ax
			WHERE ax.cluster_id = c.id
			  AND ax.user_id = c.user_id
			  AND (
				ax.is_favorite = 1
				OR ax.is_read_later = 1
	`

	if cutoffDate != nil {
		query += ` OR ax.published_at >= ?`
		args = append(args, *cutoffDate)
	}

	if requireRead != nil {
		if *requireRead {
			query += ` OR ax.is_read = 0`
		} else {
			query += ` OR ax.is_read = 1`
		}
	}

	query += `
			  )
		  )
		GROUP BY c.id
		ORDER BY MIN(a.published_at) ASC, c.id ASC
	`

	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]cleanupClusterCandidate, 0)
	for rows.Next() {
		var candidate cleanupClusterCandidate
		if err := rows.Scan(&candidate.ClusterID, &candidate.ArticleCount); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}

	return candidates, rows.Err()
}
