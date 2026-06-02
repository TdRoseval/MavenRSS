package sqlite

import (
	"crypto/md5"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	sqlite3 "github.com/mattn/go-sqlite3"
)

const (
	defaultBusyTimeout = 30000
	defaultCacheSize   = -64000
	defaultMmapSize    = 30000000000

	fileDriverName   = "mavenrss-sqlite3"
	memoryDriverName = "mavenrss-sqlite3-memory"
)

var (
	fileDriverOnce   sync.Once
	memoryDriverOnce sync.Once
	vectorDriverOnce sync.Once
	memoryDBCounter  uint64
)

type DB struct {
	*sql.DB
	writeMu                     sync.Mutex
	ready                       chan struct{}
	once                        sync.Once
	startupOnce                 sync.Once
	startupResult               StartupCheckResult
	startupErr                  error
	driverName                  string
	dataSourceName              string
	inMemory                    bool
	clusterFeedFirstPageCache   map[int64]map[string]ClusterFeedFirstPageCacheEntry
	clusterFeedFirstPageCacheMu sync.RWMutex
}

func md5Hex(input string) string {
	hash := md5.Sum([]byte(input))
	return fmt.Sprintf("%x", hash)
}

func md5Func(input string) string {
	return md5Hex(input)
}

func NewDB(dataSourceName string) (*DB, error) {
	normalizedDSN, inMemory := normalizeDataSourceName(dataSourceName)
	driverName := fileDriverName
	if inMemory {
		driverName = memoryDriverName
	}
	registerDriver(driverName, inMemory)

	db, err := sql.Open(driverName, normalizedDSN)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(1 * time.Hour)
	db.SetConnMaxIdleTime(30 * time.Minute)

	return &DB{
		DB:             db,
		ready:          make(chan struct{}),
		driverName:     driverName,
		dataSourceName: normalizedDSN,
		inMemory:       inMemory,
	}, nil
}

func normalizeDataSourceName(dataSourceName string) (string, bool) {
	normalized := stripLegacyPragmaParameters(strings.TrimSpace(dataSourceName))
	if normalized == "" {
		return normalized, false
	}
	if normalized == ":memory:" {
		return fmt.Sprintf("file:mavenrss-memory-%d?mode=memory&cache=shared", atomic.AddUint64(&memoryDBCounter, 1)), true
	}
	if isMemoryURI(normalized) {
		return ensureQueryValue(normalized, "cache", "shared"), true
	}
	return normalized, false
}

func isMemoryURI(dataSourceName string) bool {
	_, query, found := strings.Cut(dataSourceName, "?")
	if !found {
		return false
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		return false
	}
	return strings.EqualFold(values.Get("mode"), "memory")
}

func stripLegacyPragmaParameters(dataSourceName string) string {
	base, query, found := strings.Cut(dataSourceName, "?")
	if !found {
		return dataSourceName
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		return dataSourceName
	}
	if len(values["_pragma"]) == 0 {
		return dataSourceName
	}
	delete(values, "_pragma")
	encoded := values.Encode()
	if encoded == "" {
		return base
	}
	return base + "?" + encoded
}

func ensureQueryValue(dataSourceName, key, value string) string {
	base, query, found := strings.Cut(dataSourceName, "?")
	values := url.Values{}
	if found {
		parsed, err := url.ParseQuery(query)
		if err != nil {
			return dataSourceName
		}
		values = parsed
	}
	if values.Get(key) == "" {
		values.Set(key, value)
	}
	encoded := values.Encode()
	if encoded == "" {
		return base
	}
	return base + "?" + encoded
}

func registerDriver(driverName string, inMemory bool) {
	switch driverName {
	case memoryDriverName:
		memoryDriverOnce.Do(func() {
			registerSQLiteDriver(driverName, inMemory)
		})
	default:
		fileDriverOnce.Do(func() {
			registerSQLiteDriver(driverName, inMemory)
		})
	}
}

func registerSQLiteDriver(driverName string, inMemory bool) {
	vectorDriverOnce.Do(sqlite_vec.Auto)
	sql.Register(driverName, &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			if err := conn.RegisterFunc("MD5", md5Func, true); err != nil {
				return fmt.Errorf("register MD5 function: %w", err)
			}
			return applyConnectionPragmas(conn, inMemory)
		},
	})
}

func applyConnectionPragmas(conn *sqlite3.SQLiteConn, inMemory bool) error {
	pragmas := []string{
		fmt.Sprintf("PRAGMA busy_timeout = %d", defaultBusyTimeout),
		fmt.Sprintf("PRAGMA cache_size = %d", defaultCacheSize),
		"PRAGMA synchronous = NORMAL",
		"PRAGMA temp_store = MEMORY",
		fmt.Sprintf("PRAGMA mmap_size = %d", defaultMmapSize),
		"PRAGMA locking_mode = NORMAL",
		"PRAGMA foreign_keys = ON",
	}
	if !inMemory {
		pragmas = append([]string{"PRAGMA journal_mode = WAL"}, pragmas...)
	}
	for _, pragma := range pragmas {
		if _, err := conn.Exec(pragma, nil); err != nil {
			return fmt.Errorf("apply %q: %w", pragma, err)
		}
	}
	return nil
}

func (db *DB) WaitForReady() {
	<-db.ready
}
