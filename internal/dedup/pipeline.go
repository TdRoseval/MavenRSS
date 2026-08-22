package dedup

import (
	"fmt"
	"log"
	"math"
	"sort"

	"MavenRSS/internal/interest"
	"MavenRSS/internal/store/sqlite"
)

const (
	// SimHashThreshold is the maximum Hamming distance for SimHash dedup.
	SimHashThreshold = 3
	// SemanticDistanceThreshold is the maximum normalized squared L2 distance for article-level recall.
	SemanticDistanceThreshold = 0.4
	// SemanticSearchTopK is the first-stage recall breadth on the binary-
	// quantized embedding column (Hamming distance). Sign quantization is
	// lossy, so the breadth is amplified vs the legacy float top-200; the
	// exact float gate in pickBestClusterByExactGate prunes the tail.
	SemanticSearchTopK = 800
	// semanticHammingGateRatio prunes recalled articles whose Hamming distance
	// makes the 0.4 float gate unattainable: cos >= 0.8 implies an expected
	// sign-disagreement ratio <= ~0.205, and 0.30 leaves a wide quantization
	// margin while still dropping obviously-far rows before vector loading.
	semanticHammingGateRatio = 0.30
)

type ProcessArticleOptions struct {
	Batch *BatchContext
}

// ProcessArticle runs the cluster assignment pipeline for a single article.
// The article must already have a summary and summary embedding stored.
func ProcessArticle(db *sqlite.DB, articleID, userID int64, opts ...*ProcessArticleOptions) error {
	options := resolveProcessArticleOptions(opts...)
	article, err := db.GetArticleByID(articleID)
	if err != nil || article == nil {
		return err
	}

	summaryText := article.Summary
	if summaryText == "" {
		return createStandaloneCluster(db, articleID, userID, article.IsFavorite, nil, options)
	}

	currentSummaryVec, err := loadArticleSummaryVector(db, articleID)
	if err != nil {
		log.Printf("Failed to load summary embedding for article %d: %v", articleID, err)
	}
	if len(currentSummaryVec) == 0 {
		return createStandaloneCluster(db, articleID, userID, article.IsFavorite, nil, options)
	}

	if IsValidForSimHash(summaryText) {
		hash := ComputeSimHash64(summaryText)
		b1, b2, b3, b4 := SplitBands(hash)

		if err := db.UpdateArticleSimHash(articleID, hash, b1, b2, b3, b4); err != nil {
			log.Printf("Failed to store SimHash for article %d: %v", articleID, err)
		}

		clusterID, err := findBestHammingCluster(db, articleID, userID, hash, b1, b2, b3, b4, currentSummaryVec, options)
		if err != nil {
			log.Printf("Hamming cluster selection failed for article %d: %v", articleID, err)
		}
		if clusterID > 0 {
			log.Printf("Article %d matched cluster %d via SimHash + summary ranking", articleID, clusterID)
			return joinCluster(db, articleID, clusterID, article.IsFavorite, currentSummaryVec, options)
		}
	}

	clusterID, err := semanticSearch(db, articleID, userID, currentSummaryVec, options)
	if err != nil {
		log.Printf("Semantic search failed for article %d: %v", articleID, err)
	}
	if clusterID > 0 {
		log.Printf("Article %d matched cluster %d via summary centroid search", articleID, clusterID)
		return joinCluster(db, articleID, clusterID, article.IsFavorite, currentSummaryVec, options)
	}

	return createStandaloneCluster(db, articleID, userID, article.IsFavorite, currentSummaryVec, options)
}

func resolveProcessArticleOptions(opts ...*ProcessArticleOptions) *ProcessArticleOptions {
	for _, opt := range opts {
		if opt != nil {
			return opt
		}
	}
	return &ProcessArticleOptions{}
}

// semanticSearch runs the two-stage semantic recall:
//  1. binary-quantized (Hamming) ANN recall on the embedding table, gated by
//     a loose Hamming threshold
//  2. exact float32 rerank: clusters qualify only with a member within
//     SemanticDistanceThreshold, then ranked by centroid distance using
//     batch-cached member vectors
//
// Any stage-1 failure falls back to a full in-memory scan.
func semanticSearch(db *sqlite.DB, articleID, userID int64, currentSummaryVec []float32, opts *ProcessArticleOptions) (int64, error) {
	queryBlob, err := interest.SerializeVector(currentSummaryVec)
	if err != nil {
		return semanticSearchFullScan(db, articleID, userID, currentSummaryVec, opts)
	}

	candidates, err := db.FindSemanticCandidates(userID, queryBlob, SemanticSearchTopK)
	if err != nil {
		log.Printf("Semantic ANN candidate recall failed for article %d, falling back to full scan: %v", articleID, err)
		return semanticSearchFullScan(db, articleID, userID, currentSummaryVec, opts)
	}
	if len(candidates) == 0 {
		return 0, nil
	}

	hammingGate := semanticHammingGateRatio * float64(len(currentSummaryVec))
	clusterSet := make(map[int64]struct{})
	for _, candidate := range candidates {
		if candidate.ArticleID == articleID || candidate.ClusterID <= 0 {
			continue
		}
		if candidate.Distance > hammingGate {
			continue
		}
		clusterSet[candidate.ClusterID] = struct{}{}
	}
	if len(clusterSet) == 0 {
		return 0, nil
	}

	clusterIDs := sortedClusterIDs(clusterSet)
	clusterIDs, err = filterClusterIDsByBatchLimits(db, clusterIDs, opts)
	if err != nil {
		log.Printf("Semantic candidate batch filtering failed for article %d, falling back to full scan: %v", articleID, err)
		return semanticSearchFullScan(db, articleID, userID, currentSummaryVec, opts)
	}
	if len(clusterIDs) == 0 {
		return 0, nil
	}

	members, err := loadClusterMemberVectors(db, opts, userID, clusterIDs)
	if err != nil {
		log.Printf("Semantic member vector loading failed for article %d, falling back to full scan: %v", articleID, err)
		return semanticSearchFullScan(db, articleID, userID, currentSummaryVec, opts)
	}

	return pickBestClusterByExactGate(clusterIDs, members, currentSummaryVec), nil
}

func sortedClusterIDs(clusterSet map[int64]struct{}) []int64 {
	clusterIDs := make([]int64, 0, len(clusterSet))
	for clusterID := range clusterSet {
		clusterIDs = append(clusterIDs, clusterID)
	}
	sort.Slice(clusterIDs, func(i, j int) bool { return clusterIDs[i] < clusterIDs[j] })
	return clusterIDs
}

// loadClusterMemberVectors loads member summary vectors for the given
// clusters, backed by the BatchContext centroid cache. Clusters joined during
// this batch are served entirely from cache; membership stays exact because
// joinCluster/createStandaloneCluster maintain the cache alongside the DB.
func loadClusterMemberVectors(db *sqlite.DB, opts *ProcessArticleOptions, userID int64, clusterIDs []int64) (map[int64][][]float32, error) {
	members := make(map[int64][][]float32, len(clusterIDs))
	missing := make([]int64, 0, len(clusterIDs))
	for _, clusterID := range clusterIDs {
		if vecs, ok := opts.Batch.MemberVectors(clusterID); ok {
			members[clusterID] = vecs
		} else {
			missing = append(missing, clusterID)
		}
	}

	if len(missing) > 0 {
		rows, err := db.ListClusterSummaryEmbeddingsByClusterIDs(userID, missing)
		if err != nil {
			return nil, err
		}
		grouped := make(map[int64][][]float32)
		for _, row := range rows {
			vec, err := deserializeNormalizedVector(row.SummaryEmbedding)
			if err != nil || len(vec) == 0 {
				continue
			}
			grouped[row.ClusterID] = append(grouped[row.ClusterID], vec)
		}
		for _, clusterID := range missing {
			opts.Batch.SetMemberVectors(clusterID, grouped[clusterID])
			members[clusterID] = grouped[clusterID]
		}
	}

	return members, nil
}

// pickBestClusterByExactGate keeps only clusters with at least one member
// within SemanticDistanceThreshold (exact float32), then picks the cluster
// whose member centroid is closest to the query vector. Tie-break: lowest
// cluster ID.
func pickBestClusterByExactGate(clusterIDs []int64, members map[int64][][]float32, currentSummaryVec []float32) int64 {
	bestClusterID := int64(0)
	bestDistance := math.MaxFloat64
	for _, clusterID := range clusterIDs {
		vectors := members[clusterID]
		gated := false
		for _, vec := range vectors {
			if distance, err := interest.SquaredL2Distance(currentSummaryVec, vec); err == nil && distance <= SemanticDistanceThreshold {
				gated = true
				break
			}
		}
		if !gated {
			continue
		}

		center, err := interest.AverageAndNormalize(vectors)
		if err != nil {
			continue
		}
		distance, err := interest.SquaredL2Distance(currentSummaryVec, center)
		if err != nil {
			continue
		}
		if distance < bestDistance || (distance == bestDistance && (bestClusterID == 0 || clusterID < bestClusterID)) {
			bestDistance = distance
			bestClusterID = clusterID
		}
	}
	return bestClusterID
}

func semanticSearchFullScan(db *sqlite.DB, articleID, userID int64, currentSummaryVec []float32, opts *ProcessArticleOptions) (int64, error) {
	candidates, err := db.ListClusteredArticleSummaryEmbeddings(userID)
	if err != nil {
		return 0, err
	}

	clusterSet := make(map[int64]struct{})
	members := make(map[int64][][]float32)
	for _, candidate := range candidates {
		if candidate.ArticleID == articleID || candidate.ClusterID <= 0 || len(candidate.SummaryEmbedding) == 0 {
			continue
		}

		candidateVec, err := deserializeNormalizedVector(candidate.SummaryEmbedding)
		if err != nil {
			continue
		}

		distance, err := interest.SquaredL2Distance(currentSummaryVec, candidateVec)
		if err != nil {
			continue
		}
		if distance <= SemanticDistanceThreshold {
			clusterSet[candidate.ClusterID] = struct{}{}
		}
		// Centroids rank over all member vectors, not only gated ones.
		members[candidate.ClusterID] = append(members[candidate.ClusterID], candidateVec)
	}

	if len(clusterSet) == 0 {
		return 0, nil
	}

	clusterIDs, err := filterClusterIDsByBatchLimits(db, sortedClusterIDs(clusterSet), opts)
	if err != nil {
		return 0, err
	}

	return pickBestClusterByExactGate(clusterIDs, members, currentSummaryVec), nil
}

func filterClusterIDsByBatchLimits(db *sqlite.DB, clusterIDs []int64, opts *ProcessArticleOptions) ([]int64, error) {
	if len(clusterIDs) == 0 || opts == nil || opts.Batch == nil {
		return clusterIDs, nil
	}

	filtered := make([]int64, 0, len(clusterIDs))
	for _, clusterID := range clusterIDs {
		ignore, err := opts.Batch.ShouldIgnoreClusterForRecall(clusterID, db.GetClusterSnapshot)
		if err != nil {
			return nil, err
		}
		if ignore {
			continue
		}
		filtered = append(filtered, clusterID)
	}
	return filtered, nil
}

func findBestHammingCluster(
	db *sqlite.DB,
	articleID, userID int64,
	hash int64,
	b1, b2, b3, b4 int16,
	currentSummaryVec []float32,
	opts *ProcessArticleOptions,
) (int64, error) {
	candidates, err := db.FindSimHashCandidateArticles(userID, b1, b2, b3, b4)
	if err != nil {
		return 0, err
	}

	bestClusterID := int64(0)
	bestDistance := math.MaxFloat64
	bestArticleID := int64(0)

	for _, candidate := range candidates {
		if candidate.ArticleID == articleID || candidate.ClusterID <= 0 {
			continue
		}
		if opts != nil && opts.Batch != nil {
			ignore, err := opts.Batch.ShouldIgnoreClusterForRecall(candidate.ClusterID, db.GetClusterSnapshot)
			if err != nil {
				return 0, err
			}
			if ignore {
				continue
			}
		}
		if HammingDistance(hash, candidate.SimHash64) > SimHashThreshold {
			continue
		}
		if len(candidate.SummaryEmbedding) == 0 {
			continue
		}

		candidateVec, err := deserializeNormalizedVector(candidate.SummaryEmbedding)
		if err != nil {
			continue
		}

		distance, err := interest.SquaredL2Distance(currentSummaryVec, candidateVec)
		if err != nil {
			continue
		}
		if distance < bestDistance || (distance == bestDistance && (bestArticleID == 0 || candidate.ArticleID < bestArticleID)) {
			bestDistance = distance
			bestClusterID = candidate.ClusterID
			bestArticleID = candidate.ArticleID
		}
	}

	return bestClusterID, nil
}

func loadArticleSummaryVector(db *sqlite.DB, articleID int64) ([]float32, error) {
	var summaryEmbBlob []byte
	err := db.QueryRow(
		`SELECT summary_embedding FROM article_embeddings WHERE article_id = ?`,
		articleID,
	).Scan(&summaryEmbBlob)
	if err != nil || len(summaryEmbBlob) == 0 {
		return nil, nil
	}
	return deserializeNormalizedVector(summaryEmbBlob)
}

func deserializeNormalizedVector(blob []byte) ([]float32, error) {
	vec, err := interest.DeserializeVector(blob)
	if err != nil {
		return nil, fmt.Errorf("deserialize vector: %w", err)
	}
	if len(vec) == 0 {
		return nil, nil
	}
	return interest.NormalizeVector(vec), nil
}

// joinCluster assigns the article to a cluster and keeps the batch centroid
// cache in sync so subsequent semantic rankings see the new member exactly.
func joinCluster(db *sqlite.DB, articleID, clusterID int64, articleIsFavorite bool, summaryVec []float32, opts *ProcessArticleOptions) error {
	if err := db.JoinArticleCluster(articleID, clusterID, articleIsFavorite); err != nil {
		return err
	}
	if opts != nil {
		opts.Batch.RecordNewArticle(clusterID, articleID)
		opts.Batch.AppendMemberVector(clusterID, summaryVec)
	}
	return nil
}

func createStandaloneCluster(db *sqlite.DB, articleID, userID int64, articleIsFavorite bool, summaryVec []float32, opts *ProcessArticleOptions) error {
	clusterID, err := db.CreateStandaloneClusterForArticle(userID, articleID, articleIsFavorite)
	if err != nil {
		return err
	}
	if opts != nil {
		opts.Batch.MarkClusterCreated(clusterID)
		opts.Batch.RecordNewArticle(clusterID, articleID)
		if len(summaryVec) > 0 {
			opts.Batch.SetMemberVectors(clusterID, [][]float32{summaryVec})
		}
	}
	return nil
}
