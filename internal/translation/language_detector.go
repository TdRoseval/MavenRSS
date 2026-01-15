package translation

import (
	"strings"
	"sync"

	"github.com/abadojack/whatlanggo"
)

// LanguageDetector handles language detection using whatlanggo
// whatlanggo is a pure Go implementation with minimal binary size impact
type LanguageDetector struct {
	once sync.Once
}

// languageDetectorInstance is the singleton instance
var (
	languageDetectorInstance *LanguageDetector
	languageDetectorOnce     sync.Once
)

// GetLanguageDetector returns the singleton language detector instance
func GetLanguageDetector() *LanguageDetector {
	languageDetectorOnce.Do(func() {
		languageDetectorInstance = &LanguageDetector{}
	})
	return languageDetectorInstance
}

// DetectLanguage detects the language of the given text
// Returns the ISO 639-1 language code (e.g., "en", "zh", "ja")
// Returns empty string if detection fails or confidence is too low
func (ld *LanguageDetector) DetectLanguage(text string) string {
	if text == "" {
		return ""
	}

	// Clean text for better detection
	text = strings.TrimSpace(text)
	if len(text) < 3 {
		return ""
	}

	// Remove HTML tags if present
	cleanText := removeHTMLTags(text)
	textForDetection := text

	// Only use cleaned text if it's significantly different and has enough content
	if len(cleanText) > 10 && len(cleanText) < len(text) {
		textForDetection = cleanText
	}

	// Detect language with options
	// Use whitelist to only detect languages we support
	supportedLangs := supportedLanguages()
	whitelist := make(map[whatlanggo.Lang]bool)
	for _, lang := range supportedLangs {
		whitelist[lang] = true
	}

	options := whatlanggo.Options{
		Whitelist: whitelist,
	}

	info := whatlanggo.DetectWithOptions(textForDetection, options)

	// Check confidence level - only accept high confidence detections
	if info.Confidence < 0.5 {
		return ""
	}

	// Convert whatlanggo Lang to ISO 639-1 code
	detectedCode := whatlangToISOCode(info.Lang)
	return detectedCode
}

// ShouldTranslate determines if translation is needed based on language detection
// Returns true if:
// - Language detection fails (fallback to translation for safety)
// - Detected language differs from target language
// Returns false if text is already in target language
func (ld *LanguageDetector) ShouldTranslate(text, targetLang string) bool {
	detectedLang := ld.DetectLanguage(text)

	// If detection failed, assume translation is needed (fallback behavior)
	if detectedLang == "" {
		return true
	}

	// Normalize language codes for comparison
	detectedLang = normalizeLangCode(detectedLang)
	targetLang = normalizeLangCode(targetLang)

	// Check if already in target language
	return detectedLang != targetLang
}

// ShouldTranslateFullText analyzes the full text to determine if translation is needed
// It samples multiple paragraphs and calculates the language ratio
// Returns false if the target language accounts for more than 60% of the content
// This is useful for articles that are mixed-language or already mostly in target language
func (ld *LanguageDetector) ShouldTranslateFullText(text, targetLang string) bool {
	if text == "" {
		return true
	}

	// Clean text and split into paragraphs
	cleanText := removeHTMLTags(text)
	cleanText = strings.TrimSpace(cleanText)

	// Split by common paragraph delimiters
	paragraphs := splitIntoParagraphs(cleanText)

	// If too few paragraphs, fall back to simple detection
	if len(paragraphs) < 3 {
		return ld.ShouldTranslate(text, targetLang)
	}

	// Sample paragraphs (up to 10 for efficiency)
	sampleSize := len(paragraphs)
	if sampleSize > 10 {
		sampleSize = 10
	}

	targetLang = normalizeLangCode(targetLang)
	targetCount := 0
	totalAnalyzed := 0

	// Analyze each sampled paragraph
	for i := 0; i < sampleSize; i++ {
		paragraph := strings.TrimSpace(paragraphs[i])
		if len(paragraph) < 10 {
			continue // Skip very short paragraphs
		}

		detectedLang := ld.DetectLanguage(paragraph)
		if detectedLang == "" {
			continue // Skip if detection failed
		}

		detectedLang = normalizeLangCode(detectedLang)
		totalAnalyzed++

		if detectedLang == targetLang {
			targetCount++
		}
	}

	// If we couldn't analyze enough paragraphs, fall back to simple detection
	if totalAnalyzed < 3 {
		return ld.ShouldTranslate(text, targetLang)
	}

	// Calculate ratio of target language content
	ratio := float64(targetCount) / float64(totalAnalyzed)

	// Skip translation if more than 60% is already in target language
	if ratio > 0.6 {
		return false
	}

	return true
}

// splitIntoParagraphs splits text into paragraphs using common delimiters
func splitIntoParagraphs(text string) []string {
	// Replace multiple newlines with single delimiter
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Split by double newlines or single newlines
	var paragraphs []string
	current := strings.Builder{}

	for _, r := range text {
		if r == '\n' {
			if current.Len() > 0 {
				paragraphs = append(paragraphs, strings.TrimSpace(current.String()))
				current.Reset()
			}
		} else {
			current.WriteRune(r)
		}
	}

	// Add last paragraph
	if current.Len() > 0 {
		paragraphs = append(paragraphs, strings.TrimSpace(current.String()))
	}

	return paragraphs
}

// supportedLanguages returns the list of languages we want to detect
func supportedLanguages() []whatlanggo.Lang {
	return []whatlanggo.Lang{
		whatlanggo.Eng,
		whatlanggo.Cmn, // Chinese (Mandarin)
		whatlanggo.Jpn,
		whatlanggo.Kor,
		whatlanggo.Spa,
		whatlanggo.Fra,
		whatlanggo.Deu,
		whatlanggo.Por,
		whatlanggo.Rus,
		whatlanggo.Ita,
		whatlanggo.Nld,
		whatlanggo.Pol,
		whatlanggo.Tur,
		whatlanggo.Vie,
		whatlanggo.Tha,
		whatlanggo.Ind,
		whatlanggo.Hin,
	}
}

// whatlangToISOCode converts whatlanggo Lang to ISO 639-1 code
func whatlangToISOCode(lang whatlanggo.Lang) string {
	langMap := map[whatlanggo.Lang]string{
		whatlanggo.Eng: "en",
		whatlanggo.Cmn: "zh",
		whatlanggo.Jpn: "ja",
		whatlanggo.Kor: "ko",
		whatlanggo.Spa: "es",
		whatlanggo.Fra: "fr",
		whatlanggo.Deu: "de",
		whatlanggo.Por: "pt",
		whatlanggo.Rus: "ru",
		whatlanggo.Ita: "it",
		whatlanggo.Nld: "nl",
		whatlanggo.Pol: "pl",
		whatlanggo.Tur: "tr",
		whatlanggo.Vie: "vi",
		whatlanggo.Tha: "th",
		whatlanggo.Ind: "id",
		whatlanggo.Hin: "hi",
	}

	if code, ok := langMap[lang]; ok {
		return code
	}
	return ""
}

// normalizeLangCode normalizes language codes (e.g., "zh-CN" -> "zh", "en-US" -> "en")
func normalizeLangCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if len(code) > 2 {
		code = code[:2]
	}
	return code
}

// removeHTMLTags removes HTML tags from text for better language detection
func removeHTMLTags(text string) string {
	// Simple HTML tag removal
	var result strings.Builder
	inTag := false
	for _, r := range text {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}
