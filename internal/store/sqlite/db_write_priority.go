package sqlite

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

// writePriority classifies a database write for lock scheduling.
type writePriority int

const (
	// writePriorityInteractive marks latency-sensitive, user-facing writes
	// such as login sessions, settings saves, and read/favorite marks.
	// Interactive writes skip ahead of queued background writes.
	writePriorityInteractive writePriority = iota

	// writePriorityBackground marks bulk AI-pipeline writes such as
	// embeddings, summaries, translations, and cluster merges. Background
	// writes yield to waiting interactive writes, but are protected from
	// indefinite starvation by backgroundWriteStarvationTimeout.
	writePriorityBackground
)

// defaultBackgroundWriteStarvationTimeout bounds how long background writers
// can be held back by a continuous stream of interactive writers before one is
// promoted, so AI pipelines cannot stall indefinitely.
const defaultBackgroundWriteStarvationTimeout = 15 * time.Second

// writeScheduler is an exclusive write lock with two priority classes.
//
// Interactive acquisitions proceed whenever the lock is free. Background
// acquisitions additionally wait while any interactive writer is queued,
// preventing background writers from barging ahead of latency-sensitive
// writes during AI pipeline bursts.
type writeScheduler struct {
	mu           sync.Mutex
	cond         *sync.Cond
	held         bool
	highWaiting  int
	lowWaiting   int
	starvationTimeout time.Duration

	// lowQueuedSince tracks when the background queue became non-empty,
	// approximating the oldest background waiter's arrival time.
	lowQueuedSince time.Time
	// lowPromoted lets background writers through despite waiting
	// interactive writers once starvation protection kicks in.
	lowPromoted bool
}

func newWriteScheduler() *writeScheduler {
	s := &writeScheduler{starvationTimeout: defaultBackgroundWriteStarvationTimeout}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// lock acquires the exclusive write lock at the given priority.
func (s *writeScheduler) lock(priority writePriority) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if priority == writePriorityBackground {
		if s.lowWaiting == 0 {
			s.lowQueuedSince = time.Now()
			s.lowPromoted = false
		}
		s.lowWaiting++
		defer func() { s.lowWaiting-- }()
		for s.held || (s.highWaiting > 0 && !s.promoteLowLocked()) {
			s.cond.Wait()
		}
	} else {
		s.highWaiting++
		defer func() { s.highWaiting-- }()
		for s.held {
			s.cond.Wait()
		}
	}
	s.held = true
}

// unlock releases the exclusive write lock and wakes all waiters.
func (s *writeScheduler) unlock() {
	s.mu.Lock()
	s.held = false
	s.mu.Unlock()
	s.cond.Broadcast()
}

// promoteLowLocked reports whether background writers may bypass interactive
// waiters due to starvation protection. Callers must hold s.mu.
func (s *writeScheduler) promoteLowLocked() bool {
	if s.lowPromoted {
		return true
	}
	if !s.lowQueuedSince.IsZero() && time.Since(s.lowQueuedSince) >= s.starvationTimeout {
		s.lowPromoted = true
	}
	return s.lowPromoted
}

// ExecBackground executes a single write statement at background priority.
// Background writes yield to interactive writes (login sessions, settings,
// status marks) that are waiting for the SQLite write lock.
func (db *DB) ExecBackground(query string, args ...any) (sql.Result, error) {
	return db.execContextWithPriority(context.Background(), writePriorityBackground, query, args...)
}

// WithBackgroundWriteTx runs fn in a short write transaction guarded by the
// shared writer lock at background priority. See ExecBackground for scheduling
// semantics; fn must use the provided transaction and must not call db.Exec.
func (db *DB) WithBackgroundWriteTx(ctx context.Context, fn func(*sql.Tx) error) error {
	return db.withWriteTxPriority(ctx, writePriorityBackground, fn)
}

// execWithPriority executes a single write statement at an explicit priority.
// It is the shared implementation for the interactive/background method pairs.
func (db *DB) execWithPriority(priority writePriority, query string, args ...any) (sql.Result, error) {
	return db.execContextWithPriority(context.Background(), priority, query, args...)
}
