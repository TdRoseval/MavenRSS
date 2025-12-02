package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"MrRSS/internal/summary"
)

// HandleSummarizeArticle generates a summary for an article's content.
func (h *Handler) HandleSummarizeArticle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ArticleID int64  `json:"article_id"`
		Length    string `json:"length"` // "short", "medium", "long"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate length parameter
	summaryLength := summary.Medium
	switch req.Length {
	case "short":
		summaryLength = summary.Short
	case "long":
		summaryLength = summary.Long
	case "medium", "":
		summaryLength = summary.Medium
	default:
		http.Error(w, "Invalid length parameter. Use 'short', 'medium', or 'long'", http.StatusBadRequest)
		return
	}

	// Get the article content
	content, err := h.getArticleContent(req.ArticleID)
	if err != nil {
		log.Printf("Error getting article content for summary: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if content == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"summary":     "",
			"key_points":  []string{},
			"is_too_short": true,
			"error":       "No content available for this article",
		})
		return
	}

	// Generate summary
	summarizer := summary.NewSummarizer()
	result := summarizer.Summarize(content, summaryLength)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"summary":        result.Summary,
		"key_points":     result.KeyPoints,
		"sentence_count": result.SentenceCount,
		"is_too_short":   result.IsTooShort,
	})
}

// getArticleContent fetches the content of an article by ID
func (h *Handler) getArticleContent(articleID int64) (string, error) {
	// Get show_hidden_articles setting to include all articles
	allArticles, err := h.DB.GetArticles("", 0, "", true, 50000, 0)
	if err != nil {
		return "", err
	}

	var article *struct {
		FeedID int64
		URL    string
	}

	for _, a := range allArticles {
		if a.ID == articleID {
			article = &struct {
				FeedID int64
				URL    string
			}{
				FeedID: a.FeedID,
				URL:    a.URL,
			}
			break
		}
	}

	if article == nil {
		return "", nil
	}

	// Get the feed URL
	feeds, err := h.DB.GetFeeds()
	if err != nil {
		return "", err
	}

	var feedURL string
	for _, f := range feeds {
		if f.ID == article.FeedID {
			feedURL = f.URL
			break
		}
	}

	if feedURL == "" {
		return "", nil
	}

	// Parse the feed to get fresh content
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	parsedFeed, err := h.Fetcher.ParseFeed(ctx, feedURL)
	if err != nil {
		return "", err
	}

	// Find the article in the feed by URL
	for _, item := range parsedFeed.Items {
		if item.Link == article.URL {
			if item.Content != "" {
				return item.Content, nil
			}
			return item.Description, nil
		}
	}

	return "", nil
}
