package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
)

const (
	writeRetryAttempts = 5
	writeRetryBaseWait = 25 * time.Millisecond
	writeRetryMaxWait  = 400 * time.Millisecond
)

// Exec serializes application-level writes and retries transient SQLite lock errors.
// Writes issued through Exec run at interactive priority: they skip ahead of
// queued background AI-pipeline writes.
func (db *DB) Exec(query string, args ...any) (sql.Result, error) {
	return db.ExecContext(context.Background(), query, args...)
}

// ExecContext serializes application-level writes and retries transient SQLite lock errors.
func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.execContextWithPriority(ctx, writePriorityInteractive, query, args...)
}

// execContextWithPriority executes a write statement at the given lock priority.
func (db *DB) execContextWithPriority(ctx context.Context, priority writePriority, query string, args ...any) (sql.Result, error) {
	var result sql.Result
	err := db.withWriteRetry(ctx, priority, func() error {
		var execErr error
		result, execErr = db.DB.ExecContext(ctx, query, args...)
		return execErr
	})
	return result, err
}

// WithWriteTx runs fn in a short write transaction guarded by the shared writer lock.
// The transaction runs at interactive priority. fn must use the provided
// transaction and must not call db.Exec, otherwise it can deadlock on the
// writer lock.
func (db *DB) WithWriteTx(ctx context.Context, fn func(*sql.Tx) error) error {
	return db.withWriteTxPriority(ctx, writePriorityInteractive, fn)
}

// withWriteTxPriority runs fn in a short write transaction guarded by the
// shared writer lock at the given priority.
func (db *DB) withWriteTxPriority(ctx context.Context, priority writePriority, fn func(*sql.Tx) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if fn == nil {
		return nil
	}

	return db.withWriteRetry(ctx, priority, func() error {
		tx, err := db.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		if err := fn(tx); err != nil {
			_ = tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			return err
		}
		return nil
	})
}

// withWriteRetry serializes application-level writes through the priority
// write scheduler and retries transient SQLite lock errors. The writer lock is
// released while backing off between retries so interactive writes can
// proceed during the backoff window.
func (db *DB) withWriteRetry(ctx context.Context, priority writePriority, fn func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}

	var err error
	for attempt := 0; attempt < writeRetryAttempts; attempt++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		db.writeSched.lock(priority)

		// Re-check ctx after (possibly) waiting for the writer lock: a
		// cancelled context should not execute the write at all.
		if ctxErr := ctx.Err(); ctxErr != nil {
			db.writeSched.unlock()
			return ctxErr
		}

		err = fn()
		db.writeSched.unlock()

		if err == nil || !isSQLiteBusyOrLocked(err) || attempt == writeRetryAttempts-1 {
			return err
		}

		wait := writeRetryDelay(attempt)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}

	return err
}

func writeRetryDelay(attempt int) time.Duration {
	delay := writeRetryBaseWait << attempt
	if delay > writeRetryMaxWait {
		return writeRetryMaxWait
	}
	return delay
}

func isSQLiteBusyOrLocked(err error) bool {
	if err == nil {
		return false
	}

	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code == sqlite3.ErrBusy ||
			sqliteErr.Code == sqlite3.ErrLocked ||
			sqliteErr.ExtendedCode == sqlite3.ErrNoExtended(sqlite3.ErrBusy) ||
			sqliteErr.ExtendedCode == sqlite3.ErrNoExtended(sqlite3.ErrLocked)
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "sqlite_busy") ||
		strings.Contains(msg, "sqlite_locked")
}
