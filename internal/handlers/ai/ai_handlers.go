package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"MrRSS/internal/ai"
	"MrRSS/internal/config"
	"MrRSS/internal/handlers/core"
	"MrRSS/internal/handlers/response"
	"MrRSS/internal/utils/httputil"
)

type TestResult struct {
	ConfigValid       bool   `json:"config_valid"`
	ConnectionSuccess bool   `json:"connection_success"`
	ModelAvailable    bool   `json:"model_available"`
	ResponseTimeMs    int64  `json:"response_time_ms"`
	TestTime          string `json:"test_time"`
	ErrorMessage      string `json:"error_message,omitempty"`
}

func HandleTestAIConfig(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	result := TestResult{
		TestTime: time.Now().Format(time.RFC3339),
	}

	apiKey, _ := h.DB.GetEncryptedSetting("ai_api_key")
	endpoint, _ := h.DB.GetSetting("ai_endpoint")
	model, _ := h.DB.GetSetting("ai_model")

	defaults := config.Get()
	if endpoint == "" {
		endpoint = defaults.AIEndpoint
	}
	if model == "" {
		model = defaults.AIModel
	}

	result.ConfigValid = true
	validationErrors := []string{}

	if endpoint == "" {
		validationErrors = append(validationErrors, "endpoint is required")
		result.ConfigValid = false
	}

	if model == "" {
		validationErrors = append(validationErrors, "model is required")
		result.ConfigValid = false
	}

	if !result.ConfigValid {
		result.ErrorMessage = "Configuration incomplete: " + strings.Join(validationErrors, ", ")
		response.JSON(w, result)
		return
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		result.ConfigValid = false
		result.ErrorMessage = "Invalid endpoint URL: " + err.Error()
		response.JSON(w, result)
		return
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		result.ConfigValid = false
		result.ErrorMessage = "API endpoint must use HTTP or HTTPS"
		response.JSON(w, result)
		return
	}

	startTime := time.Now()

	httpClient, err := createAIHTTPClientWithProxy(h, true, 30*time.Second)
	if err != nil {
		result.ConnectionSuccess = false
		result.ModelAvailable = false
		result.ErrorMessage = fmt.Sprintf("Failed to create HTTP client: %v", err)
		result.ResponseTimeMs = time.Since(startTime).Milliseconds()
		response.JSON(w, result)
		return
	}

	clientConfig := ai.ClientConfig{
		APIKey:   apiKey,
		Endpoint: endpoint,
		Model:    model,
		Timeout:  30 * time.Second,
	}
	client := ai.NewClientWithHTTPClient(clientConfig, httpClient)

	testMessages := []map[string]string{
		{"role": "user", "content": "Hello"},
	}
	_, err = client.RequestWithMessages(testMessages)

	if err != nil {
		result.ConnectionSuccess = false
		result.ModelAvailable = false
		result.ErrorMessage = fmt.Sprintf("Connection failed: %v", err)
	} else {
		result.ConnectionSuccess = true
		result.ModelAvailable = true
	}

	result.ResponseTimeMs = time.Since(startTime).Milliseconds()

	response.JSON(w, result)
}

func HandleGetAITestInfo(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	result := TestResult{
		ConfigValid:       false,
		ConnectionSuccess: false,
		ModelAvailable:    false,
		ResponseTimeMs:    0,
		TestTime:          "",
	}

	response.JSON(w, result)
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
