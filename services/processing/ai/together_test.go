package ai_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"processing/ai"
)

func TestTogetherClient_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "Generated summary."}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := ai.NewTogetherClient("test-key", srv.URL)
	result, err := client.Complete("Summarize this: hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Generated summary." {
		t.Errorf("got %q", result)
	}
}
