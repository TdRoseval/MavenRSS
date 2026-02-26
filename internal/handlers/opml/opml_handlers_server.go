//go:build server

package opml

import (
	"io"
	"log"
	"net/http"
	"strings"

	"MavenRSS/internal/handlers/core"
	"MavenRSS/internal/handlers/response"
	"MavenRSS/internal/models"
	"MavenRSS/internal/opml"
)

// HandleOPMLImport handles OPML file import for server mode.
// @Summary      Import OPML file
// @Description  Import feeds from an OPML file (server mode - requires file upload)
// @Tags         opml
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "OPML file"
// @Success      200  {object}  map[string]interface{}  "Import result (success, imported, total)"
// @Failure      400  {object}  map[string]string  "Bad request (invalid file or format)"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /opml/import [post]
func HandleOPMLImport(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	log.Printf("HandleOPMLImport: ContentLength: %d", r.ContentLength)

	// Get user ID from request
	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		response.Error(w, nil, http.StatusUnauthorized)
		return
	}

	// Parse multipart form
	err := r.ParseMultipartForm(32 << 20) // 32MB max
	if err != nil {
		log.Printf("Error parsing multipart form: %v", err)
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	// Get the file
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("Error getting file: %v", err)
		response.Error(w, err, http.StatusBadRequest)
		return
	}
	defer file.Close()

	log.Printf("Received file: %s, size: %d", header.Filename, header.Size)

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	// Parse OPML
	feeds, err := opml.Parse(strings.NewReader(string(content)))
	if err != nil {
		log.Printf("Error parsing OPML: %v", err)
		response.Error(w, err, http.StatusBadRequest)
		return
	}

	log.Printf("Parsed %d feeds from OPML", len(feeds))

	// Import feeds
	imported := 0
	for _, feed := range feeds {
		feed.UserID = userID
		_, err := h.DB.AddFeed(&feed)
		if err != nil {
			log.Printf("Error importing feed %s: %v", feed.URL, err)
			continue
		}
		imported++
	}

	log.Printf("Successfully imported %d feeds", imported)

	// Return success response
	response.JSON(w, map[string]interface{}{
		"success":  true,
		"imported": imported,
		"total":    len(feeds),
	})
}

// HandleOPMLImportDialog is not available in server mode.
// @Summary      Import dialog (not available in server mode)
// @Description  File dialog operations are not available in server mode. Use /api/opml/import with file upload instead
// @Tags         opml
// @Accept       json
// @Produce      json
// @Success      501  {object}  map[string]string  "Not implemented error"
// @Router       /opml/import/dialog [post]
func HandleOPMLImportDialog(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	log.Printf("File dialog operations are not available in server mode")
	w.WriteHeader(http.StatusNotImplemented)
	response.JSON(w, map[string]interface{}{
		"error": "File dialog operations are not available in server mode. Use /api/opml/import endpoint with file upload instead.",
	})
}

// HandleOPMLExport handles OPML export for server mode.
// @Summary      Export OPML file
// @Description  Export current user's feeds as an OPML file (server mode - direct download)
// @Tags         opml
// @Accept       json
// @Produce      xml
// @Success      200  {string}  string  "OPML XML file"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /opml/export [get]
func HandleOPMLExport(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	userID, ok := core.GetUserIDFromRequest(r)
	if !ok {
		response.Error(w, nil, http.StatusUnauthorized)
		return
	}

	feeds, err := h.DB.GetFeedsForUser(userID)
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	localFeeds := make([]models.Feed, 0)
	for _, feed := range feeds {
		if !feed.IsFreshRSSSource {
			localFeeds = append(localFeeds, feed)
		}
	}

	log.Printf("[OPML Export] User %d: Exporting %d local feeds (excluded %d FreshRSS feeds)",
		userID, len(localFeeds), len(feeds)-len(localFeeds))

	data, err := opml.Generate(localFeeds)
	if err != nil {
		response.Error(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", "attachment; filename=subscriptions.opml")
	w.Write(data)
}

// HandleOPMLExportDialog is not available in server mode.
// @Summary      Export dialog (not available in server mode)
// @Description  File dialog operations are not available in server mode. Use /api/opml/export with direct download instead
// @Tags         opml
// @Accept       json
// @Produce      json
// @Success      501  {object}  map[string]string  "Not implemented error"
// @Router       /opml/export/dialog [post]
func HandleOPMLExportDialog(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	log.Printf("File dialog not available in server mode")
	w.WriteHeader(http.StatusNotImplemented)
	response.JSON(w, map[string]interface{}{
		"error": "File dialog not available in server mode. Use /api/opml/export endpoint with direct download instead.",
	})
}
