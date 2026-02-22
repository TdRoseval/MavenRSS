package routes

import (
	"errors"
	"net/http"
	"strings"

	aihandlers "MavenRSS/internal/handlers/ai"
	chat "MavenRSS/internal/handlers/chat"
	"MavenRSS/internal/handlers/core"
	"MavenRSS/internal/handlers/response"
	"MavenRSS/internal/middleware"
)

// registerAIRoutes registers all AI-related routes
func registerAIRoutes(mux *http.ServeMux, h *core.Handler, cfg Config) {
	var authMiddleware middleware.Middleware
	if cfg.EnableAuth && cfg.JWTManager != nil {
		authMiddleware = middleware.AuthMiddleware(cfg.JWTManager)
	}

	// AI Chat
	registerProtectedRoute(mux, "/api/ai-chat", authMiddleware, func(w http.ResponseWriter, r *http.Request) { chat.HandleAIChat(h, w, r) })
	registerProtectedRoute(mux, "/api/ai-chat/stream", authMiddleware, func(w http.ResponseWriter, r *http.Request) { chat.HandleAIChatStream(h, w, r) })
	registerProtectedRoute(mux, "/api/ai/chat/sessions/delete-all", authMiddleware, func(w http.ResponseWriter, r *http.Request) { chat.HandleDeleteAllSessions(h, w, r) })
	registerProtectedRoute(mux, "/api/ai/chat/sessions", authMiddleware, func(w http.ResponseWriter, r *http.Request) { chat.HandleListSessions(h, w, r) })
	registerProtectedRoute(mux, "/api/ai/chat/session/create", authMiddleware, func(w http.ResponseWriter, r *http.Request) { chat.HandleCreateSession(h, w, r) })
	registerProtectedRoute(mux, "/api/ai/chat/session", authMiddleware, func(w http.ResponseWriter, r *http.Request) {
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
	registerProtectedRoute(mux, "/api/ai/chat/messages", authMiddleware, func(w http.ResponseWriter, r *http.Request) { chat.HandleListMessages(h, w, r) })
	registerProtectedRoute(mux, "/api/ai/chat/message/delete", authMiddleware, func(w http.ResponseWriter, r *http.Request) { chat.HandleDeleteMessage(h, w, r) })

	// AI testing and search
	registerProtectedRoute(mux, "/api/ai/test", authMiddleware, func(w http.ResponseWriter, r *http.Request) { aihandlers.HandleTestAIConfig(h, w, r) })
	registerProtectedRoute(mux, "/api/ai/test/info", authMiddleware, func(w http.ResponseWriter, r *http.Request) { aihandlers.HandleGetAITestInfo(h, w, r) })
	registerProtectedRoute(mux, "/api/ai/search", authMiddleware, func(w http.ResponseWriter, r *http.Request) { aihandlers.HandleAISearch(h, w, r) })

	// AI Profiles
	registerProtectedRoute(mux, "/api/ai/profiles/test-all", authMiddleware, func(w http.ResponseWriter, r *http.Request) { aihandlers.HandleTestAllAIProfiles(h, w, r) })
	registerProtectedRoute(mux, "/api/ai/profiles/test-config", authMiddleware, func(w http.ResponseWriter, r *http.Request) { aihandlers.HandleTestAIProfileConfig(h, w, r) })
	registerProtectedRoute(mux, "/api/ai/profiles", authMiddleware, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			aihandlers.HandleListAIProfiles(h, w, r)
		case http.MethodPost:
			aihandlers.HandleCreateAIProfile(h, w, r)
		default:
			response.Error(w, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		}
	})
	registerProtectedRoute(mux, "/api/ai/profiles/", authMiddleware, func(w http.ResponseWriter, r *http.Request) {
		// Handle routes like /api/ai/profiles/:id, /api/ai/profiles/:id/test, /api/ai/profiles/:id/default
		path := r.URL.Path
		if strings.HasSuffix(path, "/test") {
			aihandlers.HandleTestAIProfile(h, w, r)
		} else if strings.HasSuffix(path, "/default") {
			aihandlers.HandleSetDefaultAIProfile(h, w, r)
		} else {
			// Direct profile access by ID
			switch r.Method {
			case http.MethodGet:
				aihandlers.HandleGetAIProfile(h, w, r)
			case http.MethodPut:
				aihandlers.HandleUpdateAIProfile(h, w, r)
			case http.MethodDelete:
				aihandlers.HandleDeleteAIProfile(h, w, r)
			default:
				response.Error(w, errors.New("method not allowed"), http.StatusMethodNotAllowed)
			}
		}
	})
}
