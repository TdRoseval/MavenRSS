package ai

import "testing"

func TestShouldSkipArticleRetryMatchesContentSafetyFailures(t *testing.T) {
	t.Parallel()

	err := &RequestError{
		UserMessage: "AI service unavailable",
		Diagnostics: []string{
			"OpenAI: bad request - check parameters: code=1301, 系统检测到输入或生成内容可能包含不安全或敏感内容",
		},
	}

	if !ShouldSkipArticleRetry(err) {
		t.Fatal("ShouldSkipArticleRetry() = false, want true for safety-blocked content")
	}
}

func TestShouldSkipArticleRetryIgnoresConfigFailures(t *testing.T) {
	t.Parallel()

	err := &RequestError{
		UserMessage: "AI service unavailable",
		Diagnostics: []string{
			"OpenAI: authentication failed - check API key: invalid key",
		},
	}

	if ShouldSkipArticleRetry(err) {
		t.Fatal("ShouldSkipArticleRetry() = true, want false for configuration failure")
	}
}
