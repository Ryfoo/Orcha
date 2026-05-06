package deepseek

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	providers "github.com/ryfoo/orcha/pkg"
)

func TestCompleteSuccess(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"model": "deepseek-chat",
			"choices": [{"message": {"role": "assistant", "content": "ok"}}],
			"usage": {"prompt_tokens": 4, "completion_tokens": 1}
		}`))
	}))
	defer srv.Close()

	p := &Provider{Endpoint: srv.URL, Client: srv.Client()}
	resp, err := p.Complete(providers.CompletionRequest{
		APIKey:   "test-key",
		Model:    "deepseek-chat",
		Messages: []providers.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("Content = %q, want 'ok'", resp.Content)
	}
	if resp.TokensIn != 4 || resp.TokensOut != 1 {
		t.Errorf("Usage = %d/%d, want 4/1", resp.TokensIn, resp.TokensOut)
	}
	if auth != "Bearer test-key" {
		t.Errorf("Authorization = %q, want 'Bearer test-key'", auth)
	}
}

func TestCompleteAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key","type":"auth"}}`))
	}))
	defer srv.Close()

	p := &Provider{Endpoint: srv.URL, Client: srv.Client()}
	_, err := p.Complete(providers.CompletionRequest{
		APIKey:   "k",
		Model:    "deepseek-chat",
		Messages: []providers.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid key") {
		t.Fatalf("expected error containing 'invalid key', got %v", err)
	}
}

func TestCompleteMissingKey(t *testing.T) {
	p := New()
	_, err := p.Complete(providers.CompletionRequest{Model: "deepseek-chat"})
	if err == nil || !strings.Contains(err.Error(), "DEEPSEEK_API_KEY") {
		t.Fatalf("expected DEEPSEEK_API_KEY error, got %v", err)
	}
}
