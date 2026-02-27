// Package ai provides AI usage tracking and rate limiting functionality.
package ai

import (
	"container/heap"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PriorityLevel defines the priority levels for AI requests
type PriorityLevel int

const (
	// PriorityNormal is the default priority for background requests
	PriorityNormal PriorityLevel = 0
	// PriorityHigh is for user-initiated requests on selected articles
	PriorityHigh PriorityLevel = 1
)

// Request represents a queued AI request with priority
type Request struct {
	priority PriorityLevel
	index    int       // The index of the item in the heap
	ready    chan bool // Channel to signal when the request is ready to proceed
}

// PriorityQueue implements a heap-based priority queue
type PriorityQueue []*Request

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// Higher priority requests come first
	return pq[i].priority > pq[j].priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Request)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// SettingsProvider is an interface for retrieving and storing settings.
type SettingsProvider interface {
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error
}

// UsageTracker tracks AI usage (tokens) and enforces rate limits with priority queue.
type UsageTracker struct {
	settings    SettingsProvider
	mu          sync.Mutex
	pq          PriorityQueue
	lastRequest time.Time
	minInterval time.Duration // Minimum interval between AI requests
	isProcessing bool
}

// NewUsageTracker creates a new AI usage tracker.
func NewUsageTracker(settings SettingsProvider) *UsageTracker {
	tracker := &UsageTracker{
		settings:    settings,
		minInterval: 500 * time.Millisecond, // Default: max 2 requests per second
		pq:          make(PriorityQueue, 0),
	}
	heap.Init(&tracker.pq)
	return tracker
}

// SetMinInterval sets the minimum interval between AI requests.
func (t *UsageTracker) SetMinInterval(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.minInterval = d
}

// CanMakeRequest checks if a new AI request can be made (rate limiting).
// Returns true if allowed, false if rate limited.
func (t *UsageTracker) CanMakeRequest() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	if now.Sub(t.lastRequest) < t.minInterval {
		return false
	}
	t.lastRequest = now
	return true
}

// processQueue processes the priority queue in a separate goroutine
func (t *UsageTracker) processQueue() {
	t.mu.Lock()
	if t.isProcessing {
		t.mu.Unlock()
		return
	}
	t.isProcessing = true
	t.mu.Unlock()

	for {
		t.mu.Lock()
		if t.pq.Len() == 0 {
			t.isProcessing = false
			t.mu.Unlock()
			return
		}

		// Pop the highest priority request
		req := heap.Pop(&t.pq).(*Request)
		
		// Check rate limit
		now := time.Now()
		elapsed := now.Sub(t.lastRequest)
		wait := t.minInterval - elapsed
		
		if wait > 0 {
			// Need to wait, put the request back and unlock
			heap.Push(&t.pq, req)
			t.mu.Unlock()
			time.Sleep(wait)
			continue
		}

		// Ready to process this request
		t.lastRequest = now
		t.mu.Unlock()

		// Signal the request that it's ready
		req.ready <- true
	}
}

// WaitForRateLimit blocks until a request can be made with normal priority.
func (t *UsageTracker) WaitForRateLimit() {
	t.WaitForRateLimitWithPriority(PriorityNormal)
}

// WaitForRateLimitWithPriority blocks until a request can be made with specified priority.
func (t *UsageTracker) WaitForRateLimitWithPriority(priority PriorityLevel) {
	req := &Request{
		priority: priority,
		ready:    make(chan bool, 1),
	}

	t.mu.Lock()
	heap.Push(&t.pq, req)
	t.mu.Unlock()

	// Start processing the queue
	go t.processQueue()

	// Wait for the request to be ready
	<-req.ready
}

// GetCurrentUsage returns the current token usage.
func (t *UsageTracker) GetCurrentUsage() (int64, error) {
	usageStr, err := t.settings.GetSetting("ai_usage_tokens")
	if err != nil {
		return 0, err
	}
	if usageStr == "" {
		return 0, nil
	}
	return strconv.ParseInt(usageStr, 10, 64)
}

// GetUsageLimit returns the configured usage limit (0 = unlimited).
func (t *UsageTracker) GetUsageLimit() (int64, error) {
	limitStr, err := t.settings.GetSetting("ai_usage_limit")
	if err != nil {
		return 0, err
	}
	if limitStr == "" {
		return 0, nil
	}
	return strconv.ParseInt(limitStr, 10, 64)
}

// GetHardLimit returns the hard usage limit (0 = unlimited).
func (t *UsageTracker) GetHardLimit() (int64, error) {
	limitStr, err := t.settings.GetSetting("ai_usage_hard_limit")
	if err != nil {
		return 0, err
	}
	if limitStr == "" {
		return 0, nil
	}
	return strconv.ParseInt(limitStr, 10, 64)
}

// IsLimitReached checks if the usage limit has been reached.
func (t *UsageTracker) IsLimitReached() bool {
	usage, err := t.GetCurrentUsage()
	if err != nil {
		return false
	}

	// Check user limit
	userLimit, err := t.GetUsageLimit()
	if err != nil {
		return false
	}

	// Check hard limit
	hardLimit, err := t.GetHardLimit()
	if err != nil {
		return false
	}

	// Determine the effective limit (take the smaller of user limit and hard limit if both are set)
	effectiveLimit := int64(0)
	if userLimit > 0 && hardLimit > 0 {
		effectiveLimit = min(userLimit, hardLimit)
	} else if userLimit > 0 {
		effectiveLimit = userLimit
	} else if hardLimit > 0 {
		effectiveLimit = hardLimit
	}

	// If no limit is set (both 0), then unlimited
	if effectiveLimit == 0 {
		return false
	}

	return usage >= effectiveLimit
}

// AddUsage adds tokens to the usage counter.
func (t *UsageTracker) AddUsage(tokens int64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Get current usage inside the lock to prevent race condition
	usageStr, err := t.settings.GetSetting("ai_usage_tokens")
	var current int64
	if err == nil && usageStr != "" {
		current, _ = strconv.ParseInt(usageStr, 10, 64)
	}

	newUsage := current + tokens
	return t.settings.SetSetting("ai_usage_tokens", strconv.FormatInt(newUsage, 10))
}

// ResetUsage resets the usage counter to zero.
func (t *UsageTracker) ResetUsage() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.settings.SetSetting("ai_usage_tokens", "0")
}

// TrackTranslation tracks token usage for a translation operation.
func (t *UsageTracker) TrackTranslation(sourceText, translatedText string) {
	// Estimate tokens for both input and output
	inputTokens := EstimateTokens(sourceText)
	outputTokens := EstimateTokens(translatedText)

	totalTokens := inputTokens + outputTokens
	if err := t.AddUsage(totalTokens); err != nil {
		log.Printf("Warning: failed to track AI usage: %v", err)
	}
}

// TrackSummary tracks token usage for a summarization operation.
func (t *UsageTracker) TrackSummary(content, summary string) {
	// Estimate tokens for both input and output
	inputTokens := EstimateTokens(content)
	outputTokens := EstimateTokens(summary)

	totalTokens := inputTokens + outputTokens
	if err := t.AddUsage(totalTokens); err != nil {
		log.Printf("Warning: failed to track AI usage: %v", err)
	}
}

// EstimateTokens estimates the number of tokens in a text.
// Uses a simple heuristic: ~4 characters per token for English, ~1.5 characters per token for CJK.
func EstimateTokens(text string) int64 {
	if text == "" {
		return 0
	}

	// Count CJK characters
	cjkCount := 0
	nonCJKCount := 0

	for _, r := range text {
		if isCJK(r) {
			cjkCount++
		} else if r > 32 { // Non-whitespace, non-control characters
			nonCJKCount++
		}
	}

	// Rough estimation:
	// - CJK: roughly 1 token per 1.5 characters
	// - Non-CJK: roughly 1 token per 4 characters (words average ~4-5 chars + spaces)
	cjkTokens := float64(cjkCount) / 1.5
	nonCJKTokens := float64(nonCJKCount) / 4.0

	// Add some overhead for special tokens
	total := int64(cjkTokens + nonCJKTokens + 10)
	if total < 1 {
		total = 1
	}

	return total
}

// EstimateTokensWithSegmentation estimates tokens using word-level segmentation.
// For CJK text, counts words/characters. For other text, counts space-separated words.
func EstimateTokensWithSegmentation(text string) int64 {
	if text == "" {
		return 0
	}

	tokens := int64(0)

	// Split by whitespace to handle mixed content
	words := strings.Fields(text)

	for _, word := range words {
		// Check if word contains CJK characters
		hasCJK := false
		for _, r := range word {
			if isCJK(r) {
				hasCJK = true
				break
			}
		}

		if hasCJK {
			// For CJK, count characters as rough token estimate
			// (In reality, subword tokenization is more complex)
			for _, r := range word {
				if isCJK(r) {
					tokens++
				}
			}
		} else {
			// For non-CJK, each word is roughly 1-2 tokens
			wordLen := len(word)
			if wordLen <= 4 {
				tokens++
			} else if wordLen <= 8 {
				tokens += 2
			} else {
				tokens += int64(wordLen/4) + 1
			}
		}
	}

	// Minimum 1 token
	if tokens < 1 {
		tokens = 1
	}

	return tokens
}

// isCJK checks if a rune is a CJK (Chinese, Japanese, Korean) character.
func isCJK(r rune) bool {
	// CJK Unified Ideographs
	if r >= 0x4E00 && r <= 0x9FFF {
		return true
	}
	// CJK Unified Ideographs Extension A
	if r >= 0x3400 && r <= 0x4DBF {
		return true
	}
	// CJK Unified Ideographs Extension B
	if r >= 0x20000 && r <= 0x2A6DF {
		return true
	}
	// Hiragana
	if r >= 0x3040 && r <= 0x309F {
		return true
	}
	// Katakana
	if r >= 0x30A0 && r <= 0x30FF {
		return true
	}
	// Hangul Syllables
	if r >= 0xAC00 && r <= 0xD7AF {
		return true
	}
	return false
}
