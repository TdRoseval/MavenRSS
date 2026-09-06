package sqlite

import (
	"testing"
	"time"
)

func TestWriteSchedulerInteractivePreemptsBackground(t *testing.T) {
	s := newWriteScheduler()
	order := make(chan string, 2)

	// Main goroutine acquires the lock at background priority first.
	s.lock(writePriorityBackground)

	// Queue a background waiter, then an interactive waiter.
	go func() {
		s.lock(writePriorityBackground)
		order <- "background"
		s.unlock()
	}()
	go func() {
		s.lock(writePriorityInteractive)
		order <- "interactive"
		s.unlock()
	}()

	waitForSchedulerState(t, s, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.highWaiting == 1 && s.lowWaiting == 1
	})

	// Release the main lock. The interactive waiter must be served first
	// even though the background waiter queued earlier.
	s.unlock()

	first := receiveOrder(t, order)
	if first != "interactive" {
		t.Fatalf("interactive should preempt background, got %q first", first)
	}
	if second := receiveOrder(t, order); second != "background" {
		t.Fatalf("expected background second, got %q", second)
	}
}

func TestWriteSchedulerBackgroundServesWhenIdle(t *testing.T) {
	s := newWriteScheduler()

	s.lock(writePriorityBackground)
	held := s.held
	s.unlock()
	if !held {
		t.Fatal("background lock should be held when acquired")
	}

	s.lock(writePriorityInteractive)
	held = s.held
	s.unlock()
	if !held {
		t.Fatal("interactive lock should be held when acquired")
	}
}

func TestWriteSchedulerStarvationPromotion(t *testing.T) {
	s := newWriteScheduler()
	s.starvationTimeout = 10 * time.Millisecond

	s.mu.Lock()
	s.lowQueuedSince = time.Now().Add(-2 * s.starvationTimeout)
	got := s.promoteLowLocked()
	s.mu.Unlock()

	if !got {
		t.Fatal("promoteLowLocked should promote a long-queued background writer")
	}
}

func TestWriteSchedulerNoPrematurePromotion(t *testing.T) {
	s := newWriteScheduler()
	s.starvationTimeout = time.Hour

	s.mu.Lock()
	s.lowQueuedSince = time.Now()
	got := s.promoteLowLocked()
	s.mu.Unlock()

	if got {
		t.Fatal("promoteLowLocked should not promote before the timeout elapses")
	}
}

func waitForSchedulerState(t *testing.T, s *writeScheduler, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for scheduler state")
}

func receiveOrder(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for waiter to acquire the lock")
		return ""
	}
}
