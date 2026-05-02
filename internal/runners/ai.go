package runners

import (
	"fmt"

	"github.com/ryfoo/orcha/internal/engine"
	"github.com/ryfoo/orcha/internal/parser"
	providers "github.com/ryfoo/orcha/pkg"
)

// RunAI dispatches an AI task to its registered provider, after interpolating
// {{$input}} and {{$env.VAR}} into the system and prompt fields.
func RunAI(t *parser.Task, in engine.Value) (engine.Value, error) {
	provider, err := providers.Get(t.Provider)
	if err != nil {
		return engine.Value{}, err
	}

	system, err := engine.Interpolate(t.System, in)
	if err != nil {
		return engine.Value{}, fmt.Errorf("interpolate system: %w", err)
	}
	prompt, err := engine.Interpolate(t.Prompt, in)
	if err != nil {
		return engine.Value{}, fmt.Errorf("interpolate prompt: %w", err)
	}

	req := providers.CompletionRequest{
		APIKey: providers.ResolveAPIKey(t.Provider),
		Model:  t.Model,
		Messages: []providers.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: prompt},
		},
		Temperature: t.Temperature,
		MaxTokens:   t.MaxTokens,
	}

	resp, err := provider.Complete(req)
	if err != nil {
		return engine.Value{}, fmt.Errorf("provider %s: %w", t.Provider, err)
	}
	return asOutput(resp.Content, t.OutputType)
}
