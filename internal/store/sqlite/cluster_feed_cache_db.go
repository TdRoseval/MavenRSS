package sqlite

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"MavenRSS/internal/models"
)

const (
	ClusterFeedFirstPageDefaultLimit = 30
)

var clusterFeedFirstPageRootFilters = map[string]struct{}{
	"all":       {},
	"unread":    {},
	"favorites": {},
	"readLater": {},
}

type ClusterFeedFirstPagePayload struct {
	Clusters []models.Cluster `json:"clusters"`
	HasMore  bool             `json:"has_more"`
}

type ClusterFeedFirstPageCacheEntry struct {
	UserID      int64
	Filter      string
	VectorHash  string
	Payload     ClusterFeedFirstPagePayload
	GeneratedAt time.Time
}

func NormalizeClusterFeedRootFilter(filter string) string {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return "all"
	}
	return filter
}

func IsClusterFeedFirstPageCacheable(filter string, feedID int64, category string, excludeIDs []int64) bool {
	filter = NormalizeClusterFeedRootFilter(filter)
	if _, ok := clusterFeedFirstPageRootFilters[filter]; !ok {
		return false
	}
	return feedID <= 0 && strings.TrimSpace(category) == "" && len(excludeIDs) == 0
}

func ClusterFeedInterestVectorHash(vectorBlob []byte) string {
	if len(vectorBlob) == 0 {
		return ""
	}
	hash := sha256.Sum256(vectorBlob)
	return hex.EncodeToString(hash[:])
}

func (db *DB) GetClusterFeedFirstPageCache(userID int64, filter string, vectorBlob []byte) (*ClusterFeedFirstPagePayload, bool, error) {
	db.WaitForReady()
	filter = NormalizeClusterFeedRootFilter(filter)
	vectorHash := ClusterFeedInterestVectorHash(vectorBlob)
	if userID <= 0 || vectorHash == "" {
		return nil, false, nil
	}

	if payload, ok := db.getClusterFeedFirstPageMemoryCache(userID, filter, vectorHash); ok {
		return payload, true, nil
	}

	var storedHash, payloadJSON string
	var generatedAt time.Time
	err := db.QueryRow(`
		SELECT vector_hash, payload_json, generated_at
		FROM cluster_feed_first_page_cache
		WHERE user_id = ? AND filter = ?
	`, userID, filter).Scan(&storedHash, &payloadJSON, &generatedAt)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get cluster feed first-page cache: %w", err)
	}
	if storedHash != vectorHash {
		if clearErr := db.DeleteClusterFeedFirstPageCache(userID, filter); clearErr != nil {
			return nil, false, clearErr
		}
		return nil, false, nil
	}

	var payload ClusterFeedFirstPagePayload
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		if clearErr := db.DeleteClusterFeedFirstPageCache(userID, filter); clearErr != nil {
			return nil, false, clearErr
		}
		return nil, false, nil
	}
	if payload.Clusters == nil {
		payload.Clusters = []models.Cluster{}
	}

	db.setClusterFeedFirstPageMemoryCache(ClusterFeedFirstPageCacheEntry{
		UserID:      userID,
		Filter:      filter,
		VectorHash:  vectorHash,
		Payload:     payload,
		GeneratedAt: generatedAt,
	})
	return &payload, true, nil
}

func (db *DB) SaveClusterFeedFirstPageCache(userID int64, filter string, vectorBlob []byte, payload ClusterFeedFirstPagePayload) error {
	db.WaitForReady()
	filter = NormalizeClusterFeedRootFilter(filter)
	vectorHash := ClusterFeedInterestVectorHash(vectorBlob)
	if userID <= 0 || vectorHash == "" {
		return nil
	}
	if payload.Clusters == nil {
		payload.Clusters = []models.Cluster{}
	}
	currentVectorBlob, err := db.GetUserInterestVector(userID)
	if err != nil {
		return fmt.Errorf("verify cluster feed first-page cache vector: %w", err)
	}
	if ClusterFeedInterestVectorHash(currentVectorBlob) != vectorHash {
		return nil
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal cluster feed first-page cache: %w", err)
	}
	generatedAt := time.Now()
	if _, err := db.Exec(`
		INSERT INTO cluster_feed_first_page_cache (user_id, filter, vector_hash, payload_json, generated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, filter) DO UPDATE SET
			vector_hash = excluded.vector_hash,
			payload_json = excluded.payload_json,
			generated_at = excluded.generated_at
	`, userID, filter, vectorHash, string(payloadBytes), generatedAt); err != nil {
		return fmt.Errorf("save cluster feed first-page cache: %w", err)
	}

	db.setClusterFeedFirstPageMemoryCache(ClusterFeedFirstPageCacheEntry{
		UserID:      userID,
		Filter:      filter,
		VectorHash:  vectorHash,
		Payload:     payload,
		GeneratedAt: generatedAt,
	})
	return nil
}

func (db *DB) DeleteClusterFeedFirstPageCache(userID int64, filter string) error {
	db.WaitForReady()
	filter = NormalizeClusterFeedRootFilter(filter)
	if userID <= 0 {
		return nil
	}
	if _, err := db.Exec(`DELETE FROM cluster_feed_first_page_cache WHERE user_id = ? AND filter = ?`, userID, filter); err != nil {
		return fmt.Errorf("delete cluster feed first-page cache: %w", err)
	}
	db.deleteClusterFeedFirstPageMemoryCache(userID, filter)
	return nil
}

func (db *DB) ClearClusterFeedFirstPageCacheForUser(userID int64) error {
	db.WaitForReady()
	if userID <= 0 {
		return nil
	}
	if _, err := db.Exec(`DELETE FROM cluster_feed_first_page_cache WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("clear cluster feed first-page cache: %w", err)
	}
	db.clearClusterFeedFirstPageMemoryCache(userID)
	return nil
}

func (db *DB) getClusterFeedFirstPageMemoryCache(userID int64, filter, vectorHash string) (*ClusterFeedFirstPagePayload, bool) {
	db.clusterFeedFirstPageCacheMu.RLock()
	defer db.clusterFeedFirstPageCacheMu.RUnlock()

	userEntries := db.clusterFeedFirstPageCache[userID]
	if userEntries == nil {
		return nil, false
	}
	entry, ok := userEntries[filter]
	if !ok || entry.VectorHash != vectorHash {
		return nil, false
	}
	payload := entry.Payload
	if payload.Clusters == nil {
		payload.Clusters = []models.Cluster{}
	}
	return &payload, true
}

func (db *DB) setClusterFeedFirstPageMemoryCache(entry ClusterFeedFirstPageCacheEntry) {
	if entry.UserID <= 0 || entry.Filter == "" || entry.VectorHash == "" {
		return
	}

	db.clusterFeedFirstPageCacheMu.Lock()
	defer db.clusterFeedFirstPageCacheMu.Unlock()

	if db.clusterFeedFirstPageCache == nil {
		db.clusterFeedFirstPageCache = make(map[int64]map[string]ClusterFeedFirstPageCacheEntry)
	}
	if db.clusterFeedFirstPageCache[entry.UserID] == nil {
		db.clusterFeedFirstPageCache[entry.UserID] = make(map[string]ClusterFeedFirstPageCacheEntry)
	}
	db.clusterFeedFirstPageCache[entry.UserID][entry.Filter] = entry
}

func (db *DB) deleteClusterFeedFirstPageMemoryCache(userID int64, filter string) {
	db.clusterFeedFirstPageCacheMu.Lock()
	defer db.clusterFeedFirstPageCacheMu.Unlock()

	if db.clusterFeedFirstPageCache == nil || db.clusterFeedFirstPageCache[userID] == nil {
		return
	}
	delete(db.clusterFeedFirstPageCache[userID], filter)
	if len(db.clusterFeedFirstPageCache[userID]) == 0 {
		delete(db.clusterFeedFirstPageCache, userID)
	}
}

func (db *DB) clearClusterFeedFirstPageMemoryCache(userID int64) {
	db.clusterFeedFirstPageCacheMu.Lock()
	defer db.clusterFeedFirstPageCacheMu.Unlock()

	if db.clusterFeedFirstPageCache == nil {
		return
	}
	delete(db.clusterFeedFirstPageCache, userID)
}
