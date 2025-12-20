// Package cache provides content caching functionality for article content.
package cache

import (
	"sync"
	"time"
)

// ContentCacheItem represents a cached content item with expiration
type ContentCacheItem struct {
	Content   string
	ExpiresAt time.Time
	SetAt     time.Time // When the item was set
}

// ContentCache provides LRU-style caching for article content
type ContentCache struct {
	mu      sync.RWMutex
	cache   map[int64]*ContentCacheItem
	maxSize int
	ttl     time.Duration
}

// NewContentCache creates a new content cache
func NewContentCache(maxSize int, ttl time.Duration) *ContentCache {
	return &ContentCache{
		cache:   make(map[int64]*ContentCacheItem),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get retrieves content from cache if it exists and hasn't expired
func (cc *ContentCache) Get(articleID int64) (string, bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	item, exists := cc.cache[articleID]
	if !exists {
		return "", false
	}

	// Check if expired
	if time.Now().After(item.ExpiresAt) {
		// Item expired, remove it
		go func() {
			cc.mu.Lock()
			delete(cc.cache, articleID)
			cc.mu.Unlock()
		}()
		return "", false
	}

	return item.Content, true
}

// Set stores content in cache
func (cc *ContentCache) Set(articleID int64, content string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	now := time.Now()

	// If cache is at max capacity, remove oldest item before adding new one
	if len(cc.cache) >= cc.maxSize {
		// Find oldest item by set time
		var oldestID int64
		var oldestTime = time.Now() // Initialize to current time

		for id, item := range cc.cache {
			if item.SetAt.Before(oldestTime) {
				oldestTime = item.SetAt
				oldestID = id
			}
		}

		if oldestID != 0 {
			delete(cc.cache, oldestID)
		}
	}

	cc.cache[articleID] = &ContentCacheItem{
		Content:   content,
		ExpiresAt: now.Add(cc.ttl),
		SetAt:     now,
	}
}

// Clear removes all cached content
func (cc *ContentCache) Clear() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.cache = make(map[int64]*ContentCacheItem)
}

// Size returns the current number of cached items
func (cc *ContentCache) Size() int {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return len(cc.cache)
}
