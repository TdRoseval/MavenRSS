// Package interest provides vector operations for the AI Enhanced Mode's
// 3-tier interest tracking system. It handles EMA (Exponential Moving Average)
// vector updates, L2 normalization, dynamic reading time baseline, and
// cold-start initialization from favorite cluster embeddings.
package interest

import (
	"fmt"
	"math"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

// Learning rate constants for the 3-tier system
const (
	AlphaClick    float32 = 0.05 // Level 1: shallow curiosity (click)
	AlphaDeepRead float32 = 0.10 // Level 2: deep reading (time > avg)
	AlphaBookmark float32 = 0.20 // Level 3: core endorsement (favorite)
)

// MinAvgReadTime is the minimum average reading time (seconds) to prevent
// cold-start issues and initial data fluctuation.
const MinAvgReadTime = 5.0

// UpdateVector applies an EMA update and L2-normalizes the result:
//
//	u_new = (1 - α) * u_old + α * v
//	u_final = u_new / ||u_new||
//
// old is the current interest vector, feature is the incoming feature vector,
// and alpha is the learning rate. Returns the normalized updated vector.
// If old is nil/empty, the feature vector (normalized) is used directly.
func UpdateVector(old, feature []float32, alpha float32) []float32 {
	if len(feature) == 0 {
		return old
	}

	dim := len(feature)

	if len(old) == 0 {
		// Cold start: just normalize the feature
		return normalize(feature)
	}

	if len(old) != dim {
		// Dimension mismatch — use the feature vector as-is
		return normalize(feature)
	}

	result := make([]float32, dim)
	for i := 0; i < dim; i++ {
		result[i] = (1-alpha)*old[i] + alpha*feature[i]
	}

	return normalize(result)
}

// ComputeAvgReadTime calculates the dynamic average reading time baseline:
//
//	T_avg = max(totalTime / count, MinAvgReadTime)
func ComputeAvgReadTime(totalTime, readCount int64) float64 {
	if readCount <= 0 {
		return MinAvgReadTime
	}
	avg := float64(totalTime) / float64(readCount)
	return math.Max(avg, MinAvgReadTime)
}

// InitFromFavorites averages multiple summary embedding blobs into a single
// normalized interest vector. Used for cold-start when the user has favorited
// clusters but no interest vector yet.
// Returns nil if no valid embeddings are provided.
func InitFromFavorites(embeddingBlobs [][]byte) ([]float32, error) {
	if len(embeddingBlobs) == 0 {
		return nil, nil
	}

	var vectors [][]float32
	for _, blob := range embeddingBlobs {
		vec, err := DeserializeVector(blob)
		if err != nil {
			continue
		}
		if len(vec) > 0 {
			vectors = append(vectors, vec)
		}
	}

	if len(vectors) == 0 {
		return nil, nil
	}

	dim := len(vectors[0])
	avg := make([]float32, dim)
	count := float32(len(vectors))

	for _, v := range vectors {
		if len(v) != dim {
			continue // skip mismatched dimensions
		}
		for i := 0; i < dim; i++ {
			avg[i] += v[i]
		}
	}

	for i := 0; i < dim; i++ {
		avg[i] /= count
	}

	return normalize(avg), nil
}

// SerializeVector converts a float32 slice to a byte blob for SQLite storage.
func SerializeVector(vec []float32) ([]byte, error) {
	if len(vec) == 0 {
		return nil, fmt.Errorf("empty vector")
	}
	return sqlite_vec.SerializeFloat32(vec)
}

// DeserializeVector converts a byte blob back to a float32 slice.
func DeserializeVector(blob []byte) ([]float32, error) {
	if len(blob) == 0 {
		return nil, fmt.Errorf("empty blob")
	}
	// Each float32 is 4 bytes
	dim := len(blob) / 4
	if dim == 0 {
		return nil, fmt.Errorf("blob too small")
	}
	vec := make([]float32, dim)
	for i := 0; i < dim; i++ {
		bits := uint32(blob[i*4]) | uint32(blob[i*4+1])<<8 | uint32(blob[i*4+2])<<16 | uint32(blob[i*4+3])<<24
		vec[i] = math.Float32frombits(bits)
	}
	return vec, nil
}

// normalize performs L2 normalization on a float32 vector.
// Returns the zero vector if the norm is 0.
func normalize(v []float32) []float32 {
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)

	if norm == 0 {
		return v
	}

	result := make([]float32, len(v))
	for i, x := range v {
		result[i] = float32(float64(x) / norm)
	}
	return result
}
