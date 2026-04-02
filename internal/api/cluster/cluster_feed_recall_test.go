package cluster

import (
	"testing"

	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
)

func TestPruneFavoriteRecallCandidatesDropsFavoritesByProbability(t *testing.T) {
	candidates := []sqlite.ClusterWithScore{
		{Cluster: models.Cluster{ID: 1, IsFavorite: true}},
		{Cluster: models.Cluster{ID: 2, IsFavorite: false}},
		{Cluster: models.Cluster{ID: 3, IsFavorite: true}},
		{Cluster: models.Cluster{ID: 4, IsFavorite: false}},
	}

	values := []float64{0.05, 0.50}
	index := 0
	filtered := pruneFavoriteRecallCandidates(candidates, "", func() float64 {
		value := values[index]
		index++
		return value
	})

	if len(filtered) != 3 {
		t.Fatalf("len(filtered) = %d, want 3", len(filtered))
	}

	gotIDs := []int64{filtered[0].Cluster.ID, filtered[1].Cluster.ID, filtered[2].Cluster.ID}
	wantIDs := []int64{1, 2, 4}
	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Fatalf("filtered[%d].Cluster.ID = %d, want %d", i, gotIDs[i], wantIDs[i])
		}
	}
}

func TestPruneFavoriteRecallCandidatesSkipsPruningForFavoritesFilter(t *testing.T) {
	candidates := []sqlite.ClusterWithScore{
		{Cluster: models.Cluster{ID: 1, IsFavorite: true}},
		{Cluster: models.Cluster{ID: 2, IsFavorite: true}},
	}

	filtered := pruneFavoriteRecallCandidates(candidates, "favorites", func() float64 {
		t.Fatal("randFloat should not be called for favorites filter")
		return 0
	})

	if len(filtered) != len(candidates) {
		t.Fatalf("len(filtered) = %d, want %d", len(filtered), len(candidates))
	}
	for i := range candidates {
		if filtered[i].Cluster.ID != candidates[i].Cluster.ID {
			t.Fatalf("filtered[%d].Cluster.ID = %d, want %d", i, filtered[i].Cluster.ID, candidates[i].Cluster.ID)
		}
	}
}
