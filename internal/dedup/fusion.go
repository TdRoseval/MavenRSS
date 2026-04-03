package dedup

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"unicode/utf8"

	"MavenRSS/internal/ai"
	"MavenRSS/internal/interest"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
	"MavenRSS/internal/summary"
)

// FusionResult represents the JSON output from LLM fusion.
type FusionResult struct {
	MergedTitle   string `json:"merged_title"`
	MergedSummary string `json:"merged_summary"`
	MergedContent string `json:"merged_content"`
}

// FusionConfig holds configuration for the fusion process.
type FusionConfig struct {
	Summarizer     *summary.AISummarizer // LLM client for fusion
	EmbConfigsJSON string                // Embedding model configs JSON
	GlobalProxyURL string                // Global proxy URL for embedding API
	TargetLanguage string                // Target output language for fused result
}

const clusterEmbeddingWorkerCount = 2

// RunFusion processes all pending_merge clusters for a user.
func RunFusion(ctx context.Context, db *sqlite.DB, userID int64, cfg *FusionConfig) error {
	if cfg == nil {
		cfg = &FusionConfig{}
	}

	clusters, err := db.GetClustersByStatus(userID, "pending_merge")
	if err != nil {
		return fmt.Errorf("get pending_merge clusters: %w", err)
	}

	for _, cluster := range clusters {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := db.UpdateClusterStatus(cluster.ID, "merging"); err != nil {
			log.Printf("Failed to mark cluster %d as merging: %v", cluster.ID, err)
			continue
		}

		articles, err := db.GetArticlesByClusterID(cluster.ID)
		if err != nil {
			log.Printf("Error getting articles for cluster %d: %v", cluster.ID, err)
			_ = db.UpdateClusterStatus(cluster.ID, "pending_merge")
			continue
		}

		if len(articles) <= 1 {
			if len(articles) == 1 {
				if err := copySingleArticle(db, userID, cluster.ID, articles[0]); err != nil {
					log.Printf("Single-article fusion fallback failed for cluster %d: %v", cluster.ID, err)
					_ = db.UpdateClusterStatus(cluster.ID, "pending_merge")
					continue
				}
				log.Printf("Cluster %d contains a single article, copied source article and advanced to pending_embed", cluster.ID)
			} else {
				log.Printf("Cluster %d has no articles, advancing directly to pending_embed", cluster.ID)
			}
			if err := db.UpdateClusterStatus(cluster.ID, "pending_embed"); err != nil {
				log.Printf("Failed to update cluster %d status to pending_embed: %v", cluster.ID, err)
				continue
			}
			continue
		}

		if cfg.Summarizer == nil {
			log.Printf("Cluster %d fusion model unavailable, falling back to first article content", cluster.ID)
			if fallbackErr := copySingleArticle(db, userID, cluster.ID, articles[0]); fallbackErr != nil {
				log.Printf("Fallback write failed for cluster %d without fusion model: %v", cluster.ID, fallbackErr)
				_ = db.UpdateClusterStatus(cluster.ID, "pending_merge")
				continue
			}
			if err := db.UpdateClusterStatus(cluster.ID, "pending_embed"); err != nil {
				log.Printf("Failed to update cluster %d status to pending_embed: %v", cluster.ID, err)
				continue
			}
			continue
		}

		result, err := callLLMFusion(articles, db, cfg)
		if err != nil {
			log.Printf("LLM fusion failed for cluster %d: %v", cluster.ID, err)
			if fallbackErr := copySingleArticle(db, userID, cluster.ID, articles[0]); fallbackErr != nil {
				log.Printf("Fallback write failed for cluster %d: %v", cluster.ID, fallbackErr)
				_ = db.UpdateClusterStatus(cluster.ID, "pending_merge")
				continue
			}
			log.Printf("Cluster %d fallback completed with first article, advancing to pending_embed", cluster.ID)
		} else {
			if err := db.UpdateClusterMergedContent(cluster.ID, result.MergedTitle, result.MergedSummary, result.MergedContent); err != nil {
				log.Printf("Failed to store fusion result for cluster %d: %v", cluster.ID, err)
				_ = db.UpdateClusterStatus(cluster.ID, "pending_merge")
				continue
			}
			log.Printf("Cluster %d fusion completed, advancing to pending_embed", cluster.ID)
		}
		if err := db.UpdateClusterStatus(cluster.ID, "pending_embed"); err != nil {
			log.Printf("Failed to update cluster %d status to pending_embed: %v", cluster.ID, err)
			continue
		}
	}
	return nil
}

// RunEmbedding processes all pending_embed clusters.
func RunEmbedding(ctx context.Context, db *sqlite.DB, userID int64, cfg *FusionConfig) error {
	clusters, err := db.GetClustersByStatus(userID, "pending_embed")
	if err != nil {
		return fmt.Errorf("get pending_embed clusters: %w", err)
	}

	if len(clusters) == 0 {
		return nil
	}

	workerCount := clusterEmbeddingWorkerCount
	if len(clusters) < workerCount {
		workerCount = len(clusters)
	}

	jobs := make(chan models.Cluster)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for cluster := range jobs {
			if ctx.Err() != nil {
				return
			}
			processClusterEmbedding(ctx, db, cluster, cfg)
		}
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go worker()
	}

	for _, cluster := range clusters {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		case jobs <- cluster:
		}
	}

	close(jobs)
	wg.Wait()
	return nil
}

func processClusterEmbedding(ctx context.Context, db *sqlite.DB, cluster models.Cluster, cfg *FusionConfig) {
	if cluster.MergedTitle == "" && cluster.MergedSummary == "" {
		log.Printf("Cluster %d has empty merged title and summary, skipping cluster embedding", cluster.ID)
		if err := db.UpdateClusterStatus(cluster.ID, "complete"); err != nil {
			log.Printf("Failed to update cluster %d status to complete: %v", cluster.ID, err)
		}
		return
	}

	titleEmb, err := genEmbedding(ctx, cluster.MergedTitle, cfg)
	if err != nil {
		log.Printf("Title embedding failed for cluster %d: %v", cluster.ID, err)
	}

	summaryEmb, err := genEmbedding(ctx, cluster.MergedSummary, cfg)
	if err != nil {
		log.Printf("Summary embedding failed for cluster %d: %v", cluster.ID, err)
	}

	if len(titleEmb) > 0 || len(summaryEmb) > 0 {
		if err := db.UpdateClusterEmbeddings(cluster.ID, titleEmb, summaryEmb); err != nil {
			log.Printf("Failed to store cluster %d embeddings: %v", cluster.ID, err)
		}
	}
	if err := db.UpdateClusterStatus(cluster.ID, "complete"); err != nil {
		log.Printf("Failed to update cluster %d status to complete: %v", cluster.ID, err)
		return
	}
	log.Printf("Cluster %d embedding stage completed", cluster.ID)
}

func copySingleArticle(db *sqlite.DB, userID, clusterID int64, a models.Article) error {
	content, _, _ := db.GetArticleContent(a.ID)
	title := strings.TrimSpace(a.TranslatedTitle)
	if title == "" {
		title = strings.TrimSpace(a.Title)
	}
	smry := strings.TrimSpace(a.Summary)

	targetLang, _ := db.GetSettingWithFallback(userID, "target_language")
	if targetLang == "" {
		targetLang = "zh"
	}

	feed, _ := db.GetFeedByIDForUser(userID, a.FeedID)
	if feed != nil && feed.TranslateArticles {
		if translatedContent, _, found, err := db.GetArticleTranslatedContent(a.ID, targetLang); err == nil && found && strings.TrimSpace(translatedContent) != "" {
			content = translatedContent
			smry = deriveSummaryFromTranslatedContent(translatedContent)
		}
	}

	if smry == "" {
		smry = title
	}
	if content == "" {
		content = smry
	}
	return db.UpdateClusterMergedContent(clusterID, title, smry, content)
}

func callLLMFusion(articles []models.Article, db *sqlite.DB, cfg *FusionConfig) (*FusionResult, error) {
	s := cfg.Summarizer
	targetLabel := normalizeFusionLanguageLabel(cfg.TargetLanguage)

	if cfg.TargetLanguage != "" {
		s.SetLanguage(cfg.TargetLanguage)
	}
	s.SetSystemPrompt(buildFusionSystemPrompt(targetLabel))

	var sb strings.Builder
	for i, a := range articles {
		content, _, _ := db.GetArticleContent(a.ID)
		if content == "" {
			content = a.Summary
		}

		title := strings.TrimSpace(a.TranslatedTitle)
		if title == "" {
			title = strings.TrimSpace(a.Title)
		}

		sb.WriteString(fmt.Sprintf(
			"Article %d\nTitle: %s\nAuthor: %s\nSource: %s\nSummary: %s\nContent: %s\n\n",
			i+1,
			title,
			strings.TrimSpace(a.Author),
			strings.TrimSpace(a.FeedTitle),
			strings.TrimSpace(a.Summary),
			truncate(content, 2000),
		))
	}

	result, err := s.Summarize(sb.String(), summary.Long)
	if err != nil {
		return nil, fmt.Errorf("LLM fusion call: %w", err)
	}

	jsonStr := extractJSON(result.Summary)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in LLM response")
	}

	var fr FusionResult
	if err := json.Unmarshal([]byte(jsonStr), &fr); err != nil {
		return nil, fmt.Errorf("parse fusion JSON: %w", err)
	}
	return &fr, nil
}

func genEmbedding(ctx context.Context, text string, cfg *FusionConfig) ([]byte, error) {
	if text == "" || cfg.EmbConfigsJSON == "" {
		return nil, nil
	}
	emb, err := ai.GenerateEmbeddings(ctx, text, cfg.EmbConfigsJSON, cfg.GlobalProxyURL)
	if err != nil {
		return nil, err
	}
	if len(emb) == 0 {
		return nil, nil
	}
	return interest.NormalizeAndSerialize(emb)
}

func extractJSON(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return text[start : end+1]
	}
	return ""
}

func truncate(s string, maxLen int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= maxLen {
		return string(runes)
	}
	return string(runes[:maxLen]) + "..."
}

func deriveSummaryFromTranslatedContent(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	parts := make([]string, 0, 3)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "#*- ")
		line = strings.TrimPrefix(line, ">")
		line = strings.TrimSpace(line)
		if line == "" {
			if len(parts) > 0 {
				break
			}
			continue
		}

		parts = append(parts, line)
		if utf8.RuneCountInString(strings.Join(parts, " ")) >= 180 {
			break
		}
	}

	summaryText := strings.TrimSpace(strings.Join(parts, " "))
	if summaryText == "" {
		summaryText = strings.TrimSpace(normalized)
	}
	if utf8.RuneCountInString(summaryText) > 240 {
		return truncate(summaryText, 240)
	}
	return summaryText
}

func normalizeFusionLanguageLabel(targetLanguage string) string {
	trimmed := strings.TrimSpace(strings.ToLower(targetLanguage))
	switch trimmed {
	case "", "zh", "zh-cn", "zh-hans", "中文", "简体中文":
		return "Simplified Chinese"
	case "zh-tw", "zh-hk", "zh-hant", "繁體中文", "繁体中文":
		return "Traditional Chinese"
	case "en", "en-us", "en-gb", "english":
		return "English"
	case "ja", "ja-jp", "japanese", "日本語":
		return "Japanese"
	case "ko", "ko-kr", "korean", "한국어":
		return "Korean"
	case "fr", "fr-fr", "french":
		return "French"
	case "de", "de-de", "german":
		return "German"
	case "es", "es-es", "spanish":
		return "Spanish"
	case "it", "it-it", "italian":
		return "Italian"
	case "pt", "pt-br", "pt-pt", "portuguese":
		return "Portuguese"
	case "ru", "ru-ru", "russian":
		return "Russian"
	case "ar", "ar-sa", "arabic":
		return "Arabic"
	default:
		if strings.Contains(trimmed, "中文") {
			return "Simplified Chinese"
		}
		return strings.TrimSpace(targetLanguage)
	}
}

func buildFusionSystemPrompt(targetLanguage string) string {
	switch targetLanguage {
	case "", "Simplified Chinese":
		return `你是一名资深编辑，负责融合多篇语义相近的 RSS 文章。
请始终使用简体中文输出，去除重复内容，保留不同事实、背景与观点，不要编造信息。
只返回合法 JSON，格式必须为 {"merged_title":"...","merged_summary":"...","merged_content":"..."}。
其中：
- merged_title：简洁、准确的标题
- merged_summary：2 到 4 句摘要
- merged_content：结构清晰、可直接展示的 Markdown 正文
不要输出代码块，不要输出 JSON 以外的任何说明。`
	case "Traditional Chinese":
		return `你是一名資深編輯，負責融合多篇語義相近的 RSS 文章。
請始終使用繁體中文輸出，去除重複內容，保留不同事實、背景與觀點，不要編造資訊。
只返回合法 JSON，格式必須為 {"merged_title":"...","merged_summary":"...","merged_content":"..."}。
其中：
- merged_title：簡潔、準確的標題
- merged_summary：2 到 4 句摘要
- merged_content：結構清晰、可直接展示的 Markdown 正文
不要輸出程式碼區塊，不要輸出 JSON 以外的任何說明。`
	default:
		return fmt.Sprintf(`You are a senior editor merging multiple semantically similar RSS articles into one coherent report.
Write every field in %s. Remove duplicated points, preserve unique facts and perspectives, and do not invent details.
Return valid JSON only in this exact shape: {"merged_title":"...","merged_summary":"...","merged_content":"..."}.
Requirements:
- merged_title: concise and accurate headline
- merged_summary: 2 to 4 sentence overview
- merged_content: well-structured Markdown article ready for display
Do not wrap the JSON in code fences and do not add any extra commentary.`, targetLanguage)
	}
}
