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
	// maxMemberVectorCacheClusters bounds the centroid cache memory; when
	// exceeded the cache resets (desktop-scale batches rarely revisit more).
	maxMemberVectorCacheClusters = 1024
)

type ClusterSnapshotLoader func(clusterID int64) (*models.ClusterBatchSnapshot, error)

type trackedClusterSnapshot struct {
	snapshot      models.ClusterBatchSnapshot
	newArticleIDs map[int64]struct{}
}

// BatchContext tracks pre-batch cluster baselines so the current batch's
// incoming articles do not affect cluster-size guards or compact fusion input.
// It also caches per-cluster member summary vectors so the semantic pipeline
// can gate and centroid-rank candidates without reloading embeddings from the
// database for every article. Article-to-cluster assignments change only
// through this pipeline (serialized per user), so the cache stays exact when
// maintained via SetMemberVectors/AppendMemberVector.
type BatchContext struct {
	mu            sync.Mutex
	clusters      map[int64]*trackedClusterSnapshot
	memberVectors map[int64][][]float32
}

func NewBatchContext() *BatchContext {
	return &BatchContext{
		clusters:      make(map[int64]*trackedClusterSnapshot),
		memberVectors: make(map[int64][][]float32),
	}
}

// MemberVectors returns the cached member vectors for a cluster and whether
// an entry exists (a nil slice with ok=true means "cluster has no embedded
// members", which is also a valid cached answer).
func (b *BatchContext) MemberVectors(clusterID int64) ([][]float32, bool) {
	if b == nil || clusterID <= 0 {
		return nil, false
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	vecs, ok := b.memberVectors[clusterID]
	return vecs, ok
}

// SetMemberVectors caches the member vectors of a cluster, detaching the
// stored slice from the caller's copy. Zero-length vectors are dropped.
func (b *BatchContext) SetMemberVectors(clusterID int64, vecs [][]float32) {
	if b == nil || clusterID <= 0 {
		return
	}

	stored := make([][]float32, 0, len(vecs))
	for _, vec := range vecs {
		if len(vec) > 0 {
			stored = append(stored, vec)
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.memberVectors == nil {
		b.memberVectors = make(map[int64][][]float32)
	}
	if len(b.memberVectors) >= maxMemberVectorCacheClusters {
		b.memberVectors = make(map[int64][][]float32)
	}
	b.memberVectors[clusterID] = stored
}

// AppendMemberVector records a newly joined article's vector so subsequent
// centroid rankings in the same batch see the updated membership exactly.
func (b *BatchContext) AppendMemberVector(clusterID int64, vec []float32) {
	if b == nil || clusterID <= 0 || len(vec) == 0 {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.memberVectors == nil {
		b.memberVectors = make(map[int64][][]float32)
	}
	b.memberVectors[clusterID] = append(b.memberVectors[clusterID], vec)
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
