package ai

import "testing"

func TestDetectAPIProviderRecognizesOpenAICompatibleChatCompletionsEndpoints(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"https://open.bigmodel.cn/api/paas/v4/chat/completions": "openai",
		"https://example.com/v1/chat/completions":               "openai",
		"https://example.com/v1/responses":                      "openai",
	}

	for endpoint, want := range cases {
		if got := DetectAPIProvider(endpoint); got != want {
			t.Fatalf("DetectAPIProvider(%q) = %q, want %q", endpoint, got, want)
		}
	}
}

func TestShouldShortCircuitDetectedProviderFallback(t *testing.T) {
	t.Parallel()

	if !shouldShortCircuitDetectedProviderFallback("openai", errString("bad request - check parameters")) {
		t.Fatal("expected openai bad request to stop fallback")
	}

	if shouldShortCircuitDetectedProviderFallback("openai", errString("context deadline exceeded")) {
		t.Fatal("expected timeout errors to keep fallback/retry behavior")
	}

	if shouldShortCircuitDetectedProviderFallback("unknown", errString("bad request - check parameters")) {
		t.Fatal("expected unknown providers to keep fallback behavior")
	}
}

type errString string

func (e errString) Error() string {
	return string(e)
}
