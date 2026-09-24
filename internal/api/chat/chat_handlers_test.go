package chat

import (
	"strings"
	"testing"
)

func TestOptimizeChatContextKeepsRecentHistoryAndContext(t *testing.T) {
	messages := make([]ChatMessage, 12)
	for i := range messages {
		messages[i] = ChatMessage{Role: "user", Content: "message" + string(rune('0'+i))}
	}

	optimized := optimizeChatContext(messages, "Cluster title", "", "Article context", true)
	if len(optimized) != 11 {
		t.Fatalf("expected system context plus 10 recent messages, got %d", len(optimized))
	}
	if optimized[0].Role != "system" || !strings.Contains(optimized[0].Content, "Article context") {
		t.Fatalf("missing article context system message: %#v", optimized[0])
	}
	if optimized[1].Content != messages[2].Content || optimized[len(optimized)-1].Content != messages[11].Content {
		t.Fatalf("history was not limited to the most recent messages: %#v", optimized)
	}
}

func TestOptimizeChatContextCanOmitContextForLaterArticleTurns(t *testing.T) {
	optimized := optimizeChatContext(
		[]ChatMessage{{Role: "user", Content: "follow-up"}},
		"Article title",
		"",
		"Article context",
		false,
	)

	if len(optimized) != 1 || optimized[0].Role != "user" {
		t.Fatalf("unexpected later-turn messages: %#v", optimized)
	}
}

func TestTrimChatContextCapsUnicodeByRuneCount(t *testing.T) {
	content := strings.Repeat("中", maxChatContextChars+10)
	trimmed := trimChatContext(content)
	if !strings.HasSuffix(trimmed, "[Context truncated]") {
		t.Fatalf("trimmed context is missing truncation marker")
	}
	body := strings.TrimSuffix(trimmed, "\n[Context truncated]")
	if len([]rune(body)) > maxChatContextChars {
		t.Fatalf("trimmed context exceeds configured limit")
	}
}
