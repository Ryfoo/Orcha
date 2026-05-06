// Package anthropic implements the built-in Anthropic provider. It talks to
// the Anthropic Messages REST API directly so v1 keeps zero third-party SDK
// dependencies.
package anthropic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	providers "github.com/ryfoo/orcha/pkg"
)

const (
	defaultEndpoint = "https://api.anthropic.com/v1/messages"
	apiVersion      = "2023-06-01"
	// defaultMaxTokens is what we send when the YAML omits max_tokens. The
	// Anthropic API requires the field, unlike OpenAI's which makes it
	// optional. 1024 is enough headroom for typical single-turn tasks.
	defaultMaxTokens = 1024
)

// Provider is the Anthropic implementation. Endpoint is overridable for tests.
type Provider struct {
	Endpoint string
	Client   *http.Client
}

// New returns a Provider with sensible defaults.
func New() *Provider {
	return &Provider{
		Endpoint: defaultEndpoint,
		Client:   &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *Provider) Name() string { return "anthropic" }

// DefaultModel is used when an AI task omits `model:`. Picks the cheap-and-fast
// Claude variant so default workflows don't surprise users with token bills.
func (p *Provider) DefaultModel() string { return "claude-haiku-4-5-20251001" }

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type messageEntry struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messagesRequest struct {
	Model       string         `json:"model"`
	System      string         `json:"system,omitempty"`
	Messages    []messageEntry `json:"messages"`
	MaxTokens   int            `json:"max_tokens"`
	Temperature *float64       `json:"temperature,omitempty"`
}

type messagesResponse struct {
	Model   string         `json:"model"`
	Content []contentBlock `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (p *Provider) Complete(req providers.CompletionRequest) (providers.CompletionResponse, error) {
	if req.APIKey == "" {
		return providers.CompletionResponse{}, errors.New("ANTHROPIC_API_KEY is not set")
	}
	if req.Model == "" {
		return providers.CompletionResponse{}, errors.New("anthropic: model is required")
	}

	// Anthropic separates the system prompt from the message list; pull any
	// system-role messages out into the top-level field so callers can keep
	// using the same role-based contract as OpenAI.
	var system string
	msgs := make([]messageEntry, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			if system != "" {
				system += "\n\n"
			}
			system += m.Content
			continue
		}
		msgs = append(msgs, messageEntry{Role: m.Role, Content: m.Content})
	}
	if len(msgs) == 0 {
		return providers.CompletionResponse{}, errors.New("anthropic: at least one user message is required")
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	body := messagesRequest{
		Model:     req.Model,
		System:    system,
		Messages:  msgs,
		MaxTokens: maxTokens,
	}
	if req.Temperature != 0 {
		body.Temperature = &req.Temperature
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return providers.CompletionResponse{}, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, p.Endpoint, bytes.NewReader(buf))
	if err != nil {
		return providers.CompletionResponse{}, fmt.Errorf("anthropic: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", req.APIKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	resp, err := p.Client.Do(httpReq)
	if err != nil {
		return providers.CompletionResponse{}, fmt.Errorf("anthropic: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return providers.CompletionResponse{}, fmt.Errorf("anthropic: read response: %w", err)
	}

	var parsed messagesResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return providers.CompletionResponse{}, fmt.Errorf("anthropic: decode response (status %d): %w; body=%s", resp.StatusCode, err, truncate(string(raw), 500))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return providers.CompletionResponse{}, fmt.Errorf("anthropic: %d %s", resp.StatusCode, parsed.Error.Message)
		}
		return providers.CompletionResponse{}, fmt.Errorf("anthropic: %d %s", resp.StatusCode, truncate(string(raw), 500))
	}

	// Concatenate all text blocks. The Messages API returns a list of
	// content blocks (text, tool_use, ...); v1 only handles plain text.
	var out string
	for _, block := range parsed.Content {
		if block.Type == "text" {
			out += block.Text
		}
	}
	if out == "" {
		return providers.CompletionResponse{}, errors.New("anthropic: response had no text content")
	}

	return providers.CompletionResponse{
		Content:   out,
		TokensIn:  parsed.Usage.InputTokens,
		TokensOut: parsed.Usage.OutputTokens,
		Model:     parsed.Model,
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func init() {
	providers.Register(New())
}
