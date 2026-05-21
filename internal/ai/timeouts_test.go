package ai

import (
	"testing"
	"time"
)

func TestEffectiveTimeoutFromSeconds(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want time.Duration
	}{
		{name: "unset", in: 0, want: MinimumConfigurableTimeout},
		{name: "negative", in: -1, want: MinimumConfigurableTimeout},
		{name: "below minimum", in: 90, want: MinimumConfigurableTimeout},
		{name: "above minimum", in: 600, want: 10 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveTimeoutFromSeconds(tt.in); got != tt.want {
				t.Fatalf("EffectiveTimeoutFromSeconds(%d) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestMaxEmbeddingTimeoutFromConfigJSON(t *testing.T) {
	if got := MaxEmbeddingTimeoutFromConfigJSON(`[{"modelname":"a","baseurl":"https://example.com"}]`); got != MinimumConfigurableTimeout {
		t.Fatalf("default embedding timeout = %v, want %v", got, MinimumConfigurableTimeout)
	}

	configs := `[{"modelname":"a","baseurl":"https://a.example.com","timeout_seconds":120},{"modelname":"b","baseurl":"https://b.example.com","timeout_seconds":600}]`
	if got := MaxEmbeddingTimeoutFromConfigJSON(configs); got != 10*time.Minute {
		t.Fatalf("max embedding timeout = %v, want 10m", got)
	}
}
