package parser

// OutputType is one of the four supported flow types.
type OutputType string

const (
	TypeText     OutputType = "text"
	TypeJSON     OutputType = "json"
	TypeFilepath OutputType = "filepath"
	TypeList     OutputType = "list"
)

// TaskKind discriminates the runner that handles a task.
type TaskKind string

const (
	KindAI   TaskKind = "ai"
	KindHTTP TaskKind = "http"
	KindFile TaskKind = "file"
)

// FileOperation is the read/write/append variant of a file task.
type FileOperation string

const (
	OpRead   FileOperation = "read"
	OpWrite  FileOperation = "write"
	OpAppend FileOperation = "append"
)

// Task is the union of all task-type fields. Type-specific validation lives in
// validate(). Name is filled in by the YAML loader from the map key.
type Task struct {
	Name       string     `yaml:"-"`
	Type       TaskKind   `yaml:"type"`
	OutputType OutputType `yaml:"output_type"`

	// AI fields
	Provider    string  `yaml:"provider,omitempty"`
	Model       string  `yaml:"model,omitempty"`
	System      string  `yaml:"system,omitempty"`
	Prompt      string  `yaml:"prompt,omitempty"`
	Temperature float64 `yaml:"temperature,omitempty"`
	MaxTokens   int     `yaml:"max_tokens,omitempty"`

	// HTTP fields
	Method  string            `yaml:"method,omitempty"`
	URL     string            `yaml:"url,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty"`

	// File fields
	Operation FileOperation `yaml:"operation,omitempty"`
	Path      string        `yaml:"path,omitempty"`
	Content   string        `yaml:"content,omitempty"`
}

// Step is one entry in a pipeline's `steps:` list.
type Step struct {
	Task string `yaml:"task"`
}

// Pipeline is a named, ordered sequence of task references.
type Pipeline struct {
	Name        string `yaml:"-"`
	Description string `yaml:"description,omitempty"`
	Steps       []Step `yaml:"steps"`
}

// Document is the parsed orcha.yaml.
type Document struct {
	Tasks     map[string]*Task     `yaml:"tasks"`
	Pipelines map[string]*Pipeline `yaml:"pipelines"`
}

// InputType returns the type a task expects to receive when {{$input}} is
// interpolated into its template fields. This is what typecheck compares
// against the previous step's output_type.
func (t *Task) InputType() OutputType {
	switch t.Type {
	case KindAI, KindHTTP:
		return TypeText
	case KindFile:
		if t.Operation == OpRead {
			return TypeFilepath
		}
		return TypeText
	}
	return TypeText
}
