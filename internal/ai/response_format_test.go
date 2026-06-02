package ai

import "testing"

func TestOpenAIBuildRequestIncludesJSONResponseFormat(t *testing.T) {
	t.Parallel()

	request, err := NewOpenAIHandler().BuildRequest(RequestConfig{
		Model:          "test-model",
		Messages:       []map[string]string{{"role": "user", "content": "return json"}},
		ResponseFormat: JSONResponseFormat(),
	})
	if err != nil {
		t.Fatalf("BuildRequest returned error: %v", err)
	}

	format, ok := request["response_format"].(map[string]interface{})
	if !ok {
		t.Fatalf("response_format = %#v, want map", request["response_format"])
	}
	if format["type"] != "json_object" {
		t.Fatalf("response_format.type = %#v, want json_object", format["type"])
	}
}

func TestOllamaBuildRequestMapsJSONResponseFormatToJSONMode(t *testing.T) {
	t.Parallel()

	request, err := NewOllamaHandler().BuildRequest(RequestConfig{
		Model:          "test-model",
		Messages:       []map[string]string{{"role": "user", "content": "return json"}},
		ResponseFormat: JSONResponseFormat(),
	})
	if err != nil {
		t.Fatalf("BuildRequest returned error: %v", err)
	}

	if request["format"] != "json" {
		t.Fatalf("format = %#v, want json", request["format"])
	}
}

func TestGeminiBuildRequestMapsJSONResponseFormatToMimeType(t *testing.T) {
	t.Parallel()

	request, err := NewGeminiHandler().BuildRequest(RequestConfig{
		Model:          "test-model",
		Messages:       []map[string]string{{"role": "user", "content": "return json"}},
		ResponseFormat: JSONResponseFormat(),
	})
	if err != nil {
		t.Fatalf("BuildRequest returned error: %v", err)
	}

	genConfig, ok := request["generationConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("generationConfig = %#v, want map", request["generationConfig"])
	}
	if genConfig["responseMimeType"] != "application/json" {
		t.Fatalf("responseMimeType = %#v, want application/json", genConfig["responseMimeType"])
	}
}
