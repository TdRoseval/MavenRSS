package article

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"MrRSS/internal/handlers/core"
)

// HandleGetArticleContent fetches the article content from RSS feed dynamically.
func HandleGetArticleContent(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	articleIDStr := r.URL.Query().Get("id")
	articleID, err := strconv.ParseInt(articleIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid article ID", http.StatusBadRequest)
		return
	}

	// Use the cached content fetching method
	content, err := h.GetArticleContent(articleID)
	if err != nil {
		log.Printf("Error getting article content: %v", err)
		http.Error(w, "Failed to fetch article content", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"content": content,
	})
}
