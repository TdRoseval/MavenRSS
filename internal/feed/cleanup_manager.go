package feed

import (
	"log"
	"sync"
	"time"
)

// CleanupManager manages automatic cleanup with retry mechanism
type CleanupManager struct {
	fetcher *Fetcher

	// State tracking
	isRunning bool
	mu        sync.RWMutex

	// Cleanup request tracking
	pendingCleanupByUser map[int64]bool
	pendingCleanupMu     sync.Mutex

	// Retry mechanism
	retryInterval time.Duration // 10 minutes
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// NewCleanupManager creates a new cleanup manager
func NewCleanupManager(fetcher *Fetcher) *CleanupManager {
	return &CleanupManager{
		fetcher:              fetcher,
		retryInterval:        10 * time.Minute,
		stopChan:             make(chan struct{}),
		pendingCleanupByUser: make(map[int64]bool),
	}
}

// Start starts the cleanup manager
func (cm *CleanupManager) Start() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.isRunning {
		return
	}

	cm.isRunning = true

	// Start retry goroutine
	cm.wg.Add(1)
	go cm.retryLoop()

	log.Println("Cleanup manager started")
}

// Stop stops the cleanup manager
func (cm *CleanupManager) Stop() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.isRunning {
		return
	}

	close(cm.stopChan)
	cm.wg.Wait()

	cm.isRunning = false
	log.Println("Cleanup manager stopped")
}

// RequestCleanup requests a cleanup operation for a specific user.
// If userID is 0, it falls back to legacy global cleanup behavior.
// If cleanup is blocked (tasks running), it will be retried every 10 minutes.
func (cm *CleanupManager) RequestCleanup(userID int64) {
	// Check if auto cleanup is enabled first
	autoCleanup, _ := cm.fetcher.db.GetSettingWithFallback(userID, "auto_cleanup_enabled")
	if autoCleanup != "true" {
		if userID > 0 {
			log.Printf("Auto cleanup is disabled for user %d, skipping cleanup request", userID)
		} else {
			log.Println("Auto cleanup is disabled, skipping cleanup request")
		}
		return
	}

	cm.pendingCleanupMu.Lock()
	if cm.pendingCleanupByUser == nil {
		cm.pendingCleanupByUser = make(map[int64]bool)
	}
	cm.pendingCleanupByUser[userID] = true
	cm.pendingCleanupMu.Unlock()

	// Try to execute immediately
	cm.tryCleanup()
}

// RequestManualCleanup clears all article contents immediately
// This is for manual cleanup triggered by user
func (cm *CleanupManager) RequestManualCleanup() {
	// Manual cleanup clears all content regardless of tasks
	cm.wg.Add(1)
	go func() {
		defer cm.wg.Done()

		log.Println("Executing manual cleanup (clearing all article contents)")

		count, err := cm.fetcher.db.CleanupAllArticleContents(0)
		if err != nil {
			log.Printf("Manual cleanup error: %v", err)
		} else {
			log.Printf("Manual cleanup completed: cleared %d article contents", count)
		}
	}()
}

// tryCleanup attempts to execute cleanup if conditions are met
func (cm *CleanupManager) tryCleanup() {
	// Check if we can cleanup (no tasks running)
	if !cm.canCleanup() {
		log.Println("Cleanup blocked: tasks are running, will retry later")
		return
	}

	cm.pendingCleanupMu.Lock()
	if len(cm.pendingCleanupByUser) == 0 {
		cm.pendingCleanupMu.Unlock()
		return
	}

	pendingUserIDs := make([]int64, 0, len(cm.pendingCleanupByUser))
	for userID := range cm.pendingCleanupByUser {
		pendingUserIDs = append(pendingUserIDs, userID)
	}
	cm.pendingCleanupByUser = make(map[int64]bool)
	cm.pendingCleanupMu.Unlock()

	for _, userID := range pendingUserIDs {
		cm.wg.Add(1)
		go func(uid int64) {
			defer cm.wg.Done()
			cm.executeCleanup(uid)
		}(userID)
	}
}

// canCleanup checks if cleanup can be executed (no tasks running)
func (cm *CleanupManager) canCleanup() bool {
	stats := cm.fetcher.taskManager.GetStats()

	// Check if queue, pool, or article click tasks are running
	if stats.QueueTaskCount > 0 || stats.PoolTaskCount > 0 || stats.ArticleClickCount > 0 {
		return false
	}

	return true
}

// executeCleanup executes the layered cleanup
func (cm *CleanupManager) executeCleanup(userID int64) {
	// Double-check if auto cleanup is enabled
	autoCleanup, _ := cm.fetcher.db.GetSettingWithFallback(userID, "auto_cleanup_enabled")
	if autoCleanup != "true" {
		if userID > 0 {
			log.Printf("Auto cleanup is disabled for user %d, skipping execution", userID)
		} else {
			log.Println("Auto cleanup is disabled, skipping execution")
		}
		return
	}

	if userID > 0 {
		log.Printf("Starting automatic cleanup for user %d...", userID)
	} else {
		log.Println("Starting automatic cleanup...")
	}

	maxSizeMB := cm.getTargetSize(userID)

	// Execute layered cleanup with 80% target
	totalRemoved := cm.layeredCleanup(userID, maxSizeMB*0.8)

	if totalRemoved > 0 {
		if userID > 0 {
			log.Printf("Automatic cleanup completed for user %d: removed %d items", userID, totalRemoved)
		} else {
			log.Printf("Automatic cleanup completed: removed %d items", totalRemoved)
		}
	} else {
		if userID > 0 {
			log.Printf("Automatic cleanup completed for user %d: nothing to clean", userID)
		} else {
			log.Println("Automatic cleanup completed: nothing to clean")
		}
	}
}

// getTargetSize returns the target database size in MB
// Uses the minimum of user setting and admin quota (admin quota takes precedence)
func (cm *CleanupManager) getTargetSize(userID int64) float64 {
	if userID > 0 {
		return float64(cm.fetcher.db.GetEffectiveMaxCacheSizeMB(userID))
	}

	var adminQuotaLimit int
	var userSettingLimit int = 500 // Default

	// Try to get admin quota for the first user (if multi-user)
	// In single-user mode, this will get the default user's quota
	users, _ := cm.fetcher.db.ListUsers()
	if len(users) > 0 {
		quota, err := cm.fetcher.db.GetUserQuota(users[0].ID)
		if err == nil && quota.MaxStorageMB > 0 {
			adminQuotaLimit = quota.MaxStorageMB
		}
	}

	// Get user setting
	maxSizeMBStr, _ := cm.fetcher.db.GetSetting("max_cache_size_mb")
	if maxSizeMBStr != "" {
		if size, err := parseInt(maxSizeMBStr); err == nil && size > 0 {
			userSettingLimit = size
		}
	}

	// Determine the effective limit: admin quota takes precedence if set
	if adminQuotaLimit > 0 && adminQuotaLimit < userSettingLimit {
		log.Printf("Using admin quota limit (%d MB) instead of user setting (%d MB)", adminQuotaLimit, userSettingLimit)
		return float64(adminQuotaLimit)
	}

	return float64(userSettingLimit)
}

// layeredCleanup executes cleanup in layers until target size is reached
// Cleanup order:
// 1. Expired read clusters
// 2. Expired unread clusters
// 3. Old unclustered article contents
// 4. Medium unclustered article contents
// 5. Old unclustered article metadata
// 6. New unclustered article contents
// 7. Latest unclustered article contents
// 8. Medium unclustered article metadata
// Note: New and latest article metadata are never cleaned
func (cm *CleanupManager) layeredCleanup(userID int64, targetSizeMB float64) int64 {
	totalRemoved := int64(0)

	// Get current size
	currentSizeMB, _ := cm.fetcher.db.GetStorageUsageMB(userID)

	if currentSizeMB <= targetSizeMB {
		return 0
	}

	log.Printf("Current storage usage for user %d: %.2f MB, Target: %.2f MB", userID, currentSizeMB, targetSizeMB)

	// Layer 1: Expired read clusters (30+ days old)
	if currentSizeMB > targetSizeMB {
		count, err := cm.fetcher.db.CleanupExpiredReadClusters(userID, 30)
		if err != nil {
			log.Printf("Layer 1 error: %v", err)
		} else {
			log.Printf("Layer 1: Removed %d expired read clusters", count)
			totalRemoved += count
			currentSizeMB, _ = cm.fetcher.db.GetStorageUsageMB(userID)
		}
	}

	// Layer 2: Expired unread clusters (60+ days old)
	if currentSizeMB > targetSizeMB {
		count, err := cm.fetcher.db.CleanupExpiredUnreadClusters(userID, 60)
		if err != nil {
			log.Printf("Layer 2 error: %v", err)
		} else {
			log.Printf("Layer 2: Removed %d expired unread clusters", count)
			totalRemoved += count
			currentSizeMB, _ = cm.fetcher.db.GetStorageUsageMB(userID)
		}
	}

	// Layer 3: Old article contents (7+ days old)
	if currentSizeMB > targetSizeMB {
		count, err := cm.fetcher.db.CleanupArticleContentsByAge(7, userID)
		if err != nil {
			log.Printf("Layer 3 error: %v", err)
		} else {
			log.Printf("Layer 3: Removed %d old article contents", count)
			totalRemoved += count
			currentSizeMB, _ = cm.fetcher.db.GetStorageUsageMB(userID)
		}
	}

	// Layer 4: Medium article contents (3+ days old)
	if currentSizeMB > targetSizeMB {
		count, err := cm.fetcher.db.CleanupArticleContentsByAge(3, userID)
		if err != nil {
			log.Printf("Layer 4 error: %v", err)
		} else {
			log.Printf("Layer 4: Removed %d medium article contents", count)
			totalRemoved += count
			currentSizeMB, _ = cm.fetcher.db.GetStorageUsageMB(userID)
		}
	}

	// Layer 5: Old article metadata (read, 30+ days old)
	if currentSizeMB > targetSizeMB {
		count, err := cm.fetcher.db.CleanupOldReadArticles(30, userID)
		if err != nil {
			log.Printf("Layer 5 error: %v", err)
		} else {
			log.Printf("Layer 5: Removed %d old article metadata", count)
			totalRemoved += count
			currentSizeMB, _ = cm.fetcher.db.GetStorageUsageMB(userID)
		}
	}

	// Layer 6: New article contents (1+ days old)
	if currentSizeMB > targetSizeMB {
		count, err := cm.fetcher.db.CleanupArticleContentsByAge(1, userID)
		if err != nil {
			log.Printf("Layer 6 error: %v", err)
		} else {
			log.Printf("Layer 6: Removed %d new article contents", count)
			totalRemoved += count
			currentSizeMB, _ = cm.fetcher.db.GetStorageUsageMB(userID)
		}
	}

	// Layer 7: Only cleanup article contents by size instead of deleting all
	// This is a safer approach that doesn't delete everything
	if currentSizeMB > targetSizeMB {
		count, err := cm.fetcher.db.CleanupArticleContentsBySize(userID)
		if err != nil {
			log.Printf("Layer 7 error: %v", err)
		} else {
			log.Printf("Layer 7: Removed %d article contents by size", count)
			totalRemoved += count
			currentSizeMB, _ = cm.fetcher.db.GetStorageUsageMB(userID)
		}
	}

	// Layer 8: Medium article metadata (unread, 60+ days old, not favorite/read-later)
	if currentSizeMB > targetSizeMB {
		count, err := cm.fetcher.db.CleanupOldUnreadArticles(60, userID)
		if err != nil {
			log.Printf("Layer 8 error: %v", err)
		} else {
			log.Printf("Layer 8: Removed %d medium article metadata", count)
			totalRemoved += count
			_, _ = cm.fetcher.db.GetStorageUsageMB(userID)
		}
	}

	// Final size check
	finalSizeMB, _ := cm.fetcher.db.GetStorageUsageMB(userID)
	log.Printf("Final storage usage before VACUUM for user %d: %.2f MB (target was %.2f MB)", userID, finalSizeMB, targetSizeMB)

	// Run VACUUM to reclaim space if we removed anything
	if totalRemoved > 0 {
		log.Println("Running VACUUM to reclaim disk space...")
		_, _ = cm.fetcher.db.Exec("VACUUM")

		// Log final size after VACUUM
		finalSizeAfterVACUUM, _ := cm.fetcher.db.GetStorageUsageMB(userID)
		log.Printf("Final storage usage after VACUUM for user %d: %.2f MB", userID, finalSizeAfterVACUUM)
	}

	return totalRemoved
}

// retryLoop checks every 10 minutes if pending cleanup can be executed
func (cm *CleanupManager) retryLoop() {
	defer cm.wg.Done()

	ticker := time.NewTicker(cm.retryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cm.stopChan:
			return
		case <-ticker.C:
			cm.pendingCleanupMu.Lock()
			hasPending := len(cm.pendingCleanupByUser) > 0
			cm.pendingCleanupMu.Unlock()

			if hasPending {
				log.Println("Retry: attempting pending cleanup")
				cm.tryCleanup()
			}
		}
	}
}

// CheckSizeAndCleanup checks database size and triggers cleanup if needed
func (cm *CleanupManager) CheckSizeAndCleanup() {
	// Check if auto cleanup is enabled first
	autoCleanup, _ := cm.fetcher.db.GetSetting("auto_cleanup_enabled")
	if autoCleanup != "true" {
		return
	}

	maxSizeMB := cm.getTargetSize(0)

	currentSizeMB, err := cm.fetcher.db.GetStorageUsageMB(0)
	if err != nil {
		log.Printf("Error checking database size: %v", err)
		return
	}

	if currentSizeMB > maxSizeMB {
		log.Printf("Database size %.2f MB exceeds limit %.2f MB, triggering cleanup", currentSizeMB, maxSizeMB)
		cm.RequestCleanup(0)
	}
}
