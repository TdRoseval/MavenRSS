package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
)

// TranslationCache represents a cached translation entry
type TranslationCache struct {
	ID             int64
	SourceTextHash string
	SourceText     string
	TargetLang     string
	TranslatedText string
	Provider       string
	CreatedAt      string
}

// GetCachedTranslation retrieves a translation from cache if available
func (db *DB) GetCachedTranslation(sourceTextHash, targetLang, provider string) (string, bool, error) {
	var translatedText string
	err := db.QueryRow(
		`SELECT translated_text FROM translation_cache
		 WHERE source_text_hash = ? AND target_lang = ? AND provider = ?`,
		sourceTextHash, targetLang, provider,
	).Scan(&translatedText)

	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return translatedText, true, nil
}

// GetCachedTranslations retrieves cached translations for a batch of source hashes.
func (db *DB) GetCachedTranslations(sourceTextHashes []string, targetLang, provider string) (map[string]string, error) {
	db.WaitForReady()
	result := make(map[string]string)
	if len(sourceTextHashes) == 0 {
		return result, nil
	}

	seen := make(map[string]struct{}, len(sourceTextHashes))
	hashes := make([]string, 0, len(sourceTextHashes))
	for _, hash := range sourceTextHashes {
		hash = strings.TrimSpace(hash)
		if hash == "" {
			continue
		}
		if _, ok := seen[hash]; ok {
			continue
		}
		seen[hash] = struct{}{}
		hashes = append(hashes, hash)
	}
	if len(hashes) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(hashes))
	args := make([]interface{}, 0, len(hashes)+2)
	for i, hash := range hashes {
		placeholders[i] = "?"
		args = append(args, hash)
	}
	args = append(args, targetLang, provider)

	rows, err := db.Query(fmt.Sprintf(`
		SELECT source_text_hash, translated_text
		FROM translation_cache
		WHERE source_text_hash IN (%s) AND target_lang = ? AND provider = ?
	`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var hash, translatedText string
		if err := rows.Scan(&hash, &translatedText); err != nil {
			continue
		}
		result[hash] = translatedText
	}
	return result, rows.Err()
}

// SetCachedTranslation stores a translation in cache
func (db *DB) SetCachedTranslation(sourceTextHash, sourceText, targetLang, translatedText, provider string) error {
	_, err := db.Exec(
		`INSERT OR REPLACE INTO translation_cache
			(source_text_hash, source_text, target_lang, translated_text, provider, created_at)
			VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		sourceTextHash, sourceText, targetLang, translatedText, provider,
	)
	return err
}

// SetCachedTranslationBackground is SetCachedTranslation at background write
// priority; it yields to waiting interactive writes.
func (db *DB) SetCachedTranslationBackground(sourceTextHash, sourceText, targetLang, translatedText, provider string) error {
	_, err := db.execWithPriority(
		writePriorityBackground,
		`INSERT OR REPLACE INTO translation_cache
			(source_text_hash, source_text, target_lang, translated_text, provider, created_at)
			VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		sourceTextHash, sourceText, targetLang, translatedText, provider,
	)
	return err
}

// CleanupTranslationCache removes cached translations older than maxAgeDays
// If userID > 0, no-op since translation_cache is global (no user_id field)
func (db *DB) CleanupTranslationCache(maxAgeDays int, userID int64) (int64, error) {
	result, err := db.Exec(
		`DELETE FROM translation_cache WHERE created_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", maxAgeDays),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
