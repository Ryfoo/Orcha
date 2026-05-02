package parser

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads and parses an orcha.yaml file from disk. It runs structural
// validation (required fields per task type) and type-flow validation across
// pipelines. Both classes of error are returned as parse errors so that the
// pipeline never starts with a known-bad configuration.
func Load(path string) (*Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return Parse(raw)
}

// Parse is Load's in-memory variant — used by tests and by callers that
// already hold the YAML bytes.
func Parse(data []byte) (*Document, error) {
	var doc Document
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}
	if doc.Tasks == nil {
		doc.Tasks = map[string]*Task{}
	}
	if doc.Pipelines == nil {
		doc.Pipelines = map[string]*Pipeline{}
	}
	for name, t := range doc.Tasks {
		if t == nil {
			return nil, fmt.Errorf("task %q is empty", name)
		}
		t.Name = name
		if err := validateTask(t); err != nil {
			return nil, err
		}
	}
	for name, p := range doc.Pipelines {
		if p == nil {
			return nil, fmt.Errorf("pipeline %q is empty", name)
		}
		p.Name = name
		if err := validatePipeline(p, doc.Tasks); err != nil {
			return nil, err
		}
	}
	for _, p := range doc.Pipelines {
		if err := TypeCheckPipeline(p, doc.Tasks); err != nil {
			return nil, err
		}
	}
	return &doc, nil
}

func validateTask(t *Task) error {
	if t.OutputType == "" {
		return fmt.Errorf("task %q: output_type is required", t.Name)
	}
	switch t.OutputType {
	case TypeText, TypeJSON, TypeFilepath, TypeList:
	default:
		return fmt.Errorf("task %q: unknown output_type %q", t.Name, t.OutputType)
	}

	switch t.Type {
	case KindAI:
		if t.Provider == "" {
			return fmt.Errorf("task %q: ai task requires 'provider'", t.Name)
		}
		if t.Model == "" {
			return fmt.Errorf("task %q: ai task requires 'model'", t.Name)
		}
		if t.System == "" {
			return fmt.Errorf("task %q: ai task requires 'system'", t.Name)
		}
		if t.Prompt == "" {
			return fmt.Errorf("task %q: ai task requires 'prompt'", t.Name)
		}
	case KindHTTP:
		if t.Method == "" {
			return fmt.Errorf("task %q: http task requires 'method'", t.Name)
		}
		switch strings.ToUpper(t.Method) {
		case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD":
		default:
			return fmt.Errorf("task %q: unsupported http method %q", t.Name, t.Method)
		}
		if t.URL == "" {
			return fmt.Errorf("task %q: http task requires 'url'", t.Name)
		}
	case KindFile:
		switch t.Operation {
		case OpRead, OpWrite, OpAppend:
		case "":
			return fmt.Errorf("task %q: file task requires 'operation'", t.Name)
		default:
			return fmt.Errorf("task %q: unknown file operation %q", t.Name, t.Operation)
		}
		if t.Path == "" {
			return fmt.Errorf("task %q: file task requires 'path'", t.Name)
		}
		if (t.Operation == OpWrite || t.Operation == OpAppend) && t.Content == "" {
			return fmt.Errorf("task %q: file %s requires 'content'", t.Name, t.Operation)
		}
	case "":
		return fmt.Errorf("task %q: 'type' is required", t.Name)
	default:
		return fmt.Errorf("task %q: unknown type %q (expected ai|http|file)", t.Name, t.Type)
	}
	return nil
}

func validatePipeline(p *Pipeline, tasks map[string]*Task) error {
	if len(p.Steps) == 0 {
		return fmt.Errorf("pipeline %q: must declare at least one step", p.Name)
	}
	for i, s := range p.Steps {
		if s.Task == "" {
			return fmt.Errorf("pipeline %q: step %d missing 'task'", p.Name, i)
		}
		if _, ok := tasks[s.Task]; !ok {
			return fmt.Errorf("pipeline %q: step %d references unknown task %q", p.Name, i, s.Task)
		}
	}
	return nil
}
