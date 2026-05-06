package anthropic

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	providers "github.com/ryfoo/orcha/pkg"
)

func TestCompleteSuccess(t *testing.T) {
	var captured map[string]any
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		headers = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"model": "claude-haiku-4-5-20251001",
			"content": [{"type": "text", "text": "hello"}],
			"usage": {"input_tokens": 7, "output_tokens": 3}
		}`))
	}))
	defer srv.Close()

	p := &Provider{Endpoint: srv.URL, Client: srv.Client()}
	resp, err := p.Complete(providers.CompletionRequest{
		APIKey: "test-key",
		Model:  "claude-haiku-4-5-20251001",
		Messages: []providers.Message{
			{Role: "system", Content: "be brief"},
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "hello" {
		t.Errorf("Content = %q, want 'hello'", resp.Content)
	}
	if resp.TokensIn != 7 || resp.TokensOut != 3 {
		t.Errorf("Usage = %d/%d, want 7/3", resp.TokensIn, resp.TokensOut)
	}

	if got := headers.Get("x-api-key"); got != "test-key" {
		t.Errorf("x-api-key header = %q, want test-key", got)
	}
	if got := headers.Get("anthropic-version"); got != apiVersion {
		t.Errorf("anthropic-version header = %q, want %q", got, apiVersion)
	}

	// system message must be lifted out of the messages array.
	if captured["system"] != "be brief" {
		t.Errorf("system field = %v, want 'be brief'", captured["system"])
	}
	msgs, _ := captured["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("messages length = %d, want 1 (system stripped)", len(msgs))
	}
	if max, _ := captured["max_tokens"].(float64); max != float64(defaultMaxTokens) {
		t.Errorf("max_tokens default = %v, want %d", captured["max_tokens"], defaultMaxTokens)
	}
}

func TestCompleteAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request","message":"bad model"}}`))
	}))
	defer srv.Close()

	p := &Provider{Endpoint: srv.URL, Client: srv.Client()}
	_, err := p.Complete(providers.CompletionRequest{
		APIKey:   "k",
		Model:    "x",
		Messages: []providers.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil || !strings.Contains(err.Error(), "bad model") {
		t.Fatalf("expected error containing 'bad model', got %v", err)
	}
}

func TestCompleteMissingKey(t *testing.T) {
	p := New()
	_, err := p.Complete(providers.CompletionRequest{Model: "x", Messages: []providers.Message{{Role: "user", Content: "hi"}}})
	if err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Fatalf("expected ANTHROPIC_API_KEY error, got %v", err)
	}
}
