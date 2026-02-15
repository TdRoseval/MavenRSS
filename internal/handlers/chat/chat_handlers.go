package chat

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"MrRSS/internal/ai"
	"MrRSS/internal/handlers/core"
	"MrRSS/internal/handlers/response"
	"MrRSS/internal/utils/httputil"
	"MrRSS/internal/utils/textutil"
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

func HandleAIChat(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	chatEnabled, _ := h.DB.GetSetting("ai_chat_enabled")
	if chatEnabled != "true" {
		response.Error(w, nil, http.StatusForbidden)
		return
	}

	if h.AITracker.IsLimitReached() {
		log.Printf("AI usage limit reached for chat")
		w.WriteHeader(http.StatusTooManyRequests)
		response.JSON(w, map[string]string{
			"error": "AI usage limit reached",
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
		endpoint, _ = h.DB.GetSetting("ai_endpoint")
		model, _ = h.DB.GetSetting("ai_model")
		apiKey, _ = h.DB.GetEncryptedSetting("ai_api_key")

		if endpoint == "" {
			endpoint = "https://api.openai.com/v1/chat/completions"
		}
		if model == "" {
			model = "gpt-4o-mini"
		}
		log.Printf("Using global AI settings for chat (endpoint: %s, model: %s)", endpoint, model)
	}

	optimizedMessages := optimizeChatContext(req.Messages, req.ArticleTitle, req.ArticleURL, req.ArticleContent, req.IsFirstMessage)

	messagesMap := make([]map[string]string, len(optimizedMessages))
	for i, msg := range optimizedMessages {
		messagesMap[i] = map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	httpClient, err := createAIHTTPClientWithProxy(h, useGlobalProxy, 60*time.Second)
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
	if err := h.AITracker.AddUsage(int64(estimatedTokens)); err != nil {
		log.Printf("Warning: failed to track AI usage: %v", err)
	}

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

func createAIHTTPClientWithProxy(h *core.Handler, useGlobalProxy bool, timeout time.Duration) (*http.Client, error) {
	var proxyURL string

	if useGlobalProxy {
		proxyEnabled, _ := h.DB.GetSetting("proxy_enabled")
		if proxyEnabled == "true" {
			proxyType, _ := h.DB.GetSetting("proxy_type")
			proxyHost, _ := h.DB.GetSetting("proxy_host")
			proxyPort, _ := h.DB.GetSetting("proxy_port")
			proxyUsername, _ := h.DB.GetEncryptedSetting("proxy_username")
			proxyPassword, _ := h.DB.GetEncryptedSetting("proxy_password")
			proxyURL = buildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
		}
	}

	return httputil.GetPooledAIHTTPClient(proxyURL, timeout), nil
}

func buildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword string) string {
	return httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
}
