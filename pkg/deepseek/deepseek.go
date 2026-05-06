// Package deepseek implements the built-in DeepSeek provider. DeepSeek's
// HTTP API is OpenAI-compatible (same /v1/chat/completions schema and Bearer
// auth), so this provider is a thin reskin of pkg/openai pointed at
// api.deepseek.com.
package deepseek

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

const defaultEndpoint = "https://api.deepseek.com/v1/chat/completions"

// Provider is the DeepSeek implementation. Endpoint is overridable for tests.
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

func (p *Provider) Name() string { return "deepseek" }

// DefaultModel is used when an AI task omits `model:`. deepseek-chat is the
// general-purpose chat model and the cheapest entry point for default
// workflows.
func (p *Provider) DefaultModel() string { return "deepseek-chat" }

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func (p *Provider) Complete(req providers.CompletionRequest) (providers.CompletionResponse, error) {
	if req.APIKey == "" {
		return providers.CompletionResponse{}, errors.New("DEEPSEEK_API_KEY is not set")
	}
	if req.Model == "" {
		return providers.CompletionResponse{}, errors.New("deepseek: model is required")
	}

	msgs := make([]chatMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = chatMessage{Role: m.Role, Content: m.Content}
	}

	body := chatRequest{Model: req.Model, Messages: msgs}
	if req.Temperature != 0 {
		body.Temperature = &req.Temperature
	}
	if req.MaxTokens != 0 {
		body.MaxTokens = &req.MaxTokens
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return providers.CompletionResponse{}, fmt.Errorf("deepseek: marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, p.Endpoint, bytes.NewReader(buf))
	if err != nil {
		return providers.CompletionResponse{}, fmt.Errorf("deepseek: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)

	resp, err := p.Client.Do(httpReq)
	if err != nil {
		return providers.CompletionResponse{}, fmt.Errorf("deepseek: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return providers.CompletionResponse{}, fmt.Errorf("deepseek: read response: %w", err)
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return providers.CompletionResponse{}, fmt.Errorf("deepseek: decode response (status %d): %w; body=%s", resp.StatusCode, err, truncate(string(raw), 500))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return providers.CompletionResponse{}, fmt.Errorf("deepseek: %d %s", resp.StatusCode, parsed.Error.Message)
		}
		return providers.CompletionResponse{}, fmt.Errorf("deepseek: %d %s", resp.StatusCode, truncate(string(raw), 500))
	}

	if len(parsed.Choices) == 0 {
		return providers.CompletionResponse{}, errors.New("deepseek: empty choices in response")
	}

	return providers.CompletionResponse{
		Content:   parsed.Choices[0].Message.Content,
		TokensIn:  parsed.Usage.PromptTokens,
		TokensOut: parsed.Usage.CompletionTokens,
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
