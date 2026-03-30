package cluster

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"MavenRSS/internal/api/core"
)

// HandleClusters handles GET /api/clusters — list clusters with filtering and pagination.
func HandleClusters(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	clusters, err := h.DB.GetClustersForUser(userID, filter, limit, offset)
	if err != nil {
		log.Printf("Error getting clusters: %v", err)
		http.Error(w, "Failed to get clusters", http.StatusInternalServerError)
		return
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

	// Populate associated articles
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

// HandleClusterFavorite handles PUT /api/clusters/favorite — toggle favorite.
func HandleClusterFavorite(h *core.Handler, w http.ResponseWriter, r *http.Request) {
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

	if err := h.DB.ToggleClusterFavorite(req.ID); err != nil {
		http.Error(w, "Failed to update", http.StatusInternalServerError)
		return
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
