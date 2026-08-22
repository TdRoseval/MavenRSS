package sqlite

import (
	"testing"
	"time"
)

func TestGetArticleIDsByRawUniqueIDs(t *testing.T) {
	db := newChatDBTestDB(t)

	feedID := insertBatchLookupFeed(t, db)

	now := time.Now().UTC().Truncate(time.Second)
	want := map[string]int64{}
	for _, uid := range []string{"batch-u1", "batch-u2", "batch-u3"} {
		res, err := db.Exec(
			`INSERT INTO articles (user_id, feed_id, title, url, published_at, unique_id) VALUES (1, ?, ?, ?, ?, ?)`,
			feedID, uid, "https://example.com/"+uid, now, uid,
		)
		if err != nil {
			t.Fatalf("insert article: %v", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			t.Fatalf("LastInsertId: %v", err)
		}
		want[uid] = id
	}

	got, err := db.GetArticleIDsByRawUniqueIDs(1, []string{"batch-u1", "batch-u2", "batch-u3", "batch-missing"})
	if err != nil {
		t.Fatalf("GetArticleIDsByRawUniqueIDs: %v", err)
	}
	for uid, id := range want {
		if got[uid] != id {
			t.Fatalf("id for %s = %d, want %d", uid, got[uid], id)
		}
	}
	if _, ok := got["batch-missing"]; ok {
		t.Fatal("expected missing unique_id to be absent")
	}
}

func TestGetArticleIDsByRawUniqueIDsEmpty(t *testing.T) {
	db := newChatDBTestDB(t)

	got, err := db.GetArticleIDsByRawUniqueIDs(1, nil)
	if err != nil {
		t.Fatalf("empty list error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %d entries", len(got))
	}
}

func insertBatchLookupFeed(t *testing.T, db *DB) int64 {
	t.Helper()
	res, err := db.Exec(
		`INSERT INTO feeds (user_id, title, url, category) VALUES (1, 'batch feed', 'https://example.com/batch-feed-`+time.Now().Format("150405.000000000")+`', 'test')`,
	)
	if err != nil {
		t.Fatalf("insert feed: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	return id
}