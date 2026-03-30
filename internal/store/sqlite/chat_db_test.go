package sqlite

import (
	"database/sql"
	"testing"
	"time"

	"MavenRSS/internal/models"
)

func TestChatSessionUserIsolation(t *testing.T) {
	db := newChatDBTestDB(t)

	user2ID, err := db.CreateUser(&models.User{
		Username:     "user2",
		Email:        "user2@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser error: %v", err)
	}

	feed := &models.Feed{
		Title:    "chat feed",
		URL:      "https://example.com/feed.xml",
		Category: "test",
	}
	if _, err := db.AddFeed(feed); err != nil {
		t.Fatalf("AddFeed error: %v", err)
	}

	feeds, err := db.GetFeeds()
	if err != nil || len(feeds) == 0 {
		t.Fatalf("GetFeeds error = %v, len = %d", err, len(feeds))
	}

	article := &models.Article{
		FeedID:      feeds[0].ID,
		Title:       "chat article",
		URL:         "https://example.com/article",
		PublishedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := db.SaveArticle(article); err != nil {
		t.Fatalf("SaveArticle error: %v", err)
	}

	articles, err := db.GetArticlesForUser(1, "", feeds[0].ID, "", false, 10, 0)
	if err != nil || len(articles) == 0 {
		t.Fatalf("GetArticlesForUser error = %v, len = %d", err, len(articles))
	}

	sessionID, err := db.CreateChatSession(1, articles[0].ID, "user1 chat")
	if err != nil {
		t.Fatalf("CreateChatSession error: %v", err)
	}

	messageID, err := db.CreateChatMessage(sessionID, "user", "hello", "")
	if err != nil {
		t.Fatalf("CreateChatMessage error: %v", err)
	}

	session, err := db.GetChatSessionForUser(1, sessionID)
	if err != nil {
		t.Fatalf("GetChatSessionForUser owner error: %v", err)
	}
	if session == nil {
		t.Fatal("GetChatSessionForUser() = nil, want owned session")
	}

	session, err = db.GetChatSessionForUser(user2ID, sessionID)
	if err != nil {
		t.Fatalf("GetChatSessionForUser other user error: %v", err)
	}
	if session != nil {
		t.Fatalf("GetChatSessionForUser() = %#v, want nil for non-owner", session)
	}

	if err := db.DeleteChatMessageForUser(user2ID, messageID); err != sql.ErrNoRows {
		t.Fatalf("DeleteChatMessageForUser() error = %v, want sql.ErrNoRows", err)
	}

	if err := db.DeleteChatSessionForUser(user2ID, sessionID); err != sql.ErrNoRows {
		t.Fatalf("DeleteChatSessionForUser() error = %v, want sql.ErrNoRows", err)
	}

	if err := db.DeleteChatSessionForUser(1, sessionID); err != nil {
		t.Fatalf("DeleteChatSessionForUser owner error: %v", err)
	}
}

func newChatDBTestDB(t *testing.T) *DB {
	t.Helper()

	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("db Init error: %v", err)
	}

	return db
}
