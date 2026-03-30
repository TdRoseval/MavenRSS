package feed

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"MavenRSS/internal/ai"
	"MavenRSS/internal/dedup"
	"MavenRSS/internal/store/sqlite"
	"MavenRSS/internal/summary"
	"MavenRSS/internal/translation"
	"MavenRSS/internal/utils/httputil"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

// AIEnhancedTask represents a task for AI enhanced mode processing
type AIEnhancedTask struct {
	ArticleID         int64
	UserID            int64
	FeedID            int64
	NeedsSummary      bool
	NeedsTranslation  bool
	TranslateArticles bool
	NeedsEmbedding    bool
	NeedsDedup        bool
	NeedsClusterRun   bool
}

// AIEnhancedManager manages the AI enhanced mode task queue and workers
type AIEnhancedManager struct {
	db                     *sqlite.DB
	taskChan               chan *AIEnhancedTask
	workerWg               sync.WaitGroup
	workerCount            int
	clusterMu              sync.Mutex
	clusterPipelineRunning map[int64]bool
	clusterPipelineQueued  map[int64]bool
	resolveFusionConfig    func(userID int64) (*dedup.FusionConfig, error)
	runFusion              func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error
	runEmbedding           func(ctx context.Context, db *sqlite.DB, userID int64, cfg *dedup.FusionConfig) error
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
		db:                     db,
		taskChan:               taskChan,
		workerCount:            numWorkers,
		clusterPipelineRunning: make(map[int64]bool),
		clusterPipelineQueued:  make(map[int64]bool),
	}
	manager.resolveFusionConfig = manager.buildFusionConfig
	manager.runFusion = dedup.RunFusion
	manager.runEmbedding = dedup.RunEmbedding

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

	needsContent := task.NeedsSummary || (task.NeedsTranslation && task.TranslateArticles) || task.NeedsEmbedding
	content := ""
	if needsContent {
		var hasContent bool
		var err error
		content, hasContent, err = m.db.GetArticleContent(task.ArticleID)
		if err != nil {
			log.Printf("Error getting article content for AI enhanced task: %v", err)
			return
		}
		if !hasContent || content == "" {
			log.Printf("No content available for article %d, skipping AI processing", task.ArticleID)
			if task.NeedsClusterRun && !task.NeedsSummary && !task.NeedsEmbedding && !task.NeedsDedup {
				m.scheduleClusterPipeline(task.UserID)
			}
			return
		}
	}

	if task.NeedsSummary {
		m.generateAISummary(task, content)
	}

	if task.NeedsTranslation && task.TranslateArticles {
		m.generateAITranslation(task, content)
	}

	if task.NeedsEmbedding || task.NeedsDedup || task.NeedsClusterRun {
		m.advanceArticlePipelineAsync(task, content)
	}
}

func (m *AIEnhancedManager) advanceArticlePipelineAsync(task *AIEnhancedTask, content string) {
	go func() {
		time.Sleep(2 * time.Second)

		article, err := m.db.GetArticleByID(task.ArticleID)
		if err != nil || article == nil {
			log.Printf("Failed to fetch article %d for AI enhanced pipeline: %v", task.ArticleID, err)
			return
		}

		if task.NeedsEmbedding {
			summaryText := article.Summary
			if summaryText == "" {
				summaryText = content
			}

			configsJSON, err := m.db.GetSettingWithFallback(task.UserID, "ai_embedding_models")
			if err != nil || configsJSON == "" || configsJSON == "[]" {
				log.Printf("No embedding models configured for user %d, skipping embedding", task.UserID)
			} else {
				globalProxyURL, _ := buildGlobalProxyURL(m.db, task.UserID)
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				var titleEmbBlob, summaryEmbBlob []byte

				if article.Title != "" {
					if titleEmb, err := ai.GenerateEmbeddings(ctx, article.Title, configsJSON, globalProxyURL); err == nil {
						if len(titleEmb) > 0 {
							titleEmbBlob, _ = sqlite_vec.SerializeFloat32(titleEmb)
						}
					} else {
						log.Printf("Failed to generate title embedding for article %d: %v", task.ArticleID, err)
					}
				}

				if summaryText != "" {
					if sumEmb, err := ai.GenerateEmbeddings(ctx, summaryText, configsJSON, globalProxyURL); err == nil {
						if len(sumEmb) > 0 {
							summaryEmbBlob, _ = sqlite_vec.SerializeFloat32(sumEmb)
						}
					} else {
						log.Printf("Failed to generate summary embedding for article %d: %v", task.ArticleID, err)
					}
				}

				if len(titleEmbBlob) > 0 || len(summaryEmbBlob) > 0 {
					if err := m.db.UpdateArticleEmbeddings(task.ArticleID, titleEmbBlob, summaryEmbBlob); err != nil {
						log.Printf("Failed to update article %d embeddings: %v", task.ArticleID, err)
					} else {
						log.Printf("Successfully saved embeddings for article %d", task.ArticleID)
					}
				}
			}
		}

		clusterScheduled := false
		if task.NeedsDedup {
			if err := dedup.ProcessArticle(m.db, task.ArticleID, task.UserID); err != nil {
				log.Printf("Dedup pipeline failed for article %d: %v", task.ArticleID, err)
			} else {
				log.Printf("Dedup pipeline completed for article %d", task.ArticleID)
				m.scheduleClusterPipeline(task.UserID)
				clusterScheduled = true
			}
		}

		if task.NeedsClusterRun && !clusterScheduled {
			m.scheduleClusterPipeline(task.UserID)
		}
	}()
}

func (m *AIEnhancedManager) scheduleClusterPipeline(userID int64) {
	m.clusterMu.Lock()
	if m.clusterPipelineRunning[userID] {
		m.clusterPipelineQueued[userID] = true
		m.clusterMu.Unlock()
		return
	}
	m.clusterPipelineRunning[userID] = true
	m.clusterMu.Unlock()

	go m.runClusterPipeline(userID)
}

func (m *AIEnhancedManager) runClusterPipeline(userID int64) {
	for {
		if err := m.runClusterPipelineOnce(userID); err != nil {
			log.Printf("Cluster pipeline failed for user %d: %v", userID, err)
		}

		m.clusterMu.Lock()
		if m.clusterPipelineQueued[userID] {
			m.clusterPipelineQueued[userID] = false
			m.clusterMu.Unlock()
			continue
		}
		delete(m.clusterPipelineRunning, userID)
		delete(m.clusterPipelineQueued, userID)
		m.clusterMu.Unlock()
		return
	}
}

func (m *AIEnhancedManager) runClusterPipelineOnce(userID int64) error {
	cfg, err := m.resolveFusionConfig(userID)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := m.runFusion(ctx, m.db, userID, cfg); err != nil {
		return fmt.Errorf("run fusion: %w", err)
	}
	if err := m.runEmbedding(ctx, m.db, userID, cfg); err != nil {
		return fmt.Errorf("run embedding: %w", err)
	}

	return nil
}

func (m *AIEnhancedManager) buildFusionConfig(userID int64) (*dedup.FusionConfig, error) {
	config, err := getUserFeatureAIConfig(m.db, userID, ai.FeatureFusion)
	if err != nil {
		return nil, fmt.Errorf("resolve fusion ai config: %w", err)
	}
	if !hasConfiguredAPIKey(config) {
		return nil, fmt.Errorf("missing user-level fusion ai config")
	}

	embeddingConfigsJSON, err := m.db.GetSettingWithFallback(userID, "ai_embedding_models")
	if err != nil {
		return nil, fmt.Errorf("load embedding models: %w", err)
	}

	globalProxyURL, err := buildGlobalProxyURL(m.db, userID)
	if err != nil {
		return nil, fmt.Errorf("load proxy settings: %w", err)
	}
	useGlobalProxy := ai.NewProfileProvider(m.db).UseGlobalProxyForFeatureForUser(userID, ai.FeatureFusion)

	aiSummarizer := summary.NewAISummarizerWithDB(config.APIKey, config.Endpoint, config.Model, m.db, useGlobalProxy)
	if config.CustomHeaders != "" {
		aiSummarizer.SetCustomHeaders(config.CustomHeaders)
	}
	aiSummarizer.SetLanguage("zh")

	return &dedup.FusionConfig{
		Summarizer:     aiSummarizer,
		EmbConfigsJSON: embeddingConfigsJSON,
		GlobalProxyURL: globalProxyURL,
	}, nil
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

	if result.IsTooShort && utf8.RuneCountInString(strings.TrimSpace(result.Summary)) < 10 {
		article, articleErr := m.db.GetArticleByIDForUser(task.UserID, task.ArticleID)
		if articleErr != nil {
			log.Printf("Failed to load article title for short summary fallback, article %d: %v", task.ArticleID, articleErr)
		} else if article != nil {
			title := strings.TrimSpace(article.Title)
			if title != "" {
				result.Summary = title
			}
		}
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

// BatchProcessExistingArticles scans existing articles for a user and queues them for AI processing.
// This is triggered when AI Enhanced Mode is first activated.
// It processes: all favorited articles + unfavorited articles from the last 2 days.
func (m *AIEnhancedManager) BatchProcessExistingArticles(userID int64) {
	go func() {
		log.Printf("Starting batch AI processing for user %d...", userID)

		targetLang, _ := m.db.GetSettingWithFallback(userID, "target_language")
		if targetLang == "" {
			targetLang = "zh"
		}

		articles, err := m.db.GetArticlesForAIBatchProcessing(userID, targetLang)
		if err != nil {
			log.Printf("Error fetching articles for AI batch processing (user %d): %v", userID, err)
			return
		}

		if len(articles) == 0 {
			log.Printf("No articles to batch process for user %d", userID)
			return
		}

		log.Printf("Found %d articles for AI batch processing (user %d)", len(articles), userID)

		queued := 0
		clusterRunNeeded := false
		for _, article := range articles {
			needsSummary := !article.HasSummary
			needsTranslation := article.TranslateArticles && !article.HasTranslation
			needsEmbedding := !article.HasArticleEmbedding
			needsDedup := !article.HasCluster
			needsClusterRun := article.HasCluster && !article.ClusterComplete

			if !needsSummary && !needsTranslation && !needsEmbedding && !needsDedup {
				if needsClusterRun {
					clusterRunNeeded = true
				}
				continue
			}

			task := &AIEnhancedTask{
				ArticleID:         article.Article.ID,
				UserID:            userID,
				FeedID:            article.Article.FeedID,
				NeedsSummary:      needsSummary,
				NeedsTranslation:  needsTranslation,
				TranslateArticles: article.TranslateArticles,
				NeedsEmbedding:    needsEmbedding,
				NeedsDedup:        needsDedup,
				NeedsClusterRun:   needsClusterRun,
			}

			select {
			case m.taskChan <- task:
				queued++
			default:
				log.Printf("AI enhanced task queue full during batch processing, queued %d/%d articles", queued, len(articles))
				return
			}
		}

		if clusterRunNeeded {
			m.scheduleClusterPipeline(userID)
		}

		log.Printf("Batch AI processing: queued %d tasks for user %d", queued, userID)
	}()
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

	// Check if embedding models configured (NEW prereq)
	embeddingsConfig, _ := db.GetSettingWithFallback(userID, "ai_embedding_models")
	if embeddingsConfig == "" || embeddingsConfig == "[]" {
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

	fusionConfig, err := getUserFeatureAIConfig(db, userID, ai.FeatureFusion)
	if err != nil || !hasConfiguredAPIKey(fusionConfig) {
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

func buildGlobalProxyURL(db *sqlite.DB, userID int64) (string, error) {
	proxyEnabled, err := db.GetSettingWithFallback(userID, "proxy_enabled")
	if err != nil || proxyEnabled != "true" {
		return "", err
	}

	proxyType, err := db.GetSettingWithFallback(userID, "proxy_type")
	if err != nil {
		return "", err
	}
	proxyHost, err := db.GetSettingWithFallback(userID, "proxy_host")
	if err != nil {
		return "", err
	}
	proxyPort, err := db.GetSettingWithFallback(userID, "proxy_port")
	if err != nil {
		return "", err
	}
	proxyUsername, err := db.GetEncryptedSettingWithFallback(userID, "proxy_username")
	if err != nil {
		return "", err
	}
	proxyPassword, err := db.GetEncryptedSettingWithFallback(userID, "proxy_password")
	if err != nil {
		return "", err
	}

	return httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword), nil
}
