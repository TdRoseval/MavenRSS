package translation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"MrRSS/internal/config"
)

// AITranslator implements translation using OpenAI-compatible APIs (GPT, Claude, etc.).
type AITranslator struct {
	APIKey       string
	Endpoint     string
	Model        string
	SystemPrompt string
	client       *http.Client
	db           DBInterface
}

// NewAITranslator creates a new AI translator with the given credentials.
// endpoint should be the full API URL (e.g., "https://api.openai.com/v1/chat/completions" for OpenAI, "http://localhost:11434/api/generate" for Ollama)
// model should be the model name (e.g., "gpt-4o-mini", "claude-3-haiku-20240307")
// db is optional - if nil, no proxy will be used
func NewAITranslator(apiKey, endpoint, model string) *AITranslator {
	defaults := config.Get()
	// Default to OpenAI endpoint if not specified
	if endpoint == "" {
		endpoint = defaults.AIEndpoint
	}
	// Default to a cost-effective model if not specified
	if model == "" {
		model = defaults.AIModel
	}
	return &AITranslator{
		APIKey:       apiKey,
		Endpoint:     strings.TrimSuffix(endpoint, "/"),
		Model:        model,
		SystemPrompt: "", // Will be set from settings when used
		client:       &http.Client{Timeout: 30 * time.Second},
		db:           nil,
	}
}

// NewAITranslatorWithDB creates a new AI translator with database for proxy support
func NewAITranslatorWithDB(apiKey, endpoint, model string, db DBInterface) *AITranslator {
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
	return &AITranslator{
		APIKey:       apiKey,
		Endpoint:     strings.TrimSuffix(endpoint, "/"),
		Model:        model,
		SystemPrompt: "",
		client:       client,
		db:           db,
	}
}

// SetSystemPrompt sets a custom system prompt for the translator.
func (t *AITranslator) SetSystemPrompt(prompt string) {
	t.SystemPrompt = prompt
}

// Translate translates text to the target language using an OpenAI-compatible API.
// Automatically detects and adapts to different API formats (OpenAI vs Ollama).
func (t *AITranslator) Translate(text, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}

	langName := getLanguageName(targetLang)

	// Use custom system prompt if provided, otherwise use default
	systemPrompt := t.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a translator. Translate the given text accurately. Output ONLY the translated text, nothing else."
	}
	userPrompt := fmt.Sprintf("Translate to %s:\n%s", langName, text)

	// Try OpenAI format first
	result, err := t.tryOpenAIFormat(systemPrompt, userPrompt)
	if err == nil {
		return result, nil
	}

	// If OpenAI format fails, try Ollama format
	result, err = t.tryOllamaFormat(systemPrompt, userPrompt)
	if err == nil {
		return result, nil
	}

	// Both formats failed
	return "", fmt.Errorf("all API formats failed: OpenAI error: %v, Ollama error: %v", err, err)
}

// tryOpenAIFormat attempts to use OpenAI-compatible API format
func (t *AITranslator) tryOpenAIFormat(systemPrompt, userPrompt string) (string, error) {
	requestBody := map[string]interface{}{
		"model": t.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.1, // Low temperature for consistent translations
		"max_tokens":  256, // Limit output tokens for title translations
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OpenAI request: %w", err)
	}

	resp, err := t.sendRequest(jsonBody)
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
		// Clean up the response - remove any quotes or extra whitespace
		translated := strings.TrimSpace(result.Choices[0].Message.Content)
		translated = strings.Trim(translated, "\"'")
		return translated, nil
	}

	return "", fmt.Errorf("no translation found in OpenAI response")
}

// tryOllamaFormat attempts to use Ollama API format
func (t *AITranslator) tryOllamaFormat(systemPrompt, userPrompt string) (string, error) {
	// Combine system and user prompts for Ollama
	fullPrompt := systemPrompt + "\n\n" + userPrompt

	requestBody := map[string]interface{}{
		"model":  t.Model,
		"prompt": fullPrompt,
		"stream": false,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Ollama request: %w", err)
	}

	resp, err := t.sendRequest(jsonBody)
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
		// Clean up the response - remove any quotes or extra whitespace
		translated := strings.TrimSpace(result.Response)
		translated = strings.Trim(translated, "\"'")
		return translated, nil
	}

	return "", fmt.Errorf("no translation found in Ollama response")
}

// sendRequest sends the HTTP request with proper headers
func (t *AITranslator) sendRequest(jsonBody []byte) (*http.Response, error) {
	apiURL := t.Endpoint
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Only add Authorization header if API key is provided
	if t.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.APIKey)
	}

	return t.client.Do(req)
}

// getLanguageName converts a language code to a human-readable name.
func getLanguageName(code string) string {
	langNames := map[string]string{
		"en": "English",
		"zh": "Chinese",
		"es": "Spanish",
		"fr": "French",
		"de": "German",
		"ja": "Japanese",
		"ko": "Korean",
		"pt": "Portuguese",
		"ru": "Russian",
		"it": "Italian",
		"ar": "Arabic",
	}
	if name, ok := langNames[code]; ok {
		return name
	}
	return code
}
