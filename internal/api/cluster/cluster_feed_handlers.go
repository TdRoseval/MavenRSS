package cluster

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"sort"
	"time"

	"MavenRSS/internal/api/core"
	"MavenRSS/internal/api/response"
	"MavenRSS/internal/interest"
	"MavenRSS/internal/models"
)

type dailyRecommendationItem struct {
	RecommendationDate      string         `json:"recommendation_date"`
	RecommendationRank      int            `json:"recommendation_rank"`
	RecommendationScore     float64        `json:"recommendation_score"`
	RecommendationProfileID int64          `json:"recommendation_profile_id"`
	LatestPublishedAt       string         `json:"latest_published_at,omitempty"`
	Cluster                 models.Cluster `json:"cluster"`
}

type dailyRecommendationResponse struct {
	SelectedDate    string                    `json:"selected_date"`
	AvailableDates  []string                  `json:"available_dates"`
	Recommendations []dailyRecommendationItem `json:"recommendations"`
	Total           int                       `json:"total"`
}

type clusterFeedResponse struct {
	Clusters []models.Cluster `json:"clusters"`
	HasMore  bool             `json:"has_more"`
}

func HandleAIProcessingStatus(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if h.Fetcher == nil || h.Fetcher.GetAIEnhancedManager() == nil {
		response.JSON(w, map[string]any{
			"is_enabled":                          false,
			"has_interest_vector":                 false,
			"is_config_frozen":                    false,
			"progress_percent":                    100,
			"embedding_health_blocked":            false,
			"embedding_health_sample_size":        0,
			"embedding_health_unnormalized_count": 0,
			"embedding_health_unnormalized_ratio": 0,
		})
		return
	}

	response.JSON(w, h.Fetcher.GetAIEnhancedManager().GetProcessingStatus(userID))
}

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
		Filter     string  `json:"filter"`
		FeedID     int64   `json:"feed_id"`
		Category   string  `json:"category"`
		Limit      int     `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.ExcludeIDs = nil
	}

	if err := h.DB.SyncClusterFavoriteStatesFromArticles(userID); err != nil {
		log.Printf("Error syncing cluster favorites before realtime cluster feed: %v", err)
	}

	const maxAgeDays = 3
	returnLimit := req.Limit
	if returnLimit <= 0 {
		returnLimit = 30
	}
	targetResultCount := returnLimit + 1
	recallTopK := calculateRealtimeRecallTopK(len(req.ExcludeIDs), targetResultCount)

	vecBlob, _ := h.DB.GetUserInterestVector(userID)
	if len(vecBlob) == 0 {
		favBlobs, _ := h.DB.GetFavoriteClusterSummaryEmbeddings(userID)
		if len(favBlobs) > 0 {
			initVec, err := interest.InitFromFavorites(favBlobs)
			if err == nil && len(initVec) > 0 {
				if blob, err := interest.SerializeVector(initVec); err == nil {
					vecBlob = blob
					_ = h.DB.UpdateUserInterestVector(userID, vecBlob)
				}
			}
		}
	}

	if len(vecBlob) == 0 {
		clusters, err := h.DB.GetRecentClustersChronological(
			userID,
			req.ExcludeIDs,
			req.Filter,
			req.FeedID,
			req.Category,
			targetResultCount,
		)
		if err != nil {
			log.Printf("Error fetching chronological clusters: %v", err)
			http.Error(w, "Failed to get clusters", http.StatusInternalServerError)
			return
		}
		if clusters == nil {
			clusters = []models.Cluster{}
		}
		hasMore := len(clusters) > returnLimit
		if hasMore {
			clusters = clusters[:returnLimit]
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(clusterFeedResponse{Clusters: clusters, HasMore: hasMore})
		return
	}

	candidates, err := h.DB.GetClustersByVectorSimilarity(
		userID, vecBlob, req.ExcludeIDs, req.Filter, req.FeedID, req.Category, maxAgeDays, recallTopK,
	)
	if err != nil {
		log.Printf("Error in vector similarity recall: %v", err)
		clusters, _ := h.DB.GetRecentClustersChronological(
			userID,
			req.ExcludeIDs,
			req.Filter,
			req.FeedID,
			req.Category,
			targetResultCount,
		)
		hasMore := len(clusters) > returnLimit
		if hasMore {
			clusters = clusters[:returnLimit]
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(clusterFeedResponse{Clusters: clusters, HasMore: hasMore})
		return
	}

	candidates = pruneFavoriteRecallCandidates(candidates, req.Filter, nil)

	if len(candidates) == 0 {
		clusters, _ := h.DB.GetRecentClustersChronological(
			userID,
			req.ExcludeIDs,
			req.Filter,
			req.FeedID,
			req.Category,
			targetResultCount,
		)
		hasMore := len(clusters) > returnLimit
		if hasMore {
			clusters = clusters[:returnLimit]
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(clusterFeedResponse{Clusters: clusters, HasMore: hasMore})
		return
	}

	now := time.Now()
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
		candidates[i].Distance = similarity * gravity
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Distance > candidates[j].Distance
	})

	result := make([]models.Cluster, 0, targetResultCount)
	selectedIDs := make(map[int64]struct{}, len(req.ExcludeIDs)+len(candidates))
	for _, clusterID := range req.ExcludeIDs {
		selectedIDs[clusterID] = struct{}{}
	}
	for _, candidate := range candidates {
		if len(result) >= targetResultCount {
			break
		}
		if _, exists := selectedIDs[candidate.Cluster.ID]; exists {
			continue
		}
		result = append(result, candidate.Cluster)
		selectedIDs[candidate.Cluster.ID] = struct{}{}
	}

	if len(result) < targetResultCount {
		fallbackExcludeIDs := make([]int64, 0, len(selectedIDs))
		for clusterID := range selectedIDs {
			fallbackExcludeIDs = append(fallbackExcludeIDs, clusterID)
		}

		fallbackClusters, err := h.DB.GetRecentClustersChronological(
			userID,
			fallbackExcludeIDs,
			req.Filter,
			req.FeedID,
			req.Category,
			targetResultCount-len(result),
		)
		if err != nil {
			log.Printf("Error fetching fallback chronological clusters: %v", err)
		} else {
			result = append(result, fallbackClusters...)
		}
	}
	hasMore := len(result) > returnLimit
	if hasMore {
		result = result[:returnLimit]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clusterFeedResponse{Clusters: result, HasMore: hasMore})
}

func calculateRealtimeRecallTopK(excludedCount, pageSize int) int {
	topK := excludedCount + pageSize*8
	if topK < 200 {
		topK = 200
	}
	if topK > 2000 {
		topK = 2000
	}
	return topK
}

func HandleDailyRecommendationDates(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	dates, err := h.DB.ListDailyRecommendationDates(userID)
	if err != nil {
		log.Printf("Error listing daily recommendation dates: %v", err)
		http.Error(w, "Failed to get daily recommendation dates", http.StatusInternalServerError)
		return
	}
	if dates == nil {
		dates = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dates)
}

func HandleDailyRecommendations(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	availableDates, err := h.DB.ListDailyRecommendationDates(userID)
	if err != nil {
		log.Printf("Error listing daily recommendation dates: %v", err)
		http.Error(w, "Failed to get daily recommendations", http.StatusInternalServerError)
		return
	}
	if availableDates == nil {
		availableDates = []string{}
	}

	selectedDate := r.URL.Query().Get("date")
	if selectedDate == "" && len(availableDates) > 0 {
		selectedDate = availableDates[0]
	}

	response := dailyRecommendationResponse{
		SelectedDate:    selectedDate,
		AvailableDates:  availableDates,
		Recommendations: []dailyRecommendationItem{},
		Total:           0,
	}

	if selectedDate == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	results, err := h.DB.GetDailyRecommendationsByDate(userID, selectedDate)
	if err != nil {
		log.Printf("Error getting daily recommendations by date: %v", err)
		http.Error(w, "Failed to get daily recommendations", http.StatusInternalServerError)
		return
	}

	items := make([]dailyRecommendationItem, 0, len(results))
	for _, item := range results {
		mapped := dailyRecommendationItem{
			RecommendationDate:      item.Recommendation.RecommendationDate,
			RecommendationRank:      item.Recommendation.RecommendationRank,
			RecommendationScore:     item.Recommendation.RecommendationScore,
			RecommendationProfileID: item.Recommendation.RecommendationProfileID,
			Cluster:                 item.Cluster,
		}
		if !item.LatestPublishedAt.IsZero() {
			mapped.LatestPublishedAt = item.LatestPublishedAt.Format(time.RFC3339)
		}
		items = append(items, mapped)
	}

	response.Recommendations = items
	response.Total = len(items)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func HandleDailyRecommendationTaskStatus(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if h.Fetcher == nil || h.Fetcher.GetAIEnhancedManager() == nil {
		response.JSON(w, map[string]any{
			"is_enabled":       false,
			"has_task":         false,
			"progress_percent": 100,
		})
		return
	}

	response.JSON(w, h.Fetcher.GetAIEnhancedManager().GetDailyRecommendationTaskStatus(userID))
}

func HandleRegenerateDailyRecommendations(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if h.Fetcher == nil || h.Fetcher.GetAIEnhancedManager() == nil {
		http.Error(w, "AI enhanced mode unavailable", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Date              string `json:"date"`
		WaitForIdle       bool   `json:"wait_for_idle"`
		ForceIfIncomplete bool   `json:"force_if_incomplete"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.WaitForIdle = true
		req.ForceIfIncomplete = true
	}

	if !req.WaitForIdle {
		req.WaitForIdle = true
	}
	if !req.ForceIfIncomplete {
		req.ForceIfIncomplete = true
	}

	scheduled, err := h.Fetcher.GetAIEnhancedManager().QueueDailyRecommendations(
		userID,
		req.Date,
		req.WaitForIdle,
		req.ForceIfIncomplete,
	)
	if err != nil {
		log.Printf("Error scheduling daily recommendation regeneration: %v", err)
		http.Error(w, "Failed to schedule daily recommendation regeneration", http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]any{
		"scheduled": scheduled,
		"date":      req.Date,
	})
}

func HandleRefreshDailyRecommendations(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if h.Fetcher == nil || h.Fetcher.GetAIEnhancedManager() == nil {
		http.Error(w, "AI enhanced mode unavailable", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Date        string `json:"date"`
		WaitForIdle bool   `json:"wait_for_idle"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.WaitForIdle = true
	}
	if !req.WaitForIdle {
		req.WaitForIdle = true
	}

	status, err := h.Fetcher.GetAIEnhancedManager().ForceDailyRecommendations(userID, req.Date, req.WaitForIdle)
	if err != nil {
		log.Printf("Error forcing daily recommendation refresh: %v", err)
		http.Error(w, "Failed to refresh daily recommendations", http.StatusInternalServerError)
		return
	}
	scheduled := status.HasTask || status.IsQueued || status.IsRunning

	response.JSON(w, map[string]any{
		"scheduled": scheduled,
		"date":      status.RecommendationDate,
		"status":    status,
	})
}

func HandleClusterRenormalization(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if h.Fetcher == nil || h.Fetcher.GetAIEnhancedManager() == nil {
		http.Error(w, "AI enhanced mode unavailable", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Force bool `json:"force"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	manager := h.Fetcher.GetAIEnhancedManager()
	var (
		scheduled bool
		reason    string
		err       error
	)
	if req.Force {
		scheduled, reason, err = manager.ForceStartClusterRenormalization(userID)
	} else {
		scheduled, reason, err = manager.StartClusterRenormalization(userID)
	}
	if err != nil {
		log.Printf("Error starting cluster renormalization: %v", err)
		http.Error(w, "Failed to start cluster renormalization", http.StatusInternalServerError)
		return
	}

	response.JSON(w, models.ClusterRenormalizeResponse{
		Scheduled: scheduled,
		Reason:    reason,
	})
}

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

	titleBlob, err := h.DB.GetClusterEmbedding(req.ClusterID, "title_embedding")
	if err != nil || len(titleBlob) == 0 {
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

	vecBlob, _ := h.DB.GetUserInterestVector(userID)
	var oldVec []float32
	if len(vecBlob) > 0 {
		oldVec, _ = interest.DeserializeVector(vecBlob)
	}

	newVec := interest.UpdateVector(oldVec, featureVec, interest.AlphaClick)
	if newBlob, err := interest.SerializeVector(newVec); err == nil {
		_ = h.DB.UpdateUserInterestVector(userID, newBlob)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

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

	readCount, totalReadTime, _ := h.DB.GetUserAIReadStats(userID)
	avgReadTime := interest.ComputeAvgReadTime(totalReadTime, readCount)

	newCount := readCount + 1
	newTotal := totalReadTime + req.ReadTimeSeconds
	_ = h.DB.UpdateUserAIReadStats(userID, newCount, newTotal)

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
