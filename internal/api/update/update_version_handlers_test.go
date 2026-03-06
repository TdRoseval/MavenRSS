package update

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		// Equal versions
		{"equal versions", "1.0.0", "1.0.0", 0},
		{"equal versions with different lengths", "1.0", "1.0.0", 0},

		// v1 > v2
		{"major version greater", "2.0.0", "1.0.0", 1},
		{"minor version greater", "1.2.0", "1.1.0", 1},
		{"patch version greater", "1.0.2", "1.0.1", 1},
		{"longer version greater", "1.0.0.1", "1.0.0", 1},

		// v1 < v2
		{"major version less", "1.0.0", "2.0.0", -1},
		{"minor version less", "1.1.0", "1.2.0", -1},
		{"patch version less", "1.0.1", "1.0.2", -1},
		{"shorter version less", "1.0.0", "1.0.0.1", -1},

		// Complex comparisons
		{"complex greater", "1.10.5", "1.9.10", 1},
		{"complex less", "1.9.10", "1.10.5", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.v1, tt.v2)
			if result != tt.expected {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

func TestCompareVersions_StableVsPrerelease(t *testing.T) {
	// Note: The compareVersions function only compares numeric parts
	// Pre-release filtering is done at the API level before version comparison
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		// When comparing versions with pre-release tags removed
		{"stable versions same base", "1.2.0", "1.2.0", 0},
		{"stable newer than older stable", "1.2.1", "1.2.0", 1},
		{"stable older than newer stable", "1.2.0", "1.2.1", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.v1, tt.v2)
			if result != tt.expected {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

func TestCompareVersions_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{"empty parts in v1", "1..0", "1.0.0", 0},
		{"empty parts in v2", "1.0.0", "1..0", 0},
		{"single number", "5", "3", 1},
		{"single vs multi", "2", "1.9", 1},
		{"zeros", "0.0.0", "0.0.0", 0},
		{"zero vs non-zero", "0.0.1", "0.0.0", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.v1, tt.v2)
			if result != tt.expected {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}
