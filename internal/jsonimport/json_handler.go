package jsonimport

import (
	"MavenRSS/internal/models"
	"encoding/json"
	"errors"
	"io"
	"log"
)

// FeedExport represents the JSON export format for feeds
type FeedExport struct {
	Version int           `json:"version"`
	Feeds   []models.Feed `json:"feeds"`
}

const (
	// Current export format version
	ExportVersion = 1
)

// Parse parses JSON import data and returns feeds
func Parse(r io.Reader) ([]models.Feed, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	log.Printf("JSON Parse: Read %d bytes", len(content))

	if len(content) == 0 {
		return nil, errors.New("file content is empty")
	}

	var export FeedExport
	if err := json.Unmarshal(content, &export); err != nil {
		log.Printf("JSON Parse: Unmarshal error: %v", err)
		return nil, err
	}

	if export.Version == 0 {
		// Handle old format without version (direct array)
		var feeds []models.Feed
		if err := json.Unmarshal(content, &feeds); err != nil {
			return nil, err
		}
		log.Printf("JSON Parse: Found %d feeds (legacy format)", len(feeds))
		return feeds, nil
	}

	log.Printf("JSON Parse: Found %d feeds (version %d)", len(export.Feeds), export.Version)
	return export.Feeds, nil
}

// Generate generates JSON export data from feeds
func Generate(feeds []models.Feed) ([]byte, error) {
	export := FeedExport{
		Version: ExportVersion,
		Feeds:   feeds,
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return nil, err
	}

	log.Printf("JSON Generate: Generated %d bytes for %d feeds", len(data), len(feeds))
	return data, nil
}
