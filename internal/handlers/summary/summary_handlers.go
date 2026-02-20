package summary

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"MrRSS/internal/ai"
	"MrRSS/internal/handlers/core"
	"MrRSS/internal/handlers/response"
	"MrRSS/internal/summary"
	"MrRSS/internal/utils/textutil"
)

// isAILimitReached checks if the AI usage limit is reached for a specific user
func isAILimitReached(h *core.Handler, userID int64) bool {
	usageStr, err := h.DB.GetSettingWithFallback(userID, "ai_usage_tokens")
	if err != nil {
		return false
	}
	usage, _ := strconv.ParseInt(usageStr, 10, 64)

	userLimitStr, _ := h.DB.GetSettingWithFallback(userID, "ai_usage_limit")
	hardLimitStr, _ := h.DB.GetSettingWithFallback(userID, "ai_usage_hard_limit")

	userLimit, _ := strconv.ParseInt(userLimitStr, 10, 64)
	hardLimit, _ := strconv.ParseInt(hardLimitStr, 10, 64)

	effectiveLimit := int64(0)
	if userLimit > 0 && hardLimit > 0 {
		effectiveLimit = min(userLimit, hardLimit)
	} else if userLimit > 0 {
		effectiveLimit = userLimit
	} else if hardLimit > 0 {
		effectiveLimit = hardLimit
	}

	if effectiveLimit == 0 {
		return false
	}

	return usage >= effectiveLimit
}

// addAIUsage adds tokens to the AI usage counter for a specific user
func addAIUsage(h *core.Handler, userID int64, tokens int64) {
	usageStr, _ := h.DB.GetSettingWithFallback(userID, "ai_usage_tokens")
	currentUsage, _ := strconv.ParseInt(usageStr, 10, 64)
	newUsage := currentUsage + tokens
	if userID > 0 {
		h.DB.SetSettingForUser(userID, "ai_usage_tokens", strconv.FormatInt(newUsage, 10))
	} else {
		h.DB.SetSetting("ai_usage_tokens", strconv.FormatInt(newUsage, 10))
	}
}

// HandleSummarizeArticle generates a summary for an article's content.
// @Summary      Summarize article
// @Description  Generate a summary for an article's content (uses local algorithm or AI based on settings)
// @Tags         summary
// @Accept       json
// @Produce      json
// @Param        request  body      object  true  "Summarize request (article_id, length, content)"
// @Success      200  {object}  map[string]interface{}  "Summary result (summary, html, sentence_count, is_too_short, cached, limit_reached, thinking)"
// @Failure      400  {object}  map[string]string  "Bad request (invalid length parameter)"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /summarize [post]
func HandleSummarizeArticle(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)

	var req struct {
		ArticleID int64  `json:"article_id"`
		Length    string `json:"length"`            // "short", "medium", "long"
		Content   string `json:"content,omitempty"` // Optional: use provided content instead of fetching from DB
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
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
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	// Check if article already has a cached summary in database
	// If content is provided (for on-the-fly summarization), skip this check
	if req.Content == "" {
		article, err := h.DB.GetArticleByID(req.ArticleID)
		if err == nil && article.Summary != "" && article.Summary != "<no content>" {
			// Article has a cached summary, convert it to HTML and return
			htmlSummary := textutil.ConvertMarkdownToHTML(article.Summary)
			response.JSON(w, map[string]interface{}{
				"summary":        article.Summary,
				"html":           htmlSummary,
				"sentence_count": 0, // We don't store this in DB
				"is_too_short":   false,
				"cached":         true,
			})
			return
		}
	}

	// Get the article content
	content, err := getArticleContent(h, req.ArticleID, req.Content)
	if err != nil {
		log.Printf("Error getting article content for summary: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	if content == "" {
		response.JSON(w, map[string]interface{}{
			"summary":      "",
			"is_too_short": true,
			"error":        "No content available for this article",
		})
		return
	}

	// Get summary provider from settings (with default)
	provider, err := h.DB.GetSettingWithFallback(userID, "summary_provider")
	if err != nil || provider == "" {
		provider = "local" // Default to local algorithm
	}

	var result summary.SummaryResult
	usedFallback := false
	limitReached := false

	if provider == "ai" {
		// Check if AI usage limit is reached - fallback to local if so
		if isAILimitReached(h, userID) {
			log.Printf("AI usage limit reached, falling back to local summarization")
			limitReached = true
			summarizer := summary.NewSummarizer()
			result = summarizer.Summarize(content, summaryLength)
			usedFallback = true
		} else {
			// Use AI summarization
			// Apply rate limiting for AI requests
			h.AITracker.WaitForRateLimit()

			// Try to get AI config from ProfileProvider first
			var apiKey, endpoint, model string
			var useGlobalProxy bool = true
			if h.AIProfileProvider != nil {
				cfg, err := h.AIProfileProvider.GetConfigForFeature(ai.FeatureSummary)
				if err == nil && cfg != nil {
					apiKey = cfg.APIKey
					endpoint = cfg.Endpoint
					model = cfg.Model
					useGlobalProxy = h.AIProfileProvider.UseGlobalProxyForFeature(ai.FeatureSummary)
					log.Printf("Using AI profile for summarization (endpoint: %s, model: %s, useGlobalProxy: %v)", endpoint, model, useGlobalProxy)
				}
			}

			// Fallback to global settings if ProfileProvider not available or no profile configured
			if apiKey == "" && endpoint == "" {
				apiKey, _ = h.DB.GetEncryptedSettingWithFallback(userID, "ai_api_key")
				endpoint, _ = h.DB.GetSettingWithFallback(userID, "ai_endpoint")
				model, _ = h.DB.GetSettingWithFallback(userID, "ai_model")
				// Use global proxy by default for global settings
				useGlobalProxy = true
				log.Printf("Using global AI settings for summarization (API key: %s)", func() string {
					if apiKey != "" {
						return "configured"
					}
					return "not configured (using keyless provider)"
				}())
			}

			systemPrompt, _ := h.DB.GetSettingWithFallback(userID, "ai_summary_prompt")
			customHeaders, _ := h.DB.GetSettingWithFallback(userID, "ai_custom_headers")
			language, _ := h.DB.GetSettingWithFallback(userID, "language")

			aiSummarizer := summary.NewAISummarizerWithDB(apiKey, endpoint, model, h.DB, useGlobalProxy)
			if systemPrompt != "" {
				aiSummarizer.SetSystemPrompt(systemPrompt)
			}
			if customHeaders != "" {
				aiSummarizer.SetCustomHeaders(customHeaders)
			}
			if language != "" {
				aiSummarizer.SetLanguage(language)
			}
			aiResult, err := aiSummarizer.Summarize(content, summaryLength)
			if err != nil {
				log.Printf("Error generating AI summary, falling back to local: %v", err)
				// Fallback to local algorithm on any AI error
				summarizer := summary.NewSummarizer()
				result = summarizer.Summarize(content, summaryLength)
				usedFallback = true
			} else {
				result = aiResult
				// Track AI usage only on success
				inputTokens := ai.EstimateTokens(content)
				outputTokens := ai.EstimateTokens(result.Summary)
				totalTokens := inputTokens + outputTokens
				addAIUsage(h, userID, totalTokens)
				// Track statistics
				_ = h.DB.IncrementStat("ai_summary")
			}
		}
	} else {
		// Use local algorithm
		summarizer := summary.NewSummarizer()
		result = summarizer.Summarize(content, summaryLength)
	}

	// Cache the summary in the database
	if err := h.DB.UpdateArticleSummary(req.ArticleID, result.Summary); err != nil {
		log.Printf("Failed to cache summary for article %d: %v", req.ArticleID, err)
		// Don't fail the request if caching fails
	}

	// Convert markdown summary to HTML (for all summaries, not just AI)
	htmlSummary := textutil.ConvertMarkdownToHTML(result.Summary)

	resp := map[string]interface{}{
		"summary":        result.Summary,
		"html":           htmlSummary,
		"sentence_count": result.SentenceCount,
		"is_too_short":   result.IsTooShort,
		"limit_reached":  limitReached,
		"thinking":       result.Thinking,
	}
	if usedFallback {
		resp["used_fallback"] = true
	}

	response.JSON(w, resp)
}

// getArticleContent fetches the content of an article by ID, or uses provided content
func getArticleContent(h *core.Handler, articleID int64, providedContent string) (string, error) {
	// If content is provided, use it directly
	if providedContent != "" {
		return providedContent, nil
	}

	// Otherwise, fetch from database/cache
	content, _, err := h.GetArticleContent(articleID)
	return content, err
}

// HandleClearSummaries clears all cached summaries from the database.
// @Summary      Clear all summaries
// @Description  Clear all cached article summaries from the database
// @Tags         summary
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]bool  "Success status"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /summaries/clear [delete]
func HandleClearSummaries(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	if err := h.DB.ClearAllSummaries(); err != nil {
		log.Printf("Error clearing summaries: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]interface{}{"success": true})
}
