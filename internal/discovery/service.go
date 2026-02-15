package discovery

import (
	"net/http"
	"time"

	"MrRSS/internal/utils/httputil"

	"github.com/mmcdole/gofeed"
)

const (
	MaxConcurrentRSSChecks  = 8
	MaxConcurrentPathChecks = 5
)

type ProgressCallback func(progress Progress)

type Progress struct {
	Stage      string `json:"stage"`
	Message    string `json:"message"`
	Detail     string `json:"detail"`
	Current    int    `json:"current"`
	Total      int    `json:"total"`
	FeedName   string `json:"feed_name"`
	FoundCount int    `json:"found_count"`
}

type DiscoveredBlog struct {
	Name           string          `json:"name"`
	Homepage       string          `json:"homepage"`
	RSSFeed        string          `json:"rss_feed"`
	IconURL        string          `json:"icon_url"`
	RecentArticles []RecentArticle `json:"recent_articles"`
}

type RecentArticle struct {
	Title string `json:"title"`
	Date  string `json:"date"`
}

type Service struct {
	client     *http.Client
	feedParser *gofeed.Parser
	proxyURL   string
}

func NewService() *Service {
	return NewServiceWithProxy("")
}

func NewServiceWithProxy(proxyURL string) *Service {
	redirectChecker := func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errTooManyRedirects
		}
		return nil
	}

	httpClient := httputil.GetPooledHTTPClient(proxyURL, httputil.DefaultDiscoveryTimeout)
	httpClient.CheckRedirect = redirectChecker

	feedParser := gofeed.NewParser()
	feedParser.Client = httpClient

	return &Service{
		client:     httpClient,
		feedParser: feedParser,
		proxyURL:   proxyURL,
	}
}

// SetProxy updates the proxy URL for the service.
// This allows dynamic proxy updates without recreating the service.
func (s *Service) SetProxy(proxyURL string) {
	if s.proxyURL == proxyURL {
		return
	}

	s.proxyURL = proxyURL

	redirectChecker := func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errTooManyRedirects
		}
		return nil
	}

	s.client = httputil.GetPooledHTTPClient(proxyURL, httputil.DefaultDiscoveryTimeout)
	s.client.CheckRedirect = redirectChecker
	s.feedParser.Client = s.client
}
