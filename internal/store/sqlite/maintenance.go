package sqlite

import (
	"context"
	"fmt"
	"log"
	"time"
)

const minimumVacuumInterval = 6 * time.Hour

// WALCheckpointResult contains the counters returned by SQLite's
// wal_checkpoint pragma. Busy is non-zero when at least one reader prevented
// the requested checkpoint from completing fully.
type WALCheckpointResult struct {
	Busy               int
	LogFrames          int
	CheckpointedFrames int
	Duration           time.Duration
}

// CheckpointWAL runs a serialized SQLite WAL checkpoint and records its
// result. The mode is intentionally restricted to SQLite's supported modes so
// callers cannot inject SQL through the PRAGMA expression.
func (db *DB) CheckpointWAL(mode string) (WALCheckpointResult, error) {
	if mode != "PASSIVE" && mode != "FULL" && mode != "RESTART" && mode != "TRUNCATE" {
		return WALCheckpointResult{}, fmt.Errorf("unsupported WAL checkpoint mode %q", mode)
	}

	db.WaitForReady()
	started := time.Now()
	var result WALCheckpointResult
	err := db.withWriteRetry(context.Background(), writePriorityBackground, func() error {
		return db.DB.QueryRow("PRAGMA wal_checkpoint("+mode+")").Scan(
			&result.Busy,
			&result.LogFrames,
			&result.CheckpointedFrames,
		)
	})
	result.Duration = time.Since(started)
	if err != nil {
		log.Printf("SQLite WAL checkpoint failed: mode=%s busy=%d log_frames=%d checkpointed_frames=%d duration_ms=%d error=%v",
			mode, result.Busy, result.LogFrames, result.CheckpointedFrames, result.Duration.Milliseconds(), err)
		return result, err
	}
	log.Printf("SQLite WAL checkpoint: mode=%s busy=%d log_frames=%d checkpointed_frames=%d duration_ms=%d",
		mode, result.Busy, result.LogFrames, result.CheckpointedFrames, result.Duration.Milliseconds())
	return result, nil
}

// Vacuum runs VACUUM through the same application-level writer scheduler as
// other maintenance operations and never hides an error from its caller.
func (db *DB) Vacuum() error {
	db.WaitForReady()
	db.maintenanceMu.Lock()
	defer db.maintenanceMu.Unlock()
	if !db.lastVacuumAt.IsZero() && time.Since(db.lastVacuumAt) < minimumVacuumInterval {
		log.Printf("SQLite VACUUM skipped: last_run=%s minimum_interval=%s",
			db.lastVacuumAt.Format(time.RFC3339), minimumVacuumInterval)
		return nil
	}

	started := time.Now()
	err := db.withWriteRetry(context.Background(), writePriorityBackground, func() error {
		_, err := db.DB.Exec("VACUUM")
		return err
	})
	if err != nil {
		log.Printf("SQLite VACUUM failed: duration_ms=%d error=%v", time.Since(started).Milliseconds(), err)
		return err
	}
	db.lastVacuumAt = time.Now()
	log.Printf("SQLite VACUUM completed: duration_ms=%d", time.Since(started).Milliseconds())
	return nil
}
