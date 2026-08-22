package ai

import (
	"testing"
	"time"
)

func TestParseEmbeddingConfigs(t *testing.T) {
	configsJSON := `[
		{"modelname":"embed-a","baseurl":"https://a.example.com","apikey":"k","timeout_seconds":600},
		{"modelname":"embed-b","baseurl":"https://b.example.com","apikey":"k","timeout_seconds":360}
	]`

	configs, maxTimeout, err := parseEmbeddingConfigs(configsJSON)
	if err != nil {
		t.Fatalf("parseEmbeddingConfigs error: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}
	if configs[0].ModelName != "embed-a" || configs[1].ModelName != "embed-b" {
		t.Fatalf("unexpected configs: %+v", configs)
	}
	if maxTimeout != 600*time.Second {
		t.Fatalf("maxTimeout = %v, want 10m", maxTimeout)
	}

	// A cached re-parse must return equivalent results.
	configs2, maxTimeout2, err := parseEmbeddingConfigs(configsJSON)
	if err != nil {
		t.Fatalf("cached parse error: %v", err)
	}
	if len(configs2) != 2 || configs2[0].ModelName != "embed-a" {
		t.Fatalf("cached parse mismatch: %+v", configs2)
	}
	if maxTimeout2 != maxTimeout {
		t.Fatalf("cached maxTimeout = %v, want %v", maxTimeout2, maxTimeout)
	}
}

func TestParseEmbeddingConfigsInvalidJSON(t *testing.T) {
	if _, _, err := parseEmbeddingConfigs("not-json"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}