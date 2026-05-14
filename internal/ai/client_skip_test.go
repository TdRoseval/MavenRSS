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

func TestIsRetryableStageFailureMatchesNetworkAndServiceFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "proxy connection failure",
			err: &RequestError{
				UserMessage: "AI service unavailable: Unable to connect to the AI service. Please check your network connection and proxy settings.",
				Diagnostics: []string{
					`OpenAI: request failed: proxyconnect tcp: dial tcp 127.0.0.1:7890: connectex: No connection could be made because the target machine actively refused it.`,
				},
			},
			want: true,
		},
		{
			name: "rate limit",
			err: &RequestError{
				UserMessage: "AI service unavailable: Rate limit exceeded. Please try again later.",
				Diagnostics: []string{
					"OpenAI: status 429 rate limit exceeded",
				},
			},
			want: true,
		},
		{
			name: "invalid key",
			err: &RequestError{
				UserMessage: "AI service unavailable: Invalid API key. Please check your AI configuration.",
				Diagnostics: []string{
					"OpenAI: authentication failed - check API key: invalid key",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsRetryableStageFailure(tt.err); got != tt.want {
				t.Fatalf("IsRetryableStageFailure() = %v, want %v", got, tt.want)
			}
		})
	}
}
