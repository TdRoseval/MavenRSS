package feed

import (
	"log"
	"runtime"
	"strconv"
	"sync"

	"MavenRSS/internal/ai"
	"MavenRSS/internal/store/sqlite"
	"MavenRSS/internal/summary"
	"MavenRSS/internal/translation"
)

// AIEnhancedTask represents a task for AI enhanced mode processing
type AIEnhancedTask struct {
	ArticleID         int64
	UserID            int64
	FeedID            int64
	NeedsSummary      bool
	NeedsTranslation  bool
	TranslateArticles bool
}

// AIEnhancedManager manages the AI enhanced mode task queue and workers
type AIEnhancedManager struct {
	db          *sqlite.DB
	taskChan    chan *AIEnhancedTask
	workerWg    sync.WaitGroup
	workerCount int
}

// NewAIEnhancedManager creates a new AI enhanced mode manager
func NewAIEnhancedManager(db *sqlite.DB) *AIEnhancedManager {
	// Calculate optimal worker count
	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}
	if numWorkers > 2 {
		numWorkers = 2 // Cap at 2 workers for AI tasks to avoid overload
	}

	taskChan := make(chan *AIEnhancedTask, 100)

	manager := &AIEnhancedManager{
		db:          db,
		taskChan:    taskChan,
		workerCount: numWorkers,
	}

	log.Printf("Starting %d AI enhanced mode workers", numWorkers)
	manager.startWorkers()

	return manager
}

// startWorkers starts the AI enhanced mode worker pool
func (m *AIEnhancedManager) startWorkers() {
	for i := 0; i < m.workerCount; i++ {
		m.workerWg.Add(1)
		go m.worker(i)
	}
}

// worker processes AI enhanced tasks
func (m *AIEnhancedManager) worker(workerID int) {
	defer m.workerWg.Done()
	log.Printf("AI enhanced mode worker %d started", workerID)

	for task := range m.taskChan {
		m.processTask(task, workerID)
	}

	log.Printf("AI enhanced mode worker %d stopped (channel closed)", workerID)
}

// processTask processes a single AI enhanced task
func (m *AIEnhancedManager) processTask(task *AIEnhancedTask, workerID int) {
	log.Printf("AI enhanced worker %d processing article %d for user %d", workerID, task.ArticleID, task.UserID)

	// Get article content
	content, hasContent, err := m.db.GetArticleContent(task.ArticleID)
	if err != nil {
		log.Printf("Error getting article content for AI enhanced task: %v", err)
		return
	}
	if !hasContent || content == "" {
		log.Printf("No content available for article %d, skipping AI processing", task.ArticleID)
		return
	}

	// Generate AI summary if needed
	if task.NeedsSummary {
		m.generateAISummary(task, content)
	}

	// Generate AI translation if needed
	if task.NeedsTranslation && task.TranslateArticles {
		m.generateAITranslation(task, content)
	}
}

// generateAISummary generates and saves AI summary for an article
func (m *AIEnhancedManager) generateAISummary(task *AIEnhancedTask, content string) {
	// Check if AI usage limit is reached
	if m.isAILimitReached(task.UserID) {
		log.Printf("AI usage limit reached for user %d, skipping summary", task.UserID)
		return
	}

	config, err := getUserFeatureAIConfig(m.db, task.UserID, ai.FeatureSummary)
	if err != nil {
		log.Printf("Error resolving AI summary config for user %d: %v", task.UserID, err)
		return
	}
	if !hasConfiguredAPIKey(config) {
		log.Printf("No user-level AI summary config available for user %d, skipping summary", task.UserID)
		return
	}

	systemPrompt, _ := m.db.GetSettingForUser(task.UserID, "ai_summary_prompt")
	summaryLengthStr, _ := m.db.GetSettingWithFallback(task.UserID, "summary_length")

	// Set summary length
	summaryLength := summary.Medium
	switch summaryLengthStr {
	case "short":
		summaryLength = summary.Short
	case "long":
		summaryLength = summary.Long
	}

	// Create AI summarizer with Chinese language
	aiSummarizer := summary.NewAISummarizerWithDB(config.APIKey, config.Endpoint, config.Model, m.db, true)
	if systemPrompt != "" {
		aiSummarizer.SetSystemPrompt(systemPrompt)
	}
	if config.CustomHeaders != "" {
		aiSummarizer.SetCustomHeaders(config.CustomHeaders)
	}
	aiSummarizer.SetLanguage("zh") // Force Chinese for AI enhanced mode

	// Generate summary
	result, err := aiSummarizer.Summarize(content, summaryLength)
	if err != nil {
		log.Printf("Error generating AI summary for article %d: %v", task.ArticleID, err)
		return
	}

	// Track AI usage
	inputTokens := ai.EstimateTokens(content)
	outputTokens := ai.EstimateTokens(result.Summary)
	totalTokens := inputTokens + outputTokens
	m.addAIUsage(task.UserID, totalTokens)

	// Cache the summary in the database
	if err := m.db.UpdateArticleSummary(task.ArticleID, result.Summary); err != nil {
		log.Printf("Failed to cache summary for article %d: %v", task.ArticleID, err)
	} else {
		log.Printf("Successfully cached AI summary for article %d", task.ArticleID)
	}
}

// generateAITranslation generates and saves AI translation for an article
func (m *AIEnhancedManager) generateAITranslation(task *AIEnhancedTask, content string) {
	// Check if AI usage limit is reached
	if m.isAILimitReached(task.UserID) {
		log.Printf("AI usage limit reached for user %d, skipping translation", task.UserID)
		return
	}

	config, err := getUserFeatureAIConfig(m.db, task.UserID, ai.FeatureTranslation)
	if err != nil {
		log.Printf("Error resolving AI translation config for user %d: %v", task.UserID, err)
		return
	}
	if !hasConfiguredAPIKey(config) {
		log.Printf("No user-level AI translation config available for user %d, skipping translation", task.UserID)
		return
	}

	systemPrompt, _ := m.db.GetSettingForUser(task.UserID, "ai_translation_prompt")
	targetLang, _ := m.db.GetSettingWithFallback(task.UserID, "target_language")
	if targetLang == "" {
		targetLang = "zh"
	}

	// Create AI translator
	aiTranslator := translation.NewAITranslatorWithDB(config.APIKey, config.Endpoint, config.Model, m.db, true)
	if systemPrompt != "" {
		aiTranslator.SetSystemPrompt(systemPrompt)
	}
	if config.CustomHeaders != "" {
		aiTranslator.SetCustomHeaders(config.CustomHeaders)
	}

	// Generate translation
	translatedContent, err := aiTranslator.Translate(content, targetLang)
	if err != nil {
		log.Printf("Error generating AI translation for article %d: %v", task.ArticleID, err)
		return
	}

	// Track AI usage
	inputTokens := ai.EstimateTokens(content)
	outputTokens := ai.EstimateTokens(translatedContent)
	totalTokens := inputTokens + outputTokens
	m.addAIUsage(task.UserID, totalTokens)

	// Cache the translation in the database
	if err := m.db.SetArticleTranslatedContent(task.ArticleID, translatedContent, targetLang, "ai"); err != nil {
		log.Printf("Failed to cache translation for article %d: %v", task.ArticleID, err)
	} else {
		log.Printf("Successfully cached AI translation for article %d", task.ArticleID)
	}
}

// isAILimitReached checks if the AI usage limit is reached for a specific user
func (m *AIEnhancedManager) isAILimitReached(userID int64) bool {
	usageStr, err := m.db.GetSettingWithFallback(userID, "ai_usage_tokens")
	if err != nil {
		return false
	}
	usage, _ := strconv.ParseInt(usageStr, 10, 64)

	userLimitStr, _ := m.db.GetSettingWithFallback(userID, "ai_usage_limit")
	hardLimitStr, _ := m.db.GetSettingWithFallback(userID, "ai_usage_hard_limit")

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

func getUserFeatureAIConfig(db *sqlite.DB, userID int64, feature ai.FeatureType) (*ai.ClientConfig, error) {
	if userID <= 0 {
		return nil, nil
	}
	return ai.NewProfileProvider(db).GetConfigForFeatureForUser(userID, feature)
}

func hasConfiguredAPIKey(config *ai.ClientConfig) bool {
	return config != nil && config.APIKey != ""
}

// addAIUsage adds tokens to the AI usage counter for a specific user
func (m *AIEnhancedManager) addAIUsage(userID int64, tokens int64) {
	usageStr, _ := m.db.GetSettingWithFallback(userID, "ai_usage_tokens")
	currentUsage, _ := strconv.ParseInt(usageStr, 10, 64)
	newUsage := currentUsage + tokens
	if userID > 0 {
		m.db.SetSettingForUser(userID, "ai_usage_tokens", strconv.FormatInt(newUsage, 10))
	} else {
		m.db.SetSetting("ai_usage_tokens", strconv.FormatInt(newUsage, 10))
	}
}

// AddTask adds a task to the AI enhanced mode queue
func (m *AIEnhancedManager) AddTask(task *AIEnhancedTask) {
	select {
	case m.taskChan <- task:
		log.Printf("Added AI enhanced task for article %d", task.ArticleID)
	default:
		log.Printf("AI enhanced task queue full, dropped task for article %d", task.ArticleID)
	}
}

// Stop stops the AI enhanced mode manager and cleans up resources
func (m *AIEnhancedManager) Stop() {
	log.Println("Stopping AI enhanced mode manager...")
	close(m.taskChan)
	m.workerWg.Wait()
	log.Println("AI enhanced mode manager stopped")
}

// ShouldProcess checks if AI enhanced mode should process an article for a user
func ShouldProcess(db *sqlite.DB, userID int64) bool {
	// Check if AI enhanced mode is enabled
	enhancedModeStr, _ := db.GetSettingWithFallback(userID, "ai_enhanced_mode")
	if enhancedModeStr != "true" {
		return false
	}

	summaryConfig, err := getUserFeatureAIConfig(db, userID, ai.FeatureSummary)
	if err != nil || !hasConfiguredAPIKey(summaryConfig) {
		return false
	}

	translationConfig, err := getUserFeatureAIConfig(db, userID, ai.FeatureTranslation)
	if err != nil || !hasConfiguredAPIKey(translationConfig) {
		return false
	}

	searchConfig, err := getUserFeatureAIConfig(db, userID, ai.FeatureSearch)
	if err != nil || !hasConfiguredAPIKey(searchConfig) {
		return false
	}

	chatConfig, err := getUserFeatureAIConfig(db, userID, ai.FeatureChat)
	if err != nil || !hasConfiguredAPIKey(chatConfig) {
		return false
	}

	// Check if summary is enabled and using AI
	summaryEnabled, _ := db.GetSettingWithFallback(userID, "summary_enabled")
	summaryProvider, _ := db.GetSettingWithFallback(userID, "summary_provider")
	if summaryEnabled != "true" || summaryProvider != "ai" {
		return false
	}

	// Check if translation is enabled
	translationEnabled, _ := db.GetSettingWithFallback(userID, "translation_enabled")
	if translationEnabled != "true" {
		return false
	}

	// Check if AI search is enabled
	aiSearchEnabled, _ := db.GetSettingWithFallback(userID, "ai_search_enabled")
	if aiSearchEnabled != "true" {
		return false
	}

	// Check if AI chat is enabled
	aiChatEnabled, _ := db.GetSettingWithFallback(userID, "ai_chat_enabled")
	if aiChatEnabled != "true" {
		return false
	}

	return true
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
