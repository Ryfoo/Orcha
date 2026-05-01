package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ryfoo/orcha/internal/parser"
)

// Value is a typed value flowing between steps. The Type field tracks which
// of Text / JSON / Filepath / List the underlying interface{} is.
type Value struct {
	Type parser.OutputType
	// Raw holds: string for text/filepath, map[string]any (or any) for json,
	// []string for list.
	Raw any
}

// AsText converts a Value to its string form for interpolation.
//
//	text/filepath -> raw string
//	list          -> joined by newlines
//	json          -> compact JSON
func (v Value) AsText() (string, error) {
	switch v.Type {
	case parser.TypeText, parser.TypeFilepath:
		s, ok := v.Raw.(string)
		if !ok {
			return fmt.Sprintf("%v", v.Raw), nil
		}
		return s, nil
	case parser.TypeList:
		switch xs := v.Raw.(type) {
		case []string:
			return strings.Join(xs, "\n"), nil
		case []any:
			parts := make([]string, len(xs))
			for i, x := range xs {
				parts[i] = fmt.Sprintf("%v", x)
			}
			return strings.Join(parts, "\n"), nil
		default:
			return "", fmt.Errorf("list value has unexpected concrete type %T", v.Raw)
		}
	case parser.TypeJSON:
		buf, err := json.Marshal(v.Raw)
		if err != nil {
			return "", fmt.Errorf("serialize json value: %w", err)
		}
		return string(buf), nil
	default:
		return "", fmt.Errorf("unknown value type %q", v.Type)
	}
}

// CoerceUserInput turns whatever the user passed via Python into a typed
// Value. The caller's first task tells us what type to expect, so we
// canonicalize toward that. This is best-effort — strict typecheck happens
// between steps, not at the user boundary.
func CoerceUserInput(raw any, want parser.OutputType) Value {
	switch want {
	case parser.TypeJSON:
		return Value{Type: parser.TypeJSON, Raw: raw}
	case parser.TypeList:
		switch xs := raw.(type) {
		case []string:
			return Value{Type: parser.TypeList, Raw: xs}
		case []any:
			ss := make([]string, len(xs))
			for i, x := range xs {
				ss[i] = fmt.Sprintf("%v", x)
			}
			return Value{Type: parser.TypeList, Raw: ss}
		}
	case parser.TypeFilepath:
		return Value{Type: parser.TypeFilepath, Raw: fmt.Sprintf("%v", raw)}
	}
	if s, ok := raw.(string); ok {
		return Value{Type: parser.TypeText, Raw: s}
	}
	return Value{Type: parser.TypeText, Raw: fmt.Sprintf("%v", raw)}
}

// Pattern matchers are kept as package-level vars so they compile once.
var (
	inputPattern = regexp.MustCompile(`\{\{\s*\$input\s*\}\}`)
	envPattern   = regexp.MustCompile(`\{\{\s*\$env\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)
)

// Interpolate replaces {{$input}} and {{$env.VAR}} occurrences in s. Missing
// env vars resolve to "" (matching shell semantics) so a workflow can opt to
// degrade rather than fail when an env var is absent.
func Interpolate(s string, input Value) (string, error) {
	out := s
	if inputPattern.MatchString(out) {
		text, err := input.AsText()
		if err != nil {
			return "", fmt.Errorf("interpolate $input: %w", err)
		}
		out = inputPattern.ReplaceAllLiteralString(out, text)
	}
	out = envPattern.ReplaceAllStringFunc(out, func(match string) string {
		sub := envPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		return os.Getenv(sub[1])
	})
	return out, nil
}
