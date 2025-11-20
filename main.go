package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"MrRSS/internal/database"
	"MrRSS/internal/feed"
	"MrRSS/internal/handlers"
	"MrRSS/internal/translation"
)

//go:embed frontend/dist/*
var frontendFiles embed.FS

type CombinedHandler struct {
	apiMux     *http.ServeMux
	fileServer http.Handler
}

func (h *CombinedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		h.apiMux.ServeHTTP(w, r)
		return
	}
	h.fileServer.ServeHTTP(w, r)
}

func main() {
	db, err := database.NewDB("rss.db")
	if err != nil {
		log.Fatal(err)
	}

	translator := translation.NewGoogleFreeTranslator()
	fetcher := feed.NewFetcher(db, translator)
	h := handlers.NewHandler(db, fetcher)

	// API Routes
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/feeds", h.HandleFeeds)
	apiMux.HandleFunc("/api/feeds/add", h.HandleAddFeed)
	apiMux.HandleFunc("/api/feeds/delete", h.HandleDeleteFeed)
	apiMux.HandleFunc("/api/feeds/update", h.HandleUpdateFeed)
	apiMux.HandleFunc("/api/articles", h.HandleArticles)
	apiMux.HandleFunc("/api/articles/read", h.HandleMarkRead)
	apiMux.HandleFunc("/api/articles/favorite", h.HandleToggleFavorite)
	apiMux.HandleFunc("/api/settings", h.HandleSettings)
	apiMux.HandleFunc("/api/refresh", h.HandleRefresh)
	apiMux.HandleFunc("/api/progress", h.HandleProgress)
	apiMux.HandleFunc("/api/opml/import", h.HandleOPMLImport)
	apiMux.HandleFunc("/api/opml/export", h.HandleOPMLExport)

	// Static Files
	frontendFS, err := fs.Sub(frontendFiles, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(frontendFS))

	combinedHandler := &CombinedHandler{
		apiMux:     apiMux,
		fileServer: fileServer,
	}

	// Start background scheduler
	go h.StartBackgroundScheduler()

	err = wails.Run(&options.App{
		Title:  "MrRSS",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets:  nil, // We handle everything in the handler
			Handler: combinedHandler,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
	})

	if err != nil {
		log.Fatal(err)
	}
}
