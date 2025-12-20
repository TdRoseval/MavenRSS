package summary

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"MrRSS/internal/config"
	"MrRSS/internal/utils"
)

// AISummarizer implements summarization using OpenAI-compatible APIs (GPT, Claude, etc.).
type AISummarizer struct {
	APIKey       string
	Endpoint     string
	Model        string
	SystemPrompt string
	client       *http.Client
	db           DBInterface
}

// DBInterface defines the minimal database interface needed for proxy settings
type DBInterface interface {
	GetSetting(key string) (string, error)
	GetEncryptedSetting(key string) (string, error)
}

// CreateHTTPClientWithProxy creates an HTTP client with global proxy settings if enabled
func CreateHTTPClientWithProxy(db DBInterface, timeout time.Duration) (*http.Client, error) {
	var proxyURL string

	// Check if global proxy is enabled
	proxyEnabled, _ := db.GetSetting("proxy_enabled")
	if proxyEnabled == "true" {
		// Build proxy URL from global settings
		proxyType, _ := db.GetSetting("proxy_type")
		proxyHost, _ := db.GetSetting("proxy_host")
		proxyPort, _ := db.GetSetting("proxy_port")
		proxyUsername, _ := db.GetEncryptedSetting("proxy_username")
		proxyPassword, _ := db.GetEncryptedSetting("proxy_password")
		proxyURL = utils.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
	}

	// Create HTTP client with or without proxy
	return utils.CreateHTTPClient(proxyURL, timeout)
}

// NewAISummarizer creates a new AI summarizer with the given credentials.
// endpoint should be the full API URL (e.g., "https://api.openai.com/v1/chat/completions" for OpenAI, "http://localhost:11434/api/generate" for Ollama)
// model should be the model name (e.g., "gpt-4o-mini", "claude-3-haiku-20240307")
// Uses global AI settings shared between translation and summarization.
// db is optional - if nil, no proxy will be used
func NewAISummarizer(apiKey, endpoint, model string) *AISummarizer {
	defaults := config.Get()
	// Use global AI endpoint and model
	if endpoint == "" {
		endpoint = defaults.AIEndpoint
	}
	if model == "" {
		model = defaults.AIModel
	}
	return &AISummarizer{
		APIKey:       apiKey,
		Endpoint:     strings.TrimSuffix(endpoint, "/"),
		Model:        model,
		SystemPrompt: "", // Will be set from settings when used
		client:       &http.Client{Timeout: 30 * time.Second},
		db:           nil,
	}
}

// NewAISummarizerWithDB creates a new AI summarizer with database for proxy support
func NewAISummarizerWithDB(apiKey, endpoint, model string, db DBInterface) *AISummarizer {
	defaults := config.Get()
	if endpoint == "" {
		endpoint = defaults.AIEndpoint
	}
	if model == "" {
		model = defaults.AIModel
	}
	client, err := CreateHTTPClientWithProxy(db, 30*time.Second)
	if err != nil {
		// Fallback to default client if proxy creation fails
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &AISummarizer{
		APIKey:       apiKey,
		Endpoint:     strings.TrimSuffix(endpoint, "/"),
		Model:        model,
		SystemPrompt: "",
		client:       client,
		db:           db,
	}
}

// SetSystemPrompt sets a custom system prompt for the summarizer.
func (s *AISummarizer) SetSystemPrompt(prompt string) {
	s.SystemPrompt = prompt
}

// Summarize generates a summary of the given text using an OpenAI-compatible API.
// Automatically detects and adapts to different API formats (OpenAI vs Ollama).
func (s *AISummarizer) Summarize(text string, length SummaryLength) (SummaryResult, error) {
	// Clean the text first
	cleanedText := cleanText(text)

	// Check if text is too short
	if len(cleanedText) < MinContentLength {
		return SummaryResult{
			Summary:    cleanedText,
			IsTooShort: true,
		}, nil
	}

	// Truncate text if too long to save tokens
	// Use rune slicing to avoid breaking multi-byte UTF-8 characters (e.g., Chinese, emoji)
	runes := []rune(cleanedText)
	if len(runes) > MaxInputCharsForAI {
		cleanedText = string(runes[:MaxInputCharsForAI])
	}

	targetWords := getTargetWordCount(length)

	// Use custom system prompt if provided, otherwise use default
	systemPrompt := s.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a summarizer. Generate a concise summary of the given text. Output ONLY the summary, nothing else."
	}
	userPrompt := fmt.Sprintf("Summarize the following text in approximately %d words:\n\n%s", targetWords, cleanedText)

	// Try OpenAI format first
	result, err := s.tryOpenAIFormat(systemPrompt, userPrompt)
	if err == nil {
		// Count sentences in the summary
		sentences := splitSentences(result)
		return SummaryResult{
			Summary:       result,
			SentenceCount: len(sentences),
			IsTooShort:    false,
		}, nil
	}

	// If OpenAI format fails, try Ollama format
	result, err = s.tryOllamaFormat(systemPrompt, userPrompt)
	if err == nil {
		// Count sentences in the summary
		sentences := splitSentences(result)
		return SummaryResult{
			Summary:       result,
			SentenceCount: len(sentences),
			IsTooShort:    false,
		}, nil
	}

	// Both formats failed
	return SummaryResult{}, fmt.Errorf("all API formats failed: OpenAI error: %v, Ollama error: %v", err, err)
}

// tryOpenAIFormat attempts to use OpenAI-compatible API format
func (s *AISummarizer) tryOpenAIFormat(systemPrompt, userPrompt string) (string, error) {
	requestBody := map[string]interface{}{
		"model": s.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.3, // Low temperature for consistent summaries
		"max_tokens":  512, // Limit output tokens for summaries
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OpenAI request: %w", err)
	}

	resp, err := s.sendRequest(jsonBody)
	if err != nil {
		return "", fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API returned status: %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode OpenAI response: %w", err)
	}

	if len(result.Choices) > 0 && result.Choices[0].Message.Content != "" {
		// Clean up the response
		summary := strings.TrimSpace(result.Choices[0].Message.Content)
		return summary, nil
	}

	return "", fmt.Errorf("no summary found in OpenAI response")
}

// tryOllamaFormat attempts to use Ollama API format
func (s *AISummarizer) tryOllamaFormat(systemPrompt, userPrompt string) (string, error) {
	// Combine system and user prompts for Ollama
	fullPrompt := systemPrompt + "\n\n" + userPrompt

	requestBody := map[string]interface{}{
		"model":  s.Model,
		"prompt": fullPrompt,
		"stream": false,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Ollama request: %w", err)
	}

	resp, err := s.sendRequest(jsonBody)
	if err != nil {
		return "", fmt.Errorf("Ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama API returned status: %d", resp.StatusCode)
	}

	var result struct {
		Response string `json:"response"`
		Done     bool   `json:"done"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	if result.Done && result.Response != "" {
		// Clean up the response
		summary := strings.TrimSpace(result.Response)
		return summary, nil
	}

	return "", fmt.Errorf("no summary found in Ollama response")
}

// sendRequest sends the HTTP request with proper headers
func (s *AISummarizer) sendRequest(jsonBody []byte) (*http.Response, error) {
	apiURL := s.Endpoint

	// Validate endpoint URL to prevent SSRF attacks
	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid API endpoint URL: %w", err)
	}

	// Allow HTTP for local endpoints (localhost, 127.0.0.1, etc.) for local LLM support (e.g., Ollama)
	if parsedURL.Scheme != "https" && !isLocalEndpoint(parsedURL.Host) {
		return nil, fmt.Errorf("API endpoint must use HTTPS for security (HTTP allowed only for localhost)")
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Only add Authorization header if API key is provided
	if s.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.APIKey)
	}

	return s.client.Do(req)
}

// isLocalEndpoint checks if a host is a local endpoint (localhost, 127.0.0.1, etc.)
// This allows using HTTP for local LLM services like Ollama
func isLocalEndpoint(host string) bool {
	// Remove port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		// Handle IPv6 addresses like [::1]:8080
		if !strings.Contains(host[idx:], "]") {
			host = host[:idx]
		}
	}
	// Remove brackets from IPv6 addresses
	host = strings.Trim(host, "[]")

	return host == "localhost" ||
		host == "127.0.0.1" ||
		host == "::1" ||
		strings.HasPrefix(host, "127.") ||
		host == "0.0.0.0"
}
