package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"MavenRSS/internal/models"
	"MavenRSS/internal/utils/httputil"
)

type EmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type EmbeddingResponse struct {
	Object string `json:"object"`
	Model  string `json:"model"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

var (
	modelUsageMu    sync.Mutex
	modelUsageState = make(map[string]*usageState)
)

type usageState struct {
	requests    int
	tokens      int
	windowStart time.Time
}

func checkRateLimit(config models.EmbeddingModelConfig, tokens int) bool {
	if config.RPM <= 0 && config.TPM <= 0 {
		return true
	}

	modelUsageMu.Lock()
	defer modelUsageMu.Unlock()

	key := config.ModelName + "_" + config.BaseURL
	state, exists := modelUsageState[key]
	now := time.Now()

	if !exists || now.Sub(state.windowStart) >= time.Minute {
		state = &usageState{
			requests:    0,
			tokens:      0,
			windowStart: now,
		}
		modelUsageState[key] = state
	}

	if config.RPM > 0 && state.requests >= config.RPM {
		return false
	}
	if config.TPM > 0 && state.tokens+tokens > config.TPM {
		return false
	}

	return true
}

func recordUsage(config models.EmbeddingModelConfig, tokens int) {
	if config.RPM <= 0 && config.TPM <= 0 {
		return
	}
	modelUsageMu.Lock()
	defer modelUsageMu.Unlock()

	key := config.ModelName + "_" + config.BaseURL
	if state, exists := modelUsageState[key]; exists {
		state.requests++
		state.tokens += tokens
	}
}

// GenerateEmbeddings tries embedding models in order to generate an embedding for the input.
// globalProxyURL is applied when a model has UseGlobalProxy set to true.
func GenerateEmbeddings(ctx context.Context, input string, configsJSON string, globalProxyURL string) ([]float32, error) {
	if input == "" {
		return nil, fmt.Errorf("empty input")
	}

	var configs []models.EmbeddingModelConfig
	if err := json.Unmarshal([]byte(configsJSON), &configs); err != nil {
		return nil, fmt.Errorf("failed to parse embedding configurations: %w", err)
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("no embedding models configured")
	}

	// Rough estimation of tokens
	estimatedTokens := len(input) / 2

	var lastErr error
	for _, config := range configs {
		if !checkRateLimit(config, estimatedTokens) {
			lastErr = fmt.Errorf("rate limit reached for model %s", config.ModelName)
			continue
		}

		// Determine proxy URL for this model
		proxyURL := ""
		if config.UseGlobalProxy && globalProxyURL != "" {
			proxyURL = globalProxyURL
		}

		embedding, actualTokens, err := requestEmbedding(ctx, config, input, proxyURL)
		if err != nil {
			lastErr = err
			continue
		}

		recordUsage(config, actualTokens)
		return embedding, nil
	}

	return nil, fmt.Errorf("all embedding models failed. last error: %v", lastErr)
}

func MaxEmbeddingTimeoutFromConfigJSON(configsJSON string) time.Duration {
	var configs []models.EmbeddingModelConfig
	if err := json.Unmarshal([]byte(configsJSON), &configs); err != nil {
		return MinimumConfigurableTimeout
	}
	maxTimeout := MinimumConfigurableTimeout
	for _, config := range configs {
		timeout := EffectiveTimeoutFromSeconds(config.TimeoutSeconds)
		if timeout > maxTimeout {
			maxTimeout = timeout
		}
	}
	return maxTimeout
}

func requestEmbedding(ctx context.Context, config models.EmbeddingModelConfig, input string, proxyURL string) ([]float32, int, error) {
	reqBody := EmbeddingRequest{
		Model: config.ModelName,
		Input: input,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, err
	}

	endpoint := config.BaseURL
	if !strings.HasSuffix(endpoint, "/embeddings") {
		if strings.HasSuffix(endpoint, "/") {
			endpoint += "embeddings"
		} else {
			endpoint += "/embeddings"
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	if config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}

	client := httputil.GetPooledAIHTTPClient(proxyURL, EffectiveTimeoutFromSeconds(config.TimeoutSeconds))

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var res EmbeddingResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, 0, err
	}

	if len(res.Data) == 0 {
		return nil, 0, fmt.Errorf("empty embedding data returned")
	}

	return res.Data[0].Embedding, res.Usage.TotalTokens, nil
}
