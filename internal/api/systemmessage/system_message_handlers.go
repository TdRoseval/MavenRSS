package systemmessage

import (
	"encoding/json"
	"log"
	"net/http"

	"MavenRSS/internal/api/core"
	"MavenRSS/internal/api/response"
	"MavenRSS/internal/models"
)

func HandleListSystemMessages(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	messages, err := h.DB.ListSystemMessages(userID, 100)
	if err != nil {
		log.Printf("list system messages error: %v", err)
		http.Error(w, "Failed to load system messages", http.StatusInternalServerError)
		return
	}

	response.JSON(w, models.SystemMessageListResponse{Messages: messages})
}

func HandleSystemMessageUnreadCount(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	count, err := h.DB.CountUnreadSystemMessages(userID)
	if err != nil {
		log.Printf("count unread system messages error: %v", err)
		http.Error(w, "Failed to count system messages", http.StatusInternalServerError)
		return
	}

	response.JSON(w, models.SystemMessageUnreadCountResponse{UnreadCount: count})
}

func HandleMarkSystemMessageRead(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID <= 0 {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.DB.MarkSystemMessageRead(userID, req.ID); err != nil {
		log.Printf("mark system message read error: %v", err)
		http.Error(w, "Failed to mark system message read", http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]bool{"success": true})
}

func HandleMarkAllSystemMessagesRead(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.DB.MarkAllSystemMessagesRead(userID); err != nil {
		log.Printf("mark all system messages read error: %v", err)
		http.Error(w, "Failed to mark all system messages read", http.StatusInternalServerError)
		return
	}

	response.JSON(w, map[string]bool{"success": true})
}
