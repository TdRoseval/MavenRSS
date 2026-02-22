package core

import (
	"testing"

	"MavenRSS/internal/database"
	"MavenRSS/internal/feed"
)

func TestNewHandler_ConstructsHandler(t *testing.T) {
	db, err := database.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("db Init failed: %v", err)
	}

	f := feed.NewFetcher(db)
	h := NewHandler(db, f, nil, nil)

	if h.DB == nil {
		t.Fatal("Handler DB is nil")
	}
	if h.Fetcher == nil {
		t.Fatal("Handler Fetcher is nil")
	}
	if h.DiscoveryService == nil {
		t.Fatal("DiscoveryService should be initialized")
	}
}
