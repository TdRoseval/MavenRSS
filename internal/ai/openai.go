// Package ai provides OpenAI-compatible API format handlers
package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// OpenAIHandler implements FormatHandler for OpenAI-compatible APIs
type OpenAIHandler struct{}

// NewOpenAIHandler creates a new OpenAI format handler
func NewOpenAIHandler() *OpenAIHandler {
	return &OpenAIHandler{}
}

// BuildStreamRequest builds an OpenAI-compatible API request for streaming
func (h *OpenAIHandler) BuildStreamRequest(config RequestConfig) (map[string]interface{}, error) {
	request, err := h.BuildRequest(config)
	if err != nil {
		return nil, err
	}
	request["stream"] = true
	return request, nil
}

// BuildRequest builds an OpenAI-compatible API request
func (h *OpenAIHandler) BuildRequest(config RequestConfig) (map[string]interface{}, error) {
	request := map[string]interface{}{
		"model": config.Model,
	}

	// Determine messages format
	if len(config.Messages) > 0 {
		// Use provided messages
		request["messages"] = config.Messages
	} else {
		// Build messages from system and user prompts
		messages := []map[string]string{}
		if config.SystemPrompt != "" {
			messages = append(messages, map[string]string{
				"role":    "system",
				"content": config.SystemPrompt,
			})
		}
		if config.UserPrompt != "" {
			messages = append(messages, map[string]string{
				"role":    "user",
				"content": config.UserPrompt,
			})
		}
		request["messages"] = messages
	}

	// Add optional parameters
	if config.Temperature > 0 {
		request["temperature"] = config.Temperature
	}

	// Use max_completion_tokens if provided (new parameter)
	if config.MaxCompletionTokens > 0 {
		request["max_completion_tokens"] = config.MaxCompletionTokens
	} else if config.MaxTokens > 0 {
		// Fallback to max_tokens for backward compatibility
		request["max_tokens"] = config.MaxTokens
	}

	// Reasoning effort for o-series models
	if config.ReasoningEffort != "" {
		request["reasoning_effort"] = config.ReasoningEffort
	}

	// Response format for structured outputs
	if config.ResponseFormat != nil {
		request["response_format"] = config.ResponseFormat
	}

	// Presence and frequency penalties
	if config.PresencePenalty != 0 {
		request["presence_penalty"] = config.PresencePenalty
	}
	if config.FrequencyPenalty != 0 {
		request["frequency_penalty"] = config.FrequencyPenalty
	}

	// Top-p sampling
	if config.TopP > 0 {
		request["top_p"] = config.TopP
	}

	// Seed for reproducible outputs
	if config.Seed > 0 {
		request["seed"] = config.Seed
	}

	return request, nil
}

// ParseResponse parses an OpenAI-compatible API response
func (h *OpenAIHandler) ParseResponse(body []byte) (ResponseResult, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return ResponseResult{}, fmt.Errorf("failed to decode OpenAI response: %w", err)
	}

	// Check for API error
	if response.Error != nil {
		return ResponseResult{}, fmt.Errorf("OpenAI API error: %s (type: %s)", response.Error.Message, response.Error.Type)
	}

	// Extract content
	if len(response.Choices) == 0 {
		return ResponseResult{}, fmt.Errorf("no choices in OpenAI response")
	}

	var content string
	choice := response.Choices[0]
	if choice.Message.Content != "" {
		content = strings.TrimSpace(choice.Message.Content)
	} else if choice.Text != "" {
		content = strings.TrimSpace(choice.Text)
	}

	if content == "" {
		return ResponseResult{}, fmt.Errorf("empty content in OpenAI response")
	}

	return ResponseResult{
		Content:    content,
		FormatUsed: FormatTypeOpenAI,
	}, nil
}

// ValidateResponse validates the HTTP response status
func (h *OpenAIHandler) ValidateResponse(statusCode int, body []byte) error {
	switch statusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return errors.New(withOpenAIErrorDetails("authentication failed - check API key", body))
	case http.StatusNotFound:
		return errors.New(withOpenAIErrorDetails("model not found", body))
	case http.StatusBadRequest:
		return errors.New(withOpenAIErrorDetails("bad request - check parameters", body))
	default:
		return errors.New(withOpenAIErrorDetails(fmt.Sprintf("OpenAI API returned status %d", statusCode), body))
	}
}

// FormatEndpoint returns the endpoint as-is for OpenAI format
func (h *OpenAIHandler) FormatEndpoint(endpoint, model string) string {
	return strings.TrimSuffix(endpoint, "/")
}

func withOpenAIErrorDetails(prefix string, body []byte) string {
	details := extractOpenAIErrorDetails(body)
	if details == "" {
		return prefix
	}
	return fmt.Sprintf("%s: %s", prefix, details)
}

func extractOpenAIErrorDetails(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}

	var response struct {
		Error *struct {
			Message string      `json:"message"`
			Type    string      `json:"type"`
			Code    interface{} `json:"code"`
			Param   interface{} `json:"param"`
		} `json:"error"`
		Message string      `json:"message"`
		Msg     string      `json:"msg"`
		Code    interface{} `json:"code"`
	}

	if err := json.Unmarshal(body, &response); err == nil {
		parts := make([]string, 0, 4)

		if response.Error != nil {
			if code := stringifyOpenAIErrorField(response.Error.Code); code != "" {
				parts = append(parts, "code="+code)
			}
			if response.Error.Message != "" {
				parts = append(parts, response.Error.Message)
			}
			if response.Error.Type != "" {
				parts = append(parts, "type="+response.Error.Type)
			}
			if param := stringifyOpenAIErrorField(response.Error.Param); param != "" {
				parts = append(parts, "param="+param)
			}
		} else {
			if code := stringifyOpenAIErrorField(response.Code); code != "" {
				parts = append(parts, "code="+code)
			}
			if response.Message != "" {
				parts = append(parts, response.Message)
			} else if response.Msg != "" {
				parts = append(parts, response.Msg)
			}
		}

		if len(parts) > 0 {
			return strings.Join(parts, ", ")
		}
	}

	return truncateOpenAIErrorDetail(trimmed)
}

func stringifyOpenAIErrorField(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func truncateOpenAIErrorDetail(detail string) string {
	detail = strings.Join(strings.Fields(detail), " ")
	const maxLen = 300
	if len(detail) <= maxLen {
		return detail
	}
	return detail[:maxLen] + "..."
}

// IsOpenAIError checks if an error message indicates an OpenAI API format
func IsOpenAIError(errorMessage string) bool {
	openAIErrorPatterns := []string{
		"incorrect API key provided",
		"invalid_api_key",
		"context_length_exceeded",
		"rate_limit_exceeded",
		"server_error",
		"openai",
	}

	for _, pattern := range openAIErrorPatterns {
		if strings.Contains(strings.ToLower(errorMessage), pattern) {
			return true
		}
	}

	return false
}

// ParseStreamChunk parses a single chunk from an OpenAI streaming response
func (h *OpenAIHandler) ParseStreamChunk(line string) StreamChunk {
	line = strings.TrimSpace(line)
	
	if line == "" {
		return StreamChunk{}
	}
	
	if line == "data: [DONE]" {
		return StreamChunk{Done: true}
	}
	
	if !strings.HasPrefix(line, "data: ") {
		return StreamChunk{}
	}
	
	data := strings.TrimPrefix(line, "data: ")
	
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return StreamChunk{Error: fmt.Errorf("failed to parse stream chunk: %w", err)}
	}
	
	if len(chunk.Choices) == 0 {
		return StreamChunk{}
	}
	
	choice := chunk.Choices[0]
	done := choice.FinishReason != ""
	
	return StreamChunk{
		Content: choice.Delta.Content,
		Done:    done,
	}
}
