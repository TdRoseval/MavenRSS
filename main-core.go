//go:build server

package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"MrRSS/internal/ai"
	"MrRSS/internal/database"
	"MrRSS/internal/feed"
	handlers "MrRSS/internal/handlers/core"
	"MrRSS/internal/network"
	"MrRSS/internal/routes"
	"MrRSS/internal/translation"
	"MrRSS/internal/utils/fileutil"
	"MrRSS/internal/utils/httputil"

	httpSwagger "github.com/swaggo/http-swagger"
)

// @title           MrRSS API
// @version         1.3.20
// @description     MrRSS is a modern, cross-platform desktop RSS reader with auto-translation, smart feed discovery, and AI-powered summarization.

// @contact.name   API Support
// @contact.url    https://github.com/WCY-dt/MrRSS
// @contact.email  mail@ch3nyang.top

// @license.name  GPL-3.0
// @license.url   https://www.gnu.org/licenses/gpl-3.0.en.html

// @host      localhost:1234
// @BasePath  /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

var debugLogging = os.Getenv("MRRSS_DEBUG") != ""

func debugLog(format string, args ...interface{}) {
	if debugLogging {
		log.Printf(format, args...)
	}
}

//go:embed frontend/dist
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
	
	// Use a custom response writer to capture 404s
	recorder := &statusRecorder{
		ResponseWriter: w,
		status:         200,
	}
	
	// Try to serve the file
	h.fileServer.ServeHTTP(recorder, r)
	
	// If it was a 404 and not an API route, serve index.html
	if recorder.status == http.StatusNotFound && !strings.HasPrefix(r.URL.Path, "/api/") {
		// Read index.html from embedded FS
		indexContent, err := frontendFiles.ReadFile("frontend/dist/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(indexContent)
		return
	}
}

// statusRecorder captures the HTTP status code
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	if sr.status == 0 {
		sr.status = http.StatusOK
	}
	return sr.ResponseWriter.Write(b)
}

func main() {
	// Parse flags
	flag.BoolFunc("server", "Run in headless server mode", func(s string) error {
		v, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		fileutil.SetServerMode(v)
		return nil
	})
	host := flag.String("host", "0.0.0.0", "Host to listen on in server mode")
	port := flag.String("port", "1234", "Port to listen on in server mode")
	flag.Parse()

	// Force server mode for this build
	fileutil.SetServerMode(true)

	// Get proper paths for data files
	logPath, err := fileutil.GetLogPath()
	if err != nil {
		log.Printf("Warning: Could not get log path: %v. Using current directory.", err)
		logPath = "debug.log"
	}

	// In server mode, log to both stdout and file
	f, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.SetOutput(os.Stdout) // Fallback
	} else {
		// Note: we don't close f here as it needs to stay open for logging
		// It will be closed by OS on process exit
		log.SetOutput(io.MultiWriter(os.Stdout, f))
	}

	log.Println("Starting application in server mode...")

	// Log portable mode status
	if fileutil.IsPortableMode() {
		log.Println("Running in PORTABLE mode")
	} else {
		log.Println("Running in NORMAL mode")
	}

	log.Printf("Log file: %s", logPath)

	// Get database path
	dbPath, err := fileutil.GetDBPath()
	if err != nil {
		log.Printf("Error getting database path: %v", err)
		log.Fatal(err)
	}
	debugLog("Database path: %s", dbPath)

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

	// Initialize AI profile provider
	profileProvider := ai.NewProfileProvider(db)
	translator := translation.NewDynamicTranslatorWithCache(db, db)
	translator.SetProfileProvider(profileProvider)

	fetcher := feed.NewFetcher(db)
	h := handlers.NewHandler(db, fetcher, translator, profileProvider)

	// API Routes
	log.Println("Setting up API routes...")
	apiMux := http.NewServeMux()
	routes.RegisterAPIRoutesWithConfig(apiMux, h, routes.ServerConfig())

	// Swagger Documentation - Serve swagger.json file
	apiMux.HandleFunc("/docs/SERVER_MODE/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "docs/SERVER_MODE/swagger.json")
	})

	apiMux.HandleFunc("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/docs/SERVER_MODE/swagger.json"),
	))

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

	// Wrap the combined handler with server middleware
	wrappedHandler := routes.WrapWithMiddleware(combinedHandler, routes.ServerConfig())

	log.Printf("Starting in headless server mode on http://%s:%s", *host, *port)

	// Start background scheduler
	// Use a context that we can cancel on shutdown
	bgCtx, bgCancel := context.WithCancel(context.Background())

	log.Println("Starting background scheduler...")
	go h.StartBackgroundScheduler(bgCtx)

	// Start Network Speed Detection (optional but good to have)
	go func() {
		log.Println("Detecting network speed...")

		var proxyURL string
		proxyEnabled, _ := db.GetSetting("proxy_enabled")
		if proxyEnabled == "true" {
			proxyType, _ := db.GetSetting("proxy_type")
			proxyHost, _ := db.GetSetting("proxy_host")
			proxyPort, _ := db.GetSetting("proxy_port")
			proxyUsername, _ := db.GetEncryptedSetting("proxy_username")
			proxyPassword, _ := db.GetEncryptedSetting("proxy_password")
			proxyURL = httputil.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
		}

		httpClient := httputil.GetPooledHTTPClient(proxyURL, 10*time.Second)
		detector := network.NewDetector(httpClient)
		detectCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		result := detector.DetectSpeed(detectCtx)
		if result.DetectionSuccess {
			db.SetSetting("network_speed", string(result.SpeedLevel))
			db.SetSetting("network_bandwidth_mbps", fmt.Sprintf("%.2f", result.BandwidthMbps))
			db.SetSetting("network_latency_ms", strconv.FormatInt(result.LatencyMs, 10))
			db.SetSetting("max_concurrent_refreshes", strconv.Itoa(result.MaxConcurrency))
			db.SetSetting("last_network_test", result.DetectionTime.Format(time.RFC3339))
			log.Printf("Network detection complete: %s (%.2f Mbps, %d ms latency)", result.SpeedLevel, result.BandwidthMbps, result.LatencyMs)
		} else {
			log.Printf("Network detection failed: %s", result.ErrorMessage)
		}
	}()

	// Start HTTP Server
	srv := &http.Server{
		Addr:    *host + ":" + *port,
		Handler: wrappedHandler,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	bgCancel()

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Close Database
	if err := db.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	} else {
		log.Println("Database closed")
	}

	log.Println("Server exited")
}
