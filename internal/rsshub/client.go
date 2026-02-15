package rsshub

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client handles RSSHub route validation and URL transformation
type Client struct {
	Endpoint string
	APIKey   string
}

// NewClient creates a new RSSHub client
func NewClient(endpoint, apiKey string) *Client {
	return &Client{
		Endpoint: strings.TrimSuffix(endpoint, "/"),
		APIKey:   apiKey,
	}
}

// ValidateRoute performs a GET request to check if route exists
// Using GET instead of HEAD to avoid Cloudflare blocking
func (c *Client) ValidateRoute(route string) error {
	url := c.BuildURL(route)

	// Create optimized HTTP client for RSSHub
	transport := &http.Transport{
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       60 * time.Second,
		ForceAttemptHTTP2:     true,
		ResponseHeaderTimeout: 8 * time.Second,
		TLSHandshakeTimeout:   4 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		// Disable redirect following to avoid unnecessary requests
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set browser-like headers to avoid 403 restrictions from rsshub.app
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml, application/atom+xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to RSSHub: %w", err)
	}
	defer resp.Body.Close()

	// Accept redirects (3xx) as valid routes
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("route not found: %s", route)
	}

	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("RSSHub access denied (403). The public rsshub.app instance has restrictions. Please deploy your own RSSHub instance or configure an API key in settings")
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("RSSHub returned error: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// BuildURL converts a route to full RSSHub URL
func (c *Client) BuildURL(route string) string {
	url := fmt.Sprintf("%s/%s", c.Endpoint, route)
	if c.APIKey != "" {
		url = fmt.Sprintf("%s?key=%s", url, c.APIKey)
	}
	return url
}

// IsRSSHubURL checks if a URL uses the rsshub:// protocol
func IsRSSHubURL(url string) bool {
	return strings.HasPrefix(url, "rsshub://")
}

// ExtractRoute removes the rsshub:// prefix
func ExtractRoute(url string) string {
	return strings.TrimPrefix(url, "rsshub://")
}
