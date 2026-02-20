// Package ai provides universal AI client with automatic format detection
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"MrRSS/internal/utils/httputil"
)

// ClientConfig holds the configuration for the AI client
type ClientConfig struct {
	APIKey        string
	Endpoint      string
	Model         string
	SystemPrompt  string
	CustomHeaders string
	Timeout       time.Duration
	ProxyURL      string
}

// Client represents a universal AI client that supports multiple API formats
type Client struct {
	config ClientConfig
	client *http.Client
}

// NewClient creates a new universal AI client
func NewClient(config ClientConfig) *Client {
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}

	httpClient := httputil.GetPooledAIHTTPClient(config.ProxyURL, config.Timeout)

	return &Client{
		config: config,
		client: httpClient,
	}
}

// NewClientWithHTTPClient creates a new AI client with a custom HTTP client
func NewClientWithHTTPClient(config ClientConfig, httpClient *http.Client) *Client {
	return &Client{
		config: config,
		client: httpClient,
	}
}

// Request makes an AI request with automatic format detection and fallback
func (c *Client) Request(systemPrompt, userPrompt string) (string, error) {
	result, err := c.RequestWithThinking(systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

// RequestWithThinking makes an AI request and returns both content and thinking
func (c *Client) RequestWithThinking(systemPrompt, userPrompt string) (ResponseResult, error) {
	config := RequestConfig{
		Model:        c.config.Model,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.3,
		MaxTokens:    2048,
	}

	return c.RequestWithConfig(config)
}

// RequestWithMessages makes an AI request using messages format
func (c *Client) RequestWithMessages(messages []map[string]string) (ResponseResult, error) {
	config := RequestConfig{
		Model:       c.config.Model,
		Messages:    messages,
		Temperature: 0.3,
		MaxTokens:   2048,
	}

	return c.RequestWithConfig(config)
}

// RequestWithConfig makes an AI request with full configuration
func (c *Client) RequestWithConfig(config RequestConfig) (ResponseResult, error) {
	const totalAITimeout = 120 * time.Second
	
	ctx := config.Context
	if ctx == nil {
		ctx = context.Background()
	}
	
	totalCtx, cancel := context.WithTimeout(ctx, totalAITimeout)
	defer cancel()
	
	configWithContext := config
	configWithContext.Context = totalCtx
	
	provider := DetectAPIProvider(c.config.Endpoint)

	var allErrors []string
	var networkErrors []string

	logFn := func(name string, err error) {
		errStr := fmt.Sprintf("%v", err)
		log.Printf("[AI Client] %s format failed: %v", name, err)
		allErrors = append(allErrors, fmt.Sprintf("%s: %v", name, err))

		if isNetworkError(errStr) {
			networkErrors = append(networkErrors, fmt.Sprintf("%s: %v", name, err))
		}
	}

	// Try OpenAI format first (most common and widely compatible)
	result, err := c.tryFormat(NewOpenAIHandler(), configWithContext)
	if err == nil {
		return result, nil
	}
	logFn("OpenAI", err)
	
	// Check if we've already timed out
	select {
	case <-totalCtx.Done():
		return ResponseResult{}, fmt.Errorf("AI request total timeout exceeded after %v: %w", totalAITimeout, totalCtx.Err())
	default:
	}

	// Try provider-specific format based on endpoint detection as fallback
	switch provider {
	case "gemini":
		result, err := c.tryFormat(NewGeminiHandler(), configWithContext)
		if err == nil {
			return result, nil
		}
		logFn("Gemini", err)
		select {
		case <-totalCtx.Done():
			return ResponseResult{}, fmt.Errorf("AI request total timeout exceeded after %v: %w", totalAITimeout, totalCtx.Err())
		default:
		}

	case "anthropic":
		result, err := c.tryFormat(&AnthropicHandler{}, configWithContext)
		if err == nil {
			return result, nil
		}
		logFn("Anthropic", err)
		select {
		case <-totalCtx.Done():
			return ResponseResult{}, fmt.Errorf("AI request total timeout exceeded after %v: %w", totalAITimeout, totalCtx.Err())
		default:
		}

	case "deepseek":
		result, err := c.tryFormat(&DeepSeekHandler{}, configWithContext)
		if err == nil {
			return result, nil
		}
		logFn("DeepSeek", err)
		select {
		case <-totalCtx.Done():
			return ResponseResult{}, fmt.Errorf("AI request total timeout exceeded after %v: %w", totalAITimeout, totalCtx.Err())
		default:
		}

	case "ollama":
		result, err := c.tryFormat(&OllamaHandler{}, configWithContext)
		if err == nil {
			return result, nil
		}
		logFn("Ollama", err)
		select {
		case <-totalCtx.Done():
			return ResponseResult{}, fmt.Errorf("AI request total timeout exceeded after %v: %w", totalAITimeout, totalCtx.Err())
		default:
		}
	}

	// Try remaining formats only if needed (reduced fallback list for faster failure)
	remainingHandlers := []struct {
		name    string
		handler FormatHandler
		skip    bool
	}{
		{"Anthropic", &AnthropicHandler{}, provider == "anthropic"},
		{"DeepSeek", &DeepSeekHandler{}, provider == "deepseek"},
		{"Gemini", NewGeminiHandler(), provider == "gemini"},
		{"Ollama", &OllamaHandler{}, provider == "ollama"},
	}

	for _, h := range remainingHandlers {
		if h.skip {
			continue
		}
		result, err = c.tryFormat(h.handler, configWithContext)
		if err == nil {
			return result, nil
		}
		logFn(h.name, err)
		
		select {
		case <-totalCtx.Done():
			return ResponseResult{}, fmt.Errorf("AI request total timeout exceeded after %v: %w", totalAITimeout, totalCtx.Err())
		default:
		}
	}

	// All formats failed - return detailed error
	errMsg := "all API formats failed: [" + strings.Join(allErrors, "; ") + "]"

	if len(networkErrors) > 0 {
		errMsg += " [Network errors detected: " + strings.Join(networkErrors, "; ") + ". Check your network/proxy configuration]"
	}

	return ResponseResult{}, fmt.Errorf(errMsg)
}

// tryFormat attempts to make a request using a specific format handler
func (c *Client) tryFormat(handler FormatHandler, config RequestConfig) (ResponseResult, error) {
	// Build request body
	requestBody, err := handler.BuildRequest(config)
	if err != nil {
		return ResponseResult{}, fmt.Errorf("failed to build request: %w", err)
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return ResponseResult{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Format endpoint
	formattedEndpoint := handler.FormatEndpoint(c.config.Endpoint, c.config.Model)

	// Special handling for Ollama: use /api/chat if messages are provided
	if _, ok := handler.(*OllamaHandler); ok && len(config.Messages) > 0 {
		// Replace /api/generate with /api/chat for message-based requests
		formattedEndpoint = strings.Replace(formattedEndpoint, "/api/generate", "/api/chat", 1)
	}

	// Reduced retry count for faster failure (2 attempts instead of 3)
	maxRetries := 2
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Check if context is cancelled before making request
		if config.Context != nil {
			select {
			case <-config.Context.Done():
				return ResponseResult{}, fmt.Errorf("request cancelled: %w", config.Context.Err())
			default:
			}
		}

		// Send request with formatted endpoint and handler
		resp, err := c.sendRequestToEndpointWithHandler(jsonBody, formattedEndpoint, handler)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)

			// Only retry on network errors
			errStr := fmt.Sprintf("%v", err)
			if isNetworkError(errStr) && attempt < maxRetries-1 {
				log.Printf("[AI Client] Network error on attempt %d/%d, retrying: %v", attempt+1, maxRetries, err)
				// Shorter exponential backoff: 500ms, 1s
				backoffTime := time.Duration(500*(1<<uint(attempt))) * time.Millisecond
				if backoffTime > 5*time.Second {
					backoffTime = 5 * time.Second
				}

				// Wait with context cancellation check
				if config.Context != nil {
					select {
					case <-config.Context.Done():
						return ResponseResult{}, fmt.Errorf("request cancelled during backoff: %w", config.Context.Err())
					case <-time.After(backoffTime):
					}
				} else {
					select {
					case <-time.After(backoffTime):
					}
				}
				continue
			}
			return ResponseResult{}, lastErr
		}

		// Read response body
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			if isNetworkError(fmt.Sprintf("%v", err)) && attempt < maxRetries-1 {
				log.Printf("[AI Client] Read error on attempt %d/%d, retrying: %v", attempt+1, maxRetries, err)
				backoffTime := time.Duration(500*(1<<uint(attempt))) * time.Millisecond
				if backoffTime > 5*time.Second {
					backoffTime = 5 * time.Second
				}

				// Wait with context cancellation check
				if config.Context != nil {
					select {
					case <-config.Context.Done():
						return ResponseResult{}, fmt.Errorf("request cancelled during backoff: %w", config.Context.Err())
					case <-time.After(backoffTime):
					}
				} else {
					select {
					case <-time.After(backoffTime):
					}
				}
				continue
			}
			return ResponseResult{}, lastErr
		}

		// Validate response
		if err := handler.ValidateResponse(resp.StatusCode, bodyBytes); err != nil {
			// Don't retry on validation errors (auth errors, bad requests, etc.)
			return ResponseResult{}, err
		}

		// Parse response
		result, err := handler.ParseResponse(bodyBytes)
		if err != nil {
			lastErr = fmt.Errorf("failed to parse response: %w", err)
			// Don't retry parse errors
			return ResponseResult{}, lastErr
		}

		return result, nil
	}

	return ResponseResult{}, lastErr
}

// sendRequestToEndpointWithHandler sends the HTTP request to a specific endpoint with handler-specific headers
func (c *Client) sendRequestToEndpointWithHandler(jsonBody []byte, apiURL string, handler FormatHandler) (*http.Response, error) {
	// Validate endpoint URL to prevent SSRF attacks
	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid API endpoint URL: %w", err)
	}

	// Both HTTP and HTTPS are allowed
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("API endpoint must use HTTP or HTTPS")
	}

	// Check if this is a Gemini endpoint that needs API key in URL
	isGeminiEndpoint := IsGeminiEndpoint(apiURL)

	// For Gemini API, add API key as URL query parameter instead of Authorization header
	if isGeminiEndpoint && c.config.APIKey != "" {
		// Add or update the 'key' query parameter
		query := parsedURL.Query()
		query.Set("key", c.config.APIKey)
		parsedURL.RawQuery = query.Encode()
		apiURL = parsedURL.String()
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Check if handler provides custom headers
	type HeaderProvider interface {
		GetRequiredHeaders(apiKey string) map[string]string
	}

	if handler != nil {
		if hp, ok := handler.(HeaderProvider); ok {
			// Use handler-specific headers
			requiredHeaders := hp.GetRequiredHeaders(c.config.APIKey)
			for key, value := range requiredHeaders {
				req.Header.Set(key, value)
			}
		} else {
			// Use default headers
			req.Header.Set("Content-Type", "application/json")
			// For non-Gemini endpoints, use Authorization header
			if !isGeminiEndpoint {
				// Only add Authorization header if API key is provided
				if c.config.APIKey != "" {
					req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
				}
			}
		}
	} else {
		// No handler provided, use default headers
		req.Header.Set("Content-Type", "application/json")
		// For non-Gemini endpoints, use Authorization header
		if !isGeminiEndpoint {
			// Only add Authorization header if API key is provided
			if c.config.APIKey != "" {
				req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
			}
		}
	}

	// Parse and add custom headers if provided
	if c.config.CustomHeaders != "" {
		customHeaders, err := parseCustomHeaders(c.config.CustomHeaders)
		if err != nil {
			return nil, fmt.Errorf("failed to parse custom headers: %w", err)
		}
		// Apply custom headers
		for key, value := range customHeaders {
			req.Header.Set(key, value)
		}
	}

	return c.client.Do(req)
}

// parseCustomHeaders parses the JSON string of custom headers into a map
func parseCustomHeaders(headersJSON string) (map[string]string, error) {
	// Return empty map if headers string is empty
	if headersJSON == "" {
		return make(map[string]string), nil
	}

	var headers map[string]string
	if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
		return nil, fmt.Errorf("failed to parse custom headers JSON: %w", err)
	}
	return headers, nil
}

// ExtractThinking extracts thinking content from <thinking> tags (case-insensitive)
func ExtractThinking(content string) string {
	tagVariations := []struct {
		start string
		end   string
	}{
		{"<thinking>", "</thinking>"},
		{"<THINKING>", "</THINKING>"},
		{"<Thinking>", "</Thinking>"},
		{"<think>", "</think>"},
		{"<THINK>", "</THINK>"},
		{"<Think>", "</Think>"},
	}

	for _, tags := range tagVariations {
		startIndex := strings.Index(content, tags.start)
		if startIndex == -1 {
			continue
		}

		endIndex := strings.Index(content[startIndex:], tags.end)
		if endIndex == -1 {
			continue
		}

		// Extract the content between tags (excluding tags themselves)
		thinkingStart := startIndex + len(tags.start)
		thinkingEnd := startIndex + endIndex
		thinking := strings.TrimSpace(content[thinkingStart:thinkingEnd])

		return thinking
	}

	return ""
}

// RemoveThinkingTags removes <thinking> tags and their content from the response (case-insensitive)
func RemoveThinkingTags(content string) string {
	tagVariations := []struct {
		start string
		end   string
	}{
		{"<thinking>", "</thinking>"},
		{"<THINKING>", "</THINKING>"},
		{"<Thinking>", "</Thinking>"},
		{"<think>", "</think>"},
		{"<THINK>", "</THINK>"},
		{"<Think>", "</Think>"},
	}

	result := content
	for _, tags := range tagVariations {
		for {
			startIndex := strings.Index(result, tags.start)
			if startIndex == -1 {
				break
			}

			endIndex := strings.Index(result[startIndex:], tags.end)
			if endIndex == -1 {
				break
			}

			// Remove the entire thinking block including tags
			thinkingEnd := startIndex + endIndex + len(tags.end)
			result = result[:startIndex] + result[thinkingEnd:]
		}
	}

	return strings.TrimSpace(result)
}

// isNetworkError checks if an error message indicates a network connectivity issue
func isNetworkError(errMsg string) bool {
	errLower := strings.ToLower(errMsg)
	networkErrorPatterns := []string{
		"connection refused",
		"connection reset",
		"connection timed out",
		"no route to host",
		"network is unreachable",
		"temporary failure in name resolution",
		"dial tcp",
		"i/o timeout",
		"tls:",
		"no such host",
		"connection closed",
		"broken pipe",
		"context deadline exceeded",
		"unexpected eof",
		"handshake timeout",
		"read: connection reset",
		"write: connection reset",
		"connection refused by peer",
		"remote error",
		"stream error",
		"connect: connection refused",
		"connect: no route to host",
		"connect: network is unreachable",
		"dns timeout",
		"name resolution",
		"lookup failed",
		"client.timeout exceeded",
		"deadline exceeded",
		"operation timed out",
	}

	for _, pattern := range networkErrorPatterns {
		if strings.Contains(errLower, pattern) {
			return true
		}
	}

	return false
}

// RequestStream makes a streaming AI request and returns a channel of chunks
func (c *Client) RequestStream(config RequestConfig) (<-chan StreamChunk, error) {
	const totalAITimeout = 300 * time.Second
	
	ctx := config.Context
	if ctx == nil {
		ctx = context.Background()
	}
	
	totalCtx, cancel := context.WithTimeout(ctx, totalAITimeout)
	config.Context = totalCtx
	
	chunkChan := make(chan StreamChunk, 100)
	
	go func() {
		defer close(chunkChan)
		defer cancel()
		
		// Try OpenAI format first
		handler := NewOpenAIHandler()
		if streamHandler, ok := any(handler).(StreamFormatHandler); ok {
			err := c.tryStreamFormat(streamHandler, handler, config, chunkChan)
			if err == nil {
				return
			}
			chunkChan <- StreamChunk{Error: err}
			log.Printf("[AI Client] Streaming request failed: %v", err)
		} else {
			chunkChan <- StreamChunk{Error: fmt.Errorf("OpenAI handler does not support streaming")}
		}
	}()
	
	return chunkChan, nil
}

// tryStreamFormat attempts to make a streaming request using a specific format handler
func (c *Client) tryStreamFormat(streamHandler StreamFormatHandler, handler FormatHandler, config RequestConfig, chunkChan chan<- StreamChunk) error {
	// Build streaming request body
	requestBody, err := streamHandler.BuildStreamRequest(config)
	if err != nil {
		return fmt.Errorf("failed to build stream request: %w", err)
	}
	
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal stream request: %w", err)
	}
	
	// Format endpoint
	formattedEndpoint := handler.FormatEndpoint(c.config.Endpoint, c.config.Model)
	
	// Check context before making request
	if config.Context != nil {
		select {
		case <-config.Context.Done():
			return fmt.Errorf("request cancelled: %w", config.Context.Err())
		default:
		}
	}
	
	// Send request
	resp, err := c.sendRequestToEndpointWithHandler(jsonBody, formattedEndpoint, handler)
	if err != nil {
		return fmt.Errorf("stream request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Validate response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := handler.ValidateResponse(resp.StatusCode, bodyBytes); err != nil {
			return err
		}
		return fmt.Errorf("stream request returned status %d", resp.StatusCode)
	}
	
	// Parse streaming response
	reader := resp.Body
	buf := make([]byte, 4096)
	var lineBuffer strings.Builder
	
	// Check if handler has ParseStreamChunk method
	type StreamChunkParser interface {
		ParseStreamChunk(line string) StreamChunk
	}
	
	parser, ok := handler.(StreamChunkParser)
	if !ok {
		return fmt.Errorf("handler does not support stream chunk parsing")
	}
	
	for {
		select {
		case <-config.Context.Done():
			return fmt.Errorf("request cancelled: %w", config.Context.Err())
		default:
		}
		
		n, err := reader.Read(buf)
		if n > 0 {
			for i := 0; i < n; i++ {
				if buf[i] == '\n' {
					line := lineBuffer.String()
					lineBuffer.Reset()
					
					chunk := parser.ParseStreamChunk(line)
					if chunk.Error != nil {
						log.Printf("[AI Client] Stream chunk parse error: %v", chunk.Error)
						continue
					}
					
					if chunk.Content != "" || chunk.Done || chunk.Thinking != "" {
						chunkChan <- chunk
					}
					
					if chunk.Done {
						return nil
					}
				} else {
					lineBuffer.WriteByte(buf[i])
				}
			}
		}
		
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("stream read error: %w", err)
		}
	}
}
