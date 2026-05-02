package runners

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ryfoo/orcha/internal/engine"
	"github.com/ryfoo/orcha/internal/parser"
)

// RunFile performs read / write / append against the local filesystem. For
// write/append, the resolved path is returned as the output (so an output_type
// of filepath gets the absolute or workflow-relative path written to).
func RunFile(t *parser.Task, in engine.Value) (engine.Value, error) {
	path, err := engine.Interpolate(t.Path, in)
	if err != nil {
		return engine.Value{}, fmt.Errorf("interpolate path: %w", err)
	}
	if path == "" {
		return engine.Value{}, fmt.Errorf("file %s: path resolved to empty string", t.Operation)
	}

	switch t.Operation {
	case parser.OpRead:
		raw, err := os.ReadFile(path)
		if err != nil {
			return engine.Value{}, fmt.Errorf("read %s: %w", path, err)
		}
		return asOutput(string(raw), t.OutputType)

	case parser.OpWrite, parser.OpAppend:
		content, err := engine.Interpolate(t.Content, in)
		if err != nil {
			return engine.Value{}, fmt.Errorf("interpolate content: %w", err)
		}
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return engine.Value{}, fmt.Errorf("mkdir %s: %w", dir, err)
			}
		}
		flags := os.O_CREATE | os.O_WRONLY
		if t.Operation == parser.OpAppend {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
		}
		f, err := os.OpenFile(path, flags, 0o644)
		if err != nil {
			return engine.Value{}, fmt.Errorf("open %s: %w", path, err)
		}
		if _, err := f.WriteString(content); err != nil {
			f.Close()
			return engine.Value{}, fmt.Errorf("write %s: %w", path, err)
		}
		if err := f.Close(); err != nil {
			return engine.Value{}, fmt.Errorf("close %s: %w", path, err)
		}
		if t.OutputType == parser.TypeFilepath {
			return engine.Value{Type: parser.TypeFilepath, Raw: path}, nil
		}
		return asOutput(content, t.OutputType)

	default:
		return engine.Value{}, fmt.Errorf("unknown file operation %q", t.Operation)
	}
}
