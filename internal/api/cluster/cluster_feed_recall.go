package cluster

import (
	"math/rand"
	"time"

	"MavenRSS/internal/store/sqlite"
)

const favoriteRecallKeepProbability = 0.1

func pruneFavoriteRecallCandidates(candidates []sqlite.ClusterWithScore, filter string, randFloat func() float64) []sqlite.ClusterWithScore {
	if len(candidates) == 0 || filter == "favorites" {
		return candidates
	}

	if randFloat == nil {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		randFloat = rng.Float64
	}

	filtered := make([]sqlite.ClusterWithScore, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.Cluster.IsFavorite || randFloat() < favoriteRecallKeepProbability {
			filtered = append(filtered, candidate)
		}
	}

	return filtered
}
