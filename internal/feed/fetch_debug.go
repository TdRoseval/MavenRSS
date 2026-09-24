package feed

import (
	"fmt"
	"log"
	"time"

	"MavenRSS/internal/utils"
)

// DebugTimer is a simple timer for performance debugging
type DebugTimer struct {
	name      string
	start     time.Time
	last      time.Time
	enabled   bool
	stage     int
	totalTime time.Duration
}

// NewDebugTimer creates a new debug timer
func NewDebugTimer(name string, enabled bool) *DebugTimer {
	now := time.Now()
	return &DebugTimer{
		name:    name,
		start:   now,
		last:    now,
		enabled: enabled,
		stage:   0,
	}
}

// Stage marks a stage with timing
func (dt *DebugTimer) Stage(stageName string) {
	if !dt.enabled {
		return
	}
	now := time.Now()
	sinceLast := now.Sub(dt.last)
	sinceStart := now.Sub(dt.start)
	dt.stage++
	dt.last = now

	log.Printf("[DEBUG %s] Stage %d: %s - Since last: %v, Total: %v",
		dt.name, dt.stage, stageName, sinceLast, sinceStart)
}

// End marks the end of timing
func (dt *DebugTimer) End() {
	if !dt.enabled {
		return
	}
	dt.totalTime = time.Since(dt.start)
	log.Printf("[DEBUG %s] COMPLETED - Total time: %v, Stages: %d",
		dt.name, dt.totalTime, dt.stage)
}

// IsEnabled returns whether debug timing is enabled
func (dt *DebugTimer) IsEnabled() bool {
	return dt.enabled
}

// LogWithTime logs a message with elapsed time
func (dt *DebugTimer) LogWithTime(format string, args ...interface{}) {
	if !dt.enabled {
		return
	}
	elapsed := time.Since(dt.start)
	sinceLast := time.Since(dt.last)
	dt.last = time.Now()

	msg := fmt.Sprintf(format, args...)
	log.Printf("[DEBUG %s] [%v total, %v since last] %s",
		dt.name, elapsed, sinceLast, msg)
}

// debugEnabled controls whether feed fetching debug logging is enabled
// Set to true to enable detailed performance logging
var debugEnabled = utils.DebugLoggingEnabled()

// shouldEnableDebugLogging checks if debug logging should be enabled for this feed
func shouldEnableDebugLogging(feedURL string) bool {
	if !debugEnabled {
		return false
	}
	// Enable debug for specific feed that's having issues
	return feedURL == "https://rss.buzzsprout.com/1982525.rss"
}
