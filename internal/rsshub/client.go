package rsshub

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"MrRSS/internal/utils/httputil"
)

const maxRetries = 2

type Client struct {
	Endpoint string
	APIKey   string
	proxyURL string
}

func NewClient(endpoint, apiKey string) *Client {
	return &Client{
		Endpoint: strings.TrimSuffix(endpoint, "/"),
		APIKey:   apiKey,
		proxyURL: "",
	}
}

func NewClientWithProxy(endpoint, apiKey, proxyURL string) *Client {
	client := NewClient(endpoint, apiKey)
	client.proxyURL = proxyURL
	return client
}

// SetProxy updates the proxy URL for the client
func (c *Client) SetProxy(proxyURL string) {
	c.proxyURL = proxyURL
}

func (c *Client) ValidateRoute(route string) error {
	return c.ValidateRouteWithContext(context.Background(), route)
}

// ValidateRouteWithContext validates a route with context support
func (c *Client) ValidateRouteWithContext(ctx context.Context, route string) error {
	url := c.BuildURL(route)

	client := httputil.GetPooledHTTPClient(c.proxyURL, httputil.DefaultRSSHubTimeout)

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml, application/atom+xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")
		req.Header.Set("Connection", "keep-alive")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if httputil.IsNetworkError(err.Error()) && attempt < maxRetries-1 {
				backoff := httputil.CalculateBackoffSimple(attempt)
				log.Printf("[RSSHub] Network error on attempt %d/%d, retrying in %v: %v", attempt+1, maxRetries, backoff, err)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff):
				}
				continue
			}
			return fmt.Errorf("failed to connect to RSSHub: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			return nil
		}

		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("route not found: %s", route)
		}

		if resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("RSSHub access denied (403). The public rsshub.app instance has restrictions. Please deploy your own RSSHub instance or configure an API key in settings")
		}

		if resp.StatusCode >= 500 && attempt < maxRetries-1 {
			lastErr = fmt.Errorf("RSSHub returned error: %d %s", resp.StatusCode, resp.Status)
			backoff := httputil.CalculateBackoffSimple(attempt)
			log.Printf("[RSSHub] Server error on attempt %d/%d, retrying in %v", attempt+1, maxRetries, backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			continue
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("RSSHub returned error: %d %s", resp.StatusCode, resp.Status)
		}

		return nil
	}

	return lastErr
}

// TestEndpoint tests if the RSSHub endpoint is reachable
func (c *Client) TestEndpoint() error {
	return c.TestEndpointWithContext(context.Background())
}

// TestEndpointWithContext tests if the RSSHub endpoint is reachable with context support
func (c *Client) TestEndpointWithContext(ctx context.Context) error {
	testURL := c.Endpoint
	if !strings.HasSuffix(testURL, "/") {
		testURL += "/"
	}

	client := httputil.GetPooledHTTPClient(c.proxyURL, httputil.DefaultRSSHubTimeout)

	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to RSSHub endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("RSSHub server returned error: %d %s", resp.StatusCode, resp.Status)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("RSSHub server returned error: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}

func (c *Client) BuildURL(route string) string {
	url := fmt.Sprintf("%s/%s", c.Endpoint, route)
	if c.APIKey != "" {
		url = fmt.Sprintf("%s?key=%s", url, c.APIKey)
	}
	return url
}

func IsRSSHubURL(url string) bool {
	return strings.HasPrefix(url, "rsshub://")
}

func ExtractRoute(url string) string {
	return strings.TrimPrefix(url, "rsshub://")
}
