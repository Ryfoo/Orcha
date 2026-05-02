package ipc

// Command is what the Python SDK sends on stdin. v1 supports a single command
// type ("run"); the field is kept so future commands ("validate", "list",
// etc.) can be added without breaking the wire format.
type Command struct {
	Command  string `json:"command"`
	Pipeline string `json:"pipeline"`
	YAMLPath string `json:"yaml_path,omitempty"`
	YAML     string `json:"yaml,omitempty"`
	Input    any    `json:"input,omitempty"`
}
