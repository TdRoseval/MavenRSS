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
	// SemanticSearchTopK limits ANN recall breadth before centroid reranking.
	SemanticSearchTopK = 200
)

// ProcessArticle runs the cluster assignment pipeline for a single article.
// The article must already have a summary and summary embedding stored.
func ProcessArticle(db *sqlite.DB, articleID, userID int64) error {
	article, err := db.GetArticleByID(articleID)
	if err != nil || article == nil {
		return err
	}

	summaryText := article.Summary
	if summaryText == "" {
		return createStandaloneCluster(db, articleID, userID, article.IsFavorite)
	}

	currentSummaryVec, err := loadArticleSummaryVector(db, articleID)
	if err != nil {
		log.Printf("Failed to load summary embedding for article %d: %v", articleID, err)
	}
	if len(currentSummaryVec) == 0 {
		return createStandaloneCluster(db, articleID, userID, article.IsFavorite)
	}

	if IsValidForSimHash(summaryText) {
		hash := ComputeSimHash64(summaryText)
		b1, b2, b3, b4 := SplitBands(hash)

		if err := db.UpdateArticleSimHash(articleID, hash, b1, b2, b3, b4); err != nil {
			log.Printf("Failed to store SimHash for article %d: %v", articleID, err)
		}

		clusterID, err := findBestHammingCluster(db, articleID, userID, hash, b1, b2, b3, b4, currentSummaryVec)
		if err != nil {
			log.Printf("Hamming cluster selection failed for article %d: %v", articleID, err)
		}
		if clusterID > 0 {
			log.Printf("Article %d matched cluster %d via SimHash + summary ranking", articleID, clusterID)
			return joinCluster(db, articleID, clusterID, article.IsFavorite)
		}
	}

	clusterID, err := semanticSearch(db, articleID, userID, currentSummaryVec)
	if err != nil {
		log.Printf("Semantic search failed for article %d: %v", articleID, err)
	}
	if clusterID > 0 {
		log.Printf("Article %d matched cluster %d via summary centroid search", articleID, clusterID)
		return joinCluster(db, articleID, clusterID, article.IsFavorite)
	}

	return createStandaloneCluster(db, articleID, userID, article.IsFavorite)
}

func semanticSearch(db *sqlite.DB, articleID, userID int64, currentSummaryVec []float32) (int64, error) {
	clusterIDs, err := semanticCandidateClusterIDs(db, articleID, userID, currentSummaryVec)
	if err != nil {
		log.Printf("Semantic ANN candidate recall failed for article %d, falling back to full scan: %v", articleID, err)
		return semanticSearchFullScan(db, articleID, userID, currentSummaryVec)
	}
	if len(clusterIDs) == 0 {
		return 0, nil
	}

	return selectBestClusterByCentroid(db, userID, clusterIDs, currentSummaryVec)
}

func semanticCandidateClusterIDs(db *sqlite.DB, articleID, userID int64, currentSummaryVec []float32) ([]int64, error) {
	queryBlob, err := interest.SerializeVector(currentSummaryVec)
	if err != nil {
		return nil, err
	}

	candidates, err := db.FindSemanticCandidates(userID, queryBlob, SemanticSearchTopK)
	if err != nil {
		return nil, err
	}

	clusterSet := make(map[int64]struct{})
	for _, candidate := range candidates {
		if candidate.ArticleID == articleID || candidate.ClusterID <= 0 {
			continue
		}
		if candidate.Distance > SemanticDistanceThreshold {
			continue
		}
		clusterSet[candidate.ClusterID] = struct{}{}
	}

	clusterIDs := make([]int64, 0, len(clusterSet))
	for clusterID := range clusterSet {
		clusterIDs = append(clusterIDs, clusterID)
	}
	sort.Slice(clusterIDs, func(i, j int) bool { return clusterIDs[i] < clusterIDs[j] })
	return clusterIDs, nil
}

func semanticSearchFullScan(db *sqlite.DB, articleID, userID int64, currentSummaryVec []float32) (int64, error) {
	candidates, err := db.ListClusteredArticleSummaryEmbeddings(userID)
	if err != nil {
		return 0, err
	}

	clusterSet := make(map[int64]struct{})
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
	}

	if len(clusterSet) == 0 {
		return 0, nil
	}

	clusterIDs := make([]int64, 0, len(clusterSet))
	for clusterID := range clusterSet {
		clusterIDs = append(clusterIDs, clusterID)
	}
	sort.Slice(clusterIDs, func(i, j int) bool { return clusterIDs[i] < clusterIDs[j] })

	return selectBestClusterByCentroid(db, userID, clusterIDs, currentSummaryVec)
}

func selectBestClusterByCentroid(db *sqlite.DB, userID int64, clusterIDs []int64, currentSummaryVec []float32) (int64, error) {
	clusterEmbeddings, err := db.ListClusterSummaryEmbeddingsByClusterIDs(userID, clusterIDs)
	if err != nil {
		return 0, err
	}

	clusterVectors := make(map[int64][][]float32)
	for _, embedding := range clusterEmbeddings {
		vec, err := deserializeNormalizedVector(embedding.SummaryEmbedding)
		if err != nil {
			continue
		}
		clusterVectors[embedding.ClusterID] = append(clusterVectors[embedding.ClusterID], vec)
	}

	bestClusterID := int64(0)
	bestDistance := math.MaxFloat64
	for _, clusterID := range clusterIDs {
		vectors := clusterVectors[clusterID]
		if len(vectors) == 0 {
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
			bestClusterID = clusterID
			bestDistance = distance
		}
	}

	return bestClusterID, nil
}

func findBestHammingCluster(
	db *sqlite.DB,
	articleID, userID int64,
	hash int64,
	b1, b2, b3, b4 int16,
	currentSummaryVec []float32,
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

func joinCluster(db *sqlite.DB, articleID, clusterID int64, articleIsFavorite bool) error {
	if err := db.UpdateArticleClusterID(articleID, clusterID); err != nil {
		return err
	}
	if err := syncClusterFavoriteFromArticle(db, clusterID, articleIsFavorite); err != nil {
		return err
	}
	if err := db.UpdateClusterArticleCount(clusterID); err != nil {
		return err
	}
	return db.UpdateClusterStatus(clusterID, "pending_merge")
}

func createStandaloneCluster(db *sqlite.DB, articleID, userID int64, articleIsFavorite bool) error {
	clusterID, err := db.CreateCluster(userID, "pending_merge")
	if err != nil {
		return err
	}
	if err := db.UpdateArticleClusterID(articleID, clusterID); err != nil {
		return err
	}
	if err := syncClusterFavoriteFromArticle(db, clusterID, articleIsFavorite); err != nil {
		return err
	}
	if err := db.UpdateClusterArticleCount(clusterID); err != nil {
		return err
	}
	return nil
}

func syncClusterFavoriteFromArticle(db *sqlite.DB, clusterID int64, articleIsFavorite bool) error {
	if !articleIsFavorite {
		return nil
	}
	return db.SetClusterFavorite(clusterID, true)
}
