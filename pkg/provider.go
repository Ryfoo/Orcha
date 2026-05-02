// Package providers defines the plugin interface that lets third parties add
// new AI providers to Orcha. Built-in providers (e.g. OpenAI) implement the
// same interface and register themselves through the registry in this package.
package providers

// Message is one entry in a chat-style completion request. Role is "system" or
// "user" in v1; assistant messages are not supported because tasks are
// single-turn.
type Message struct {
	Role    string
	Content string
}

// CompletionRequest is what the engine sends to a provider. APIKey is resolved
// by the engine from environment variables before the call so providers do not
// read env vars themselves.
type CompletionRequest struct {
	APIKey      string
	Model       string
	Messages    []Message
	Temperature float64
	MaxTokens   int
}

// CompletionResponse is the normalized result returned by every provider.
type CompletionResponse struct {
	Content   string
	TokensIn  int
	TokensOut int
	Model     string
}

// Provider is the contract every AI backend must implement.
type Provider interface {
	Name() string
	Complete(req CompletionRequest) (CompletionResponse, error)
}
