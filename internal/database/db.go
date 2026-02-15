package database

import (
	"database/sql"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps sql.DB with initialization state tracking.
type DB struct {
	*sql.DB
	ready chan struct{}
	once  sync.Once
}

// NewDB creates a new database connection with optimized settings.
func NewDB(dataSourceName string) (*DB, error) {
	// Add busy_timeout to prevent "database is locked" errors
	// Also enable WAL mode for better concurrency
	// Add performance optimizations: increase cache size, set synchronous=NORMAL
	if !strings.Contains(dataSourceName, "?") {
		dataSourceName += "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=cache_size(-64000)&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(30000000000)&_pragma=locking_mode(NORMAL)"
	} else {
		dataSourceName += "&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=cache_size(-64000)&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(30000000000)&_pragma=locking_mode(NORMAL)"
	}

	db, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, err
	}

	// Set connection pool limits for better performance
	// Optimized for read-heavy workloads like RSS readers
	db.SetMaxOpenConns(5)  // SQLite works best with low connection count in WAL mode
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(1 * time.Hour)
	db.SetConnMaxIdleTime(30 * time.Minute)

	return &DB{
		DB:    db,
		ready: make(chan struct{}),
	}, nil
}

// WaitForReady blocks until the database is initialized.
func (db *DB) WaitForReady() {
	<-db.ready
}
