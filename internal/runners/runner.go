package runners

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ryfoo/orcha/internal/engine"
	"github.com/ryfoo/orcha/internal/parser"
)

// Dispatch routes a task to the runner that matches its Type. This is the
// entry point the engine uses; the engine itself stays unaware of concrete
// task kinds.
func Dispatch(t *parser.Task, in engine.Value) (engine.Value, error) {
	switch t.Type {
	case parser.KindAI:
		return RunAI(t, in)
	case parser.KindHTTP:
		return RunHTTP(t, in)
	case parser.KindFile:
		return RunFile(t, in)
	default:
		return engine.Value{}, fmt.Errorf("unsupported task type %q", t.Type)
	}
}

// asOutput converts a raw string produced by a runner into the declared
// output_type. The returned Value has its Type set to want, ready for the
// executor to validate against task.OutputType. Errors include enough context
// to surface as a task_fail event.
func asOutput(raw string, want parser.OutputType) (engine.Value, error) {
	switch want {
	case parser.TypeText:
		return engine.Value{Type: parser.TypeText, Raw: raw}, nil
	case parser.TypeFilepath:
		return engine.Value{Type: parser.TypeFilepath, Raw: strings.TrimSpace(raw)}, nil
	case parser.TypeJSON:
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return engine.Value{}, fmt.Errorf("expected json output but received empty response")
		}
		var parsed any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return engine.Value{}, fmt.Errorf("expected json output but failed to parse: %w", err)
		}
		return engine.Value{Type: parser.TypeJSON, Raw: parsed}, nil
	case parser.TypeList:
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return engine.Value{Type: parser.TypeList, Raw: []string{}}, nil
		}
		// Try JSON array first; fall back to newline-separated.
		if strings.HasPrefix(trimmed, "[") {
			var arr []any
			if err := json.Unmarshal([]byte(trimmed), &arr); err == nil {
				out := make([]string, len(arr))
				for i, x := range arr {
					out[i] = fmt.Sprintf("%v", x)
				}
				return engine.Value{Type: parser.TypeList, Raw: out}, nil
			}
		}
		lines := strings.Split(trimmed, "\n")
		out := make([]string, 0, len(lines))
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l != "" {
				out = append(out, l)
			}
		}
		return engine.Value{Type: parser.TypeList, Raw: out}, nil
	default:
		return engine.Value{}, fmt.Errorf("unknown output_type %q", want)
	}
}
