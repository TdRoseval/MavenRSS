//go:build server

package article

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"MavenRSS/internal/handlers/core"
	"MavenRSS/internal/handlers/response"
	"MavenRSS/internal/models"
)

// HandleExportToObsidian exports an article to Obsidian in server mode.
// Instead of writing directly to file system, it provides a download of the Markdown file.
// @Summary      Export article to Obsidian (server mode)
// @Description  Export an article as a Markdown file for download (server mode)
// @Tags         articles
// @Accept       json
// @Produce      text/markdown
// @Param        request  body      ExportToObsidianRequest  true  "Article export request"
// @Success      200  {string}  string  "Markdown file download"
// @Failure      400  {object}  map[string]string  "Bad request"
// @Failure      404  {object}  map[string]string  "Article not found"
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

	// Check if Obsidian integration is enabled (optional in server mode, but still check)
	obsidianEnabled, _ := h.DB.GetSetting("obsidian_enabled")

	// Get article content
	content, _, err := h.GetArticleContent(int64(req.ArticleID))
	if err != nil {
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

	// In server mode, provide download instead of writing to vault
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(markdownContent))

	// Log that we're providing download instead of direct vault write
	if obsidianEnabled != "true" {
		logMsg := "Obsidian integration not enabled, providing Markdown download instead"
		fmt.Println(logMsg)
	} else {
		logMsg := "Server mode detected, providing Markdown download instead of direct vault write"
		fmt.Println(logMsg)
	}
}

// generateObsidianMarkdown converts an article to Markdown format for Obsidian
func generateObsidianMarkdown(article models.Article, content string) string {
	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: \"%s\"\n", escapeYamlString(article.Title)))
	sb.WriteString(fmt.Sprintf("feed: \"%s\"\n", escapeYamlString(article.FeedTitle)))
	sb.WriteString(fmt.Sprintf("published: \"%s\"\n", article.PublishedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("tags: [rss, %s]\n", sanitizeTag(article.FeedTitle)))
	sb.WriteString("---\n\n")

	sb.WriteString(fmt.Sprintf("# %s\n\n", article.Title))
	sb.WriteString(fmt.Sprintf("**Source:** %s\n\n", htmlEncodeURL(article.URL)))

	if content != "" {
		decodedContent := html.UnescapeString(content)
		markdownContent := htmlToMarkdown(decodedContent)
		sb.WriteString(markdownContent)
		sb.WriteString("\n\n")
	}

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("**Added to Obsidian:** %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Article ID:** %d\n", article.ID))

	return sb.String()
}
