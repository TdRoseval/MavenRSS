package sqlite

import (
	"fmt"
	"strings"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

func setupPartitionTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	if err := db.Init(); err != nil {
		db.Close()
		t.Fatalf("Init error: %v", err)
	}
	return db
}

func mustPartitionTestBlob(t *testing.T, values map[int]float32) []byte {
	t.Helper()
	vec := make([]float32, 1024)
	for idx, value := range values {
		if idx < 0 || idx >= 1024 {
			t.Fatalf("index %d out of range [0, 1024)", idx)
		}
		vec[idx] = value
	}
	blob, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		t.Fatalf("SerializeFloat32 error: %v", err)
	}
	return blob
}

// mustSignedTestBlob builds a normalized vector of ±1/sqrt(1024) components
// from a sign pattern, so binary quantization is exact and deterministic.
func mustSignedTestBlob(t *testing.T, positiveDims int) []byte {
	t.Helper()
	vec := make([]float32, 1024)
	sqrtInv := float32(0.03125) // 1/sqrt(1024) = 1/32
	for i := range vec {
		if i < positiveDims {
			vec[i] = sqrtInv
		} else {
			vec[i] = -sqrtInv
		}
	}
	blob, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		t.Fatalf("SerializeFloat32 error: %v", err)
	}
	return blob
}

func tableCreateSQL(t *testing.T, db *DB, tableName string) string {
	t.Helper()
	var createSQL string
	if err := db.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?`,
		tableName,
	).Scan(&createSQL); err != nil {
		t.Fatalf("read create SQL for %s: %v", tableName, err)
	}
	return createSQL
}

func requireVecTargetSchema(t *testing.T, db *DB, tables ...string) {
	t.Helper()
	for _, table := range tables {
		createSQL := strings.ToLower(tableCreateSQL(t, db, table))
		if !strings.Contains(createSQL, "partition key") {
			t.Fatalf("%s missing partition key: %s", table, createSQL)
		}
		if !strings.Contains(createSQL, "summary_embedding_bin") {
			t.Fatalf("%s missing summary_embedding_bin column: %s", table, createSQL)
		}
	}
}

// TestVecTablesCreatedWithTargetSchema verifies a fresh database creates the
// embedding vec0 tables with a user_id partition key and a binary-quantized
// recall column.
func TestVecTablesCreatedWithTargetSchema(t *testing.T) {
	db := setupPartitionTestDB(t)
	defer db.Close()
	requireVecTargetSchema(t, db, "article_embeddings", "cluster_embeddings")
}

func createPartitionTestUserArticles(t *testing.T, db *DB, userID int64, count int) []int64 {
	t.Helper()

	// The desktop-mode bootstrap only creates user 1; extra users must be
	// inserted explicitly to satisfy the articles.user_id foreign key.
	if userID > 1 {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO users (id, username, email, password_hash, role, status) VALUES (?, ?, ?, 'hash', 'user', 'active')`,
			userID, fmt.Sprintf("partition-user-%d", userID), fmt.Sprintf("partition-%d@example.com", userID),
		); err != nil {
			t.Fatalf("ensure user %d: %v", userID, err)
		}
	}

	result, err := db.Exec(
		`INSERT INTO feeds (user_id, title, url, last_updated) VALUES (?, ?, ?, datetime('now'))`,
		userID, "Partition Feed", fmt.Sprintf("https://example.com/partition-feed-%d.xml", userID),
	)
	if err != nil {
		t.Fatalf("insert feed for user %d: %v", userID, err)
	}
	feedID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("feed LastInsertId error: %v", err)
	}

	ids := make([]int64, 0, count)
	for i := 0; i < count; i++ {
		result, err := db.Exec(
			`INSERT INTO articles (user_id, feed_id, title, url, published_at, summary, unique_id)
			 VALUES (?, ?, ?, ?, datetime('now'), ?, ?)`,
			userID, feedID,
			"partition article", fmt.Sprintf("https://example.com/partition/%d/%d", userID, i),
			"partition summary", fmt.Sprintf("partition-%d-%d", userID, i),
		)
		if err != nil {
			t.Fatalf("insert article for user %d: %v", userID, err)
		}
		articleID, err := result.LastInsertId()
		if err != nil {
			t.Fatalf("article LastInsertId error: %v", err)
		}
		ids = append(ids, articleID)
	}
	return ids
}

// TestFindSemanticCandidatesBitRecallPartitionIsolated verifies the
// binary-quantized KNN query only returns articles belonging to the querying
// user's partition, with Hamming distances.
func TestFindSemanticCandidatesBitRecallPartitionIsolated(t *testing.T) {
	db := setupPartitionTestDB(t)
	defer db.Close()

	userOneArticles := createPartitionTestUserArticles(t, db, 1, 2)
	userTwoArticles := createPartitionTestUserArticles(t, db, 2, 2)

	// Same-direction vectors: Hamming distance must be 0.
	sameBlob := mustPartitionTestBlob(t, map[int]float32{0: 1})
	if err := db.UpdateArticleEmbeddings(userOneArticles[0], nil, sameBlob); err != nil {
		t.Fatalf("UpdateArticleEmbeddings user one error: %v", err)
	}
	if err := db.UpdateArticleEmbeddings(userTwoArticles[0], nil, sameBlob); err != nil {
		t.Fatalf("UpdateArticleEmbeddings user two error: %v", err)
	}

	userOneCluster, err := db.CreateCluster(1, "complete")
	if err != nil {
		t.Fatalf("CreateCluster user one error: %v", err)
	}
	if err := db.UpdateArticleClusterID(userOneArticles[0], userOneCluster); err != nil {
		t.Fatalf("UpdateArticleClusterID user one error: %v", err)
	}
	userTwoCluster, err := db.CreateCluster(2, "complete")
	if err != nil {
		t.Fatalf("CreateCluster user two error: %v", err)
	}
	if err := db.UpdateArticleClusterID(userTwoArticles[0], userTwoCluster); err != nil {
		t.Fatalf("UpdateArticleClusterID user two error: %v", err)
	}

	candidates, err := db.FindSemanticCandidates(1, sameBlob, 10)
	if err != nil {
		t.Fatalf("FindSemanticCandidates error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].ArticleID != userOneArticles[0] {
		t.Fatalf("user one candidates = %v, want exactly article %d", candidates, userOneArticles[0])
	}
	if candidates[0].Distance != 0 {
		t.Fatalf("same-direction Hamming distance = %v, want 0", candidates[0].Distance)
	}

	candidatesTwo, err := db.FindSemanticCandidates(2, sameBlob, 10)
	if err != nil {
		t.Fatalf("FindSemanticCandidates user two error: %v", err)
	}
	if len(candidatesTwo) != 1 || candidatesTwo[0].ArticleID != userTwoArticles[0] {
		t.Fatalf("user two candidates = %v, want exactly article %d", candidatesTwo, userTwoArticles[0])
	}
}

// TestFindSemanticCandidatesHammingOrdering verifies Hamming distances order
// correctly on the bit column: aligned vectors first, orthogonal (~half
// sign flips) far away.
func TestFindSemanticCandidatesHammingOrdering(t *testing.T) {
	db := setupPartitionTestDB(t)
	defer db.Close()

	articles := createPartitionTestUserArticles(t, db, 1, 3)
	clusterID, err := db.CreateCluster(1, "complete")
	if err != nil {
		t.Fatalf("CreateCluster error: %v", err)
	}

	// Query pattern: dims 0-511 positive, 512-1023 negative.
	queryBlob := mustSignedTestBlob(t, 512)
	// Aligned candidate: identical sign pattern (Hamming 0).
	if err := db.UpdateArticleEmbeddings(articles[0], nil, mustSignedTestBlob(t, 512)); err != nil {
		t.Fatalf("UpdateArticleEmbeddings aligned error: %v", err)
	}
	// Partially aligned candidate: 300 positive dims → sign disagreements on
	// dims 300-511 → Hamming = 212.
	if err := db.UpdateArticleEmbeddings(articles[1], nil, mustSignedTestBlob(t, 300)); err != nil {
		t.Fatalf("UpdateArticleEmbeddings half error: %v", err)
	}
	for _, id := range articles[:2] {
		if err := db.UpdateArticleClusterID(id, clusterID); err != nil {
			t.Fatalf("UpdateArticleClusterID %d error: %v", id, err)
		}
	}

	candidates, err := db.FindSemanticCandidates(1, queryBlob, 10)
	if err != nil {
		t.Fatalf("FindSemanticCandidates error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidates = %d, want 2", len(candidates))
	}
	if candidates[0].ArticleID != articles[0] || candidates[0].Distance != 0 {
		t.Fatalf("first candidate = (%d, %v), want (%d, 0)", candidates[0].ArticleID, candidates[0].Distance, articles[0])
	}
	if candidates[1].ArticleID != articles[1] || candidates[1].Distance != 212 {
		t.Fatalf("second candidate = (%d, %v), want (%d, 212)", candidates[1].ArticleID, candidates[1].Distance, articles[1])
	}
}

// TestUpdateArticleEmbeddingsPopulatesBinaryColumn verifies the write path
// derives summary_embedding_bin from the summary vector via
// vec_quantize_binary, matching an independent SQL evaluation.
func TestUpdateArticleEmbeddingsPopulatesBinaryColumn(t *testing.T) {
	db := setupPartitionTestDB(t)
	defer db.Close()

	articleID := createPartitionTestUserArticles(t, db, 1, 1)[0]
	summaryBlob := mustSignedTestBlob(t, 100) // 100 positive dims

	if err := db.UpdateArticleEmbeddings(articleID, nil, summaryBlob); err != nil {
		t.Fatalf("UpdateArticleEmbeddings error: %v", err)
	}

	var storedBin []byte
	if err := db.QueryRow(
		`SELECT summary_embedding_bin FROM article_embeddings WHERE article_id = ?`,
		articleID,
	).Scan(&storedBin); err != nil {
		t.Fatalf("query binary column error: %v", err)
	}
	if len(storedBin) != 128 {
		t.Fatalf("binary blob length = %d, want 128 (1024 dims / 8)", len(storedBin))
	}

	var expectedBin []byte
	if err := db.QueryRow(
		`SELECT vec_quantize_binary(summary_embedding) FROM article_embeddings WHERE article_id = ?`,
		articleID,
	).Scan(&expectedBin); err != nil {
		t.Fatalf("query vec_quantize_binary error: %v", err)
	}
	if string(storedBin) != string(expectedBin) {
		t.Fatal("stored binary column does not match vec_quantize_binary(summary_embedding)")
	}

	// Sign convention spot-check: dims 0-99 positive → bits 0-99 set
	// (byte 11 full, byte 12 low nibble, byte 13 clear).
	if storedBin[11] != 0xFF || storedBin[12] != 0x0F {
		t.Fatalf("expected bits 0-99 set: bytes 11-12 = %x %x", storedBin[11], storedBin[12])
	}
	if storedBin[13] != 0x00 {
		t.Fatalf("expected bits 100+ clear: byte 13 = %x", storedBin[13])
	}
}

// TestGetClustersByVectorSimilarityReranksWithFloat verifies the two-stage
// query: bit (Hamming) recall followed by exact float32 rerank. The vectors
// use magnitude skew (large dims 0-9, tiny dims 10+) so the Hamming ranking
// disagrees with the float ranking — the final result must follow the float
// ranking.
func TestGetClustersByVectorSimilarityReranksWithFloat(t *testing.T) {
	db := setupPartitionTestDB(t)
	defer db.Close()

	userID := int64(3)
	if _, err := db.Exec(
		`INSERT INTO users (id, username, email, password_hash, role, status) VALUES (?, ?, ?, 'hash', 'user', 'active')`,
		userID, "rerank-user", "rerank@example.com",
	); err != nil {
		t.Fatalf("insert user error: %v", err)
	}

	// q: +1.0 on dims 0-9, +0.01 on the rest (all-positive → bin all 1s).
	// near: identical signs on the big dims, but flipped signs on the 1014
	//   tiny dims → Hamming(q,near) = 1014 (worst), yet float distance is tiny
	//   because the tiny dims barely contribute to the dot product.
	// far: flipped signs on the 10 big dims, aligned elsewhere →
	//   Hamming(q,far) = 10 (best), but float distance is dominated by the
	//   big-dim disagreement.
	buildVec := func(bigSign, smallSign float32) []float32 {
		vec := make([]float32, 1024)
		for i := range vec {
			if i < 10 {
				vec[i] = bigSign * 1.0
			} else {
				vec[i] = smallSign * 0.01
			}
		}
		return vec
	}
	queryVec := buildVec(1, 1)
	nearVec := buildVec(1, -1)
	farVec := buildVec(-1, 1)

	queryBlob := mustSerializeTestVector(t, queryVec)
	nearCluster := createRerankTestCluster(t, db, userID, mustSerializeTestVector(t, nearVec))
	farCluster := createRerankTestCluster(t, db, userID, mustSerializeTestVector(t, farVec))

	results, err := db.GetClustersByVectorSimilarity(userID, queryBlob, nil, "", 0, "", 30, 10)
	if err != nil {
		t.Fatalf("GetClustersByVectorSimilarity error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	// Hamming recall order would be [far(10), near(1014)]; float rerank must
	// flip it to [near, far].
	if results[0].Cluster.ID != nearCluster || results[1].Cluster.ID != farCluster {
		t.Fatalf("rerank order = [%d, %d], want [%d, %d] (float distance)",
			results[0].Cluster.ID, results[1].Cluster.ID, nearCluster, farCluster)
	}

	// The reported distances must be exact float32 squared-L2 values (same
	// semantics as the legacy float MATCH ordering on raw stored vectors).
	expectedDistance := func(v []float32) float64 {
		var dot, normQ, normV float64
		for i := range v {
			dot += float64(queryVec[i]) * float64(v[i])
			normQ += float64(queryVec[i]) * float64(queryVec[i])
			normV += float64(v[i]) * float64(v[i])
		}
		return normQ + normV - 2*dot
	}
	if diff := results[0].Distance - expectedDistance(nearVec); diff > 1e-4 || diff < -1e-4 {
		t.Fatalf("near float distance = %v, want %v", results[0].Distance, expectedDistance(nearVec))
	}
	if diff := results[1].Distance - expectedDistance(farVec); diff > 1e-4 || diff < -1e-4 {
		t.Fatalf("far float distance = %v, want %v", results[1].Distance, expectedDistance(farVec))
	}
	if results[0].Distance >= results[1].Distance {
		t.Fatalf("rerank distances not ascending: %v >= %v", results[0].Distance, results[1].Distance)
	}
}

func mustSerializeTestVector(t *testing.T, vec []float32) []byte {
	t.Helper()
	blob, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		t.Fatalf("SerializeFloat32 error: %v", err)
	}
	return blob
}

// TestUpdateArticleEmbeddingsAllNilDeletesRow verifies that clearing both
// embeddings removes the row entirely instead of storing 8KB of zero vectors
// that would pollute every KNN scan.
func TestUpdateArticleEmbeddingsAllNilDeletesRow(t *testing.T) {
	db := setupPartitionTestDB(t)
	defer db.Close()

	articleID := createPartitionTestUserArticles(t, db, 1, 1)[0]

	blob := mustPartitionTestBlob(t, map[int]float32{0: 1})
	if err := db.UpdateArticleEmbeddings(articleID, nil, blob); err != nil {
		t.Fatalf("UpdateArticleEmbeddings with summary error: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM article_embeddings WHERE article_id = ?`, articleID).Scan(&count); err != nil {
		t.Fatalf("count rows error: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count after summary-only upsert = %d, want 1", count)
	}

	if err := db.UpdateArticleEmbeddings(articleID, nil, nil); err != nil {
		t.Fatalf("UpdateArticleEmbeddings all nil error: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM article_embeddings WHERE article_id = ?`, articleID).Scan(&count); err != nil {
		t.Fatalf("count rows after clear error: %v", err)
	}
	if count != 0 {
		t.Fatalf("row count after clearing both embeddings = %d, want 0", count)
	}
}

// TestUpdateArticleEmbeddingsSingleColumnFallback verifies a missing column
// reuses the provided blob, since vec0 rejects NULL vectors.
func TestUpdateArticleEmbeddingsSingleColumnFallback(t *testing.T) {
	db := setupPartitionTestDB(t)
	defer db.Close()

	articleID := createPartitionTestUserArticles(t, db, 1, 1)[0]
	blob := mustPartitionTestBlob(t, map[int]float32{1: 1})
	if err := db.UpdateArticleEmbeddings(articleID, blob, nil); err != nil {
		t.Fatalf("UpdateArticleEmbeddings title-only error: %v", err)
	}

	var titleBlob, summaryBlob []byte
	if err := db.QueryRow(
		`SELECT title_embedding, summary_embedding FROM article_embeddings WHERE article_id = ?`,
		articleID,
	).Scan(&titleBlob, &summaryBlob); err != nil {
		t.Fatalf("query fallback blobs error: %v", err)
	}
	if len(titleBlob) == 0 || len(titleBlob) != len(summaryBlob) {
		t.Fatalf("fallback blob lengths differ: title=%d summary=%d", len(titleBlob), len(summaryBlob))
	}
}

func createRerankTestCluster(t *testing.T, db *DB, userID int64, summaryBlob []byte) int64 {
	t.Helper()
	clusterID, err := db.CreateCluster(userID, "complete")
	if err != nil {
		t.Fatalf("CreateCluster error: %v", err)
	}
	if _, err := db.Exec(`UPDATE clusters SET updated_at = datetime('now') WHERE id = ?`, clusterID); err != nil {
		t.Fatalf("touch cluster error: %v", err)
	}
	if err := db.UpdateClusterEmbeddings(clusterID, summaryBlob, summaryBlob); err != nil {
		t.Fatalf("UpdateClusterEmbeddings error: %v", err)
	}
	return clusterID
}

// TestMigrateVecTablesToLatest verifies the one-time rebuild of legacy vec0
// tables (both the pre-partition-key format and the partition-key-without-bin
// format): rows are preserved, backfilled with user_id, the binary column is
// derived via vec_quantize_binary, orphaned rows are dropped, and a second
// run is a no-op.
func TestMigrateVecTablesToLatest(t *testing.T) {
	db := setupPartitionTestDB(t)
	defer db.Close()

	articles := createPartitionTestUserArticles(t, db, 3, 2)
	blob := mustPartitionTestBlob(t, map[int]float32{0: 1})

	// Recreate the legacy (partition-less) format with data.
	dropAndCreateLegacyVecTables(t, db, false)
	insertLegacyArticleEmbedding(t, db, articles[0], blob)
	insertLegacyArticleEmbedding(t, db, articles[1], blob)
	// Orphan row: article 999999 does not exist and must be dropped by the JOIN.
	insertLegacyArticleEmbedding(t, db, 999999, blob)

	legacyClusterID, err := db.CreateCluster(3, "complete")
	if err != nil {
		t.Fatalf("CreateCluster error: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO cluster_embeddings (cluster_id, title_embedding, summary_embedding) VALUES (?, ?, ?)`,
		legacyClusterID, blob, blob,
	); err != nil {
		t.Fatalf("insert legacy cluster embedding: %v", err)
	}

	if err := migrateVecTablesToLatest(db.DB); err != nil {
		t.Fatalf("migrateVecTablesToLatest error: %v", err)
	}
	requireVecTargetSchema(t, db, "article_embeddings", "cluster_embeddings")

	var migratedCount, storedUserID int64
	if err := db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(user_id), 0) FROM article_embeddings`,
	).Scan(&migratedCount, &storedUserID); err != nil {
		t.Fatalf("count migrated article embeddings: %v", err)
	}
	if migratedCount != 2 {
		t.Fatalf("migrated article embedding rows = %d, want 2 (orphan dropped)", migratedCount)
	}
	if storedUserID != 6 { // two rows, both owned by user 3
		t.Fatalf("migrated user_id sum = %d, want 6", storedUserID)
	}

	// The migrated binary column must match vec_quantize_binary of the
	// migrated summary embedding.
	var migratedBin, expectedBin []byte
	if err := db.QueryRow(
		`SELECT summary_embedding_bin, vec_quantize_binary(summary_embedding)
		 FROM article_embeddings WHERE article_id = ?`, articles[0],
	).Scan(&migratedBin, &expectedBin); err != nil {
		t.Fatalf("query migrated binary column: %v", err)
	}
	if len(migratedBin) != 128 || string(migratedBin) != string(expectedBin) {
		t.Fatal("migrated binary column missing or mismatched with vec_quantize_binary")
	}

	var clusterCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM cluster_embeddings`).Scan(&clusterCount); err != nil {
		t.Fatalf("count migrated cluster embeddings: %v", err)
	}
	if clusterCount != 1 {
		t.Fatalf("migrated cluster embedding rows = %d, want 1", clusterCount)
	}

	// Migration tables must not linger.
	var migrCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE name LIKE '%_migr%'`,
	).Scan(&migrCount); err != nil {
		t.Fatalf("count leftover migration tables: %v", err)
	}
	if migrCount != 0 {
		t.Fatalf("leftover _migr tables = %d, want 0", migrCount)
	}

	// Idempotency: re-running must neither fail nor duplicate rows.
	if err := migrateVecTablesToLatest(db.DB); err != nil {
		t.Fatalf("migrateVecTablesToLatest second run error: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM article_embeddings`).Scan(&migratedCount); err != nil {
		t.Fatalf("count article embeddings after second run: %v", err)
	}
	if migratedCount != 2 {
		t.Fatalf("article embedding rows after second run = %d, want 2", migratedCount)
	}

	// The migrated partition key must actually constrain KNN: user 1 finds
	// nothing even though user 3 has an identical vector stored.
	candidates, err := db.FindSemanticCandidates(1, blob, 10)
	if err != nil {
		t.Fatalf("FindSemanticCandidates after migration error: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("user 1 candidates after migration = %d, want 0 (rows belong to user 3)", len(candidates))
	}
}

// TestMigrateVecTablesV1ToLatest verifies tables that already have the
// partition key but lack the binary column (the batch-1 schema) are also
// rebuilt to the target schema.
func TestMigrateVecTablesV1ToLatest(t *testing.T) {
	db := setupPartitionTestDB(t)
	defer db.Close()

	articles := createPartitionTestUserArticles(t, db, 1, 1)
	blob := mustPartitionTestBlob(t, map[int]float32{5: 1})

	dropAndCreateLegacyVecTables(t, db, true)
	insertLegacyArticleEmbedding(t, db, articles[0], blob)

	if err := migrateVecTablesToLatest(db.DB); err != nil {
		t.Fatalf("migrateVecTablesToLatest v1 error: %v", err)
	}
	requireVecTargetSchema(t, db, "article_embeddings", "cluster_embeddings")

	var binBlob []byte
	if err := db.QueryRow(
		`SELECT summary_embedding_bin FROM article_embeddings WHERE article_id = ?`,
		articles[0],
	).Scan(&binBlob); err != nil {
		t.Fatalf("query v1-migrated binary column: %v", err)
	}
	if len(binBlob) != 128 {
		t.Fatalf("v1-migrated binary blob length = %d, want 128", len(binBlob))
	}
}

// dropAndCreateLegacyVecTables recreates the embedding tables in a legacy
// format: withPartitionKey selects the batch-1 schema (partition key, no bin
// column) instead of the original schema (neither).
func dropAndCreateLegacyVecTables(t *testing.T, db *DB, withPartitionKey bool) {
	t.Helper()

	if _, err := db.Exec(`DROP TABLE article_embeddings`); err != nil {
		t.Fatalf("drop article_embeddings: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE cluster_embeddings`); err != nil {
		t.Fatalf("drop cluster_embeddings: %v", err)
	}

	partitionClause := ""
	if withPartitionKey {
		partitionClause = ",\n\t\tuser_id INTEGER partition key"
	}
	if _, err := db.Exec(fmt.Sprintf(`CREATE VIRTUAL TABLE article_embeddings USING vec0(
		article_id INTEGER PRIMARY KEY%s,
		title_embedding float[1024],
		summary_embedding float[1024]
	)`, partitionClause)); err != nil {
		t.Fatalf("create legacy article_embeddings: %v", err)
	}
	if _, err := db.Exec(fmt.Sprintf(`CREATE VIRTUAL TABLE cluster_embeddings USING vec0(
		cluster_id INTEGER PRIMARY KEY%s,
		title_embedding float[1024],
		summary_embedding float[1024]
	)`, partitionClause)); err != nil {
		t.Fatalf("create legacy cluster_embeddings: %v", err)
	}
}

func insertLegacyArticleEmbedding(t *testing.T, db *DB, articleID int64, blob []byte) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO article_embeddings (article_id, title_embedding, summary_embedding) VALUES (?, ?, ?)`,
		articleID, blob, blob,
	); err != nil {
		t.Fatalf("insert legacy article embedding %d: %v", articleID, err)
	}
}
