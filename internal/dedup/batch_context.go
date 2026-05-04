package dedup

import (
	"fmt"
	"sort"
	"sync"

	"MavenRSS/internal/models"
)

const (
	fusionCompactExistingArticleThreshold = 100
	fusionCompactExistingCharThreshold    = 100000
	clusterRecallExistingArticleLimit     = 200
	clusterRecallExistingCharLimit        = 200000
)

type ClusterSnapshotLoader func(clusterID int64) (*models.ClusterBatchSnapshot, error)

type trackedClusterSnapshot struct {
	snapshot      models.ClusterBatchSnapshot
	newArticleIDs map[int64]struct{}
}

// BatchContext tracks pre-batch cluster baselines so the current batch's
// incoming articles do not affect cluster-size guards or compact fusion input.
type BatchContext struct {
	mu       sync.Mutex
	clusters map[int64]*trackedClusterSnapshot
}

func NewBatchContext() *BatchContext {
	return &BatchContext{
		clusters: make(map[int64]*trackedClusterSnapshot),
	}
}

func (b *BatchContext) EnsureClusterSnapshot(clusterID int64, loader ClusterSnapshotLoader) (*models.ClusterBatchSnapshot, error) {
	if b == nil || clusterID <= 0 {
		return nil, nil
	}

	b.mu.Lock()
	if tracked := b.clusters[clusterID]; tracked != nil {
		snapshot := tracked.snapshot
		b.mu.Unlock()
		return &snapshot, nil
	}
	b.mu.Unlock()

	if loader == nil {
		return nil, fmt.Errorf("cluster snapshot loader is nil")
	}

	loaded, err := loader(clusterID)
	if err != nil || loaded == nil {
		return loaded, err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if tracked := b.clusters[clusterID]; tracked != nil {
		snapshot := tracked.snapshot
		return &snapshot, nil
	}

	copied := *loaded
	b.clusters[clusterID] = &trackedClusterSnapshot{
		snapshot:      copied,
		newArticleIDs: make(map[int64]struct{}),
	}

	snapshot := copied
	return &snapshot, nil
}

func (b *BatchContext) MarkClusterCreated(clusterID int64) {
	if b == nil || clusterID <= 0 {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if existing := b.clusters[clusterID]; existing != nil {
		existing.snapshot.CreatedInBatch = true
		existing.snapshot.ExistingArticleCount = 0
		existing.snapshot.ExistingTotalChars = 0
		return
	}

	b.clusters[clusterID] = &trackedClusterSnapshot{
		snapshot: models.ClusterBatchSnapshot{
			ClusterID:      clusterID,
			CreatedInBatch: true,
		},
		newArticleIDs: make(map[int64]struct{}),
	}
}

func (b *BatchContext) RecordNewArticle(clusterID, articleID int64) {
	if b == nil || clusterID <= 0 || articleID <= 0 {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	tracked := b.clusters[clusterID]
	if tracked == nil {
		tracked = &trackedClusterSnapshot{
			snapshot: models.ClusterBatchSnapshot{
				ClusterID: clusterID,
			},
			newArticleIDs: make(map[int64]struct{}),
		}
		b.clusters[clusterID] = tracked
	}
	if tracked.newArticleIDs == nil {
		tracked.newArticleIDs = make(map[int64]struct{})
	}
	tracked.newArticleIDs[articleID] = struct{}{}
}

func (b *BatchContext) ShouldIgnoreClusterForRecall(clusterID int64, loader ClusterSnapshotLoader) (bool, error) {
	snapshot, err := b.EnsureClusterSnapshot(clusterID, loader)
	if err != nil || snapshot == nil {
		return false, err
	}
	if snapshot.CreatedInBatch {
		return false, nil
	}
	return snapshot.ExistingArticleCount > clusterRecallExistingArticleLimit ||
		snapshot.ExistingTotalChars > clusterRecallExistingCharLimit, nil
}

func (b *BatchContext) ShouldUseCompactFusion(clusterID int64, loader ClusterSnapshotLoader) (bool, *models.ClusterBatchSnapshot, error) {
	snapshot, err := b.EnsureClusterSnapshot(clusterID, loader)
	if err != nil || snapshot == nil {
		return false, snapshot, err
	}
	if snapshot.CreatedInBatch {
		return false, snapshot, nil
	}
	compact := snapshot.ExistingArticleCount > fusionCompactExistingArticleThreshold ||
		snapshot.ExistingTotalChars > fusionCompactExistingCharThreshold
	return compact, snapshot, nil
}

func (b *BatchContext) NewArticleIDs(clusterID int64) []int64 {
	if b == nil || clusterID <= 0 {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	tracked := b.clusters[clusterID]
	if tracked == nil || len(tracked.newArticleIDs) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(tracked.newArticleIDs))
	for articleID := range tracked.newArticleIDs {
		ids = append(ids, articleID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
