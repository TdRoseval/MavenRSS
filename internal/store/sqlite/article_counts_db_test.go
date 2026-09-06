package sqlite

import (
	"testing"
	"time"
)

func TestGetAllClusterFilterCountsForAllFeeds(t *testing.T) {
	db := newChatDBTestDB(t)

	// User 1 is seeded by db.Init(); create the second user for isolation checks.
	insertClusterCountsUser(t, db, 2, "counts-user-2")

	feedA := insertClusterCountsFeed(t, db, "cluster counts feed A")
	feedB := insertClusterCountsFeed(t, db, "cluster counts feed B")

	// Cluster 1: unread + favorite, articles in both feeds, no image.
	cluster1 := insertClusterCountsCluster(t, db, 1, false, true, false, false)
	insertClusterCountsArticle(t, db, 1, feedA, cluster1, "")
	insertClusterCountsArticle(t, db, 1, feedB, cluster1, "")

	// Cluster 2: read + read_later + image article in feed A.
	cluster2 := insertClusterCountsCluster(t, db, 1, true, false, true, false)
	insertClusterCountsArticle(t, db, 1, feedA, cluster2, "https://example.com/img.png")

	// Cluster 3: hidden, must be excluded.
	cluster3 := insertClusterCountsCluster(t, db, 1, false, false, false, true)
	insertClusterCountsArticle(t, db, 1, feedA, cluster3, "")

	// Cluster 4: another user's cluster, must be excluded.
	cluster4 := insertClusterCountsCluster(t, db, 2, false, false, false, false)
	insertClusterCountsArticle(t, db, 2, feedA, cluster4, "")

	counts, err := db.GetAllClusterFilterCountsForAllFeeds(1)
	if err != nil {
		t.Fatalf("GetAllClusterFilterCountsForAllFeeds: %v", err)
	}

	assertClusterCount(t, "Unread", counts.Unread, map[int64]int{feedA: 1, feedB: 1})
	assertClusterCount(t, "Favorites", counts.Favorites, map[int64]int{feedA: 1, feedB: 1})
	assertClusterCount(t, "FavoritesUnread", counts.FavoritesUnread, map[int64]int{feedA: 1, feedB: 1})
	assertClusterCount(t, "ReadLater", counts.ReadLater, map[int64]int{feedA: 1})
	assertClusterCount(t, "ReadLaterUnread", counts.ReadLaterUnread, map[int64]int{})
	assertClusterCount(t, "Images", counts.Images, map[int64]int{feedA: 1})
	assertClusterCount(t, "ImagesUnread", counts.ImagesUnread, map[int64]int{})

	// Cross-check the unread slice against the dedicated unread query.
	unread, err := db.GetUnreadClusterCountsForAllFeeds(1)
	if err != nil {
		t.Fatalf("GetUnreadClusterCountsForAllFeeds: %v", err)
	}
	assertClusterCount(t, "Unread (dedicated query)", unread, map[int64]int{feedA: 1, feedB: 1})
}

func insertClusterCountsUser(t *testing.T, db *DB, id int64, username string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO users (id, username, email, password_hash, role, status) VALUES (?, ?, ?, 'hash', 'user', 'active')`,
		id, username, username+"@example.com",
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func insertClusterCountsFeed(t *testing.T, db *DB, title string) int64 {
	t.Helper()
	res, err := db.Exec(
		`INSERT INTO feeds (user_id, title, url, category) VALUES (1, ?, ?, 'test')`,
		title, "https://example.com/"+title+"-"+time.Now().Format("150405.000000000"),
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

func insertClusterCountsCluster(t *testing.T, db *DB, userID int64, isRead, isFavorite, isReadLater, isHidden bool) int64 {
	t.Helper()
	res, err := db.Exec(
		`INSERT INTO clusters (user_id, is_read, is_favorite, is_read_later, is_hidden)
		 VALUES (?, ?, ?, ?, ?)`,
		userID, isRead, isFavorite, isReadLater, isHidden,
	)
	if err != nil {
		t.Fatalf("insert cluster: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	return id
}

var clusterCountsArticleSeq int64

func insertClusterCountsArticle(t *testing.T, db *DB, userID int64, feedID int64, clusterID int64, imageURL string) {
	t.Helper()
	clusterCountsArticleSeq++
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO articles (user_id, feed_id, cluster_id, title, url, published_at, unique_id, image_url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, feedID, clusterID, "title", "https://example.com/a-"+now, now, "u-cluster-counts-"+string(rune('A'+clusterCountsArticleSeq%26))+string(rune('a'+clusterCountsArticleSeq%26)), imageURL,
	)
	if err != nil {
		t.Fatalf("insert article: %v", err)
	}
}

func assertClusterCount(t *testing.T, name string, got map[int64]int, want map[int64]int) {
	t.Helper()
	// The merged query may emit zero-valued entries for feeds that appear in
	// the join but match no rows for a given filter; the frontend reads counts
	// with `|| 0`, so zero entries are equivalent to absent entries.
	effective := make(map[int64]int, len(got))
	for feedID, count := range got {
		if count != 0 {
			effective[feedID] = count
		}
	}
	if len(effective) != len(want) {
		t.Fatalf("%s: got %v, want %v", name, effective, want)
	}
	for feedID, wantCount := range want {
		if effective[feedID] != wantCount {
			t.Fatalf("%s: feed %d got %d, want %d (full: %v)", name, feedID, effective[feedID], wantCount, effective)
		}
	}
}
