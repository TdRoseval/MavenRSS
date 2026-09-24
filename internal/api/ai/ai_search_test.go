package handlers

import (
	"strings"
	"testing"
)

func TestBuildClusterSearchSQLAggregatesArticleMatches(t *testing.T) {
	query := buildClusterSearchSQL(
		&SearchTerms{
			Required: []string{"sqlite"},
			Optional: []string{"wal"},
		},
		100,
		42,
		false,
	)

	for _, fragment := range []string{
		"SUM(",
		"GROUP BY a.cluster_id",
		"ORDER BY m.relevance_score DESC, MAX(all_articles.published_at) DESC",
		"a.user_id = 42",
		"c.user_id = 42",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("cluster search query does not contain %q:\n%s", fragment, query)
		}
	}

	if strings.Contains(strings.ToLower(query), "interest") || strings.Contains(query, "ORDER BY c.recommendation_score") {
		t.Fatalf("cluster search query unexpectedly uses interest/recommendation ranking:\n%s", query)
	}
	if strings.Contains(query, "a.is_read = 0") || strings.Contains(query, "a.feed_id = 42") {
		t.Fatalf("cluster search query unexpectedly applies article-list filters:\n%s", query)
	}
}
