package translation

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"MavenRSS/internal/ai"
	"MavenRSS/internal/handlers/core"
	"MavenRSS/internal/handlers/response"
	"MavenRSS/internal/translation"
	"MavenRSS/internal/utils/textutil"
)

// TestCustomTranslationRequest represents a request to test custom translation configuration
type TestCustomTranslationRequest struct {
	Text   string                             `json:"text" example:"Hello, world!"`
	Target string                             `json:"target_lang" example:"zh"`
	Config translation.CustomTranslatorConfig `json:"config"`
}

// TestCustomTranslationResponse represents the response from a custom translation test
type TestCustomTranslationResponse struct {
	Success bool   `json:"success"`
	Result  string `json:"result,omitempty"`
	Error   string `json:"error,omitempty"`
}

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

// Helper to get AI translator with user-specific settings
func getAITranslatorForUser(h *core.Handler, userID int64) (*translation.AITranslator, error) {
	log.Printf("[getAITranslatorForUser] Starting for user %d", userID)
	// First, try to use AI profile provider if available
	if h.AIProfileProvider != nil {
		log.Printf("[getAITranslatorForUser] AIProfileProvider available, trying to get config for translation feature")
		cfg, err := h.AIProfileProvider.GetConfigForFeature(ai.FeatureTranslation)
		if err == nil && cfg != nil {
			log.Printf("[getAITranslatorForUser] Got AI profile config, endpoint: %s, model: %s", cfg.Endpoint, cfg.Model)
			// Get system prompt from settings
			systemPrompt, _ := h.DB.GetSettingWithFallback(userID, "ai_translation_prompt")
			translator := translation.NewAITranslatorWithDB(cfg.APIKey, cfg.Endpoint, cfg.Model, h.DB)
			if systemPrompt != "" {
				log.Printf("[getAITranslatorForUser] Setting system prompt")
				translator.SetSystemPrompt(systemPrompt)
			}
			if cfg.CustomHeaders != "" {
				log.Printf("[getAITranslatorForUser] Setting custom headers")
				translator.SetCustomHeaders(cfg.CustomHeaders)
			}
			return translator, nil
		} else {
			log.Printf("[getAITranslatorForUser] No AI profile config found (err: %v, cfg: %v), falling back to legacy settings", err, cfg)
		}
	} else {
		log.Printf("[getAITranslatorForUser] AIProfileProvider not available, falling back to legacy settings")
	}

	// Fallback to legacy settings
	apiKey, _ := h.DB.GetEncryptedSettingWithFallback(userID, "ai_api_key")
	endpoint, _ := h.DB.GetSettingWithFallback(userID, "ai_endpoint")
	model, _ := h.DB.GetSettingWithFallback(userID, "ai_model")
	systemPrompt, _ := h.DB.GetSettingWithFallback(userID, "ai_translation_prompt")
	customHeaders, _ := h.DB.GetSettingWithFallback(userID, "ai_custom_headers")

	log.Printf("[getAITranslatorForUser] Legacy settings - endpoint: %s, model: %s, has API key: %v", endpoint, model, apiKey != "")
	translator := translation.NewAITranslatorWithDB(apiKey, endpoint, model, h.DB)
	if systemPrompt != "" {
		log.Printf("[getAITranslatorForUser] Setting system prompt from legacy settings")
		translator.SetSystemPrompt(systemPrompt)
	}
	if customHeaders != "" {
		log.Printf("[getAITranslatorForUser] Setting custom headers from legacy settings")
		translator.SetCustomHeaders(customHeaders)
	}
	return translator, nil
}

// HandleTranslateArticle translates an article's title.
// @Summary      Translate article title
// @Description  Translate an article's title to the target language (uses AI or Google based on settings)
// @Tags         translation
// @Accept       json
// @Produce      json
// @Param        request  body      object  true  "Translation request (article_id, title, target_language)"
// @Success      200  {object}  map[string]interface{}  "Translation result (translated_title, limit_reached)"
// @Failure      400  {object}  map[string]string  "Bad request (missing required fields)"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /translate/article [post]
func HandleTranslateArticle(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)

	var req struct {
		ArticleID  int64  `json:"article_id"`
		Title      string `json:"title"`
		TargetLang string `json:"target_language"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	if req.Title == "" || req.TargetLang == "" {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	// Step 0: Check if article already has a translation in database
	// This prevents re-translating already translated content
	article, err := h.DB.GetArticleByID(req.ArticleID)
	if err == nil && article != nil {
		if article.TranslatedTitle != "" && article.TranslatedTitle != article.Title {
			// Translation already exists and is different from original
			response.JSON(w, map[string]interface{}{
				"translated_title": article.TranslatedTitle,
				"limit_reached":    false,
				"skipped":          true, // Indicate translation was skipped (from cache)
				"cached":           true,
			})
			return
		}
	}

	// Step 1: Pre-translation language detection to avoid unnecessary API calls
	detector := translation.GetLanguageDetector()
	shouldTranslate := detector.ShouldTranslate(req.Title, req.TargetLang)

	if !shouldTranslate {
		// Text is already in target language, return original title
		if updateErr := h.DB.UpdateArticleTranslation(req.ArticleID, req.Title); updateErr != nil {
			response.Error(w, updateErr, http.StatusInternalServerError)
			return
		}
		response.JSON(w, map[string]interface{}{
			"translated_title": req.Title,
			"limit_reached":    false,
			"skipped":          true, // Indicate translation was skipped
			"reason":           "already_target_language",
		})
		return
	}

	// Step 2: Proceed with translation
	// Check if we should use AI translation or other provider
	provider, _ := h.DB.GetSettingWithFallback(userID, "translation_provider")
	isAIProvider := provider == "ai"

	var translatedTitle string
	var translateErr error
	var limitReached = false

	if isAIProvider {
		// Check if AI usage limit is reached
		if isAILimitReached(h, userID) {
			log.Printf("AI usage limit reached for article translation, falling back to non-AI provider")
			limitReached = true
			// Fall back to non-AI provider gracefully
			translatedTitle, translateErr = translation.TranslateMarkdownPreservingStructure(req.Title, h.Translator, req.TargetLang)
		} else {
			// Apply rate limiting for AI requests
			h.AITracker.WaitForRateLimit()

			// Create AI translator directly with user-specific settings
			aiTranslator, err := getAITranslatorForUser(h, userID)
			if err != nil {
				log.Printf("Failed to create AI translator, falling back to non-AI: %v", err)
				translatedTitle, translateErr = translation.TranslateMarkdownPreservingStructure(req.Title, h.Translator, req.TargetLang)
			} else {
				// Use markdown-preserving translation for better list structure
				translatedTitle, translateErr = translation.TranslateMarkdownAIPrompt(req.Title, aiTranslator, req.TargetLang)

				// If AI fails, fall back to non-AI provider gracefully
				if translateErr != nil {
					log.Printf("AI translation failed, falling back to non-AI: %v", translateErr)
					translatedTitle, translateErr = translation.TranslateMarkdownPreservingStructure(req.Title, h.Translator, req.TargetLang)
				} else {
					// Track AI usage only on success
					inputTokens := ai.EstimateTokens(req.Title)
					outputTokens := ai.EstimateTokens(translatedTitle)
					totalTokens := inputTokens + outputTokens
					addAIUsage(h, userID, totalTokens)
				}
			}
		}
		
		// If even the fallback fails, return error
		if translateErr != nil {
			log.Printf("Translation failed even with fallback: %v", translateErr)
			response.Error(w, translateErr, http.StatusInternalServerError)
			return
		}
	} else {
		// Non-AI provider, use original logic with h.Translator
		translatedTitle, translateErr = translation.TranslateMarkdownPreservingStructure(req.Title, h.Translator, req.TargetLang)
		
		// If translation fails, return error
		if translateErr != nil {
			log.Printf("Translation failed for provider %s: %v", provider, translateErr)
			response.Error(w, translateErr, http.StatusInternalServerError)
			return
		}
	}

	// Step 3: Post-translation check - if translation equals original, it was already in target language
	// This provides a safety net in case pre-translation detection was inaccurate
	if translatedTitle == req.Title {
		// Still update DB with the "translated" text (which is the original)
		if updateErr := h.DB.UpdateArticleTranslation(req.ArticleID, translatedTitle); updateErr != nil {
			response.Error(w, updateErr, http.StatusInternalServerError)
			return
		}
		response.JSON(w, map[string]interface{}{
			"translated_title": translatedTitle,
			"limit_reached":    limitReached,
			"skipped":          true, // Indicate no actual translation was performed
			"reason":           "translation_equals_original",
		})
		return
	}

	// Update the article with the translated title
	if updateErr := h.DB.UpdateArticleTranslation(req.ArticleID, translatedTitle); updateErr != nil {
		response.Error(w, updateErr, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]interface{}{
		"translated_title": translatedTitle,
		"limit_reached":    limitReached,
		"skipped":          false, // Translation was performed
	})
}

// HandleClearTranslations clears all translated titles from the database.
// @Summary      Clear all translations
// @Description  Clear all translated article titles from the database
// @Tags         translation
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]bool  "Success status"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /translations/clear [post]
func HandleClearTranslations(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	if err := h.DB.ClearAllTranslations(); err != nil {
		log.Printf("Error clearing translations: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]bool{"success": true})
}

// HandleTranslateText translates any text to the target language.
// This is used for translating content, summaries, etc.
// @Summary      Translate text
// @Description  Translate any text to the target language (uses AI or Google based on settings)
// @Tags         translation
// @Accept       json
// @Produce      json
// @Param        request  body      object  true  "Translation request (text, target_language)"
// @Success      200  {object}  map[string]string  "Translation result (translated_text, html)"
// @Failure      400  {object}  map[string]string  "Bad request (missing required fields)"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /translate/text [post]
func HandleTranslateText(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)
	log.Printf("[TranslateText] Starting translation for user %d", userID)

	var req struct {
		Text       string `json:"text"`
		TargetLang string `json:"target_language"`
		Force      bool   `json:"force"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[TranslateText] Error decoding translation request: %v", err)
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	if req.Text == "" || req.TargetLang == "" {
		log.Printf("[TranslateText] Missing required fields in translation request")
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	log.Printf("[TranslateText] Text length: %d, TargetLang: %s, Force: %v", len(req.Text), req.TargetLang, req.Force)

	// Step 1: Pre-translation language detection to avoid unnecessary API calls
	detector := translation.GetLanguageDetector()
	// Use full-text analysis for better accuracy on longer content
	// Skip language detection if force flag is set
	shouldTranslate := req.Force || detector.ShouldTranslateFullText(req.Text, req.TargetLang)
	log.Printf("[TranslateText] shouldTranslate: %v", shouldTranslate)

	if !shouldTranslate {
		// Text is already in target language, return original text
		htmlText := textutil.ConvertMarkdownToHTML(req.Text)
		log.Printf("[TranslateText] Skipping translation (already in target language)")
		response.JSON(w, map[string]interface{}{
			"translated_text": req.Text,
			"html":            htmlText,
			"skipped":         "true", // Indicate translation was skipped
			"reason":          "already_target_language",
		})
		return
	}

	// Step 2: Proceed with translation
	// Check if we should use AI translation or other provider
	provider, _ := h.DB.GetSettingWithFallback(userID, "translation_provider")
	isAIProvider := provider == "ai"
	log.Printf("[TranslateText] provider: %s, isAIProvider: %v", provider, isAIProvider)

	var translatedText string
	var err error

	if isAIProvider {
		// Check if AI usage limit is reached
		if isAILimitReached(h, userID) {
			log.Printf("[TranslateText] AI usage limit reached, falling back to non-AI provider")
			// Fall back to non-AI provider gracefully
			translatedText, err = translation.TranslateMarkdownPreservingStructure(req.Text, h.Translator, req.TargetLang)
		} else {
			// Apply rate limiting for AI requests
			h.AITracker.WaitForRateLimit()

			// Create AI translator directly with user-specific settings
			log.Printf("[TranslateText] Creating AI translator for user %d", userID)
			aiTranslator, translatorErr := getAITranslatorForUser(h, userID)
			if translatorErr != nil {
				log.Printf("[TranslateText] Failed to create AI translator, falling back to non-AI: %v", translatorErr)
				translatedText, err = translation.TranslateMarkdownPreservingStructure(req.Text, h.Translator, req.TargetLang)
			} else {
				log.Printf("[TranslateText] AI translator created successfully, endpoint: %s, model: %s", aiTranslator.Endpoint, aiTranslator.Model)

				// Use markdown-preserving translation for better list structure
				log.Printf("[TranslateText] Starting AI translation...")
				translatedText, err = translation.TranslateMarkdownAIPrompt(req.Text, aiTranslator, req.TargetLang)

				// If AI fails, fall back to non-AI provider gracefully
				if err != nil {
					log.Printf("[TranslateText] AI translation failed, falling back to non-AI: %v", err)
					translatedText, err = translation.TranslateMarkdownPreservingStructure(req.Text, h.Translator, req.TargetLang)
				} else {
					log.Printf("[TranslateText] AI translation succeeded, translated length: %d", len(translatedText))
					// Track AI usage only on success
					inputTokens := ai.EstimateTokens(req.Text)
					outputTokens := ai.EstimateTokens(translatedText)
					totalTokens := inputTokens + outputTokens
					addAIUsage(h, userID, totalTokens)
				}
			}
		}
		
		// If even the fallback fails, return error
		if err != nil {
			log.Printf("[TranslateText] Translation failed even with fallback: %v", err)
			response.Error(w, err, http.StatusInternalServerError)
			return
		}
	} else {
		// Non-AI provider, use original logic with h.Translator
		log.Printf("[TranslateText] Using non-AI provider: %s", provider)
		translatedText, err = translation.TranslateMarkdownPreservingStructure(req.Text, h.Translator, req.TargetLang)
		
		// If translation fails, return error
		if err != nil {
			log.Printf("[TranslateText] Translation failed for provider %s: %v", provider, err)
			response.Error(w, err, http.StatusInternalServerError)
			return
		}
	}

	// Step 3: Post-translation check - if translation equals original, it was already in target language
	// This provides a safety net in case pre-translation detection was inaccurate
	if translatedText == req.Text {
		htmlText := textutil.ConvertMarkdownToHTML(translatedText)
		log.Printf("[TranslateText] Translation equals original, skipping")
		response.JSON(w, map[string]string{
			"translated_text": translatedText,
			"html":            htmlText,
			"skipped":         "true", // Indicate no actual translation was performed
		})
		return
	}

	// Convert translated markdown to HTML
	htmlText := textutil.ConvertMarkdownToHTML(translatedText)

	log.Printf("[TranslateText] Translation completed successfully")
	response.JSON(w, map[string]string{
		"translated_text": translatedText,
		"html":            htmlText,
		"skipped":         "false", // Translation was performed
	})
}

// HandleResetAIUsage resets the AI usage counter.
// @Summary      Reset AI usage counter
// @Description  Reset the AI usage token counter to zero
// @Tags         translation
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]bool  "Success status"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /ai/usage/reset [post]
func HandleResetAIUsage(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	if err := h.AITracker.ResetUsage(); err != nil {
		log.Printf("Error resetting AI usage: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]bool{"success": true})
}

// HandleGetAIUsage returns the current AI usage statistics.
// @Summary      Get AI usage statistics
// @Description  Get current AI usage (tokens used, limit, and whether limit is reached)
// @Tags         translation
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "AI usage stats (usage, limit, limit_reached)"
// @Router       /ai/usage [get]
func HandleGetAIUsage(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	userID, _ := core.GetUserIDFromRequest(r)

	usageStr, _ := h.DB.GetSettingWithFallback(userID, "ai_usage_tokens")
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

	limitReached := isAILimitReached(h, userID)

	response.JSON(w, map[string]interface{}{
		"usage":         usage,
		"limit":         effectiveLimit,
		"limit_reached": limitReached,
	})
}

// EstimateTokens exposes the token estimation function for testing/display.
func EstimateTokens(text string) int64 {
	return ai.EstimateTokens(text)
}

// HandleTestCustomTranslation tests a custom translation configuration.
// @Summary      Test custom translation
// @Description  Test a custom translation API configuration
// @Tags         translation
// @Accept       json
// @Produce      json
// @Param        request  body      TestCustomTranslationRequest  true  "Test request"
// @Success      200  {object}  TestCustomTranslationResponse  "Test result"
// @Failure      400  {object}  map[string]string  "Bad request"
// @Router       /translation/test-custom [post]
func HandleTestCustomTranslation(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Text   string                             `json:"text"`
		Target string                             `json:"target_lang"`
		Config translation.CustomTranslatorConfig `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.Text == "" {
		req.Text = "Hello, world!"
	}
	if req.Target == "" {
		req.Target = "zh"
	}

	// Create custom translator
	customTranslator := translation.NewCustomTranslator(&req.Config)

	// Test translation
	result, err := customTranslator.Translate(req.Text, req.Target)

	resp := map[string]interface{}{
		"success": err == nil,
	}

	if err != nil {
		resp["error"] = err.Error()
		w.WriteHeader(http.StatusBadRequest)
	} else {
		resp["translation"] = result
	}

	response.JSON(w, resp)
}
