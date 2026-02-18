package source

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"MrRSS/internal/utils/httputil"

	"github.com/mmcdole/gofeed"
)

const maxRSSRetries = 2

// RSSSource fetches feeds via standard HTTP RSS/Atom requests.
type RSSSource struct {
	parser *gofeed.Parser
	client *http.Client
}

// NewRSSSource creates a new RSS source.
// Uses a pooled HTTP client with 20s timeout.
func NewRSSSource() *RSSSource {
	return NewRSSSourceWithProxy("")
}

// NewRSSSourceWithProxy creates a new RSS source with custom proxy.
func NewRSSSourceWithProxy(proxyURL string) *RSSSource {
	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	client := httputil.GetPooledUserAgentClient(proxyURL, 20*time.Second, userAgent)

	parser := gofeed.NewParser()
	parser.Client = client

	return &RSSSource{
		parser: parser,
		client: client,
	}
}

// Type returns the source type identifier.
func (s *RSSSource) Type() Type {
	return TypeRSS
}

// Validate checks if the configuration is valid for RSS source.
func (s *RSSSource) Validate(config *Config) error {
	if config == nil {
		return errors.New("config is nil")
	}
	if config.URL == "" {
		return errors.New("URL is required for RSS source")
	}
	return nil
}

// Fetch retrieves and parses the RSS/Atom feed from the URL with retry support.
func (s *RSSSource) Fetch(ctx context.Context, config *Config) (*gofeed.Feed, error) {
	if err := s.Validate(config); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < maxRSSRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		feed, err := s.parser.ParseURLWithContext(config.URL, ctx)
		if err == nil {
			return feed, nil
		}

		lastErr = err
		errStr := fmt.Sprintf("%v", err)

		if httputil.IsNetworkError(errStr) && attempt < maxRSSRetries-1 {
			backoff := httputil.CalculateBackoffSimple(attempt)
			log.Printf("[RSSSource] Network error on attempt %d/%d for %s, retrying in %v: %v",
				attempt+1, maxRSSRetries, config.URL, backoff, err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			continue
		}

		return nil, fmt.Errorf("failed to parse feed from %s: %w", config.URL, err)
	}

	return nil, fmt.Errorf("all %d attempts failed, last error: %w", maxRSSRetries, lastErr)
}

// SetHTTPClient updates the HTTP client used for requests.
func (s *RSSSource) SetHTTPClient(client *http.Client) {
	s.client = client
	s.parser.Client = client
}
