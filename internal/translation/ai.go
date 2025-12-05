package translation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AITranslator implements translation using OpenAI-compatible APIs (GPT, Claude, etc.).
type AITranslator struct {
	APIKey   string
	Endpoint string
	Model    string
	client   *http.Client
}

// NewAITranslator creates a new AI translator with the given credentials.
// endpoint should be the API base URL (e.g., "https://api.openai.com/v1" for OpenAI)
// model should be the model name (e.g., "gpt-4o-mini", "claude-3-haiku-20240307")
func NewAITranslator(apiKey, endpoint, model string) *AITranslator {
	// Default to OpenAI endpoint if not specified
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	// Default to a cost-effective model if not specified
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &AITranslator{
		APIKey:   apiKey,
		Endpoint: strings.TrimSuffix(endpoint, "/"),
		Model:    model,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

// Translate translates text to the target language using an OpenAI-compatible API.
func (t *AITranslator) Translate(text, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}

	langName := getLanguageName(targetLang)

	// Concise prompt to minimize token usage
	systemPrompt := "You are a translator. Translate the given text accurately. Output ONLY the translated text, nothing else."
	userPrompt := fmt.Sprintf("Translate to %s:\n%s", langName, text)

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
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := t.Endpoint + "/chat/completions"
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.APIKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ai api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errorResp)
		if errorResp.Error.Message != "" {
			return "", fmt.Errorf("ai api error: %s", errorResp.Error.Message)
		}
		return "", fmt.Errorf("ai api returned status: %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode ai response: %w", err)
	}

	if len(result.Choices) > 0 && result.Choices[0].Message.Content != "" {
		// Clean up the response - remove any quotes or extra whitespace
		translated := strings.TrimSpace(result.Choices[0].Message.Content)
		translated = strings.Trim(translated, "\"'")
		return translated, nil
	}

	return "", fmt.Errorf("no translation found in ai response")
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
