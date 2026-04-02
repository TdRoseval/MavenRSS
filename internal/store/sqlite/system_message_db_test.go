package sqlite_test

import (
	"testing"

	"MavenRSS/internal/models"
)

func TestUpsertSystemMessageMergesRepeatedAlerts(t *testing.T) {
	db := setupTestDB(t)

	userID, err := db.CreateUser(&models.User{
		Username:     "message-user",
		Email:        "message@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser error = %v", err)
	}

	first, err := db.UpsertSystemMessage(userID, "kind-a", "Title A", "Body A", `{"v":1}`)
	if err != nil {
		t.Fatalf("UpsertSystemMessage first error = %v", err)
	}
	if first == nil {
		t.Fatal("first message = nil")
	}

	if err := db.MarkSystemMessageRead(userID, first.ID); err != nil {
		t.Fatalf("MarkSystemMessageRead error = %v", err)
	}

	second, err := db.UpsertSystemMessage(userID, "kind-a", "Title B", "Body B", `{"v":2}`)
	if err != nil {
		t.Fatalf("UpsertSystemMessage second error = %v", err)
	}
	if second == nil {
		t.Fatal("second message = nil")
	}
	if second.ID != first.ID {
		t.Fatalf("second.ID = %d, want %d", second.ID, first.ID)
	}
	if second.Title != "Title B" || second.Body != "Body B" {
		t.Fatalf("merged message = %#v", second)
	}
	if second.IsRead {
		t.Fatal("merged message should be unread again")
	}

	messages, err := db.ListSystemMessages(userID, 100)
	if err != nil {
		t.Fatalf("ListSystemMessages error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}

	unread, err := db.CountUnreadSystemMessages(userID)
	if err != nil {
		t.Fatalf("CountUnreadSystemMessages error = %v", err)
	}
	if unread != 1 {
		t.Fatalf("unread = %d, want 1", unread)
	}

	if err := db.MarkAllSystemMessagesRead(userID); err != nil {
		t.Fatalf("MarkAllSystemMessagesRead error = %v", err)
	}

	unread, err = db.CountUnreadSystemMessages(userID)
	if err != nil {
		t.Fatalf("CountUnreadSystemMessages after mark all read error = %v", err)
	}
	if unread != 0 {
		t.Fatalf("unread after mark all read = %d, want 0", unread)
	}
}
