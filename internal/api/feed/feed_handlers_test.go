package feed_test

import (
	"testing"

	"MavenRSS/internal/store/sqlite"
	ff "MavenRSS/internal/feed"
	"MavenRSS/internal/api/core"
)

func setupHandler(t *testing.T) *core.Handler {
	t.Helper()
	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("db Init error: %v", err)
	}
	f := ff.NewFetcher(db)
	return core.NewHandler(db, f, nil, nil)
}
