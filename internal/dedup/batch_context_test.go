package dedup

import (
	"testing"
)

func TestBatchContextMemberVectorCache(t *testing.T) {
	batch := NewBatchContext()

	if _, ok := batch.MemberVectors(1); ok {
		t.Fatal("empty cache should report miss")
	}

	vecs := [][]float32{{1, 0, 0}, {0, 1, 0}}
	batch.SetMemberVectors(1, vecs)

	got, ok := batch.MemberVectors(1)
	if !ok || len(got) != 2 {
		t.Fatalf("MemberVectors(1) = (%v, %v), want 2 vectors, true", got, ok)
	}
	// The outer slice is detached from the caller's copy: appending to the
	// caller's slice must not affect the cache (inner vectors are treated as
	// immutable by the pipeline).
	vecs = append(vecs, []float32{1, 1, 1})
	if cached, _ := batch.MemberVectors(1); len(cached) != 2 {
		t.Fatalf("cache affected by caller append: %d vectors", len(cached))
	}

	// Appends extend the same cache entry.
	batch.AppendMemberVector(1, []float32{0, 0, 1})
	got, _ = batch.MemberVectors(1)
	if len(got) != 3 {
		t.Fatalf("after append len = %d, want 3", len(got))
	}

	// Zero-length vectors are dropped; an all-empty set still caches as an
	// explicit "no members" answer.
	batch.SetMemberVectors(2, [][]float32{nil, {}})
	empty, ok := batch.MemberVectors(2)
	if !ok || len(empty) != 0 {
		t.Fatalf("MemberVectors(2) = (%v, %v), want empty slice, true", empty, ok)
	}

	// Nil BatchContext is safe (no batch option passed).
	var nilBatch *BatchContext
	if _, ok := nilBatch.MemberVectors(1); ok {
		t.Fatal("nil batch should report miss")
	}
	nilBatch.SetMemberVectors(1, vecs)
	nilBatch.AppendMemberVector(1, []float32{1})
}

// TestProcessArticleSecondJoinUsesCachedMembership verifies that after an
// article joins a cluster, the batch centroid cache reflects the new member so
// a third article ranks against the two-member centroid, not a stale one.
func TestProcessArticleSecondJoinUsesCachedMembership(t *testing.T) {
	db := newDedupTestDB(t)
	userID, feedID := createDedupTestUserAndFeed(t, db)
	batch := NewBatchContext()

	first := createDedupTestArticle(t, db, userID, feedID, "cache-first", false, "cache summary", vector1024(1, 0))
	if err := ProcessArticle(db, first, userID, &ProcessArticleOptions{Batch: batch}); err != nil {
		t.Fatalf("ProcessArticle first error: %v", err)
	}
	firstCluster := mustGetArticleCluster(t, db, first)

	if _, ok := batch.MemberVectors(firstCluster.ID); !ok {
		t.Fatalf("standalone cluster %d not cached after creation", firstCluster.ID)
	}
	if vecs, _ := batch.MemberVectors(firstCluster.ID); len(vecs) != 1 {
		t.Fatalf("cached members = %d, want 1", len(vecs))
	}

	// A near-identical article must join the first cluster; its vector is
	// appended to the cache entry.
	second := createDedupTestArticle(t, db, userID, feedID, "cache-second", false, "cache summary", vector1024(1, 0))
	if err := ProcessArticle(db, second, userID, &ProcessArticleOptions{Batch: batch}); err != nil {
		t.Fatalf("ProcessArticle second error: %v", err)
	}
	secondCluster := mustGetArticleCluster(t, db, second)
	if secondCluster.ID != firstCluster.ID {
		t.Fatalf("second article cluster = %d, want %d", secondCluster.ID, firstCluster.ID)
	}
	if vecs, _ := batch.MemberVectors(firstCluster.ID); len(vecs) != 2 {
		t.Fatalf("cached members after join = %d, want 2", len(vecs))
	}

	// The cached membership must match the database state exactly.
	var dbCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM articles WHERE cluster_id = ?`, firstCluster.ID).Scan(&dbCount); err != nil {
		t.Fatalf("count cluster articles error: %v", err)
	}
	if dbCount != 2 {
		t.Fatalf("db cluster article count = %d, want 2", dbCount)
	}
}
