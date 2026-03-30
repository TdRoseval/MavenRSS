package sqlite_test

import (
	"bytes"
	"testing"

	"MavenRSS/internal/interest"
	"MavenRSS/internal/models"
)

func TestUpdateUserInterestVectorSyncsVecTable(t *testing.T) {
	db := setupTestDB(t)

	userID, err := db.CreateUser(&models.User{
		Username:     "interest-user",
		Email:        "interest@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	vectorBlob := mustSerializeVector(t, vectorWithLeadingValue(1))
	if err := db.UpdateUserInterestVector(userID, vectorBlob); err != nil {
		t.Fatalf("UpdateUserInterestVector() error = %v", err)
	}

	gotBlob, err := db.GetUserInterestVector(userID)
	if err != nil {
		t.Fatalf("GetUserInterestVector() error = %v", err)
	}
	if !bytes.Equal(gotBlob, vectorBlob) {
		t.Fatalf("GetUserInterestVector() = %v bytes, want stored blob", len(gotBlob))
	}

	var vecTableBlob []byte
	if err := db.QueryRow(
		`SELECT interest_embedding FROM user_interest_embeddings WHERE user_id = ?`,
		userID,
	).Scan(&vecTableBlob); err != nil {
		t.Fatalf("vec table scan error = %v", err)
	}
	if !bytes.Equal(vecTableBlob, vectorBlob) {
		t.Fatalf("vec table blob mismatch")
	}

	if err := db.UpdateUserInterestVector(userID, nil); err != nil {
		t.Fatalf("UpdateUserInterestVector(nil) error = %v", err)
	}

	var vecTableCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM user_interest_embeddings WHERE user_id = ?`,
		userID,
	).Scan(&vecTableCount); err != nil {
		t.Fatalf("vec table count scan error = %v", err)
	}
	if vecTableCount != 0 {
		t.Fatalf("vec table count = %d, want 0", vecTableCount)
	}
}

func TestGetClustersByVectorSimilarityAppliesSQLFilters(t *testing.T) {
	db := setupTestDB(t)

	userID, err := db.CreateUser(&models.User{
		Username:     "cluster-user",
		Email:        "cluster@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser(userID) error = %v", err)
	}

	otherUserID, err := db.CreateUser(&models.User{
		Username:     "other-user",
		Email:        "other@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser(otherUserID) error = %v", err)
	}

	queryBlob := mustSerializeVector(t, vectorWithLeadingValue(1))
	candidateBlob := mustSerializeVector(t, vectorWithLeadingValues(0.9, 0.1))
	oldBlob := mustSerializeVector(t, vectorWithLeadingValue(1))

	excludedClusterID := createClusterWithEmbedding(t, db, userID, "complete", queryBlob)
	validClusterID := createClusterWithEmbedding(t, db, userID, "complete", candidateBlob)
	oldClusterID := createClusterWithEmbedding(t, db, userID, "complete", oldBlob)
	otherUserClusterID := createClusterWithEmbedding(t, db, otherUserID, "complete", queryBlob)
	pendingClusterID := createClusterWithEmbedding(t, db, userID, "pending_merge", queryBlob)

	if _, err := db.Exec(
		`UPDATE clusters SET updated_at = datetime('now', '-5 days') WHERE id = ?`,
		oldClusterID,
	); err != nil {
		t.Fatalf("update old cluster time error = %v", err)
	}

	results, err := db.GetClustersByVectorSimilarity(userID, queryBlob, []int64{excludedClusterID}, 3, 100)
	if err != nil {
		t.Fatalf("GetClustersByVectorSimilarity() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Cluster.ID != validClusterID {
		t.Fatalf("results[0].Cluster.ID = %d, want %d", results[0].Cluster.ID, validClusterID)
	}

	for _, result := range results {
		if result.Cluster.ID == excludedClusterID {
			t.Fatalf("excluded cluster returned")
		}
		if result.Cluster.ID == oldClusterID {
			t.Fatalf("stale cluster returned")
		}
		if result.Cluster.ID == otherUserClusterID {
			t.Fatalf("other user cluster returned")
		}
		if result.Cluster.ID == pendingClusterID {
			t.Fatalf("pending cluster returned")
		}
	}
}

func createClusterWithEmbedding(t *testing.T, db interface {
	CreateCluster(userID int64, status string) (int64, error)
	UpdateClusterEmbeddings(clusterID int64, titleEmb, summaryEmb []byte) error
}, userID int64, status string, embedding []byte) int64 {
	t.Helper()

	clusterID, err := db.CreateCluster(userID, status)
	if err != nil {
		t.Fatalf("CreateCluster() error = %v", err)
	}
	if err := db.UpdateClusterEmbeddings(clusterID, embedding, embedding); err != nil {
		t.Fatalf("UpdateClusterEmbeddings() error = %v", err)
	}
	return clusterID
}

func mustSerializeVector(t *testing.T, vec []float32) []byte {
	t.Helper()

	blob, err := interest.SerializeVector(vec)
	if err != nil {
		t.Fatalf("SerializeVector() error = %v", err)
	}
	return blob
}

func vectorWithLeadingValue(first float32) []float32 {
	return vectorWithLeadingValues(first)
}

func vectorWithLeadingValues(values ...float32) []float32 {
	vec := make([]float32, 1024)
	copy(vec, values)
	return vec
}
