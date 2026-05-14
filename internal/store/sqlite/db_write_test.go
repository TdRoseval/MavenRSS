package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"MavenRSS/internal/models"

	sqlite3 "github.com/mattn/go-sqlite3"
)

func TestIsSQLiteBusyOrLocked(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "database locked text", err: errors.New("database is locked"), want: true},
		{name: "table locked text", err: errors.New("database table is locked"), want: true},
		{name: "sqlite busy text", err: errors.New("SQLITE_BUSY: busy"), want: true},
		{name: "sqlite locked text", err: errors.New("SQLITE_LOCKED: locked"), want: true},
		{name: "sqlite busy error", err: sqlite3.Error{Code: sqlite3.ErrBusy}, want: true},
		{name: "sqlite locked error", err: sqlite3.Error{Code: sqlite3.ErrLocked}, want: true},
		{name: "other error", err: errors.New("constraint failed"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSQLiteBusyOrLocked(tt.err); got != tt.want {
				t.Fatalf("isSQLiteBusyOrLocked() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithWriteTxCommitsAndRollsBack(t *testing.T) {
	db := newWriteTestDB(t)
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE write_tx_test (id INTEGER PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatalf("create table error: %v", err)
	}

	if err := db.WithWriteTx(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(context.Background(), `INSERT INTO write_tx_test (id, value) VALUES (1, 'committed')`)
		return err
	}); err != nil {
		t.Fatalf("WithWriteTx commit error: %v", err)
	}

	var value string
	if err := db.QueryRow(`SELECT value FROM write_tx_test WHERE id = 1`).Scan(&value); err != nil {
		t.Fatalf("query committed row error: %v", err)
	}
	if value != "committed" {
		t.Fatalf("committed value = %q, want committed", value)
	}

	errRollback := errors.New("rollback please")
	if err := db.WithWriteTx(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(context.Background(), `INSERT INTO write_tx_test (id, value) VALUES (2, 'rolled-back')`); err != nil {
			return err
		}
		return errRollback
	}); !errors.Is(err, errRollback) {
		t.Fatalf("WithWriteTx rollback error = %v, want %v", err, errRollback)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM write_tx_test WHERE id = 2`).Scan(&count); err != nil {
		t.Fatalf("query rolled back row error: %v", err)
	}
	if count != 0 {
		t.Fatalf("rolled back row count = %d, want 0", count)
	}
}

func TestConcurrentAIClusterWritesDoNotReturnLocked(t *testing.T) {
	db := newWriteTestDB(t)
	defer db.Close()

	feedID := createWriteTestFeed(t, db)
	articleIDs := createWriteTestArticles(t, db, feedID, 24, "cluster-write")

	var wg sync.WaitGroup
	errs := make(chan error, len(articleIDs))
	for _, articleID := range articleIDs {
		articleID := articleID
		wg.Add(1)
		go func() {
			defer wg.Done()

			if err := db.UpdateArticleEmbeddings(articleID, nil, nil); err != nil {
				errs <- fmt.Errorf("update article embedding %d: %w", articleID, err)
				return
			}
			clusterID, err := db.CreateStandaloneClusterForArticle(1, articleID, articleID%2 == 0)
			if err != nil {
				errs <- fmt.Errorf("create standalone cluster %d: %w", articleID, err)
				return
			}
			if err := db.UpdateClusterEmbeddings(clusterID, nil, nil); err != nil {
				errs <- fmt.Errorf("update cluster embedding %d: %w", clusterID, err)
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		if isSQLiteBusyOrLocked(err) || strings.Contains(strings.ToLower(err.Error()), "locked") {
			t.Fatalf("concurrent write returned lock error: %v", err)
		}
		if err != nil {
			t.Fatalf("concurrent write error: %v", err)
		}
	}
}

func TestSaveArticlesAndClusterWritesDoNotReturnLocked(t *testing.T) {
	db := newWriteTestDB(t)
	defer db.Close()

	feedID := createWriteTestFeed(t, db)
	articleIDs := createWriteTestArticles(t, db, feedID, 18, "save-cluster")

	errs := make(chan error, 2)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		articles := make([]*models.Article, 40)
		for i := range articles {
			articles[i] = &models.Article{
				UserID:      1,
				FeedID:      feedID,
				Title:       fmt.Sprintf("Concurrent Save %d", i),
				URL:         fmt.Sprintf("https://example.com/concurrent-save-%d", i),
				PublishedAt: time.Now().Add(time.Duration(i) * time.Second),
			}
		}
		if err := db.SaveArticles(context.Background(), articles); err != nil {
			errs <- fmt.Errorf("save articles: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, articleID := range articleIDs {
			if _, err := db.CreateStandaloneClusterForArticle(1, articleID, false); err != nil {
				errs <- fmt.Errorf("cluster article %d: %w", articleID, err)
				return
			}
		}
	}()

	wg.Wait()
	close(errs)
	for err := range errs {
		if isSQLiteBusyOrLocked(err) || strings.Contains(strings.ToLower(err.Error()), "locked") {
			t.Fatalf("save/cluster returned lock error: %v", err)
		}
		if err != nil {
			t.Fatalf("save/cluster error: %v", err)
		}
	}
}

func newWriteTestDB(t *testing.T) *DB {
	t.Helper()

	db, err := NewDB(filepath.Join(t.TempDir(), "write.db"))
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	if err := db.Init(); err != nil {
		db.Close()
		t.Fatalf("Init error: %v", err)
	}
	return db
}

func createWriteTestFeed(t *testing.T, db *DB) int64 {
	t.Helper()

	result, err := db.Exec(`INSERT INTO feeds (user_id, title, url, last_updated) VALUES (1, 'Write Test Feed', 'https://example.com/write-feed.xml', ?)`, time.Now())
	if err != nil {
		t.Fatalf("insert feed error: %v", err)
	}
	feedID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("feed LastInsertId error: %v", err)
	}
	return feedID
}

func createWriteTestArticles(t *testing.T, db *DB, feedID int64, count int, prefix string) []int64 {
	t.Helper()

	ids := make([]int64, 0, count)
	for i := 0; i < count; i++ {
		result, err := db.Exec(
			`INSERT INTO articles (user_id, feed_id, title, url, published_at, summary, unique_id) VALUES (1, ?, ?, ?, ?, ?, ?)`,
			feedID,
			fmt.Sprintf("%s article %d", prefix, i),
			fmt.Sprintf("https://example.com/%s/%d", prefix, i),
			time.Now().Add(time.Duration(i)*time.Minute),
			fmt.Sprintf("%s summary %d", prefix, i),
			fmt.Sprintf("%s-%d", prefix, i),
		)
		if err != nil {
			t.Fatalf("insert article error: %v", err)
		}
		articleID, err := result.LastInsertId()
		if err != nil {
			t.Fatalf("article LastInsertId error: %v", err)
		}
		ids = append(ids, articleID)
	}
	return ids
}
