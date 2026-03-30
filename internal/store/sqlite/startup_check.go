package sqlite

import (
	"fmt"
	"strings"
)

type StartupCheckResult struct {
	DriverName         string
	DataSourceName     string
	SQLiteVersion      string
	VecVersion         string
	JournalMode        string
	BusyTimeout        int
	ForeignKeysEnabled bool
	InMemory           bool
}

func (db *DB) StartupCheck() (StartupCheckResult, error) {
	db.startupOnce.Do(func() {
		result := StartupCheckResult{
			DriverName:     db.driverName,
			DataSourceName: db.dataSourceName,
			InMemory:       db.inMemory,
		}

		if err := db.Ping(); err != nil {
			db.startupErr = fmt.Errorf("ping sqlite database: %w", err)
			return
		}
		if err := db.QueryRow(`SELECT sqlite_version()`).Scan(&result.SQLiteVersion); err != nil {
			db.startupErr = fmt.Errorf("query sqlite version: %w", err)
			return
		}
		if err := db.QueryRow(`SELECT vec_version()`).Scan(&result.VecVersion); err != nil {
			db.startupErr = fmt.Errorf("query sqlite vec version: %w", err)
			return
		}

		var md5Value string
		if err := db.QueryRow(`SELECT MD5(?)`, "mavenrss").Scan(&md5Value); err != nil {
			db.startupErr = fmt.Errorf("verify MD5 function: %w", err)
			return
		}
		if md5Value != md5Hex("mavenrss") {
			db.startupErr = fmt.Errorf("verify MD5 function: unexpected value %q", md5Value)
			return
		}

		var foreignKeys int
		if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
			db.startupErr = fmt.Errorf("query foreign_keys pragma: %w", err)
			return
		}
		result.ForeignKeysEnabled = foreignKeys == 1
		if !result.ForeignKeysEnabled {
			db.startupErr = fmt.Errorf("query foreign_keys pragma: expected 1, got %d", foreignKeys)
			return
		}

		if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&result.BusyTimeout); err != nil {
			db.startupErr = fmt.Errorf("query busy_timeout pragma: %w", err)
			return
		}
		if result.BusyTimeout != defaultBusyTimeout {
			db.startupErr = fmt.Errorf("query busy_timeout pragma: expected %d, got %d", defaultBusyTimeout, result.BusyTimeout)
			return
		}

		if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&result.JournalMode); err != nil {
			db.startupErr = fmt.Errorf("query journal_mode pragma: %w", err)
			return
		}
		if !result.InMemory && !strings.EqualFold(result.JournalMode, "wal") {
			db.startupErr = fmt.Errorf("query journal_mode pragma: expected WAL, got %s", result.JournalMode)
			return
		}

		db.startupResult = result
	})

	return db.startupResult, db.startupErr
}
