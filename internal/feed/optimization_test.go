package feed

import (
	"strconv"
	"sync"
	"testing"

	"MavenRSS/internal/ai"
	"MavenRSS/internal/models"
	"MavenRSS/internal/store/sqlite"
)

// TestShouldProcessCacheInvalidatesOnSettingChange verifies that the cached
// ShouldProcess result is invalidated when a setting is written, so callers
// never observe a stale per-user decision after the user changes configuration.
func TestShouldProcessCacheInvalidatesOnSettingChange(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	userID := int64(9901)

	if _, err := db.Exec(`INSERT INTO users (id, username, password_hash, email, role, status) VALUES (?, 'u9901', 'hash', 'u9901@test.com', 'user', 'active')`, userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	mustEnableAIEnhancedProcessing(t, db, userID)

	if !ShouldProcess(db, userID) {
		t.Fatal("ShouldProcess() = false, want true with full config")
	}
	// A second call with no settings change must stay consistent (either cached
	// or recomputed, the observable result must not change).
	if !ShouldProcess(db, userID) {
		t.Fatal("ShouldProcess() = false on second call without changes")
	}

	// Disabling AI enhanced mode bumps the settings revision, which must
	// invalidate the cached result.
	mustSetUserSetting(t, db, userID, "ai_enhanced_mode", "false")
	if ShouldProcess(db, userID) {
		t.Fatal("ShouldProcess() = true, want false after disabling ai_enhanced_mode")
	}
}

// TestAddAIUsageConcurrentDoesNotLoseUpdates exercises the serialized
// read-modify-write path added for concurrent stage-2 recommendation scoring.
func TestAddAIUsageConcurrentDoesNotLoseUpdates(t *testing.T) {
	db := newAIEnhancedModeTestDB(t)
	manager := NewAIEnhancedManager(db)
	defer manager.Stop()

	const userID = int64(77)
	const goroutines = 32
	const perGoroutine = 50

	if _, err := db.Exec(`INSERT INTO users (id, username, password_hash, email, role, status) VALUES (?, 'u77', 'hash', 'u77@test.com', 'user', 'active')`, userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				manager.addAIUsage(userID, 1)
			}
		}()
	}
	wg.Wait()

	usageStr, err := db.GetSettingWithFallback(userID, "ai_usage_tokens")
	if err != nil {
		t.Fatalf("get usage: %v", err)
	}
	usage, err := strconv.ParseInt(usageStr, 10, 64)
	if err != nil {
		t.Fatalf("parse usage %q: %v", usageStr, err)
	}
	want := int64(goroutines * perGoroutine)
	if usage != want {
		t.Fatalf("usage = %d, want %d", usage, want)
	}
}

// TestCleanupManagerShouldVacuumThrottles verifies VACUUM is rate-limited.
func TestCleanupManagerShouldVacuumThrottles(t *testing.T) {
	cm := NewCleanupManager(&Fetcher{})

	if !cm.shouldVacuum() {
		t.Fatal("first shouldVacuum() = false, want true")
	}
	if cm.shouldVacuum() {
		t.Fatal("second shouldVacuum() = true, want false within vacuumInterval")
	}
}

// TestRunRecommendationStageTwoConcurrentPreservesResults exercises the
// concurrent stage-2 scoring loop. Using candidates with empty content hits the
// no-LLM fallback branch, so we can verify the concurrency bookkeeping
// (semaphore, WaitGroup, index-ordered result writes, failure propagation)
// without any network.
func TestRunRecommendationStageTwoConcurrentPreservesResults(t *testing.T) {
	manager := &AIEnhancedManager{
		recommendationStatusByUser: make(map[int64]DailyRecommendationTaskStatus),
	}

	const userID = int64(1)
	const profileID = int64(5)
	const n = 20

	candidates := make([]rankedRecommendationCandidate, n)
	for i := 0; i < n; i++ {
		candidates[i] = rankedRecommendationCandidate{
			Candidate: sqlite.DailyRecommendationCandidate{
				Cluster: models.Cluster{ID: int64(i + 1)},
				// MergedContent/Synopsis left empty so scoring falls back without LLM.
			},
			RecallScore: float64(i + 1),
		}
	}

	results, ok := manager.runRecommendationStageTwo(userID, candidates, &ai.ClientConfig{}, profileID)
	if !ok {
		t.Fatal("runRecommendationStageTwo() = not ok, want ok")
	}
	if len(results) != n {
		t.Fatalf("len(results) = %d, want %d", len(results), n)
	}

	for i, r := range results {
		if r.Candidate.Cluster.ID != candidates[i].Candidate.Cluster.ID {
			t.Fatalf("result %d cluster id = %d, want %d (order must be preserved)", i, r.Candidate.Cluster.ID, candidates[i].Candidate.Cluster.ID)
		}
		if r.ProfileID != profileID {
			t.Fatalf("result %d ProfileID = %d, want %d", i, r.ProfileID, profileID)
		}
		if r.FinalScore != candidates[i].RecallScore {
			t.Fatalf("result %d FinalScore = %f, want %f (fallback keeps RecallScore)", i, r.FinalScore, candidates[i].RecallScore)
		}
	}
}