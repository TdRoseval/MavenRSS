package cluster

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"MavenRSS/internal/api/core"
	"MavenRSS/internal/interest"
	"MavenRSS/internal/models"
)

// HandleClusters handles GET /api/clusters — list clusters with filtering and pagination.
func HandleClusters(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	if _, exists := r.URL.Query()["offset"]; !exists {
		offset = (page - 1) * limit
	}

	var feedID int64
	if feedIDStr := r.URL.Query().Get("feed_id"); feedIDStr != "" {
		feedID, _ = strconv.ParseInt(feedIDStr, 10, 64)
	}
	category := r.URL.Query().Get("category")

	if err := h.DB.SyncClusterFavoriteStatesFromArticles(userID); err != nil {
		log.Printf("Error syncing cluster favorites before listing clusters: %v", err)
	}

	clusters, err := h.DB.GetClustersForUser(userID, filter, feedID, category, limit, offset)
	if err != nil {
		log.Printf("Error getting clusters: %v", err)
		http.Error(w, "Failed to get clusters", http.StatusInternalServerError)
		return
	}
	if clusters == nil {
		clusters = []models.Cluster{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clusters)
}

// HandleClusterDetail handles GET /api/clusters/detail?id=N — get cluster with articles.
func HandleClusterDetail(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clusterID, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if clusterID <= 0 {
		http.Error(w, "Missing cluster id", http.StatusBadRequest)
		return
	}

	cluster, err := h.DB.GetClusterByID(clusterID)
	if err != nil || cluster == nil {
		http.Error(w, "Cluster not found", http.StatusNotFound)
		return
	}

	articles, err := h.DB.GetArticlesByClusterID(clusterID)
	if err == nil {
		cluster.Articles = articles
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cluster)
}

// HandleClusterRead handles PUT /api/clusters/read — mark cluster read/unread.
func HandleClusterRead(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID   int64 `json:"id"`
		Read bool  `json:"read"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.DB.MarkClusterRead(req.ID, req.Read); err != nil {
		http.Error(w, "Failed to update", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// HandleMarkAllClustersRead handles POST /api/clusters/mark-all-read — mark all clusters as read.
func HandleMarkAllClustersRead(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}

	var feedID int64
	if feedIDStr := r.URL.Query().Get("feed_id"); feedIDStr != "" {
		feedID, _ = strconv.ParseInt(feedIDStr, 10, 64)
	}
	category := r.URL.Query().Get("category")

	if err := h.DB.MarkAllClustersReadForUser(userID, filter, feedID, category); err != nil {
		http.Error(w, "Failed to update", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// HandleClusterFavorite handles PUT /api/clusters/favorite — toggle favorite.
// Also applies Level 3 interest vector update (α=0.20) when favoriting.
func HandleClusterFavorite(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	clusterObj, _ := h.DB.GetClusterByID(req.ID)
	wasFavorite := clusterObj != nil && clusterObj.IsFavorite

	if err := h.DB.ToggleClusterFavorite(req.ID); err != nil {
		http.Error(w, "Failed to update", http.StatusInternalServerError)
		return
	}

	if !wasFavorite {
		go func() {
			summaryBlob, err := h.DB.GetClusterEmbedding(req.ID, "summary_embedding")
			if err != nil || len(summaryBlob) == 0 {
				return
			}
			featureVec, err := interest.DeserializeVector(summaryBlob)
			if err != nil || len(featureVec) == 0 {
				return
			}
			vecBlob, _ := h.DB.GetUserInterestVector(userID)
			var oldVec []float32
			if len(vecBlob) > 0 {
				oldVec, _ = interest.DeserializeVector(vecBlob)
			}
			newVec := interest.UpdateVector(oldVec, featureVec, interest.AlphaBookmark)
			if newBlob, err := interest.SerializeVector(newVec); err == nil {
				_ = h.DB.UpdateUserInterestVector(userID, newBlob)
			}
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// HandleClusterReadLater handles PUT /api/clusters/read-later — toggle read-later.
func HandleClusterReadLater(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.DB.ToggleClusterReadLater(req.ID); err != nil {
		http.Error(w, "Failed to update", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
