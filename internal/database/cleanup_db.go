package database

import (
	"database/sql"
	"log"
	"strconv"
	"time"
)

// CleanupOldArticles removes articles based on age and status.
// - Articles older than configured days: delete except favorited or read later
// - Also checks database size against max_cache_size_mb setting
func (db *DB) CleanupOldArticles() (int64, error) {
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
	result, err := db.Exec(`
		DELETE FROM articles
		WHERE published_at < ?
		AND is_favorite = 0
		AND is_read_later = 0
	`, cutoffDate)
	if err != nil {
		return 0, err
	}

	count, _ := result.RowsAffected()
	totalDeleted += count

	// Step 2: Check database size and clean up if over limit
	sizeDeleted, err := db.CleanupBySize()
	if err != nil {
		log.Printf("Error during size-based cleanup: %v", err)
	} else {
		totalDeleted += sizeDeleted
	}

	// Also cleanup related caches with the same age limit
	_, _ = db.CleanupTranslationCache(maxAgeDays)
	_, _ = db.CleanupOldArticleContents(maxAgeDays)

	// Run VACUUM to reclaim space
	_, _ = db.Exec("VACUUM")

	return totalDeleted, nil
}

// CleanupAllArticleContents removes all cached article contents for a specific user
// If userID is 0, removes all (legacy behavior)
func (db *DB) CleanupAllArticleContents(userID int64) (int64, error) {
	db.WaitForReady()
	var result sql.Result
	var err error
	if userID > 0 {
		result, err = db.Exec(`
			DELETE FROM article_contents
			WHERE article_id IN (SELECT id FROM articles WHERE user_id = ?)
		`, userID)
	} else {
		result, err = db.Exec(`DELETE FROM article_contents`)
	}
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteAllArticles removes ALL articles from the database for a specific user
// This keeps feeds, settings, and other metadata intact
// If userID is 0, removes all (legacy behavior)
func (db *DB) DeleteAllArticles(userID int64) (int64, error) {
	db.WaitForReady()
	var result sql.Result
	var err error
	if userID > 0 {
		result, err = db.Exec(`DELETE FROM articles WHERE user_id = ?`, userID)
	} else {
		result, err = db.Exec(`DELETE FROM articles`)
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
		`, userID)
	} else {
		result, err = db.Exec(`
			DELETE FROM articles
			WHERE is_read = 0
			AND is_favorite = 0
			AND is_read_later = 0
		`)
	}
	if err != nil {
		return 0, err
	}

	count, _ := result.RowsAffected()

	// Also cleanup related caches (remove entries older than 7 days)
	_, _ = db.CleanupTranslationCache(7)
	_, _ = db.CleanupOldArticleContents(7)

	// Run VACUUM to reclaim space
	_, _ = db.Exec("VACUUM")

	return count, nil
}

// GetDatabaseSizeMB returns the current database size in megabytes.
func (db *DB) GetDatabaseSizeMB() (float64, error) {
	db.WaitForReady()

	var pageCount, pageSize int64
	err := db.QueryRow("PRAGMA page_count").Scan(&pageCount)
	if err != nil {
		return 0, err
	}

	err = db.QueryRow("PRAGMA page_size").Scan(&pageSize)
	if err != nil {
		return 0, err
	}

	sizeBytes := pageCount * pageSize
	sizeMB := float64(sizeBytes) / (1024 * 1024)

	return sizeMB, nil
}

// ShouldCleanupBeforeSave checks if database is approaching the size limit.
// Returns true if database size is over 80% of max_cache_size_mb.
// Admin quota takes precedence over user setting.
func (db *DB) ShouldCleanupBeforeSave() (bool, error) {
	db.WaitForReady()

	var adminQuotaLimit int
	var userSettingLimit int = 500 // Default

	// Try to get admin quota for the first user
	users, _ := db.ListUsers()
	if len(users) > 0 {
		quota, err := db.GetUserQuota(users[0].ID)
		if err == nil && quota.MaxStorageMB > 0 {
			adminQuotaLimit = quota.MaxStorageMB
		}
	}

	// Get user setting
	maxSizeMBStr, err := db.GetSetting("max_cache_size_mb")
	if err == nil {
		if size, err := strconv.Atoi(maxSizeMBStr); err == nil && size > 0 {
			userSettingLimit = size
		}
	}

	// Determine the effective limit: admin quota takes precedence if set
	maxSizeMB := userSettingLimit
	if adminQuotaLimit > 0 && adminQuotaLimit < userSettingLimit {
		maxSizeMB = adminQuotaLimit
	}

	// Get current database size
	currentSizeMB, err := db.GetDatabaseSizeMB()
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
func (db *DB) CleanupBySize() (int64, error) {
	db.WaitForReady()

	var adminQuotaLimit int
	var userSettingLimit int = 500 // Default

	// Try to get admin quota for the first user
	users, _ := db.ListUsers()
	if len(users) > 0 {
		quota, err := db.GetUserQuota(users[0].ID)
		if err == nil && quota.MaxStorageMB > 0 {
			adminQuotaLimit = quota.MaxStorageMB
		}
	}

	// Get user setting
	maxSizeMBStr, err := db.GetSetting("max_cache_size_mb")
	if err == nil {
		if size, err := strconv.Atoi(maxSizeMBStr); err == nil && size > 0 {
			userSettingLimit = size
		}
	}

	// Determine the effective limit: admin quota takes precedence if set
	maxSizeMB := userSettingLimit
	if adminQuotaLimit > 0 && adminQuotaLimit < userSettingLimit {
		log.Printf("CleanupBySize: Using admin quota limit (%d MB) instead of user setting (%d MB)", adminQuotaLimit, userSettingLimit)
		maxSizeMB = adminQuotaLimit
	}

	// Get current database size
	currentSizeMB, err := db.GetDatabaseSizeMB()
	if err != nil {
		return 0, err
	}

	// If under limit, no cleanup needed
	if currentSizeMB <= float64(maxSizeMB) {
		return 0, nil
	}

	log.Printf("Database size (%.2f MB) exceeds limit (%d MB), starting cleanup...", currentSizeMB, maxSizeMB)

	totalDeleted := int64(0)
	targetSizeMB := float64(maxSizeMB) * 0.95 // Aim for 95% of limit

	// Step 1: Delete oldest read articles (not favorited, not read later)
	for currentSizeMB > targetSizeMB {
		result, err := db.Exec(`
			DELETE FROM articles
			WHERE id IN (
				SELECT id FROM articles
				WHERE is_read = 1
				AND is_favorite = 0
				AND is_read_later = 0
				ORDER BY published_at ASC
				LIMIT 100
			)
		`)
		if err != nil {
			break
		}

		count, _ := result.RowsAffected()
		if count == 0 {
			break // No more read articles to delete
		}

		totalDeleted += count
		currentSizeMB, _ = db.GetDatabaseSizeMB()
		log.Printf("Deleted %d read articles, current size: %.2f MB", count, currentSizeMB)
	}

	// Step 2: If still over limit, delete oldest unread articles (not favorited, not read later)
	for currentSizeMB > targetSizeMB {
		result, err := db.Exec(`
			DELETE FROM articles
			WHERE id IN (
				SELECT id FROM articles
				WHERE is_favorite = 0
				AND is_read_later = 0
				ORDER BY published_at ASC
				LIMIT 100
			)
		`)
		if err != nil {
			break
		}

		count, _ := result.RowsAffected()
		if count == 0 {
			break // No more articles to delete
		}

		totalDeleted += count
		currentSizeMB, _ = db.GetDatabaseSizeMB()
		log.Printf("Deleted %d unread articles, current size: %.2f MB", count, currentSizeMB)
	}

	if totalDeleted > 0 {
		log.Printf("Size-based cleanup completed: removed %d articles, final size: %.2f MB", totalDeleted, currentSizeMB)
	}

	return totalDeleted, nil
}

// CleanupArticleContentsByAge removes article content cache entries older than maxAgeDays
// This only deletes content, not article metadata
func (db *DB) CleanupArticleContentsByAge(maxAgeDays int) (int64, error) {
	db.WaitForReady()
	result, err := db.Exec(
		`DELETE FROM article_contents WHERE fetched_at < datetime('now', '-' || ? || ' days')`,
		maxAgeDays,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CleanupArticleContentsBySize removes oldest article contents to reduce database size
// This only deletes content, not article metadata
func (db *DB) CleanupArticleContentsBySize() (int64, error) {
	db.WaitForReady()

	// Get max cache size from settings (default 500 MB)
	maxSizeMBStr, err := db.GetSetting("max_cache_size_mb")
	maxSizeMB := 500
	if err == nil {
		if size, err := strconv.Atoi(maxSizeMBStr); err == nil && size > 0 {
			maxSizeMB = size
		}
	}

	// Get current database size
	currentSizeMB, err := db.GetDatabaseSizeMB()
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
		result, err := db.Exec(`
			DELETE FROM article_contents
			WHERE article_id IN (
				SELECT article_id FROM article_contents
				ORDER BY fetched_at ASC
				LIMIT 100
			)
		`)
		if err != nil {
			break
		}

		count, _ := result.RowsAffected()
		if count == 0 {
			break
		}

		totalDeleted += count
		currentSizeMB, _ = db.GetDatabaseSizeMB()
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
func (db *DB) CleanupOldReadArticles(maxAgeDays int) (int64, error) {
	db.WaitForReady()

	cutoffDate := time.Now().AddDate(0, 0, -maxAgeDays)
	result, err := db.Exec(`
		DELETE FROM articles
		WHERE published_at < ?
		AND is_read = 1
		AND is_favorite = 0
		AND is_read_later = 0
	`, cutoffDate)
	if err != nil {
		return 0, err
	}

	count, _ := result.RowsAffected()
	return count, nil
}

// CleanupOldUnreadArticles removes unread articles older than specified days
// Protects favorited and read later articles
func (db *DB) CleanupOldUnreadArticles(maxAgeDays int) (int64, error) {
	db.WaitForReady()

	cutoffDate := time.Now().AddDate(0, 0, -maxAgeDays)
	result, err := db.Exec(`
		DELETE FROM articles
		WHERE published_at < ?
		AND is_read = 0
		AND is_favorite = 0
		AND is_read_later = 0
	`, cutoffDate)
	if err != nil {
		return 0, err
	}

	count, _ := result.RowsAffected()
	return count, nil
}
