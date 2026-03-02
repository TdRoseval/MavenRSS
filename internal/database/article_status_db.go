package database

import (
	"database/sql"
	"errors"
)

var ErrArticleNotFound = errors.New("article not found or does not belong to user")

// MarkArticleRead marks an article as read or unread.
// When marking as read, also removes from read later list.
func (db *DB) MarkArticleRead(id int64, read bool) error {
	db.WaitForReady()
	isRead := 0
	if read {
		isRead = 1
		// When marking as read, also remove from read later
		_, err := db.Exec("UPDATE articles SET is_read = 1, is_read_later = 0 WHERE id = ?", id)
		return err
	}
	_, err := db.Exec("UPDATE articles SET is_read = ? WHERE id = ?", isRead, id)
	return err
}

// ToggleFavorite toggles the favorite status of an article.
func (db *DB) ToggleFavorite(id int64) error {
	db.WaitForReady()
	// First get current state
	var isFav bool
	err := db.QueryRow("SELECT is_favorite FROM articles WHERE id = ?", id).Scan(&isFav)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE articles SET is_favorite = ? WHERE id = ?", !isFav, id)
	return err
}

// SetArticleFavorite sets the favorite status of an article.
func (db *DB) SetArticleFavorite(id int64, favorite bool) error {
	db.WaitForReady()
	_, err := db.Exec("UPDATE articles SET is_favorite = ? WHERE id = ?", favorite, id)
	return err
}

// ToggleArticleHidden toggles the is_hidden status of an article.
func (db *DB) ToggleArticleHidden(id int64) error {
	db.WaitForReady()
	// First get current state
	var isHidden bool
	err := db.QueryRow("SELECT is_hidden FROM articles WHERE id = ?", id).Scan(&isHidden)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE articles SET is_hidden = ? WHERE id = ?", !isHidden, id)
	return err
}

// SetArticleHidden sets the hidden status of an article.
func (db *DB) SetArticleHidden(id int64, hidden bool) error {
	db.WaitForReady()
	_, err := db.Exec("UPDATE articles SET is_hidden = ? WHERE id = ?", hidden, id)
	return err
}

// ToggleReadLater toggles the read later status of an article.
// When adding to read later, also marks article as unread.
func (db *DB) ToggleReadLater(id int64) error {
	db.WaitForReady()
	// First get current state
	var isReadLater bool
	err := db.QueryRow("SELECT is_read_later FROM articles WHERE id = ?", id).Scan(&isReadLater)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	newState := !isReadLater
	// If adding to read later, also mark as unread
	if newState {
		_, err = db.Exec("UPDATE articles SET is_read_later = 1, is_read = 0 WHERE id = ?", id)
	} else {
		_, err = db.Exec("UPDATE articles SET is_read_later = 0 WHERE id = ?", id)
	}
	return err
}

// SetArticleReadLater sets the read later status of an article.
// When adding to read later, also marks article as unread.
func (db *DB) SetArticleReadLater(id int64, readLater bool) error {
	db.WaitForReady()
	// If adding to read later, also mark as unread
	if readLater {
		_, err := db.Exec("UPDATE articles SET is_read_later = 1, is_read = 0 WHERE id = ?", id)
		return err
	}
	_, err := db.Exec("UPDATE articles SET is_read_later = 0 WHERE id = ?", id)
	return err
}

// MarkArticleReadForUser marks an article as read or unread for a specific user.
// Returns ErrArticleNotFound if the article doesn't belong to the user.
// When marking as read, also removes from read later list.
func (db *DB) MarkArticleReadForUser(userID int64, id int64, read bool) error {
	db.WaitForReady()
	isRead := 0
	if read {
		isRead = 1
		result, err := db.Exec("UPDATE articles SET is_read = 1, is_read_later = 0 WHERE id = ? AND user_id = ?", id, userID)
		if err != nil {
			return err
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			return ErrArticleNotFound
		}
		return nil
	}
	result, err := db.Exec("UPDATE articles SET is_read = ? WHERE id = ? AND user_id = ?", isRead, id, userID)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrArticleNotFound
	}
	return nil
}

// ToggleFavoriteForUser toggles the favorite status of an article for a specific user.
// Returns ErrArticleNotFound if the article doesn't belong to the user.
func (db *DB) ToggleFavoriteForUser(userID int64, id int64) error {
	db.WaitForReady()
	var isFav bool
	err := db.QueryRow("SELECT is_favorite FROM articles WHERE id = ? AND user_id = ?", id, userID).Scan(&isFav)
	if err == sql.ErrNoRows {
		return ErrArticleNotFound
	}
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE articles SET is_favorite = ? WHERE id = ? AND user_id = ?", !isFav, id, userID)
	return err
}

// SetArticleFavoriteForUser sets the favorite status of an article for a specific user.
// Returns ErrArticleNotFound if the article doesn't belong to the user.
func (db *DB) SetArticleFavoriteForUser(userID int64, id int64, favorite bool) error {
	db.WaitForReady()
	result, err := db.Exec("UPDATE articles SET is_favorite = ? WHERE id = ? AND user_id = ?", favorite, id, userID)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrArticleNotFound
	}
	return nil
}

// ToggleArticleHiddenForUser toggles the is_hidden status of an article for a specific user.
// Returns ErrArticleNotFound if the article doesn't belong to the user.
func (db *DB) ToggleArticleHiddenForUser(userID int64, id int64) error {
	db.WaitForReady()
	var isHidden bool
	err := db.QueryRow("SELECT is_hidden FROM articles WHERE id = ? AND user_id = ?", id, userID).Scan(&isHidden)
	if err == sql.ErrNoRows {
		return ErrArticleNotFound
	}
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE articles SET is_hidden = ? WHERE id = ? AND user_id = ?", !isHidden, id, userID)
	return err
}

// ToggleReadLaterForUser toggles the read later status of an article for a specific user.
// Returns ErrArticleNotFound if the article doesn't belong to the user.
// When adding to read later, also marks article as unread.
func (db *DB) ToggleReadLaterForUser(userID int64, id int64) error {
	db.WaitForReady()
	var isReadLater bool
	err := db.QueryRow("SELECT is_read_later FROM articles WHERE id = ? AND user_id = ?", id, userID).Scan(&isReadLater)
	if err == sql.ErrNoRows {
		return ErrArticleNotFound
	}
	if err != nil {
		return err
	}
	newState := !isReadLater
	if newState {
		_, err = db.Exec("UPDATE articles SET is_read_later = 1, is_read = 0 WHERE id = ? AND user_id = ?", id, userID)
	} else {
		_, err = db.Exec("UPDATE articles SET is_read_later = 0 WHERE id = ? AND user_id = ?", id, userID)
	}
	return err
}
