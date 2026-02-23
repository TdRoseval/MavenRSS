package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"MavenRSS/internal/ai"
	"MavenRSS/internal/config"
	"MavenRSS/internal/database"
	"MavenRSS/internal/models"
	"MavenRSS/internal/utils/httputil"
)

type aiService struct {
	registry *Registry
	db       *database.DB
}

func NewAIService(registry *Registry, db *database.DB) AIService {
	return &aiService{
		registry: registry,
		db:       db,
	}
}

func (s *aiService) Summarize(ctx context.Context, content string) (string, error) {
	if !s.registry.AITracker().CanMakeRequest() {
		return "", fmt.Errorf("daily AI usage limit reached")
	}

	apiKey, _ := s.db.GetEncryptedSetting("ai_api_key")
	endpoint, _ := s.db.GetSetting("ai_endpoint")
	model, _ := s.db.GetSetting("ai_model")

	defaults := config.Get()
	if endpoint == "" {
		endpoint = defaults.AIEndpoint
	}
	if model == "" {
		model = defaults.AIModel
	}

	httpClient, err := s.createAIHTTPClient()
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP client: %w", err)
	}

	clientConfig := ai.ClientConfig{
		APIKey:   apiKey,
		Endpoint: endpoint,
		Model:    model,
		Timeout:  60 * time.Second,
	}
	client := ai.NewClientWithHTTPClient(clientConfig, httpClient)

	messages := []map[string]string{
		{"role": "user", "content": content + "\n\nSummarize this article"},
	}
	response, err := client.RequestWithMessages(messages)
	if err != nil {
		return "", err
	}

	s.registry.AITracker().AddUsage(int64(len(content)))

	return response.Content, nil
}

func (s *aiService) Chat(ctx context.Context, sessionID int64, message string) (string, error) {
	if !s.registry.AITracker().CanMakeRequest() {
		return "", fmt.Errorf("daily AI usage limit reached")
	}

	apiKey, _ := s.db.GetEncryptedSetting("ai_api_key")
	endpoint, _ := s.db.GetSetting("ai_endpoint")
	model, _ := s.db.GetSetting("ai_model")

	defaults := config.Get()
	if endpoint == "" {
		endpoint = defaults.AIEndpoint
	}
	if model == "" {
		model = defaults.AIModel
	}

	httpClient, err := s.createAIHTTPClient()
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP client: %w", err)
	}

	clientConfig := ai.ClientConfig{
		APIKey:   apiKey,
		Endpoint: endpoint,
		Model:    model,
		Timeout:  60 * time.Second,
	}
	client := ai.NewClientWithHTTPClient(clientConfig, httpClient)

	messages := []map[string]string{
		{"role": "user", "content": message},
	}
	response, err := client.RequestWithMessages(messages)
	if err != nil {
		return "", err
	}

	s.registry.AITracker().AddUsage(int64(len(message)))

	return response.Content, nil
}

func (s *aiService) Search(ctx context.Context, query string) ([]models.Article, error) {
	return []models.Article{}, nil
}

func (s *aiService) TestConfig(ctx context.Context) error {
	apiKey, _ := s.db.GetEncryptedSetting("ai_api_key")
	endpoint, _ := s.db.GetSetting("ai_endpoint")
	model, _ := s.db.GetSetting("ai_model")

	defaults := config.Get()
	if endpoint == "" {
		endpoint = defaults.AIEndpoint
	}
	if model == "" {
		model = defaults.AIModel
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("API endpoint must use HTTP or HTTPS")
	}

	httpClient, err := s.createAIHTTPClient()
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	clientConfig := ai.ClientConfig{
		APIKey:   apiKey,
		Endpoint: endpoint,
		Model:    model,
		Timeout:  60 * time.Second,
	}
	client := ai.NewClientWithHTTPClient(clientConfig, httpClient)

	testMessages := []map[string]string{
		{"role": "user", "content": "Hello"},
	}
	_, err = client.RequestWithMessages(testMessages)
	return err
}

func (s *aiService) createAIHTTPClient() (*http.Client, error) {
	var proxyURL string

	proxyEnabled, _ := s.db.GetSetting("proxy_enabled")
	if proxyEnabled == "true" {
		proxyType, _ := s.db.GetSetting("proxy_type")
		proxyHost, _ := s.db.GetSetting("proxy_host")
		proxyPort, _ := s.db.GetSetting("proxy_port")
		proxyUsername, _ := s.db.GetEncryptedSetting("proxy_username")
		proxyPassword, _ := s.db.GetEncryptedSetting("proxy_password")
		proxyURL = s.buildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
	}

	return httputil.GetPooledAIHTTPClient(proxyURL, 60*time.Second), nil
}

func (s *aiService) buildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword string) string {
	return httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
}
