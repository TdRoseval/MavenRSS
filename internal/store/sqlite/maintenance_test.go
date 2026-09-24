package sqlite_test

import (
	"testing"
)

func TestCheckpointWALRejectsUnsupportedMode(t *testing.T) {
	db := setupTestDB(t)
	if _, err := db.CheckpointWAL("NOT_A_MODE"); err == nil {
		t.Fatal("CheckpointWAL() accepted an unsupported mode")
	}
}

func TestCheckpointWALAndVacuum(t *testing.T) {
	db := setupTestDB(t)
	result, err := db.CheckpointWAL("PASSIVE")
	if err != nil {
		t.Fatalf("CheckpointWAL(PASSIVE) error: %v", err)
	}
	// SQLite reports -1 for frame counters when the in-memory test database has
	// no WAL file; Busy must still be a valid 0/1 result.
	if result.Busy < 0 || result.Busy > 1 || result.LogFrames < -1 || result.CheckpointedFrames < -1 {
		t.Fatalf("invalid checkpoint result: %+v", result)
	}
	if err := db.Vacuum(); err != nil {
		t.Fatalf("Vacuum() error: %v", err)
	}
}
