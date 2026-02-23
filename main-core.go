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

	"MavenRSS/internal/ai"
	"MavenRSS/internal/auth"
	"MavenRSS/internal/crypto"
	"MavenRSS/internal/database"
	"MavenRSS/internal/feed"
	auth_handlers "MavenRSS/internal/handlers/auth"
	handlers "MavenRSS/internal/handlers/core"
	"MavenRSS/internal/models"
	"MavenRSS/internal/network"
	"MavenRSS/internal/routes"
	"MavenRSS/internal/translation"
	"MavenRSS/internal/utils/fileutil"
	"MavenRSS/internal/utils/httputil"

	httpSwagger "github.com/swaggo/http-swagger"
)

// @title           MavenRSS API
// @version         1.3.20
// @description     MavenRSS is a modern, cross-platform desktop RSS reader with auto-translation, smart feed discovery, and AI-powered summarization.

// @contact.name   API Support
// @contact.url    https://github.com/WCY-dt/MavenRSS
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

// cachedStaticHandler adds caching headers and ETag support for static files
type cachedStaticHandler struct {
	handler http.Handler
	fs      fs.FS
}

func (h *cachedStaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// First check if the file actually exists
	filePath := strings.TrimPrefix(r.URL.Path, "/")
	if filePath == "" {
		filePath = "index.html"
	}

	// Try to open the file to verify existence and get info
	file, err := h.fs.Open(filePath)
	fileExists := err == nil

	var fileInfo fs.FileInfo
	if fileExists {
		fileInfo, _ = file.Stat()
		file.Close()
	}

	// Only set caching headers and ETag if file exists
	if fileExists {
		// Set security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// index.html should never be cached for long - it references versioned assets
		if filePath == "index.html" || r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		} else if strings.Contains(r.URL.Path, "/assets/") {
			// Versioned assets can be cached forever
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if strings.HasSuffix(filePath, ".css") || strings.HasSuffix(filePath, ".js") {
			// Other static assets with short cache
			w.Header().Set("Cache-Control", "public, max-age=86400")
		} else if strings.HasSuffix(filePath, ".jpg") || strings.HasSuffix(filePath, ".jpeg") ||
			strings.HasSuffix(filePath, ".png") || strings.HasSuffix(filePath, ".gif") ||
			strings.HasSuffix(filePath, ".svg") || strings.HasSuffix(filePath, ".webp") {
			// Images - longer cache
			w.Header().Set("Cache-Control", "public, max-age=604800")
		} else if strings.HasSuffix(filePath, ".woff") || strings.HasSuffix(filePath, ".woff2") ||
			strings.HasSuffix(filePath, ".ttf") || strings.HasSuffix(filePath, ".otf") {
			// Fonts - cache forever
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			// Default cache
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}

		// Add ETag support using file modification time and size
		if fileInfo != nil {
			etag := fmt.Sprintf(`"%x-%x"`, fileInfo.ModTime().Unix(), fileInfo.Size())
			if match := r.Header.Get("If-None-Match"); match != "" {
				if match == etag && !(filePath == "index.html" || r.URL.Path == "/") {
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}
			w.Header().Set("ETag", etag)
			w.Header().Set("Last-Modified", fileInfo.ModTime().UTC().Format(http.TimeFormat))
		}
	}

	h.handler.ServeHTTP(w, r)
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

	// Set crypto key directory to data directory for server mode
	dataDir, err := fileutil.GetDataDir()
	if err == nil {
		crypto.SetServerModeKeyDir(dataDir)
		log.Printf("Encryption key directory: %s", dataDir)
	}

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

	jwtSecret := os.Getenv("MRRSS_JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default-jwt-secret-key-please-change-in-production"
		log.Println("========================================")
		log.Println("⚠️  SECURITY WARNING: Using default JWT secret!")
		log.Println("⚠️  This is INSECURE for production use.")
		log.Println("⚠️  Set MRRSS_JWT_SECRET environment variable with a strong secret.")
		log.Println("========================================")
	}
	jwtManager := auth.NewJWTManager(jwtSecret)

	authHandler := auth_handlers.NewHandler(db, jwtSecret)

	adminUsername := os.Getenv("MRRSS_ADMIN_USERNAME")
	adminEmail := os.Getenv("MRRSS_ADMIN_EMAIL")
	adminPassword := os.Getenv("MRRSS_ADMIN_PASSWORD")
	if adminUsername == "" {
		adminUsername = "admin"
	}
	if adminEmail == "" {
		adminEmail = "admin@example.com"
	}
	if adminPassword == "" {
		adminPassword = "admin123"
		log.Println("========================================")
		log.Println("⚠️  SECURITY WARNING: Using default admin password!")
		log.Println("⚠️  Default credentials: admin / admin123")
		log.Println("⚠️  This is EXTREMELY INSECURE!")
		log.Println("⚠️  Set MRRSS_ADMIN_PASSWORD environment variable IMMEDIATELY!")
		log.Println("========================================")
	}
	_, err = db.GetUserByUsername(adminUsername)
	if err != nil {
		hashedPassword, err := auth.HashPassword(adminPassword)
		if err == nil {
			adminUser := &models.User{
				Username:     adminUsername,
				Email:        adminEmail,
				PasswordHash: hashedPassword,
				Role:         models.RoleAdmin,
				Status:       "active",
			}
			userID, err := db.CreateUser(adminUser)
			if err == nil {
				adminQuota := &models.UserQuota{
					UserID:                   userID,
					MaxFeeds:                 10000,
					MaxArticles:              10000000,
					MaxAITokens:              1000000000,
					MaxAIConcurrency:         10,
					MaxFeedFetchConcurrency:  20,
					MaxDBQueryConcurrency:    10,
					MaxMediaCacheConcurrency: 5,
					MaxRSSDiscoveryConcurrency: 5,
					MaxRSSPathCheckConcurrency: 3,
					MaxTranslationConcurrency: 5,
					MaxStorageMB:             10000,
				}
				db.CreateUserQuota(adminQuota)
				log.Printf("Admin user created: %s (password: %s)", adminUsername, adminPassword)
			}
		}
	}

	// Create template user if not exists
	templateUsername := os.Getenv("MRRSS_TEMPLATE_USERNAME")
	templateEmail := os.Getenv("MRRSS_TEMPLATE_EMAIL")
	templatePassword := os.Getenv("MRRSS_TEMPLATE_PASSWORD")
	if templateUsername == "" {
		templateUsername = "template"
	}
	if templateEmail == "" {
		templateEmail = "template@example.com"
	}
	if templatePassword == "" {
		templatePassword = "template"
	}
	_, err = db.GetUserByUsername(templateUsername)
	if err != nil {
		hashedPassword, err := auth.HashPassword(templatePassword)
		if err == nil {
			templateUser := &models.User{
				Username:     templateUsername,
				Email:        templateEmail,
				PasswordHash: hashedPassword,
				Role:         models.RoleTemplate,
				Status:       "active",
			}
			userID, err := db.CreateUser(templateUser)
			if err == nil {
				templateQuota := &models.UserQuota{
					UserID:                   userID,
					MaxFeeds:                 1000,
					MaxArticles:              1000000,
					MaxAITokens:              100000000,
					MaxAIConcurrency:         5,
					MaxFeedFetchConcurrency:  10,
					MaxDBQueryConcurrency:    5,
					MaxMediaCacheConcurrency: 3,
					MaxRSSDiscoveryConcurrency: 3,
					MaxRSSPathCheckConcurrency: 2,
					MaxTranslationConcurrency: 3,
					MaxStorageMB:             5000,
				}
				db.CreateUserQuota(templateQuota)
				log.Printf("Template user created: %s (password: %s)", templateUsername, templatePassword)
			}
		}
	}

	// Initialize AI profile provider
	profileProvider := ai.NewProfileProvider(db)
	translator := translation.NewDynamicTranslatorWithCache(db, db)
	translator.SetProfileProvider(profileProvider)

	fetcher := feed.NewFetcher(db)
	h := handlers.NewHandler(db, fetcher, translator, profileProvider)

	// API Routes
	log.Println("Setting up API routes...")
	apiMux := http.NewServeMux()
	routes.RegisterAPIRoutesWithConfig(apiMux, h, routes.ServerConfig(jwtManager))
	routes.RegisterAuthRoutes(apiMux, authHandler, jwtManager)

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

	// Create a caching file server
	fileServer := http.FileServer(http.FS(frontendFS))
	cachedFileServer := &cachedStaticHandler{handler: fileServer, fs: frontendFS}

	combinedHandler := &CombinedHandler{
		apiMux:     apiMux,
		fileServer: cachedFileServer,
	}

	// Wrap the combined handler with server middleware
	wrappedHandler := routes.WrapWithMiddleware(combinedHandler, routes.ServerConfig(jwtManager))

	log.Printf("Starting in headless server mode on http://%s:%s", *host, *port)

	// Start background scheduler
	// Use a context that we can cancel on shutdown
	bgCtx, bgCancel := context.WithCancel(context.Background())

	log.Println("Starting background scheduler...")
	go h.StartBackgroundScheduler(bgCtx)

	// Start Network Speed Detection (optional but good to have)
	// Run asynchronously in background - don't block server startup
	go func() {
		log.Println("Detecting network speed in background...")

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
