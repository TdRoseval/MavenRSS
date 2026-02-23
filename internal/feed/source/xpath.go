package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"MavenRSS/internal/utils/httputil"

	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"
)

const maxXPathRetries = 3

// XPathSource fetches content from web pages using XPath/CSS selectors.
type XPathSource struct {
	client *http.Client
}

// NewXPathSource creates a new XPath source.
func NewXPathSource() *XPathSource {
	return NewXPathSourceWithProxy("")
}

// NewXPathSourceWithProxy creates a new XPath source with custom proxy.
func NewXPathSourceWithProxy(proxyURL string) *XPathSource {
	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	return &XPathSource{
		client: httputil.GetPooledUserAgentClient(proxyURL, 20*time.Second, userAgent),
	}
}

// Type returns the source type identifier.
func (x *XPathSource) Type() Type {
	return TypeXPath
}

// Validate checks if the configuration is valid for XPath source.
func (x *XPathSource) Validate(config *Config) error {
	if config == nil {
		return errors.New("config is nil")
	}
	if config.URL == "" {
		return errors.New("URL is required for XPath source")
	}
	if config.XPathItemSelector == "" {
		return errors.New("item selector is required for XPath source")
	}
	return nil
}

// SetHTTPClient allows setting a custom HTTP client.
func (x *XPathSource) SetHTTPClient(client *http.Client) {
	if client != nil {
		x.client = client
	}
}

// Fetch retrieves content from the URL and extracts items using selectors with retry support.
func (x *XPathSource) Fetch(ctx context.Context, config *Config) (*gofeed.Feed, error) {
	if err := x.Validate(config); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < maxXPathRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		feed, err := x.doFetch(ctx, config)
		if err == nil {
			return feed, nil
		}

		lastErr = err
		errStr := fmt.Sprintf("%v", err)

		if httputil.IsNetworkError(errStr) && attempt < maxXPathRetries-1 {
			backoff := httputil.CalculateBackoffSimple(attempt)
			log.Printf("[XPathSource] Network error on attempt %d/%d for %s, retrying in %v: %v",
				attempt+1, maxXPathRetries, config.URL, backoff, err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			continue
		}

		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}

	return nil, fmt.Errorf("all %d attempts failed, last error: %w", maxXPathRetries, lastErr)
}

// doFetch performs the actual fetch without retries
func (x *XPathSource) doFetch(ctx context.Context, config *Config) (*gofeed.Feed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", config.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if config.UserAgent != "" {
		req.Header.Set("User-Agent", config.UserAgent)
	} else {
		req.Header.Set("User-Agent", "MavenRSS/1.0")
	}

	resp, err := x.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	feed := x.extractFeed(doc, config)
	return feed, nil
}

// extractFeed extracts feed items from the HTML document.
func (x *XPathSource) extractFeed(doc *goquery.Document, config *Config) *gofeed.Feed {
	feed := &gofeed.Feed{
		Title:       x.extractText(doc, config.XPathTitleSelector),
		Description: x.extractText(doc, config.XPathDescSelector),
		Link:        config.URL,
		Items:       []*gofeed.Item{},
	}

	// Extract items
	doc.Find(config.XPathItemSelector).Each(func(i int, s *goquery.Selection) {
		item := &gofeed.Item{}

		// Extract title
		if config.XPathItemTitleSelector != "" {
			item.Title = strings.TrimSpace(s.Find(config.XPathItemTitleSelector).Text())
		} else {
			item.Title = strings.TrimSpace(s.Text())
		}

		// Extract link
		if config.XPathItemLinkSelector != "" {
			link, _ := s.Find(config.XPathItemLinkSelector).Attr("href")
			item.Link = x.resolveURL(config.URL, link)
		} else {
			link, _ := s.Find("a").First().Attr("href")
			item.Link = x.resolveURL(config.URL, link)
		}

		// Extract content
		if config.XPathItemContentSelector != "" {
			html, _ := s.Find(config.XPathItemContentSelector).Html()
			item.Content = html
			item.Description = strings.TrimSpace(s.Find(config.XPathItemContentSelector).Text())
		}

		// Extract date
		if config.XPathItemDateSelector != "" {
			dateStr := strings.TrimSpace(s.Find(config.XPathItemDateSelector).Text())
			if t := x.parseDate(dateStr); t != nil {
				item.PublishedParsed = t
			}
		}

		// Only add items with title or link
		if item.Title != "" || item.Link != "" {
			feed.Items = append(feed.Items, item)
		}
	})

	return feed
}

// extractText extracts text content using a selector.
func (x *XPathSource) extractText(doc *goquery.Document, selector string) string {
	if selector == "" {
		return ""
	}
	return strings.TrimSpace(doc.Find(selector).First().Text())
}

// resolveURL resolves a relative URL against a base URL.
func (x *XPathSource) resolveURL(base, href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	if strings.HasPrefix(href, "/") {
		// Extract base domain
		parts := strings.SplitN(base, "/", 4)
		if len(parts) >= 3 {
			return parts[0] + "//" + parts[2] + href
		}
	}
	return base + "/" + href
}

// parseDate attempts to parse a date string.
func (x *XPathSource) parseDate(s string) *time.Time {
	if s == "" {
		return nil
	}

	formats := []string{
		time.RFC3339,
		time.RFC1123,
		time.RFC1123Z,
		"2006-01-02",
		"2006/01/02",
		"01/02/2006",
		"02-01-2006",
		"January 2, 2006",
		"Jan 2, 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return &t
		}
	}

	return nil
}

// FetchRaw fetches raw HTML content from a URL.
func (x *XPathSource) FetchRaw(ctx context.Context, url string, userAgent string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	} else {
		req.Header.Set("User-Agent", "MavenRSS/1.0")
	}

	resp, err := x.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}
