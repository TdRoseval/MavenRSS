package dedup

import (
	"log"

	"MavenRSS/internal/store/sqlite"
)

const (
	// SimHashThreshold is the maximum Hamming distance for SimHash dedup
	SimHashThreshold = 3
	// SemanticThreshold is the max cosine distance for semantic dedup (1 - 0.85 = 0.15)
	SemanticThreshold = 0.15
	// SemanticTopK is the number of nearest neighbors to retrieve
	SemanticTopK = 5
)

// ProcessArticle runs the dedup pipeline (Steps 1-3) for a single article.
// It computes SimHash, checks for duplicates, and assigns the article to a cluster.
// The article must already have a summary and embedding stored.
func ProcessArticle(db *sqlite.DB, articleID, userID int64) error {
	// Get article with summary
	article, err := db.GetArticleByID(articleID)
	if err != nil || article == nil {
		return err
	}

	summaryText := article.Summary
	if summaryText == "" {
		// No summary available, create a standalone cluster
		return createStandaloneCluster(db, articleID, userID, article.IsFavorite)
	}

	// Step 1: SimHash-based literal dedup
	if !IsValidForSimHash(summaryText) {
		return createStandaloneCluster(db, articleID, userID, article.IsFavorite)
	}

	hash := ComputeSimHash64(summaryText)
	b1, b2, b3, b4 := SplitBands(hash)

	// Store SimHash
	if err := db.UpdateArticleSimHash(articleID, hash, b1, b2, b3, b4); err != nil {
		log.Printf("Failed to store SimHash for article %d: %v", articleID, err)
	}

	// Find SimHash candidates via pigeonhole
	candidates, err := db.FindSimHashCandidates(userID, b1, b2, b3, b4)
	if err != nil {
		log.Printf("SimHash candidate search failed for article %d: %v", articleID, err)
	}

	for _, c := range candidates {
		if c.ArticleID == articleID {
			continue
		}
		dist := HammingDistance(hash, c.SimHash64)
		if dist <= SimHashThreshold {
			log.Printf("Article %d matched article %d via SimHash (distance=%d), joining cluster %d",
				articleID, c.ArticleID, dist, c.ClusterID)
			return joinCluster(db, articleID, c.ClusterID, article.IsFavorite)
		}
	}

	// Step 2: Vector semantic dedup
	clusterID, err := semanticSearch(db, articleID, userID)
	if err != nil {
		log.Printf("Semantic search failed for article %d: %v", articleID, err)
	}
	if clusterID > 0 {
		log.Printf("Article %d matched cluster %d via semantic search", articleID, clusterID)
		return joinCluster(db, articleID, clusterID, article.IsFavorite)
	}

	// Step 3: No match — create new standalone cluster
	return createStandaloneCluster(db, articleID, userID, article.IsFavorite)
}

func semanticSearch(db *sqlite.DB, articleID, userID int64) (int64, error) {
	// Get the article's summary embedding from article_embeddings
	var summaryEmbBlob []byte
	err := db.QueryRow(
		`SELECT summary_embedding FROM article_embeddings WHERE article_id = ?`,
		articleID,
	).Scan(&summaryEmbBlob)
	if err != nil || len(summaryEmbBlob) == 0 {
		return 0, nil // No embedding available
	}

	results, err := db.FindSemanticCandidates(userID, summaryEmbBlob, SemanticTopK)
	if err != nil {
		return 0, err
	}

	for _, r := range results {
		if r.ArticleID == articleID {
			continue
		}
		if r.Distance <= SemanticThreshold {
			return r.ClusterID, nil
		}
	}
	return 0, nil
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
	// Mark cluster for re-fusion
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
