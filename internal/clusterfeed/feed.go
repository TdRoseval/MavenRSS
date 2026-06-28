package clusterfeed

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"time"

	"MavenRSS/internal/interest"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
)

const (
	MaxAgeDays                    = 3
	DefaultLimit                  = sqlite.ClusterFeedFirstPageDefaultLimit
	FavoriteRecallKeepProbability = 0.1
)

var RootFilters = []string{"all", "unread", "favorites", "readLater"}

type Request struct {
	ExcludeIDs []int64
	Filter     string
	FeedID     int64
	Category   string
	Limit      int
	UseCache   bool
}

type Response struct {
	Clusters []models.Cluster `json:"clusters"`
	HasMore  bool             `json:"has_more"`
	CacheHit bool             `json:"cache_hit,omitempty"`
}

func Build(db *sqlite.DB, userID int64, req Request) (Response, error) {
	if db == nil || userID <= 0 {
		return Response{Clusters: []models.Cluster{}}, nil
	}

	req.Filter = sqlite.NormalizeClusterFeedRootFilter(req.Filter)
	returnLimit := req.Limit
	if returnLimit <= 0 {
		returnLimit = DefaultLimit
	}
	targetResultCount := returnLimit + 1

	vecBlob, err := resolveInterestVector(db, userID)
	if err != nil {
		log.Printf("Failed to resolve interest vector for user %d: %v", userID, err)
	}

	cacheable := sqlite.IsClusterFeedFirstPageCacheable(req.Filter, req.FeedID, req.Category, req.ExcludeIDs)
	if req.UseCache && cacheable && len(vecBlob) > 0 {
		payload, ok, err := db.GetClusterFeedFirstPageCache(userID, req.Filter, vecBlob)
		if err != nil {
			log.Printf("Failed to read cluster feed first-page cache for user %d filter %s: %v", userID, req.Filter, err)
		} else if ok {
			return Response{Clusters: payload.Clusters, HasMore: payload.HasMore, CacheHit: true}, nil
		}
	}

	response, err := buildLive(db, userID, req, vecBlob, returnLimit, targetResultCount)
	if err != nil {
		return response, err
	}

	if cacheable && len(vecBlob) > 0 {
		if err := db.SaveClusterFeedFirstPageCache(userID, req.Filter, vecBlob, sqlite.ClusterFeedFirstPagePayload{
			Clusters: response.Clusters,
			HasMore:  response.HasMore,
		}); err != nil {
			log.Printf("Failed to save cluster feed first-page cache for user %d filter %s: %v", userID, req.Filter, err)
		}
	}

	return response, nil
}

func PrewarmRootFilters(db *sqlite.DB, userID int64) error {
	if db == nil || userID <= 0 {
		return nil
	}

	vecBlob, err := resolveInterestVector(db, userID)
	if err != nil {
		return fmt.Errorf("resolve interest vector: %w", err)
	}
	if len(vecBlob) == 0 {
		return nil
	}

	for _, filter := range RootFilters {
		req := Request{Filter: filter, Limit: DefaultLimit, UseCache: false}
		response, err := buildLive(db, userID, req, vecBlob, DefaultLimit, DefaultLimit+1)
		if err != nil {
			return fmt.Errorf("prewarm %s cluster feed: %w", filter, err)
		}
		if err := db.SaveClusterFeedFirstPageCache(userID, filter, vecBlob, sqlite.ClusterFeedFirstPagePayload{
			Clusters: response.Clusters,
			HasMore:  response.HasMore,
		}); err != nil {
			return fmt.Errorf("save %s cluster feed cache: %w", filter, err)
		}
	}
	return nil
}

func buildLive(
	db *sqlite.DB,
	userID int64,
	req Request,
	vecBlob []byte,
	returnLimit int,
	targetResultCount int,
) (Response, error) {
	if len(vecBlob) == 0 {
		clusters, err := db.GetRecentClustersChronological(
			userID,
			req.ExcludeIDs,
			req.Filter,
			req.FeedID,
			req.Category,
			targetResultCount,
		)
		if err != nil {
			return Response{}, fmt.Errorf("fetch chronological clusters: %w", err)
		}
		return trimResponse(clusters, returnLimit), nil
	}

	recallTopK := CalculateRealtimeRecallTopK(len(req.ExcludeIDs), targetResultCount)
	candidates, err := db.GetClustersByVectorSimilarity(
		userID, vecBlob, req.ExcludeIDs, req.Filter, req.FeedID, req.Category, MaxAgeDays, recallTopK,
	)
	if err != nil {
		log.Printf("Error in vector similarity recall: %v", err)
		clusters, fallbackErr := db.GetRecentClustersChronological(
			userID,
			req.ExcludeIDs,
			req.Filter,
			req.FeedID,
			req.Category,
			targetResultCount,
		)
		if fallbackErr != nil {
			return Response{}, fmt.Errorf("fallback chronological clusters: %w", fallbackErr)
		}
		return trimResponse(clusters, returnLimit), nil
	}

	candidates = PruneFavoriteRecallCandidates(candidates, req.Filter, nil)
	if len(candidates) == 0 {
		clusters, err := db.GetRecentClustersChronological(
			userID,
			req.ExcludeIDs,
			req.Filter,
			req.FeedID,
			req.Category,
			targetResultCount,
		)
		if err != nil {
			return Response{}, fmt.Errorf("empty recall chronological clusters: %w", err)
		}
		return trimResponse(clusters, returnLimit), nil
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

		fallbackClusters, err := db.GetRecentClustersChronological(
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

	// Populate meta (feed titles, authors, display title) only for the final
	// selected clusters rather than every recalled candidate. Fallback
	// clusters already have meta populated by GetRecentClustersChronological,
	// but re-populating is harmless and keeps the batch in one query.
	db.PopulateClustersMeta(result)

	return trimResponse(result, returnLimit), nil
}

func trimResponse(clusters []models.Cluster, returnLimit int) Response {
	if clusters == nil {
		clusters = []models.Cluster{}
	}
	hasMore := len(clusters) > returnLimit
	if hasMore {
		clusters = clusters[:returnLimit]
	}
	return Response{Clusters: clusters, HasMore: hasMore}
}

func resolveInterestVector(db *sqlite.DB, userID int64) ([]byte, error) {
	vecBlob, err := db.GetUserInterestVector(userID)
	if err != nil {
		return nil, err
	}
	if len(vecBlob) > 0 {
		return vecBlob, nil
	}

	favBlobs, err := db.GetFavoriteClusterSummaryEmbeddings(userID)
	if err != nil {
		return nil, err
	}
	if len(favBlobs) == 0 {
		return nil, nil
	}

	initVec, err := interest.InitFromFavorites(favBlobs)
	if err != nil {
		return nil, err
	}
	if len(initVec) == 0 {
		return nil, nil
	}

	vecBlob, err = interest.SerializeVector(initVec)
	if err != nil {
		return nil, err
	}
	if err := db.UpdateUserInterestVector(userID, vecBlob); err != nil {
		return nil, err
	}
	return vecBlob, nil
}

func CalculateRealtimeRecallTopK(excludedCount, pageSize int) int {
	topK := excludedCount + pageSize*4
	if topK < 100 {
		topK = 100
	}
	if topK > 500 {
		topK = 500
	}
	return topK
}

func PruneFavoriteRecallCandidates(candidates []sqlite.ClusterWithScore, filter string, randFloat func() float64) []sqlite.ClusterWithScore {
	if len(candidates) == 0 || filter == "favorites" {
		return candidates
	}

	if randFloat == nil {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		randFloat = rng.Float64
	}

	filtered := make([]sqlite.ClusterWithScore, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.Cluster.IsFavorite || randFloat() < FavoriteRecallKeepProbability {
			filtered = append(filtered, candidate)
		}
	}

	return filtered
}
