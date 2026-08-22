package dedup

import (
	"hash/fnv"
	"math/bits"
	"strings"
	"unicode/utf8"
)

// ComputeSimHash64 computes a 64-bit SimHash fingerprint for the given text.
// It uses bigram (2-char sliding window) tokenization which works well for
// both CJK and Latin text without external segmentation libraries.
func ComputeSimHash64(text string) int64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	// Generate bigram tokens
	tokens := bigramTokenize(text)
	if len(tokens) == 0 {
		return 0
	}

	// Weighted vector: each bit position accumulates +1 or -1
	var v [64]int

	// Reuse a single hasher: fnv.New64a allocates per call, and creating one per
	// bigram (thousands per article) produces avoidable GC pressure. Reset puts
	// it back to the identical initial state, so results are byte-for-byte equal.
	h := fnv.New64a()
	for _, token := range tokens {
		h.Reset()
		h.Write([]byte(token))
		hash := h.Sum64()

		for i := 0; i < 64; i++ {
			if (hash>>uint(i))&1 == 1 {
				v[i]++
			} else {
				v[i]--
			}
		}
	}

	// Convert accumulated vector to fingerprint
	var fingerprint uint64
	for i := 0; i < 64; i++ {
		if v[i] > 0 {
			fingerprint |= 1 << uint(i)
		}
	}

	return int64(fingerprint)
}

// SplitBands splits a 64-bit SimHash into 4 × 16-bit bands for pigeonhole lookup.
func SplitBands(hash int64) (b1, b2, b3, b4 int16) {
	h := uint64(hash)
	b1 = int16(h & 0xFFFF)
	b2 = int16((h >> 16) & 0xFFFF)
	b3 = int16((h >> 32) & 0xFFFF)
	b4 = int16((h >> 48) & 0xFFFF)
	return
}

// HammingDistance computes the Hamming distance between two 64-bit SimHash values.
// Returns the number of differing bit positions (0-64).
func HammingDistance(a, b int64) int {
	return bits.OnesCount64(uint64(a) ^ uint64(b))
}

// bigramTokenize produces bigram tokens from text using a sliding window of 2 runes.
// Whitespace is normalized but not removed, allowing cross-word bigrams.
func bigramTokenize(text string) []string {
	// Normalize: lowercase + collapse whitespace
	text = strings.ToLower(text)
	fields := strings.Fields(text)
	normalized := strings.Join(fields, " ")

	runes := []rune(normalized)
	if len(runes) < 2 {
		if len(runes) == 1 {
			return []string{string(runes)}
		}
		return nil
	}

	tokens := make([]string, 0, len(runes)-1)
	for i := 0; i < len(runes)-1; i++ {
		tokens = append(tokens, string(runes[i:i+2]))
	}
	return tokens
}

// IsValidForSimHash checks if text has enough content for meaningful SimHash computation.
func IsValidForSimHash(text string) bool {
	return utf8.RuneCountInString(strings.TrimSpace(text)) >= 10
}
