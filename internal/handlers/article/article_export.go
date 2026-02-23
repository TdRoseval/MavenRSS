//go:build !server

package article

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"MavenRSS/internal/handlers/core"
	"MavenRSS/internal/handlers/response"
	"MavenRSS/internal/models"
)

// HandleExportToObsidian exports an article to Obsidian using direct file system access
// @Summary      Export article to Obsidian
// @Description  Export an article to Obsidian vault as a Markdown file (requires obsidian_enabled and obsidian_vault_path settings)
// @Tags         articles
// @Accept       json
// @Produce      json
// @Param        request  body      ExportToObsidianRequest  true  "Article export request"
// @Success      200  {object}  map[string]string  "Export result (success, file_path, message)"
// @Failure      400  {object}  map[string]string  "Bad request (Obsidian not configured or invalid article ID)"
// @Failure      404  {object}  map[string]string  "Article not found"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /articles/export/obsidian [post]
func HandleExportToObsidian(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, nil, http.StatusMethodNotAllowed)
		return
	}

	var req ExportToObsidianRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	if req.ArticleID <= 0 {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	// Get article from database
	article, err := h.DB.GetArticleByID(int64(req.ArticleID))
	if err != nil {
		response.Error(w, err, http.StatusNotFound)
		return
	}

	// Check if Obsidian integration is enabled
	obsidianEnabled, _ := h.DB.GetSetting("obsidian_enabled")
	if obsidianEnabled != "true" {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	// Get vault path (required for direct file access)
	vaultPath, _ := h.DB.GetSetting("obsidian_vault_path")
	if vaultPath == "" {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	// Validate vault path exists and is a directory
	if info, err := os.Stat(vaultPath); os.IsNotExist(err) {
		response.Error(w, nil, http.StatusBadRequest)
		return
	} else if !info.IsDir() {
		response.Error(w, nil, http.StatusBadRequest)
		return
	}

	// Get article content
	content, _, err := h.GetArticleContent(int64(req.ArticleID))
	if err != nil {
		// If content fetch fails, continue with empty content
		content = ""
	}

	// Generate Markdown content
	markdownContent := generateObsidianMarkdown(*article, content)

	// Generate filename (sanitize title)
	filename := sanitizeFilename(article.Title)
	if filename == "" {
		filename = fmt.Sprintf("Article_%d", article.ID)
	}
	filename += ".md"

	// Create full file path
	filePath := filepath.Join(vaultPath, filename)

	// Write file to Obsidian vault
	if err := os.WriteFile(filePath, []byte(markdownContent), 0644); err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	// Return success response
	response.JSON(w, map[string]string{
		"success":   "true",
		"file_path": filePath,
		"message":   "Article exported to Obsidian successfully",
	})
}

// generateObsidianMarkdown converts an article to Markdown format for Obsidian
func generateObsidianMarkdown(article models.Article, content string) string {
	var sb strings.Builder

	// Front matter - exclude URL to avoid URI parsing issues
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: \"%s\"\n", escapeYamlString(article.Title)))
	sb.WriteString(fmt.Sprintf("feed: \"%s\"\n", escapeYamlString(article.FeedTitle)))
	sb.WriteString(fmt.Sprintf("published: \"%s\"\n", article.PublishedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("tags: [rss, %s]\n", sanitizeTag(article.FeedTitle)))
	sb.WriteString("---\n\n")

	// Title
	sb.WriteString(fmt.Sprintf("# %s\n\n", article.Title))

	// Source URL (HTML encoded to avoid URI parsing issues)
	sb.WriteString(fmt.Sprintf("**Source:** %s\n\n", htmlEncodeURL(article.URL)))

	// Content
	if content != "" {
		// Decode HTML entities first, then convert HTML to Markdown
		decodedContent := html.UnescapeString(content)
		markdownContent := htmlToMarkdown(decodedContent)
		sb.WriteString(markdownContent)
		sb.WriteString("\n\n")
	}

	// Add metadata at the end
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("**Added to Obsidian:** %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Article ID:** %d\n", article.ID))

	return sb.String()
}
