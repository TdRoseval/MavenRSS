package dedup

import (
	"testing"
)

func TestComputeSimHash64_BasicText(t *testing.T) {
	hash := ComputeSimHash64("Hello world, this is a test article about technology")
	if hash == 0 {
		t.Fatal("expected non-zero SimHash for non-empty text")
	}
}

func TestComputeSimHash64_Empty(t *testing.T) {
	hash := ComputeSimHash64("")
	if hash != 0 {
		t.Fatalf("expected 0 for empty text, got %d", hash)
	}
}

func TestComputeSimHash64_SimilarTexts(t *testing.T) {
	text1 := "苹果公司今天发布了最新的iPhone手机，搭载了全新的A20芯片"
	text2 := "苹果公司今天发布了全新的iPhone手机，搭载了最新的A20处理器"

	h1 := ComputeSimHash64(text1)
	h2 := ComputeSimHash64(text2)

	dist := HammingDistance(h1, h2)
	if dist > 10 {
		t.Fatalf("similar texts should have small Hamming distance, got %d", dist)
	}
	t.Logf("Similar texts Hamming distance: %d", dist)
}

func TestComputeSimHash64_DifferentTexts(t *testing.T) {
	text1 := "苹果公司今天发布了最新的iPhone手机"
	text2 := "特斯拉宣布将在中国建设新的超级工厂"

	h1 := ComputeSimHash64(text1)
	h2 := ComputeSimHash64(text2)

	dist := HammingDistance(h1, h2)
	if dist < 10 {
		t.Fatalf("very different texts should have large Hamming distance, got %d", dist)
	}
	t.Logf("Different texts Hamming distance: %d", dist)
}

func TestSplitBands(t *testing.T) {
	hash := int64(0x1234567890ABCDEF)
	b1, b2, b3, b4 := SplitBands(hash)

	// Reconstruct and verify
	reconstructed := int64(uint16(b1)) | int64(uint16(b2))<<16 | int64(uint16(b3))<<32 | int64(uint16(b4))<<48
	if reconstructed != hash {
		t.Fatalf("band split/reconstruct mismatch: %x != %x", reconstructed, hash)
	}
}

func TestHammingDistance(t *testing.T) {
	tests := []struct {
		a, b int64
		want int
	}{
		{0, 0, 0},
		{0, 1, 1},
		{0, -1, 64}, // all bits differ
		{0xFF, 0xFE, 1},
	}
	for _, tt := range tests {
		got := HammingDistance(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("HammingDistance(%x, %x) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestIsValidForSimHash(t *testing.T) {
	if IsValidForSimHash("short") {
		t.Error("expected false for short text")
	}
	if !IsValidForSimHash("这是一段足够长的中文文本用于测试") {
		t.Error("expected true for long enough text")
	}
}

func TestComputeSimHash64Deterministic(t *testing.T) {
	text := "这是用于验证 SimHash 在复用 hasher 后仍保持确定性的测试文本，包含 mixed English content 与数字 12345。"

	h1 := ComputeSimHash64(text)
	h2 := ComputeSimHash64(text)
	if h1 != h2 {
		t.Fatalf("same input produced different hashes: %d vs %d", h1, h2)
	}
}
