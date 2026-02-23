package rules

import (
	"encoding/json"
	"os"
	"testing"

	"MavenRSS/internal/database"
	"MavenRSS/internal/models"
)

func setupTestEngine(t *testing.T) *Engine {
	t.Helper()

	// Create temporary database
	dbFile := "test_rules.db"
	t.Cleanup(func() { os.Remove(dbFile) })

	db, err := database.NewDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return NewEngine(db)
}

func TestEngine_ApplyRulesToArticles(t *testing.T) {
	engine := setupTestEngine(t)

	// Create test rule
	rule := Rule{
		Name:    "Test Rule",
		Enabled: true,
		Conditions: []Condition{
			{
				Field:    "article_title",
				Operator: "contains",
				Value:    "test",
			},
		},
		Actions: []string{"favorite", "mark_read"},
	}

	rules := []Rule{rule}
	rulesJSON, _ := json.Marshal(rules)
	engine.db.SetSetting("rules", string(rulesJSON))

	// Create test articles
	articles := []models.Article{
		{
			ID:    1,
			Title: "This is a test article",
		},
		{
			ID:    2,
			Title: "This is another article",
		},
	}

	// Apply rules
	count, err := engine.ApplyRulesToArticles(articles)
	if err != nil {
		t.Fatalf("ApplyRulesToArticles failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 article to be processed, got %d", count)
	}
}

func TestEngine_ApplyRule(t *testing.T) {
	engine := setupTestEngine(t)

	// Create test rule
	rule := Rule{
		Name:    "Test Rule",
		Enabled: true,
		Conditions: []Condition{
			{
				Field:    "article_title",
				Operator: "contains",
				Value:    "test",
			},
		},
		Actions: []string{"favorite"},
	}

	// Apply rule
	count, err := engine.ApplyRule(rule)
	if err != nil {
		t.Fatalf("ApplyRule failed: %v", err)
	}

	// Since no articles in DB, count should be 0
	if count != 0 {
		t.Errorf("Expected 0 articles to be processed, got %d", count)
	}
}
