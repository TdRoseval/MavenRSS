package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"MrRSS/internal/database"
	"MrRSS/internal/feed"
	article "MrRSS/internal/handlers/article"
	handlers "MrRSS/internal/handlers/core"
	discovery "MrRSS/internal/handlers/discovery"
	feedhandlers "MrRSS/internal/handlers/feed"
	media "MrRSS/internal/handlers/media"
	opml "MrRSS/internal/handlers/opml"
	rules "MrRSS/internal/handlers/rules"
	script "MrRSS/internal/handlers/script"
	settings "MrRSS/internal/handlers/settings"
	summary "MrRSS/internal/handlers/summary"
	translationhandlers "MrRSS/internal/handlers/translation"
	update "MrRSS/internal/handlers/update"
	window "MrRSS/internal/handlers/window"
	"MrRSS/internal/translation"
	"MrRSS/internal/tray"
	"MrRSS/internal/utils"
)

//go:embed frontend/dist/*
var frontendFiles embed.FS

//go:embed build/appicon.png
var trayIconPNG []byte

//go:embed build/windows/icon.ico
var trayIconICO []byte

type windowState struct {
	width  int
	height int
	x      int
	y      int
	valid  atomic.Bool
}

// getTrayIcon returns the appropriate icon bytes for the current platform
func getTrayIcon() []byte {
	// Windows requires .ico format, other platforms use .png
	if utils.IsWindows() {
		return trayIconICO
	}
	return trayIconPNG
}

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
	// Get proper paths for data files
	logPath, err := utils.GetLogPath()
	if err != nil {
		log.Printf("Warning: Could not get log path: %v. Using current directory.", err)
		logPath = "debug.log"
	}

	f, _ := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	defer f.Close()
	log.SetOutput(f)

	log.Println("Starting application...")

	// Log portable mode status
	if utils.IsPortableMode() {
		log.Println("Running in PORTABLE mode")
	} else {
		log.Println("Running in NORMAL mode")
	}

	log.Printf("Log file: %s", logPath)

	// Get database path
	dbPath, err := utils.GetDBPath()
	if err != nil {
		log.Printf("Error getting database path: %v", err)
		log.Fatal(err)
	}
	log.Printf("Database path: %s", dbPath)

	// Initialize database
	log.Println("Initializing Database...")
	db, err := database.NewDB(dbPath)
	if err != nil {
		log.Printf("Error initializing database: %v", err)
		log.Fatal(err)
	}

	// Run database schema initialization synchronously to ensure it's ready
	log.Println("Running DB migrations...")
	if err := db.Init(); err != nil {
		log.Printf("Error initializing database schema: %v", err)
		log.Fatal(err)
	}
	log.Println("Database initialized successfully")

	translator := translation.NewDynamicTranslator(db)
	fetcher := feed.NewFetcher(db, translator)
	h := handlers.NewHandler(db, fetcher, translator)

	// Use platform-specific icon format
	trayIconBytes := getTrayIcon()
	trayManager := tray.NewManager(h, trayIconBytes)
	var quitRequested atomic.Bool
	var lastWindowState windowState

	// API Routes
	log.Println("Setting up API routes...")
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/feeds", func(w http.ResponseWriter, r *http.Request) { feedhandlers.HandleFeeds(h, w, r) })
	apiMux.HandleFunc("/api/feeds/add", func(w http.ResponseWriter, r *http.Request) { feedhandlers.HandleAddFeed(h, w, r) })
	apiMux.HandleFunc("/api/feeds/delete", func(w http.ResponseWriter, r *http.Request) { feedhandlers.HandleDeleteFeed(h, w, r) })
	apiMux.HandleFunc("/api/feeds/update", func(w http.ResponseWriter, r *http.Request) { feedhandlers.HandleUpdateFeed(h, w, r) })
	apiMux.HandleFunc("/api/feeds/refresh", func(w http.ResponseWriter, r *http.Request) { feedhandlers.HandleRefreshFeed(h, w, r) })
	apiMux.HandleFunc("/api/feeds/discover", func(w http.ResponseWriter, r *http.Request) { discovery.HandleDiscoverBlogs(h, w, r) })
	apiMux.HandleFunc("/api/feeds/discover-all", func(w http.ResponseWriter, r *http.Request) { discovery.HandleDiscoverAllFeeds(h, w, r) })
	apiMux.HandleFunc("/api/feeds/discover/start", func(w http.ResponseWriter, r *http.Request) { discovery.HandleStartSingleDiscovery(h, w, r) })
	apiMux.HandleFunc("/api/feeds/discover/progress", func(w http.ResponseWriter, r *http.Request) { discovery.HandleGetSingleDiscoveryProgress(h, w, r) })
	apiMux.HandleFunc("/api/feeds/discover/clear", func(w http.ResponseWriter, r *http.Request) { discovery.HandleClearSingleDiscovery(h, w, r) })
	apiMux.HandleFunc("/api/feeds/discover-all/start", func(w http.ResponseWriter, r *http.Request) { discovery.HandleStartBatchDiscovery(h, w, r) })
	apiMux.HandleFunc("/api/feeds/discover-all/progress", func(w http.ResponseWriter, r *http.Request) { discovery.HandleGetBatchDiscoveryProgress(h, w, r) })
	apiMux.HandleFunc("/api/feeds/discover-all/clear", func(w http.ResponseWriter, r *http.Request) { discovery.HandleClearBatchDiscovery(h, w, r) })
	apiMux.HandleFunc("/api/articles", func(w http.ResponseWriter, r *http.Request) { article.HandleArticles(h, w, r) })
	apiMux.HandleFunc("/api/articles/filter", func(w http.ResponseWriter, r *http.Request) { article.HandleFilteredArticles(h, w, r) })
	apiMux.HandleFunc("/api/articles/read", func(w http.ResponseWriter, r *http.Request) { article.HandleMarkRead(h, w, r) })
	apiMux.HandleFunc("/api/articles/favorite", func(w http.ResponseWriter, r *http.Request) { article.HandleToggleFavorite(h, w, r) })
	apiMux.HandleFunc("/api/articles/cleanup", func(w http.ResponseWriter, r *http.Request) { article.HandleCleanupArticles(h, w, r) })
	apiMux.HandleFunc("/api/articles/translate", func(w http.ResponseWriter, r *http.Request) { translationhandlers.HandleTranslateArticle(h, w, r) })
	apiMux.HandleFunc("/api/articles/translate-text", func(w http.ResponseWriter, r *http.Request) { translationhandlers.HandleTranslateText(h, w, r) })
	apiMux.HandleFunc("/api/articles/clear-translations", func(w http.ResponseWriter, r *http.Request) { translationhandlers.HandleClearTranslations(h, w, r) })
	apiMux.HandleFunc("/api/articles/toggle-hide", func(w http.ResponseWriter, r *http.Request) { article.HandleToggleHideArticle(h, w, r) })
	apiMux.HandleFunc("/api/articles/toggle-read-later", func(w http.ResponseWriter, r *http.Request) { article.HandleToggleReadLater(h, w, r) })
	apiMux.HandleFunc("/api/articles/content", func(w http.ResponseWriter, r *http.Request) { article.HandleGetArticleContent(h, w, r) })
	apiMux.HandleFunc("/api/articles/unread-counts", func(w http.ResponseWriter, r *http.Request) { article.HandleGetUnreadCounts(h, w, r) })
	apiMux.HandleFunc("/api/articles/mark-all-read", func(w http.ResponseWriter, r *http.Request) { article.HandleMarkAllAsRead(h, w, r) })
	apiMux.HandleFunc("/api/articles/clear-read-later", func(w http.ResponseWriter, r *http.Request) { article.HandleClearReadLater(h, w, r) })
	apiMux.HandleFunc("/api/articles/summarize", func(w http.ResponseWriter, r *http.Request) { summary.HandleSummarizeArticle(h, w, r) })
	apiMux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) { settings.HandleSettings(h, w, r) })
	apiMux.HandleFunc("/api/refresh", func(w http.ResponseWriter, r *http.Request) { article.HandleRefresh(h, w, r) })
	apiMux.HandleFunc("/api/progress", func(w http.ResponseWriter, r *http.Request) { article.HandleProgress(h, w, r) })
	apiMux.HandleFunc("/api/opml/import", func(w http.ResponseWriter, r *http.Request) { opml.HandleOPMLImport(h, w, r) })
	apiMux.HandleFunc("/api/opml/export", func(w http.ResponseWriter, r *http.Request) { opml.HandleOPMLExport(h, w, r) })
	apiMux.HandleFunc("/api/check-updates", func(w http.ResponseWriter, r *http.Request) { update.HandleCheckUpdates(h, w, r) })
	apiMux.HandleFunc("/api/download-update", func(w http.ResponseWriter, r *http.Request) { update.HandleDownloadUpdate(h, w, r) })
	apiMux.HandleFunc("/api/install-update", func(w http.ResponseWriter, r *http.Request) { update.HandleInstallUpdate(h, w, r) })
	apiMux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) { update.HandleVersion(h, w, r) })
	apiMux.HandleFunc("/api/rules/apply", func(w http.ResponseWriter, r *http.Request) { rules.HandleApplyRule(h, w, r) })
	apiMux.HandleFunc("/api/scripts/dir", func(w http.ResponseWriter, r *http.Request) { script.HandleGetScriptsDir(h, w, r) })
	apiMux.HandleFunc("/api/scripts/open", func(w http.ResponseWriter, r *http.Request) { script.HandleOpenScriptsDir(h, w, r) })
	apiMux.HandleFunc("/api/scripts/list", func(w http.ResponseWriter, r *http.Request) { script.HandleListScripts(h, w, r) })
	apiMux.HandleFunc("/api/media/proxy", func(w http.ResponseWriter, r *http.Request) { media.HandleMediaProxy(h, w, r) })
	apiMux.HandleFunc("/api/media/cleanup", func(w http.ResponseWriter, r *http.Request) { media.HandleMediaCacheCleanup(h, w, r) })
	apiMux.HandleFunc("/api/media/info", func(w http.ResponseWriter, r *http.Request) { media.HandleMediaCacheInfo(h, w, r) })
	apiMux.HandleFunc("/api/window/state", func(w http.ResponseWriter, r *http.Request) { window.HandleGetWindowState(h, w, r) })
	apiMux.HandleFunc("/api/window/save", func(w http.ResponseWriter, r *http.Request) { window.HandleSaveWindowState(h, w, r) })

	// Static Files
	log.Println("Setting up static files...")
	frontendFS, err := fs.Sub(frontendFiles, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}

	fileServer := http.FileServer(http.FS(frontendFS))

	combinedHandler := &CombinedHandler{
		apiMux:     apiMux,
		fileServer: fileServer,
	}

	shouldCloseToTray := func() bool {
		val, err := db.GetSetting("close_to_tray")
		return err == nil && val == "true"
	}

	startTray := func(ctx context.Context) {
		if trayManager == nil || trayManager.IsRunning() {
			return
		}
		trayManager.Start(ctx, func() {
			quitRequested.Store(true)
			runtime.Quit(ctx)
		}, func() {
			if lastWindowState.valid.Load() {
				runtime.WindowSetSize(ctx, lastWindowState.width, lastWindowState.height)
				runtime.WindowSetPosition(ctx, lastWindowState.x, lastWindowState.y)
			}
			runtime.WindowShow(ctx)
			runtime.WindowUnminimise(ctx)
		})
	}

	storeWindowState := func(ctx context.Context) {
		if ctx == nil {
			return
		}

		w, h := runtime.WindowGetSize(ctx)
		lastWindowState.width = w
		lastWindowState.height = h

		x, y := runtime.WindowGetPosition(ctx)
		lastWindowState.x = x
		lastWindowState.y = y
		lastWindowState.valid.Store(true)
	}

	// Start background scheduler
	log.Println("Starting background scheduler...")
	bgCtx, bgCancel := context.WithCancel(context.Background())

	log.Println("Starting Wails...")
	err = wails.Run(&options.App{
		Title:    "MrRSS",
		Width:    1024,
		Height:   768,
		LogLevel: logger.DEBUG,
		AssetServer: &assetserver.Options{
			Assets:  frontendFS,
			Handler: combinedHandler,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnShutdown: func(ctx context.Context) {
			log.Println("Shutting down...")

			if trayManager != nil {
				trayManager.Stop()
			}

			// Stop background tasks first
			bgCancel()
			// Give some time for tasks to finish
			time.Sleep(500 * time.Millisecond)

			// Close DB with timeout
			done := make(chan struct{})
			go func() {
				if err := db.Close(); err != nil {
					log.Printf("Error closing database: %v", err)
				}
				close(done)
			}()

			select {
			case <-done:
				log.Println("Database closed")
			case <-time.After(2 * time.Second):
				log.Println("Database close timed out")
			}
		},
		OnStartup: func(ctx context.Context) {
			log.Println("App started")

			if shouldCloseToTray() {
				startTray(ctx)
			}

			// Start background scheduler after a longer delay to allow UI to show first
			go func() {
				time.Sleep(5 * time.Second)
				log.Println("Starting background scheduler...")
				h.StartBackgroundScheduler(bgCtx)
			}()
		},
		OnBeforeClose: func(ctx context.Context) bool {
			if quitRequested.Load() {
				return false
			}

			if shouldCloseToTray() {
				storeWindowState(ctx)
				// Fallback start in case tray failed to start on startup
				startTray(ctx)
				if trayManager != nil && trayManager.IsRunning() {
					runtime.WindowHide(ctx)
				} else {
					runtime.WindowMinimise(ctx)
				}
				return true
			}

			return false
		},
	})

	if err != nil {
		log.Printf("Error running Wails: %v", err)
		log.Fatal(err)
	}
	log.Println("Application finished")
}
