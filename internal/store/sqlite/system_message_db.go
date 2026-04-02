package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"MavenRSS/internal/models"
)

// UpsertSystemMessage creates or updates the active message for the given user and kind.
// Repeated alerts are merged into one row and marked unread again.
func (db *DB) UpsertSystemMessage(userID int64, kind, title, body, metadataJSON string) (*models.SystemMessage, error) {
	db.WaitForReady()

	now := time.Now()
	if _, err := db.Exec(
		`INSERT INTO system_messages (user_id, kind, title, body, metadata_json, is_read, read_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 0, NULL, ?, ?)
		 ON CONFLICT(user_id, kind) DO UPDATE SET
			title = excluded.title,
			body = excluded.body,
			metadata_json = excluded.metadata_json,
			is_read = 0,
			read_at = NULL,
			updated_at = excluded.updated_at`,
		userID, kind, title, body, metadataJSON, now, now,
	); err != nil {
		return nil, fmt.Errorf("upsert system message: %w", err)
	}

	return db.GetSystemMessageByKind(userID, kind)
}

// GetSystemMessageByKind returns a single message by user and kind.
func (db *DB) GetSystemMessageByKind(userID int64, kind string) (*models.SystemMessage, error) {
	db.WaitForReady()

	row := db.QueryRow(
		`SELECT id, user_id, kind, title, body, metadata_json, is_read, read_at, created_at, updated_at
		 FROM system_messages
		 WHERE user_id = ? AND kind = ?`,
		userID, kind,
	)

	message, err := scanSystemMessage(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get system message by kind: %w", err)
	}
	return message, nil
}

// ListSystemMessages returns the latest system messages for a user.
func (db *DB) ListSystemMessages(userID int64, limit int) ([]models.SystemMessage, error) {
	db.WaitForReady()

	if limit <= 0 {
		limit = 100
	}

	rows, err := db.Query(
		`SELECT id, user_id, kind, title, body, metadata_json, is_read, read_at, created_at, updated_at
		 FROM system_messages
		 WHERE user_id = ?
		 ORDER BY updated_at DESC, id DESC
		 LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list system messages: %w", err)
	}
	defer rows.Close()

	messages := make([]models.SystemMessage, 0)
	for rows.Next() {
		message, err := scanSystemMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan system message: %w", err)
		}
		messages = append(messages, *message)
	}

	return messages, nil
}

// CountUnreadSystemMessages returns the unread system message count for a user.
func (db *DB) CountUnreadSystemMessages(userID int64) (int, error) {
	db.WaitForReady()

	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM system_messages WHERE user_id = ? AND is_read = 0`,
		userID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count unread system messages: %w", err)
	}
	return count, nil
}

// MarkSystemMessageRead marks a single message as read.
func (db *DB) MarkSystemMessageRead(userID, messageID int64) error {
	db.WaitForReady()

	_, err := db.Exec(
		`UPDATE system_messages
		 SET is_read = 1, read_at = ?, updated_at = ?
		 WHERE id = ? AND user_id = ?`,
		time.Now(), time.Now(), messageID, userID,
	)
	if err != nil {
		return fmt.Errorf("mark system message read: %w", err)
	}
	return nil
}

// MarkAllSystemMessagesRead marks all unread messages as read for a user.
func (db *DB) MarkAllSystemMessagesRead(userID int64) error {
	db.WaitForReady()

	now := time.Now()
	_, err := db.Exec(
		`UPDATE system_messages
		 SET is_read = 1, read_at = ?, updated_at = ?
		 WHERE user_id = ? AND is_read = 0`,
		now, now, userID,
	)
	if err != nil {
		return fmt.Errorf("mark all system messages read: %w", err)
	}
	return nil
}

type systemMessageScanner interface {
	Scan(dest ...any) error
}

func scanSystemMessage(scanner systemMessageScanner) (*models.SystemMessage, error) {
	var (
		message models.SystemMessage
		readAt  sql.NullTime
	)

	if err := scanner.Scan(
		&message.ID,
		&message.UserID,
		&message.Kind,
		&message.Title,
		&message.Body,
		&message.MetadataJSON,
		&message.IsRead,
		&readAt,
		&message.CreatedAt,
		&message.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if readAt.Valid {
		message.ReadAt = &readAt.Time
	}

	return &message, nil
}
