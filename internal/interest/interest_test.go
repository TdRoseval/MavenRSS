package interest

import (
	"math"
	"testing"
)

func TestUpdateVector_ColdStart(t *testing.T) {
	feature := []float32{3.0, 4.0}
	result := UpdateVector(nil, feature, AlphaClick)

	// Should normalize: [3/5, 4/5] = [0.6, 0.8]
	if len(result) != 2 {
		t.Fatalf("expected length 2, got %d", len(result))
	}
	assertClose(t, result[0], 0.6, "result[0]")
	assertClose(t, result[1], 0.8, "result[1]")
}

func TestUpdateVector_EMA(t *testing.T) {
	old := []float32{1.0, 0.0}
	feature := []float32{0.0, 1.0}
	alpha := float32(0.5)

	result := UpdateVector(old, feature, alpha)

	// u_new = 0.5*[1,0] + 0.5*[0,1] = [0.5, 0.5]
	// norm = sqrt(0.5) ≈ 0.7071
	// normalized = [0.7071, 0.7071]
	if len(result) != 2 {
		t.Fatalf("expected length 2, got %d", len(result))
	}
	expected := float32(1.0 / math.Sqrt(2.0))
	assertClose(t, result[0], expected, "result[0]")
	assertClose(t, result[1], expected, "result[1]")
}

func TestUpdateVector_NormalizationPreserved(t *testing.T) {
	old := []float32{0.6, 0.8}
	feature := []float32{1.0, 0.0}
	result := UpdateVector(old, feature, 0.1)

	// Verify the result is normalized
	var norm float64
	for _, x := range result {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)
	assertClose64(t, norm, 1.0, "norm")
}

func TestUpdateVector_EmptyFeature(t *testing.T) {
	old := []float32{1.0, 0.0}
	result := UpdateVector(old, nil, AlphaClick)
	if len(result) != 2 || result[0] != 1.0 || result[1] != 0.0 {
		t.Errorf("expected old vector returned, got %v", result)
	}
}

func TestComputeAvgReadTime(t *testing.T) {
	tests := []struct {
		totalTime int64
		readCount int64
		expected  float64
	}{
		{0, 0, MinAvgReadTime},
		{100, 10, 10.0},
		{10, 10, MinAvgReadTime}, // 1.0 < 5.0 so floor at 5
		{300, 20, 15.0},
	}

	for _, tt := range tests {
		result := ComputeAvgReadTime(tt.totalTime, tt.readCount)
		if math.Abs(result-tt.expected) > 0.001 {
			t.Errorf("ComputeAvgReadTime(%d, %d) = %f, want %f",
				tt.totalTime, tt.readCount, result, tt.expected)
		}
	}
}

func TestInitFromFavorites_Empty(t *testing.T) {
	result, err := InitFromFavorites(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestInitFromFavorites_Single(t *testing.T) {
	vec := []float32{3.0, 4.0}
	blob, err := SerializeVector(vec)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	result, err := InitFromFavorites([][]byte{blob})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Average of single vector = same vector, normalized
	assertClose(t, result[0], 0.6, "result[0]")
	assertClose(t, result[1], 0.8, "result[1]")
}

func TestInitFromFavorites_Multiple(t *testing.T) {
	v1 := []float32{1.0, 0.0}
	v2 := []float32{0.0, 1.0}
	b1, _ := SerializeVector(v1)
	b2, _ := SerializeVector(v2)

	result, err := InitFromFavorites([][]byte{b1, b2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Average = [0.5, 0.5], normalized = [1/sqrt(2), 1/sqrt(2)]
	expected := float32(1.0 / math.Sqrt(2.0))
	assertClose(t, result[0], expected, "result[0]")
	assertClose(t, result[1], expected, "result[1]")
}

func TestSerializeDeserialize(t *testing.T) {
	original := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	blob, err := SerializeVector(original)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	restored, err := DeserializeVector(blob)
	if err != nil {
		t.Fatalf("deserialize: %v", err)
	}

	if len(restored) != len(original) {
		t.Fatalf("length mismatch: %d vs %d", len(restored), len(original))
	}

	expected := NormalizeVector(original)
	for i := range original {
		assertClose(t, restored[i], expected[i], "restored["+string(rune('0'+i))+"]")
	}
}

func TestNormalizeVectorZeroVectorStable(t *testing.T) {
	original := []float32{0, 0, 0}
	normalized := NormalizeVector(original)
	if len(normalized) != len(original) {
		t.Fatalf("len(normalized) = %d, want %d", len(normalized), len(original))
	}
	for i := range normalized {
		if normalized[i] != 0 {
			t.Fatalf("normalized[%d] = %f, want 0", i, normalized[i])
		}
	}
}

func TestSquaredL2Distance(t *testing.T) {
	distance, err := SquaredL2Distance([]float32{1, 0}, []float32{0, 1})
	if err != nil {
		t.Fatalf("SquaredL2Distance error: %v", err)
	}
	assertClose64(t, distance, 2, "distance")
}

func TestAverageAndNormalize(t *testing.T) {
	center, err := AverageAndNormalize([][]float32{
		{1, 0},
		{1, 0},
	})
	if err != nil {
		t.Fatalf("AverageAndNormalize error: %v", err)
	}
	if !IsNormalized(center, 1e-3) {
		t.Fatalf("center should be normalized, got %v", center)
	}
	assertClose(t, center[0], 1, "center[0]")
	assertClose(t, center[1], 0, "center[1]")
}

func TestIsNormalized(t *testing.T) {
	if !IsNormalized([]float32{1, 0}, 1e-3) {
		t.Fatal("unit vector should be normalized")
	}
	if IsNormalized([]float32{2, 0}, 1e-3) {
		t.Fatal("non-unit vector should not be normalized")
	}
}

func assertClose(t *testing.T, got, want float32, name string) {
	t.Helper()
	if diff := math.Abs(float64(got - want)); diff > 0.001 {
		t.Errorf("%s = %f, want %f (diff=%f)", name, got, want, diff)
	}
}

func assertClose64(t *testing.T, got, want float64, name string) {
	t.Helper()
	if diff := math.Abs(got - want); diff > 0.001 {
		t.Errorf("%s = %f, want %f (diff=%f)", name, got, want, diff)
	}
}
