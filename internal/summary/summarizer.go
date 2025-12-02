// Package summary provides text summarization using local algorithms.
// It implements TF-IDF and TextRank-based sentence scoring for extractive summarization.
package summary

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// SummaryLength represents the desired length of the summary
type SummaryLength string

const (
	// Short summary with fewer sentences
	Short SummaryLength = "short"
	// Medium summary with moderate sentences
	Medium SummaryLength = "medium"
	// Long summary with more sentences
	Long SummaryLength = "long"
)

// MinContentLength is the minimum text length required for meaningful summarization
const MinContentLength = 200

// MinSentenceCount is the minimum number of sentences required for summarization
const MinSentenceCount = 3

// Summarizer provides text summarization capabilities
type Summarizer struct{}

// NewSummarizer creates a new Summarizer instance
func NewSummarizer() *Summarizer {
	return &Summarizer{}
}

// SummaryResult contains the generated summary and metadata
type SummaryResult struct {
	Summary      string   `json:"summary"`
	KeyPoints    []string `json:"key_points"`
	SentenceCount int     `json:"sentence_count"`
	IsTooShort   bool     `json:"is_too_short"`
}

// Summarize generates a summary of the given text using combined TF-IDF and TextRank scoring
func (s *Summarizer) Summarize(text string, length SummaryLength) SummaryResult {
	// Clean the text
	cleanedText := cleanText(text)

	// Check if text is too short
	if len(cleanedText) < MinContentLength {
		return SummaryResult{
			Summary:    cleanedText,
			KeyPoints:  []string{},
			IsTooShort: true,
		}
	}

	// Split into sentences
	sentences := splitSentences(cleanedText)

	// Check if we have enough sentences
	if len(sentences) < MinSentenceCount {
		return SummaryResult{
			Summary:       cleanedText,
			KeyPoints:     sentences,
			SentenceCount: len(sentences),
			IsTooShort:    true,
		}
	}

	// Calculate number of sentences to extract based on length
	numSentences := calculateNumSentences(len(sentences), length)

	// Score sentences using combined TF-IDF and TextRank
	scoredSentences := s.scoreSentences(sentences)

	// Sort by score (descending)
	sort.Slice(scoredSentences, func(i, j int) bool {
		return scoredSentences[i].score > scoredSentences[j].score
	})

	// Get top sentences
	topSentences := scoredSentences[:numSentences]

	// Sort by original position to maintain narrative flow
	sort.Slice(topSentences, func(i, j int) bool {
		return topSentences[i].position < topSentences[j].position
	})

	// Build summary
	var summaryParts []string
	var keyPoints []string
	for _, sent := range topSentences {
		summaryParts = append(summaryParts, sent.text)
		keyPoints = append(keyPoints, sent.text)
	}

	return SummaryResult{
		Summary:       strings.Join(summaryParts, " "),
		KeyPoints:     keyPoints,
		SentenceCount: numSentences,
		IsTooShort:    false,
	}
}

// scoredSentence holds a sentence with its calculated score and position
type scoredSentence struct {
	text     string
	score    float64
	position int
}

// scoreSentences calculates scores for each sentence using combined TF-IDF and TextRank
func (s *Summarizer) scoreSentences(sentences []string) []scoredSentence {
	// Calculate TF-IDF scores
	tfidfScores := calculateTFIDF(sentences)

	// Calculate TextRank scores
	textRankScores := calculateTextRank(sentences)

	// Combine scores
	result := make([]scoredSentence, len(sentences))
	for i, sentence := range sentences {
		// Weight TF-IDF at 0.6 and TextRank at 0.4
		combinedScore := 0.6*tfidfScores[i] + 0.4*textRankScores[i]

		// Boost first and second sentences slightly (they often contain key info)
		if i == 0 {
			combinedScore *= 1.2
		} else if i == 1 {
			combinedScore *= 1.1
		}

		result[i] = scoredSentence{
			text:     sentence,
			score:    combinedScore,
			position: i,
		}
	}

	return result
}

// calculateTFIDF computes TF-IDF scores for each sentence
func calculateTFIDF(sentences []string) []float64 {
	// Build document frequency map
	docFreq := make(map[string]int)
	allTerms := make([]map[string]int, len(sentences))

	for i, sentence := range sentences {
		terms := tokenize(sentence)
		termFreq := make(map[string]int)
		seenTerms := make(map[string]bool)

		for _, term := range terms {
			termFreq[term]++
			if !seenTerms[term] {
				docFreq[term]++
				seenTerms[term] = true
			}
		}
		allTerms[i] = termFreq
	}

	numDocs := float64(len(sentences))
	scores := make([]float64, len(sentences))

	for i, termFreq := range allTerms {
		var score float64
		totalTerms := 0
		for _, count := range termFreq {
			totalTerms += count
		}

		for term, count := range termFreq {
			// TF: normalized term frequency
			tf := float64(count) / float64(totalTerms)

			// IDF: inverse document frequency
			idf := math.Log(numDocs / float64(docFreq[term]))

			score += tf * idf
		}
		scores[i] = score
	}

	// Normalize scores
	maxScore := 0.0
	for _, score := range scores {
		if score > maxScore {
			maxScore = score
		}
	}
	if maxScore > 0 {
		for i := range scores {
			scores[i] /= maxScore
		}
	}

	return scores
}

// calculateTextRank computes TextRank scores using sentence similarity
func calculateTextRank(sentences []string) []float64 {
	n := len(sentences)
	if n == 0 {
		return []float64{}
	}

	// Build similarity matrix
	similarity := make([][]float64, n)
	for i := range similarity {
		similarity[i] = make([]float64, n)
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			sim := sentenceSimilarity(sentences[i], sentences[j])
			similarity[i][j] = sim
			similarity[j][i] = sim
		}
	}

	// Initialize scores
	scores := make([]float64, n)
	for i := range scores {
		scores[i] = 1.0
	}

	// Damping factor
	d := 0.85
	iterations := 30

	// Iterate to convergence
	for iter := 0; iter < iterations; iter++ {
		newScores := make([]float64, n)
		for i := 0; i < n; i++ {
			sum := 0.0
			for j := 0; j < n; j++ {
				if i != j {
					// Calculate sum of similarities for sentence j
					sumSim := 0.0
					for k := 0; k < n; k++ {
						if k != j {
							sumSim += similarity[j][k]
						}
					}
					if sumSim > 0 {
						sum += similarity[i][j] / sumSim * scores[j]
					}
				}
			}
			newScores[i] = (1 - d) + d*sum
		}
		scores = newScores
	}

	// Normalize scores
	maxScore := 0.0
	for _, score := range scores {
		if score > maxScore {
			maxScore = score
		}
	}
	if maxScore > 0 {
		for i := range scores {
			scores[i] /= maxScore
		}
	}

	return scores
}

// sentenceSimilarity calculates similarity between two sentences using word overlap
func sentenceSimilarity(s1, s2 string) float64 {
	words1 := tokenize(s1)
	words2 := tokenize(s2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0
	}

	// Create word sets
	set1 := make(map[string]bool)
	for _, w := range words1 {
		set1[w] = true
	}

	set2 := make(map[string]bool)
	for _, w := range words2 {
		set2[w] = true
	}

	// Guard against empty sets to avoid math.Log(0) which returns -Inf
	if len(set1) == 0 || len(set2) == 0 {
		return 0
	}

	// Count common words
	common := 0
	for w := range set1 {
		if set2[w] {
			common++
		}
	}

	// Normalized overlap
	denom := math.Log(float64(len(set1))) + math.Log(float64(len(set2)))
	if denom == 0 {
		return 0
	}

	return float64(common) / denom
}

// cleanText removes HTML tags and normalizes whitespace
func cleanText(text string) string {
	// Remove HTML tags
	htmlRegex := regexp.MustCompile(`<[^>]*>`)
	text = htmlRegex.ReplaceAllString(text, " ")

	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	// Normalize whitespace
	spaceRegex := regexp.MustCompile(`\s+`)
	text = spaceRegex.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// splitSentences splits text into sentences
func splitSentences(text string) []string {
	// Simple sentence splitting with common abbreviations handling
	// Split on sentence-ending punctuation followed by space (or end of text)
	sentenceRegex := regexp.MustCompile(`([.!?。！？]+)(\s+|$)`)

	// Split the text
	parts := sentenceRegex.Split(text, -1)

	// Get the delimiters
	matches := sentenceRegex.FindAllStringSubmatch(text, -1)

	var sentences []string
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Add back the punctuation if available
		if i < len(matches) && len(matches[i]) > 1 {
			part += matches[i][1]
		}

		// Filter out very short sentences (likely fragments)
		// Use a lower threshold to support various languages
		if len(part) > 10 {
			sentences = append(sentences, part)
		}
	}

	return sentences
}

// tokenize splits text into lowercase tokens, removing stopwords
func tokenize(text string) []string {
	// Convert to lowercase
	text = strings.ToLower(text)

	// Split into words
	var tokens []string
	var currentWord strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			currentWord.WriteRune(r)
		} else {
			if currentWord.Len() > 0 {
				word := currentWord.String()
				// Skip stopwords and very short words
				if len(word) > 2 && !isStopWord(word) {
					tokens = append(tokens, word)
				}
				currentWord.Reset()
			}
		}
	}

	// Don't forget the last word
	if currentWord.Len() > 0 {
		word := currentWord.String()
		if len(word) > 2 && !isStopWord(word) {
			tokens = append(tokens, word)
		}
	}

	return tokens
}

// isStopWord checks if a word is a common stopword (English and Chinese)
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		// English stopwords
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "from": true, "as": true, "is": true, "was": true,
		"are": true, "were": true, "been": true, "be": true, "have": true, "has": true,
		"had": true, "do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true, "must": true,
		"shall": true, "can": true, "this": true, "that": true, "these": true,
		"those": true, "it": true, "its": true, "they": true, "them": true,
		"their": true, "what": true, "which": true, "who": true, "whom": true,
		"whose": true, "where": true, "when": true, "why": true, "how": true,
		"all": true, "each": true, "every": true, "both": true, "few": true,
		"more": true, "most": true, "other": true, "some": true, "such": true,
		"than": true, "too": true, "very": true, "just": true, "only": true,
		"own": true, "same": true, "so": true, "not": true, "also": true,
		"into": true, "about": true, "your": true, "you": true, "our": true,
		"his": true, "her": true, "my": true, "we": true, "he": true, "she": true,
		"over": true, "out": true, "up": true, "down": true, "then": true, "now": true,
		// Chinese stopwords
		"的": true, "了": true, "和": true, "是": true, "在": true, "有": true,
		"这": true, "个": true, "我": true, "不": true, "人": true, "都": true,
		"一": true, "他": true, "就": true, "们": true, "上": true, "也": true,
		"你": true, "说": true, "着": true, "对": true, "为": true, "与": true,
	}
	return stopWords[word]
}

// calculateNumSentences determines how many sentences to include based on length setting
func calculateNumSentences(totalSentences int, length SummaryLength) int {
	var ratio float64
	switch length {
	case Short:
		ratio = 0.2
	case Medium:
		ratio = 0.35
	case Long:
		ratio = 0.5
	default:
		ratio = 0.35
	}

	numSentences := int(math.Ceil(float64(totalSentences) * ratio))

	// Ensure minimum of 1 sentence
	if numSentences < 1 {
		numSentences = 1
	}

	// Cap at total sentences
	if numSentences > totalSentences {
		numSentences = totalSentences
	}

	return numSentences
}
