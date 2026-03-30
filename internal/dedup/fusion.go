package dedup

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"MavenRSS/internal/ai"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
	"MavenRSS/internal/summary"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
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
}

// RunFusion processes all pending_merge clusters for a user.
func RunFusion(ctx context.Context, db *sqlite.DB, userID int64, cfg *FusionConfig) error {
	if cfg == nil || cfg.Summarizer == nil {
		return fmt.Errorf("fusion config or summarizer is nil")
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

		articles, err := db.GetArticlesByClusterID(cluster.ID)
		if err != nil {
			log.Printf("Error getting articles for cluster %d: %v", cluster.ID, err)
			continue
		}

		if len(articles) <= 1 {
			if len(articles) == 1 {
				if err := copySingleArticle(db, cluster.ID, articles[0]); err != nil {
					log.Printf("Single-article fusion fallback failed for cluster %d: %v", cluster.ID, err)
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

		result, err := callLLMFusion(articles, db, cfg.Summarizer)
		if err != nil {
			log.Printf("LLM fusion failed for cluster %d: %v", cluster.ID, err)
			if fallbackErr := copySingleArticle(db, cluster.ID, articles[0]); fallbackErr != nil {
				log.Printf("Fallback write failed for cluster %d: %v", cluster.ID, fallbackErr)
				continue
			}
			log.Printf("Cluster %d fallback completed with first article, advancing to pending_embed", cluster.ID)
		} else {
			if err := db.UpdateClusterMergedContent(cluster.ID, result.MergedTitle, result.MergedSummary, result.MergedContent); err != nil {
				log.Printf("Failed to store fusion result for cluster %d: %v", cluster.ID, err)
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

	for _, cluster := range clusters {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if cluster.MergedTitle == "" && cluster.MergedSummary == "" {
			log.Printf("Cluster %d has empty merged title and summary, skipping cluster embedding", cluster.ID)
			if err := db.UpdateClusterStatus(cluster.ID, "complete"); err != nil {
				log.Printf("Failed to update cluster %d status to complete: %v", cluster.ID, err)
			}
			continue
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
			continue
		}
		log.Printf("Cluster %d embedding stage completed", cluster.ID)
	}
	return nil
}

func copySingleArticle(db *sqlite.DB, clusterID int64, a models.Article) error {
	content, _, _ := db.GetArticleContent(a.ID)
	title := a.Title
	smry := a.Summary
	if smry == "" {
		smry = title
	}
	if content == "" {
		content = smry
	}
	return db.UpdateClusterMergedContent(clusterID, title, smry, content)
}

func callLLMFusion(articles []models.Article, db *sqlite.DB, s *summary.AISummarizer) (*FusionResult, error) {
	var sb strings.Builder
	for i, a := range articles {
		content, _, _ := db.GetArticleContent(a.ID)
		if content == "" {
			content = a.Summary
		}
		sb.WriteString(fmt.Sprintf("--- 文章 %d ---\n标题: %s\n作者: %s\n来源: %s\n摘要: %s\n内容: %s\n\n",
			i+1, a.Title, a.Author, a.FeedTitle, a.Summary, truncate(content, 2000)))
	}

	// Set a custom system prompt for fusion
	s.SetSystemPrompt("你是一个资深编辑。请整合以下多篇语义相近的RSS文章，去除重复内容，保留各方不同的事实与观点，输出一篇逻辑连贯的综合报道。必须以JSON格式返回：{\"merged_title\": \"...\", \"merged_summary\": \"...\", \"merged_content\": \"...\"}")

	result, err := s.Summarize(sb.String(), summary.Long)
	if err != nil {
		return nil, fmt.Errorf("LLM fusion call: %w", err)
	}

	// Parse JSON from response
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
	return sqlite_vec.SerializeFloat32(emb)
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
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
