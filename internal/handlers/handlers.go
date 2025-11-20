// Package handlers contains the HTTP handlers for the application.
// It defines the Handler struct which holds dependencies like the database and fetcher.
package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"MrRSS/internal/database"
	"MrRSS/internal/feed"
	"MrRSS/internal/opml"
)

type Handler struct {
	DB      *database.DB
	Fetcher *feed.Fetcher
}

func NewHandler(db *database.DB, fetcher *feed.Fetcher) *Handler {
	return &Handler{
		DB:      db,
		Fetcher: fetcher,
	}
}

func (h *Handler) StartBackgroundScheduler() {
	for {
		intervalStr, err := h.DB.GetSetting("update_interval")
		interval := 10
		if err == nil {
			if i, err := strconv.Atoi(intervalStr); err == nil && i > 0 {
				interval = i
			}
		}

		log.Printf("Next auto-update in %d minutes", interval)
		time.Sleep(time.Duration(interval) * time.Minute)
		h.Fetcher.FetchAll()
	}
}

func (h *Handler) HandleFeeds(w http.ResponseWriter, r *http.Request) {
	feeds, err := h.DB.GetFeeds()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(feeds)
}

func (h *Handler) HandleAddFeed(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL      string `json:"url"`
		Category string `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.Fetcher.AddSubscription(req.URL, req.Category); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleDeleteFeed(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	if err := h.DB.DeleteFeed(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleUpdateFeed(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		URL      string `json:"url"`
		Category string `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.DB.UpdateFeed(req.ID, req.Title, req.URL, req.Category); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleArticles(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("filter")
	feedIDStr := r.URL.Query().Get("feed_id")
	category := r.URL.Query().Get("category")
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	var feedID int64
	if feedIDStr != "" {
		feedID, _ = strconv.ParseInt(feedIDStr, 10, 64)
	}

	page := 1
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}

	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	offset := (page - 1) * limit

	articles, err := h.DB.GetArticles(filter, feedID, category, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(articles)
}

func (h *Handler) HandleProgress(w http.ResponseWriter, r *http.Request) {
	progress := h.Fetcher.GetProgress()
	json.NewEncoder(w).Encode(progress)
}

func (h *Handler) HandleMarkRead(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	readStr := r.URL.Query().Get("read")
	read := true
	if readStr == "false" || readStr == "0" {
		read = false
	}

	if err := h.DB.MarkArticleRead(id, read); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleToggleFavorite(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	if err := h.DB.ToggleFavorite(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	go h.Fetcher.FetchAll()
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleOPMLImport(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	feeds, err := opml.Parse(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go func() {
		for _, f := range feeds {
			h.Fetcher.ImportSubscription(f.Title, f.URL, f.Category)
		}
		h.Fetcher.FetchAll()
	}()

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleOPMLExport(w http.ResponseWriter, r *http.Request) {
	feeds, err := h.DB.GetFeeds()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := opml.Generate(feeds)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=subscriptions.opml")
	w.Header().Set("Content-Type", "text/xml")
	w.Write(data)
}

func (h *Handler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		interval, _ := h.DB.GetSetting("update_interval")
		translationEnabled, _ := h.DB.GetSetting("translation_enabled")
		targetLang, _ := h.DB.GetSetting("target_language")
		provider, _ := h.DB.GetSetting("translation_provider")
		apiKey, _ := h.DB.GetSetting("deepl_api_key")
		json.NewEncoder(w).Encode(map[string]string{
			"update_interval":      interval,
			"translation_enabled":  translationEnabled,
			"target_language":      targetLang,
			"translation_provider": provider,
			"deepl_api_key":        apiKey,
		})
	} else if r.Method == http.MethodPost {
		var req struct {
			UpdateInterval      string `json:"update_interval"`
			TranslationEnabled  string `json:"translation_enabled"`
			TargetLanguage      string `json:"target_language"`
			TranslationProvider string `json:"translation_provider"`
			DeepLAPIKey         string `json:"deepl_api_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.UpdateInterval != "" {
			h.DB.SetSetting("update_interval", req.UpdateInterval)
		}
		if req.TranslationEnabled != "" {
			h.DB.SetSetting("translation_enabled", req.TranslationEnabled)
		}
		if req.TargetLanguage != "" {
			h.DB.SetSetting("target_language", req.TargetLanguage)
		}
		if req.TranslationProvider != "" {
			h.DB.SetSetting("translation_provider", req.TranslationProvider)
		}
		// Always update API key as it might be cleared
		h.DB.SetSetting("deepl_api_key", req.DeepLAPIKey)

		w.WriteHeader(http.StatusOK)
	}
}
