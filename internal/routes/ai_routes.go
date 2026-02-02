package routes

import (
	"errors"
	"net/http"

	aihandlers "MrRSS/internal/handlers/ai"
	chat "MrRSS/internal/handlers/chat"
	"MrRSS/internal/handlers/core"
	"MrRSS/internal/handlers/response"
)

// registerAIRoutes registers all AI-related routes
func registerAIRoutes(mux *http.ServeMux, h *core.Handler) {
	// AI Chat
	mux.HandleFunc("/api/ai-chat", func(w http.ResponseWriter, r *http.Request) { chat.HandleAIChat(h, w, r) })
	mux.HandleFunc("/api/ai/chat/sessions/delete-all", func(w http.ResponseWriter, r *http.Request) { chat.HandleDeleteAllSessions(h, w, r) })
	mux.HandleFunc("/api/ai/chat/sessions", func(w http.ResponseWriter, r *http.Request) { chat.HandleListSessions(h, w, r) })
	mux.HandleFunc("/api/ai/chat/session/create", func(w http.ResponseWriter, r *http.Request) { chat.HandleCreateSession(h, w, r) })
	mux.HandleFunc("/api/ai/chat/session", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			chat.HandleGetSession(h, w, r)
		case http.MethodPut, http.MethodPatch:
			chat.HandleUpdateSession(h, w, r)
		case http.MethodDelete:
			chat.HandleDeleteSession(h, w, r)
		default:
			response.Error(w, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/ai/chat/messages", func(w http.ResponseWriter, r *http.Request) { chat.HandleListMessages(h, w, r) })
	mux.HandleFunc("/api/ai/chat/message/delete", func(w http.ResponseWriter, r *http.Request) { chat.HandleDeleteMessage(h, w, r) })

	// AI testing and search
	mux.HandleFunc("/api/ai/test", func(w http.ResponseWriter, r *http.Request) { aihandlers.HandleTestAIConfig(h, w, r) })
	mux.HandleFunc("/api/ai/test/info", func(w http.ResponseWriter, r *http.Request) { aihandlers.HandleGetAITestInfo(h, w, r) })
	mux.HandleFunc("/api/ai/search", func(w http.ResponseWriter, r *http.Request) { aihandlers.HandleAISearch(h, w, r) })
}
