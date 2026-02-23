package chat

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"MavenRSS/internal/ai"
	"MavenRSS/internal/handlers/core"
	"MavenRSS/internal/handlers/response"
	"MavenRSS/internal/utils/httputil"
	"MavenRSS/internal/utils/textutil"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Messages       []ChatMessage `json:"messages"`
	ArticleTitle   string        `json:"article_title,omitempty"`
	ArticleURL     string        `json:"article_url,omitempty"`
	ArticleContent string        `json:"article_content,omitempty"`
	IsFirstMessage bool          `json:"is_first_message,omitempty"`
}

type ChatResponse struct {
	Response string `json:"response"`
	HTML     string `json:"html,omitempty"`
}

// isAILimitReached checks if the AI usage limit is reached for a specific user
func isAILimitReached(h *core.Handler, userID int64) bool {
	usageStr, err := h.DB.GetSettingWithFallback(userID, "ai_usage_tokens")
	if err != nil {
		return false
	}
	usage, _ := strconv.ParseInt(usageStr, 10, 64)

	userLimitStr, _ := h.DB.GetSettingWithFallback(userID, "ai_usage_limit")
	hardLimitStr, _ := h.DB.GetSettingWithFallback(userID, "ai_usage_hard_limit")

	userLimit, _ := strconv.ParseInt(userLimitStr, 10, 64)
	hardLimit, _ := strconv.ParseInt(hardLimitStr, 10, 64)

	effectiveLimit := int64(0)
	if userLimit > 0 && hardLimit > 0 {
		effectiveLimit = min(userLimit, hardLimit)
	} else if userLimit > 0 {
		effectiveLimit = userLimit
	} else if hardLimit > 0 {
		effectiveLimit = hardLimit
	}

	if effectiveLimit == 0 {
		return false
	}

	return usage >= effectiveLimit
}

// addAIUsage adds tokens to the AI usage counter for a specific user
func addAIUsage(h *core.Handler, userID int64, tokens int64) {
	usageStr, _ := h.DB.GetSettingWithFallback(userID, "ai_usage_tokens")
	currentUsage, _ := strconv.ParseInt(usageStr, 10, 64)
	newUsage := currentUsage + tokens
	if userID > 0 {
		h.DB.SetSettingForUser(userID, "ai_usage_tokens", strconv.FormatInt(newUsage, 10))
	} else {
		h.DB.SetSetting("ai_usage_tokens", strconv.FormatInt(newUsage, 10))
	}
}

func HandleAIChat(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	chatEnabled, _ := h.DB.GetSettingWithFallback(userID, "ai_chat_enabled")
	if chatEnabled != "true" {
		response.Error(w, nil, http.StatusForbidden)
		return
	}

	if isAILimitReached(h, userID) {
		log.Printf("AI usage limit reached for chat, returning graceful message")
		response.JSON(w, ChatResponse{
			Response: "AI 使用量已达到限制，请稍后再试或联系管理员。",
		})
		return
	}

	h.AITracker.WaitForRateLimit()

	var apiKey, endpoint, model string
	var useGlobalProxy bool = true
	if h.AIProfileProvider != nil {
		cfg, err := h.AIProfileProvider.GetConfigForFeature(ai.FeatureChat)
		if err == nil && cfg != nil && (cfg.APIKey != "" || cfg.Endpoint != "") {
			apiKey = cfg.APIKey
			endpoint = cfg.Endpoint
			model = cfg.Model
			useGlobalProxy = h.AIProfileProvider.UseGlobalProxyForFeature(ai.FeatureChat)
			log.Printf("Using AI profile for chat (endpoint: %s, model: %s, useGlobalProxy: %v)", endpoint, model, useGlobalProxy)
		}
	}

	if endpoint == "" {
		endpoint, _ = h.DB.GetSettingWithFallback(userID, "ai_endpoint")
		model, _ = h.DB.GetSettingWithFallback(userID, "ai_model")
		apiKey, _ = h.DB.GetEncryptedSettingWithFallback(userID, "ai_api_key")

		if endpoint == "" {
			endpoint = "https://api.openai.com/v1/chat/completions"
		}
		if model == "" {
			model = "gpt-4o-mini"
		}
		log.Printf("Using AI settings for chat (endpoint: %s, model: %s)", endpoint, model)
	}

	optimizedMessages := optimizeChatContext(req.Messages, req.ArticleTitle, req.ArticleURL, req.ArticleContent, req.IsFirstMessage)

	messagesMap := make([]map[string]string, len(optimizedMessages))
	for i, msg := range optimizedMessages {
		messagesMap[i] = map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	httpClient, err := createAIHTTPClientWithProxy(h, useGlobalProxy, userID, 60*time.Second)
	if err != nil {
		log.Printf("Failed to create HTTP client with proxy: %v", err)
		response.Error(w, fmt.Errorf("failed to create HTTP client: %w", err), http.StatusInternalServerError)
		return
	}

	clientConfig := ai.ClientConfig{
		APIKey:   apiKey,
		Endpoint: endpoint,
		Model:    model,
		Timeout:  60 * time.Second,
	}
	client := ai.NewClientWithHTTPClient(clientConfig, httpClient)

	result, err := client.RequestWithMessages(messagesMap)
	if err != nil {
		log.Printf("AI chat request failed: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	respContent := result.Content
	thinking := ai.ExtractThinking(respContent)
	respContent = ai.RemoveThinkingTags(respContent)

	htmlResponse := textutil.ConvertMarkdownToHTML(respContent)

	if thinking != "" {
		log.Printf("AI chat thinking: %s", thinking)
	}

	estimatedTokens := estimateChatTokens(optimizedMessages, respContent)
	addAIUsage(h, userID, int64(estimatedTokens))

	_ = h.DB.IncrementStat("ai_chat")

	response.JSON(w, ChatResponse{Response: respContent, HTML: htmlResponse})
}

func optimizeChatContext(messages []ChatMessage, articleTitle, articleURL, articleContent string, isFirstMessage bool) []ChatMessage {
	if isFirstMessage && articleContent != "" {
		systemMsg := ChatMessage{
			Role: "system",
			Content: fmt.Sprintf("You are discussing an article titled: %s\nURL: %s\n\nArticle content:\n%s\n\nPlease help the user understand and discuss this article.",
				articleTitle, articleURL, articleContent),
		}
		return append([]ChatMessage{systemMsg}, messages...)
	}

	const maxHistoryLength = 10
	if len(messages) <= maxHistoryLength {
		return messages
	}

	return messages[len(messages)-maxHistoryLength:]
}

func estimateChatTokens(messages []ChatMessage, response string) int {
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)
	}
	totalChars += len(response)

	totalChars = int(float64(totalChars) * 1.2)

	return totalChars / 4
}

func createAIHTTPClientWithProxy(h *core.Handler, useGlobalProxy bool, userID int64, timeout time.Duration) (*http.Client, error) {
	var proxyURL string

	if useGlobalProxy {
		proxyEnabled, _ := h.DB.GetSettingWithFallback(userID, "proxy_enabled")
		if proxyEnabled == "true" {
			proxyType, _ := h.DB.GetSettingWithFallback(userID, "proxy_type")
			proxyHost, _ := h.DB.GetSettingWithFallback(userID, "proxy_host")
			proxyPort, _ := h.DB.GetSettingWithFallback(userID, "proxy_port")
			proxyUsername, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_username")
			proxyPassword, _ := h.DB.GetEncryptedSettingWithFallback(userID, "proxy_password")
			proxyURL = buildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
		}
	}

	return httputil.GetPooledAIHTTPClient(proxyURL, timeout), nil
}

func buildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword string) string {
	return httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
}

// HandleAIChatStream handles AI chat with streaming response
func HandleAIChatStream(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	chatEnabled, _ := h.DB.GetSettingWithFallback(userID, "ai_chat_enabled")
	if chatEnabled != "true" {
		response.Error(w, nil, http.StatusForbidden)
		return
	}

	if isAILimitReached(h, userID) {
		log.Printf("AI usage limit reached for chat stream, returning graceful message")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		fmt.Fprintf(w, "data: {\"content\":\"AI 使用量已达到限制，请稍后再试或联系管理员。\"}\n\n")
		fmt.Fprintf(w, "event: done\ndata: {\"done\":true,\"response\":\"AI 使用量已达到限制，请稍后再试或联系管理员。\"}\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		return
	}

	h.AITracker.WaitForRateLimit()

	var apiKey, endpoint, model string
	var useGlobalProxy bool = true
	if h.AIProfileProvider != nil {
		cfg, err := h.AIProfileProvider.GetConfigForFeature(ai.FeatureChat)
		if err == nil && cfg != nil && (cfg.APIKey != "" || cfg.Endpoint != "") {
			apiKey = cfg.APIKey
			endpoint = cfg.Endpoint
			model = cfg.Model
			useGlobalProxy = h.AIProfileProvider.UseGlobalProxyForFeature(ai.FeatureChat)
			log.Printf("Using AI profile for chat stream (endpoint: %s, model: %s, useGlobalProxy: %v)", endpoint, model, useGlobalProxy)
		}
	}

	if endpoint == "" {
		endpoint, _ = h.DB.GetSettingWithFallback(userID, "ai_endpoint")
		model, _ = h.DB.GetSettingWithFallback(userID, "ai_model")
		apiKey, _ = h.DB.GetEncryptedSettingWithFallback(userID, "ai_api_key")

		if endpoint == "" {
			endpoint = "https://api.openai.com/v1/chat/completions"
		}
		if model == "" {
			model = "gpt-4o-mini"
		}
		log.Printf("Using AI settings for chat stream (endpoint: %s, model: %s)", endpoint, model)
	}

	optimizedMessages := optimizeChatContext(req.Messages, req.ArticleTitle, req.ArticleURL, req.ArticleContent, req.IsFirstMessage)

	messagesMap := make([]map[string]string, len(optimizedMessages))
	for i, msg := range optimizedMessages {
		messagesMap[i] = map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	httpClient, err := createAIHTTPClientWithProxy(h, useGlobalProxy, userID, 300*time.Second)
	if err != nil {
		log.Printf("Failed to create HTTP client with proxy: %v", err)
		response.Error(w, fmt.Errorf("failed to create HTTP client: %w", err), http.StatusInternalServerError)
		return
	}

	clientConfig := ai.ClientConfig{
		APIKey:   apiKey,
		Endpoint: endpoint,
		Model:    model,
		Timeout:  300 * time.Second,
	}
	client := ai.NewClientWithHTTPClient(clientConfig, httpClient)

	config := ai.RequestConfig{
		Model:       clientConfig.Model,
		Messages:    messagesMap,
		Temperature: 0.3,
		MaxTokens:   2048,
		Context:     r.Context(),
	}

	chunkChan, err := client.RequestStream(config)
	if err != nil {
		log.Printf("AI chat stream request failed: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var fullContent strings.Builder
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Printf("Streaming not supported - ResponseWriter doesn't implement Flusher")
		response.Error(w, fmt.Errorf("streaming not supported"), http.StatusInternalServerError)
		return
	}

	log.Printf("Starting to send stream chunks...")
	chunkCount := 0

	for {
		select {
		case <-r.Context().Done():
			log.Printf("Client disconnected from chat stream")
			return
		case chunk, ok := <-chunkChan:
			if !ok {
				log.Printf("Chunk channel closed, sending complete...")
				goto sendComplete
			}
			if chunk.Error != nil {
				log.Printf("Stream error: %v", chunk.Error)
				errorEvent := map[string]string{
					"error": chunk.Error.Error(),
				}
				if data, err := json.Marshal(errorEvent); err == nil {
					fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
					flusher.Flush()
				}
				return
			}
			if chunk.Content != "" {
				chunkCount++
				fullContent.WriteString(chunk.Content)
				event := map[string]string{
					"content": chunk.Content,
				}
				if data, err := json.Marshal(event); err == nil {
					fmt.Fprintf(w, "data: %s\n\n", data)
					flusher.Flush()
					if chunkCount%10 == 0 {
						log.Printf("Sent %d chunks so far...", chunkCount)
					}
				}
			}
			if chunk.Done {
				log.Printf("Received done chunk, total chunks: %d", chunkCount)
				goto sendComplete
			}
		}
	}

sendComplete:
	// Process the full response
	respContent := fullContent.String()
	thinking := ai.ExtractThinking(respContent)
	respContent = ai.RemoveThinkingTags(respContent)
	htmlResponse := textutil.ConvertMarkdownToHTML(respContent)

	if thinking != "" {
		log.Printf("AI chat stream thinking: %s", thinking)
	}

	// Send completion event with full content and HTML
	completeEvent := map[string]interface{}{
		"done":     true,
		"response": respContent,
		"html":     htmlResponse,
		"thinking": thinking,
	}
	if data, err := json.Marshal(completeEvent); err == nil {
		fmt.Fprintf(w, "event: done\ndata: %s\n\n", data)
		flusher.Flush()
	}

	// Track usage
	estimatedTokens := estimateChatTokens(optimizedMessages, respContent)
	addAIUsage(h, userID, int64(estimatedTokens))

	_ = h.DB.IncrementStat("ai_chat")
}
