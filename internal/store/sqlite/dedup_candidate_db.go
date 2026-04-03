package sqlite

import (
	"fmt"
	"strings"
)

type SimHashCandidateArticle struct {
	ArticleID        int64
	ClusterID        int64
	SimHash64        int64
	SummaryEmbedding []byte
}

type ClusteredArticleEmbedding struct {
	ArticleID        int64
	ClusterID        int64
	SummaryEmbedding []byte
}

type ClusterSummaryEmbedding struct {
	ClusterID        int64
	ArticleID        int64
	SummaryEmbedding []byte
}

// FindSimHashCandidateArticles returns clustered articles that match at least one SimHash band.
func (db *DB) FindSimHashCandidateArticles(userID int64, b1, b2, b3, b4 int16) ([]SimHashCandidateArticle, error) {
	db.WaitForReady()

	rows, err := db.Query(
		`SELECT a.id, a.cluster_id, a.simhash_64, ae.summary_embedding
		 FROM articles a
		 LEFT JOIN article_embeddings ae ON ae.article_id = a.id
		 WHERE a.user_id = ? AND a.cluster_id IS NOT NULL
		   AND (a.simhash_b1 = ? OR a.simhash_b2 = ? OR a.simhash_b3 = ? OR a.simhash_b4 = ?)
		 ORDER BY a.id ASC`,
		userID, b1, b2, b3, b4,
	)
	if err != nil {
		return nil, fmt.Errorf("find simhash candidate articles: %w", err)
	}
	defer rows.Close()

	candidates := make([]SimHashCandidateArticle, 0)
	for rows.Next() {
		var candidate SimHashCandidateArticle
		if err := rows.Scan(
			&candidate.ArticleID,
			&candidate.ClusterID,
			&candidate.SimHash64,
			&candidate.SummaryEmbedding,
		); err != nil {
			return nil, fmt.Errorf("scan simhash candidate article: %w", err)
		}
		candidates = append(candidates, candidate)
	}

	return candidates, nil
}

// ListClusteredArticleSummaryEmbeddings returns summary embeddings for all clustered articles.
func (db *DB) ListClusteredArticleSummaryEmbeddings(userID int64) ([]ClusteredArticleEmbedding, error) {
	db.WaitForReady()

	rows, err := db.Query(
		`SELECT a.id, a.cluster_id, ae.summary_embedding
		 FROM articles a
		 JOIN article_embeddings ae ON ae.article_id = a.id
		 WHERE a.user_id = ? AND a.cluster_id IS NOT NULL AND ae.summary_embedding IS NOT NULL
		 ORDER BY a.id ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list clustered article summary embeddings: %w", err)
	}
	defer rows.Close()

	results := make([]ClusteredArticleEmbedding, 0)
	for rows.Next() {
		var item ClusteredArticleEmbedding
		if err := rows.Scan(&item.ArticleID, &item.ClusterID, &item.SummaryEmbedding); err != nil {
			return nil, fmt.Errorf("scan clustered article summary embedding: %w", err)
		}
		results = append(results, item)
	}

	return results, nil
}

// ListClusterSummaryEmbeddingsByClusterIDs returns all summary embeddings for the given clusters.
func (db *DB) ListClusterSummaryEmbeddingsByClusterIDs(userID int64, clusterIDs []int64) ([]ClusterSummaryEmbedding, error) {
	db.WaitForReady()

	if len(clusterIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(clusterIDs))
	args := make([]any, 0, len(clusterIDs)+1)
	args = append(args, userID)
	for idx, clusterID := range clusterIDs {
		placeholders[idx] = "?"
		args = append(args, clusterID)
	}

	rows, err := db.Query(
		`SELECT a.cluster_id, a.id, ae.summary_embedding
		 FROM articles a
		 JOIN article_embeddings ae ON ae.article_id = a.id
		 WHERE a.user_id = ? AND a.cluster_id IN (`+strings.Join(placeholders, ",")+`)
		   AND ae.summary_embedding IS NOT NULL
		 ORDER BY a.cluster_id ASC, a.id ASC`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("list cluster summary embeddings by cluster ids: %w", err)
	}
	defer rows.Close()

	results := make([]ClusterSummaryEmbedding, 0)
	for rows.Next() {
		var item ClusterSummaryEmbedding
		if err := rows.Scan(&item.ClusterID, &item.ArticleID, &item.SummaryEmbedding); err != nil {
			return nil, fmt.Errorf("scan cluster summary embedding: %w", err)
		}
		results = append(results, item)
	}

	return results, nil
}

// SampleArticleSummaryEmbeddings returns up to limit random article summary embeddings for health checks.
func (db *DB) SampleArticleSummaryEmbeddings(userID int64, limit int) ([][]byte, error) {
	db.WaitForReady()

	if limit <= 0 {
		limit = 100
	}

	rows, err := db.Query(
		`SELECT ae.summary_embedding
		 FROM article_embeddings ae
		 JOIN articles a ON a.id = ae.article_id
		 WHERE a.user_id = ? AND ae.summary_embedding IS NOT NULL
		 ORDER BY RANDOM()
		 LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("sample article summary embeddings: %w", err)
	}
	defer rows.Close()

	samples := make([][]byte, 0)
	for rows.Next() {
		var blob []byte
		if err := rows.Scan(&blob); err != nil {
			return nil, fmt.Errorf("scan sampled article summary embedding: %w", err)
		}
		samples = append(samples, blob)
	}

	return samples, nil
}
