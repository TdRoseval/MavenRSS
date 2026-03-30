package cluster

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"sort"
	"time"

	"MavenRSS/internal/api/core"
	"MavenRSS/internal/interest"
	"MavenRSS/internal/models"
)

// HandleClustersFeed handles POST /api/clusters/feed — AI-enhanced lazy-load with
// vector similarity recall and Gravity time-decay reranking.
func HandleClustersFeed(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ExcludeIDs []int64 `json:"exclude_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body (first load)
		req.ExcludeIDs = nil
	}

	const (
		recallTopK  = 100
		returnLimit = 30
		maxAgeDays  = 3
	)

	// Load user interest vector
	vecBlob, _ := h.DB.GetUserInterestVector(userID)

	// Cold-start: try to initialize from favorites
	if len(vecBlob) == 0 {
		favBlobs, _ := h.DB.GetFavoriteClusterSummaryEmbeddings(userID)
		if len(favBlobs) > 0 {
			initVec, err := interest.InitFromFavorites(favBlobs)
			if err == nil && len(initVec) > 0 {
				if blob, err := interest.SerializeVector(initVec); err == nil {
					vecBlob = blob
					// Persist the initialized vector
					_ = h.DB.UpdateUserInterestVector(userID, vecBlob)
				}
			}
		}
	}

	// Fallback: no interest vector → chronological ordering
	if len(vecBlob) == 0 {
		clusters, err := h.DB.GetRecentClustersChronological(userID, req.ExcludeIDs, returnLimit)
		if err != nil {
			log.Printf("Error fetching chronological clusters: %v", err)
			http.Error(w, "Failed to get clusters", http.StatusInternalServerError)
			return
		}
		if clusters == nil {
			clusters = []models.Cluster{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(clusters)
		return
	}

	// Vector similarity recall (top 100)
	candidates, err := h.DB.GetClustersByVectorSimilarity(
		userID, vecBlob, req.ExcludeIDs, maxAgeDays, recallTopK,
	)
	if err != nil {
		log.Printf("Error in vector similarity recall: %v", err)
		// Fallback to chronological
		clusters, _ := h.DB.GetRecentClustersChronological(userID, req.ExcludeIDs, returnLimit)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(clusters)
		return
	}

	if len(candidates) == 0 {
		// No vector matches, fallback to chronological
		clusters, _ := h.DB.GetRecentClustersChronological(userID, req.ExcludeIDs, returnLimit)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(clusters)
		return
	}

	// Gravity reranking: FinalScore = Similarity × 1/(T+2)^1.5
	now := time.Now()

	// Apply Gravity decay
	for i := range candidates {
		hoursOld := now.Sub(candidates[i].Cluster.UpdatedAt).Hours()
		if hoursOld < 0 {
			hoursOld = 0
		}
		similarity := 1.0 - candidates[i].Distance
		if similarity < 0 {
			similarity = 0
		}
		gravity := 1.0 / math.Pow(hoursOld+2.0, 1.5)
		candidates[i].Distance = similarity * gravity // Reuse Distance field as FinalScore
	}

	// Sort by final score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Distance > candidates[j].Distance
	})

	// Truncate to returnLimit
	if len(candidates) > returnLimit {
		candidates = candidates[:returnLimit]
	}

	// Extract clusters for response
	result := make([]models.Cluster, len(candidates))
	for i, c := range candidates {
		result[i] = c.Cluster
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleClusterClick handles POST /api/clusters/click — Level 1 interest update.
// Updates user interest vector with cluster title embedding at α=0.05.
func HandleClusterClick(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ClusterID int64 `json:"cluster_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ClusterID <= 0 {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Get cluster title embedding
	titleBlob, err := h.DB.GetClusterEmbedding(req.ClusterID, "title_embedding")
	if err != nil || len(titleBlob) == 0 {
		// No embedding available, silently succeed
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	}

	featureVec, err := interest.DeserializeVector(titleBlob)
	if err != nil || len(featureVec) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	}

	// Load current interest vector
	vecBlob, _ := h.DB.GetUserInterestVector(userID)
	var oldVec []float32
	if len(vecBlob) > 0 {
		oldVec, _ = interest.DeserializeVector(vecBlob)
	}

	// EMA update with α_click = 0.05
	newVec := interest.UpdateVector(oldVec, featureVec, interest.AlphaClick)
	if newBlob, err := interest.SerializeVector(newVec); err == nil {
		_ = h.DB.UpdateUserInterestVector(userID, newBlob)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// HandleClusterReadTime handles POST /api/clusters/read-time — Level 2 deep-read update.
// If actual read time exceeds the user's dynamic average, updates interest vector
// with cluster summary embedding at α=0.10 and accumulates reading stats.
func HandleClusterReadTime(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ClusterID       int64 `json:"cluster_id"`
		ReadTimeSeconds int64 `json:"read_time_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ClusterID <= 0 || req.ReadTimeSeconds <= 0 {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Compute dynamic average reading time
	readCount, totalReadTime, _ := h.DB.GetUserAIReadStats(userID)
	avgReadTime := interest.ComputeAvgReadTime(totalReadTime, readCount)

	// Always update reading stats (count + time)
	newCount := readCount + 1
	newTotal := totalReadTime + req.ReadTimeSeconds
	_ = h.DB.UpdateUserAIReadStats(userID, newCount, newTotal)

	// Level 2 trigger: actual read time exceeds dynamic average
	if float64(req.ReadTimeSeconds) > avgReadTime {
		summaryBlob, err := h.DB.GetClusterEmbedding(req.ClusterID, "summary_embedding")
		if err == nil && len(summaryBlob) > 0 {
			featureVec, err := interest.DeserializeVector(summaryBlob)
			if err == nil && len(featureVec) > 0 {
				vecBlob, _ := h.DB.GetUserInterestVector(userID)
				var oldVec []float32
				if len(vecBlob) > 0 {
					oldVec, _ = interest.DeserializeVector(vecBlob)
				}

				newVec := interest.UpdateVector(oldVec, featureVec, interest.AlphaDeepRead)
				if newBlob, err := interest.SerializeVector(newVec); err == nil {
					_ = h.DB.UpdateUserInterestVector(userID, newBlob)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
