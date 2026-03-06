package article

// ExportToObsidianRequest represents the request for exporting to Obsidian
type ExportToObsidianRequest struct {
	ArticleID int `json:"article_id"`
}

// ExportToNotionRequest represents the request for exporting to Notion
type ExportToNotionRequest struct {
	ArticleID int `json:"article_id"`
}
