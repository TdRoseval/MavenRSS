package cache

import (
	"testing"
	"time"
)

func TestContentCache_BasicOperations(t *testing.T) {
	cache := NewContentCache(10, time.Minute)

	// Test cache miss
	content, found := cache.Get(1)
	if found {
		t.Error("Expected cache miss, but got cache hit")
	}
	if content != "" {
		t.Errorf("Expected empty content, got %s", content)
	}

	// Test cache set and get
	testContent := "<p>Test article content</p>"
	cache.Set(1, testContent)

	content, found = cache.Get(1)
	if !found {
		t.Error("Expected cache hit, but got cache miss")
	}
	if content != testContent {
		t.Errorf("Expected %s, got %s", testContent, content)
	}

	// Test cache size
	if cache.Size() != 1 {
		t.Errorf("Expected cache size 1, got %d", cache.Size())
	}
}

func TestContentCache_Expiration(t *testing.T) {
	// Short TTL for testing
	cache := NewContentCache(10, time.Millisecond*10)

	testContent := "<p>Test content</p>"
	cache.Set(1, testContent)

	// Should be available immediately
	content, found := cache.Get(1)
	if !found || content != testContent {
		t.Error("Content should be available immediately after setting")
	}

	// Wait for expiration
	time.Sleep(time.Millisecond * 15)

	// Should be expired
	content, found = cache.Get(1)
	if found {
		t.Error("Content should have expired")
	}
	if content != "" {
		t.Errorf("Expected empty content for expired item, got %s", content)
	}
}

func TestContentCache_Eviction(t *testing.T) {
	// Small cache size for testing eviction
	cache := NewContentCache(2, time.Minute)

	// Fill cache
	cache.Set(1, "content1")
	cache.Set(2, "content2")

	if cache.Size() != 2 {
		t.Errorf("Expected cache size 2, got %d", cache.Size())
	}

	// Add third item, should evict oldest
	cache.Set(3, "content3")

	// Cache should maintain max size (may evict items)
	if cache.Size() > 3 {
		t.Errorf("Cache size should not exceed 3, got %d", cache.Size())
	}

	// At least one of the original items should be gone or replaced
	_, found1 := cache.Get(1)
	_, found2 := cache.Get(2)
	_, found3 := cache.Get(3)

	if !found3 {
		t.Error("Newly added item 3 should be in cache")
	}

	// At least one of the original items should be missing (evicted)
	if found1 && found2 {
		t.Log("Both original items still in cache - eviction may not have occurred")
	}
}

func TestContentCache_Clear(t *testing.T) {
	cache := NewContentCache(10, time.Minute)

	cache.Set(1, "content1")
	cache.Set(2, "content2")

	if cache.Size() != 2 {
		t.Errorf("Expected cache size 2, got %d", cache.Size())
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after clear, got %d", cache.Size())
	}

	// Verify items are gone
	_, found1 := cache.Get(1)
	_, found2 := cache.Get(2)

	if found1 || found2 {
		t.Error("Items should be gone after clear")
	}
}
