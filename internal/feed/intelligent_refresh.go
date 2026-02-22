package feed

import (
	"MavenRSS/internal/database"
	"MavenRSS/internal/models"
	"math"
	"time"
)

const (
	// Minimum refresh interval (5 minutes)
	MinRefreshInterval = 5 * time.Minute
	// Maximum refresh interval (24 hours)
	MaxRefreshInterval = 24 * time.Hour
	// Default interval if no history
	DefaultRefreshInterval = 30 * time.Minute
)

// IntelligentRefreshCalculator calculates optimal refresh intervals based on feed activity
type IntelligentRefreshCalculator struct {
	db *database.DB
}

// NewIntelligentRefreshCalculator creates a new calculator
func NewIntelligentRefreshCalculator(db *database.DB) *IntelligentRefreshCalculator {
	return &IntelligentRefreshCalculator{db: db}
}

// CalculateInterval calculates the optimal refresh interval for a feed
// based on its recent article publication frequency
// Interval range: 5 minutes to 24 hours
func (irc *IntelligentRefreshCalculator) CalculateInterval(feed models.Feed) time.Duration {
	// Get recent articles (last 30 days) to analyze frequency
	articles, err := irc.db.GetArticles("", feed.ID, "", false, 100, 0)
	if err != nil || len(articles) == 0 {
		return DefaultRefreshInterval
	}

	// Calculate average time between articles
	avgInterval := irc.calculateAverageInterval(articles)

	// Apply smart scaling: refresh more frequently than publication rate
	// but with reasonable bounds
	optimalInterval := avgInterval / 2

	// Clamp to min/max bounds (5 minutes to 24 hours)
	if optimalInterval < MinRefreshInterval {
		return MinRefreshInterval
	}
	if optimalInterval > MaxRefreshInterval {
		return MaxRefreshInterval
	}

	return optimalInterval
}

// calculateAverageInterval computes the average time between article publications
func (irc *IntelligentRefreshCalculator) calculateAverageInterval(articles []models.Article) time.Duration {
	if len(articles) < 2 {
		return DefaultRefreshInterval
	}

	// Note: Articles should be ordered by published_at DESC from the database query.
	// If ordering is not guaranteed, explicit sorting should be added here.
	// Calculate intervals between consecutive articles
	var totalInterval time.Duration
	validIntervals := 0

	for i := 0; i < len(articles)-1; i++ {
		// Articles already have time.Time for PublishedAt
		published1 := articles[i].PublishedAt
		published2 := articles[i+1].PublishedAt

		interval := published1.Sub(published2)
		// Only count positive intervals (skip negative or zero)
		if interval > 0 {
			totalInterval += interval
			validIntervals++
		}
	}

	if validIntervals == 0 {
		return DefaultRefreshInterval
	}

	avgInterval := totalInterval / time.Duration(validIntervals)

	// Round to nearest second for cleaner intervals
	return time.Duration(math.Round(avgInterval.Seconds())) * time.Second
}

// GetStaggeredDelay calculates a staggered delay for a feed
// to avoid all feeds refreshing at the same time
func GetStaggeredDelay(feedID int64, totalFeeds int) time.Duration {
	if totalFeeds <= 1 {
		return 0
	}

	// Spread feeds evenly across the interval
	// Use feedID as a deterministic seed for distribution
	staggerFactor := float64(feedID%int64(totalFeeds)) / float64(totalFeeds)
	maxStagger := 5 * time.Minute // Maximum stagger of 5 minutes

	return time.Duration(staggerFactor * float64(maxStagger))
}
