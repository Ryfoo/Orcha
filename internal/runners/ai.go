package runners

import (
	"fmt"

	"github.com/ryfoo/orcha/internal/engine"
	"github.com/ryfoo/orcha/internal/parser"
	providers "github.com/ryfoo/orcha/pkg"
)

// RunAI dispatches an AI task to its registered provider, after interpolating
// {{$input}} and {{$env.VAR}} into the system and prompt fields. The model
// falls back to provider.DefaultModel() when the YAML omits `model:`. The
// system message is dropped from the request when `system:` is omitted.
func RunAI(t *parser.Task, in engine.Value) (engine.Value, error) {
	provider, err := providers.Get(t.Provider)
	if err != nil {
		return engine.Value{}, err
	}

	prompt, err := engine.Interpolate(t.Prompt, in)
	if err != nil {
		return engine.Value{}, fmt.Errorf("interpolate prompt: %w", err)
	}

	messages := make([]providers.Message, 0, 2)
	if t.System != "" {
		system, err := engine.Interpolate(t.System, in)
		if err != nil {
			return engine.Value{}, fmt.Errorf("interpolate system: %w", err)
		}
		messages = append(messages, providers.Message{Role: "system", Content: system})
	}
	messages = append(messages, providers.Message{Role: "user", Content: prompt})

	model := t.Model
	if model == "" {
		model = provider.DefaultModel()
		if model == "" {
			return engine.Value{}, fmt.Errorf("task %q: model not set and provider %q has no default", t.Name, t.Provider)
		}
	}

	resp, err := provider.Complete(providers.CompletionRequest{
		APIKey:      providers.ResolveAPIKey(t.Provider),
		Model:       model,
		Messages:    messages,
		Temperature: t.Temperature,
		MaxTokens:   t.MaxTokens,
	})
	if err != nil {
		return engine.Value{}, fmt.Errorf("provider %s: %w", t.Provider, err)
	}
	return asOutput(resp.Content, t.OutputType)
}
