package sqlite

import (
	"log"
)

// CleanupFreshRSSData removes all FreshRSS-related feeds, articles, and sync queue items
// This should be called when FreshRSS is disabled or its settings are changed
func (db *DB) CleanupFreshRSSData() error {
	db.WaitForReady()

	log.Printf("[FreshRSS Cleanup] Starting cleanup of FreshRSS data...")

	// Step 1: Get all FreshRSS feed IDs
	feedIDs, err := db.getFreshRSSFeedIDs()
	if err != nil {
		log.Printf("[FreshRSS Cleanup] Error getting FreshRSS feed IDs: %v", err)
		return err
	}

	if len(feedIDs) == 0 {
		log.Printf("[FreshRSS Cleanup] No FreshRSS feeds found, nothing to clean")
		return nil
	}

	log.Printf("[FreshRSS Cleanup] Found %d FreshRSS feeds to delete", len(feedIDs))

	// Step 2: Delete all articles from FreshRSS feeds
	// SQLite doesn't support batch DELETE with IN clause for large lists,
	// so we need to delete in batches or use a subquery
	_, err = db.Exec(`DELETE FROM article_contents WHERE article_id IN (
		SELECT id FROM articles WHERE feed_id IN (
			SELECT id FROM feeds WHERE is_freshrss_source = 1
		)
	)`)
	if err != nil {
		log.Printf("[FreshRSS Cleanup] Error deleting article contents: %v", err)
		// Continue anyway
	} else {
		log.Printf("[FreshRSS Cleanup] Deleted article contents for FreshRSS feeds")
	}

	// Step 3: Delete all articles from FreshRSS feeds
	result, err := db.Exec(`DELETE FROM articles WHERE feed_id IN (
		SELECT id FROM feeds WHERE is_freshrss_source = 1
	)`)
	if err != nil {
		log.Printf("[FreshRSS Cleanup] Error deleting articles: %v", err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("[FreshRSS Cleanup] Deleted %d articles from FreshRSS feeds", rowsAffected)

	// Step 4: Delete all FreshRSS feeds
	result, err = db.Exec("DELETE FROM feeds WHERE is_freshrss_source = 1")
	if err != nil {
		log.Printf("[FreshRSS Cleanup] Error deleting feeds: %v", err)
		return err
	}

	rowsAffected, _ = result.RowsAffected()
	log.Printf("[FreshRSS Cleanup] Deleted %d FreshRSS feeds", rowsAffected)

	// Step 5: Clear FreshRSS sync queue
	_, err = db.Exec("DELETE FROM freshrss_sync_queue")
	if err != nil {
		log.Printf("[FreshRSS Cleanup] Error clearing sync queue: %v", err)
		// Continue anyway
	} else {
		log.Printf("[FreshRSS Cleanup] Cleared FreshRSS sync queue")
	}

	// Step 6: Clear FreshRSS settings (keep enabled status as it will be set by caller)
	// Note: We don't clear freshrss_enabled here as it's managed by the settings handler
	log.Printf("[FreshRSS Cleanup] Completed successfully")

	return nil
}

// getFreshRSSFeedIDs returns IDs of all FreshRSS feeds
func (db *DB) getFreshRSSFeedIDs() ([]int64, error) {
	rows, err := db.Query("SELECT id FROM feeds WHERE is_freshrss_source = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feedIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		feedIDs = append(feedIDs, id)
	}

	return feedIDs, rows.Err()
}
