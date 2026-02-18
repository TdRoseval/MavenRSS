package discovery

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"MrRSS/internal/utils/httputil"

	"github.com/PuerkitoBio/goquery"
)

func (s *Service) DiscoverFromFeed(ctx context.Context, feedURL string) ([]DiscoveredBlog, error) {
	return s.DiscoverFromFeedWithProgress(ctx, feedURL, nil)
}

func (s *Service) DiscoverFromFeedWithProgress(ctx context.Context, feedURL string, progressCb ProgressCallback) ([]DiscoveredBlog, error) {
	if progressCb != nil {
		progressCb(Progress{
			Stage:   "fetching_homepage",
			Message: "Fetching homepage from feed",
			Detail:  feedURL,
		})
	}

	homepage, err := s.getFeedHomepage(ctx, feedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get homepage from feed: %w", err)
	}

	if progressCb != nil {
		progressCb(Progress{
			Stage:   "finding_friend_links",
			Message: "Searching for friend links",
			Detail:  homepage,
		})
	}

	friendLinks, err := s.findFriendLinksWithProgress(ctx, homepage, progressCb)
	if err != nil {
		return nil, fmt.Errorf("failed to find friend links: %w", err)
	}

	if len(friendLinks) == 0 {
		return []DiscoveredBlog{}, nil
	}

	if progressCb != nil {
		progressCb(Progress{
			Stage:   "checking_rss",
			Message: "Checking RSS feeds",
			Total:   len(friendLinks),
		})
	}

	discovered := s.discoverRSSFeedsWithProgress(ctx, friendLinks, progressCb)

	return discovered, nil
}

func (s *Service) getFeedHomepage(ctx context.Context, feedURL string) (string, error) {
	feed, err := s.feedParser.ParseURLWithContext(feedURL, ctx)
	if err != nil {
		return "", err
	}

	if feed.Link != "" {
		return feed.Link, nil
	}

	u, err := url.Parse(feedURL)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), nil
}

func (s *Service) fetchHTML(ctx context.Context, urlStr string) (*goquery.Document, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = err
			if httputil.IsNetworkError(err.Error()) && attempt < 2 {
				log.Printf("[Discovery] Network error on attempt %d/3 for %s, retrying: %v", attempt+1, urlStr, err)
				backoff := time.Duration(attempt+1) * time.Second
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
				}
				continue
			}
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 && attempt < 2 {
			lastErr = fmt.Errorf("HTTP error: %d", resp.StatusCode)
			log.Printf("[Discovery] Server error on attempt %d/3 for %s, retrying", attempt+1, urlStr)
			backoff := time.Duration(attempt+1) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			return nil, err
		}

		return doc, nil
	}

	return nil, lastErr
}

var _ = log.Println
