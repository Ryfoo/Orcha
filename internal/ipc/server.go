package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/ryfoo/orcha/internal/engine"
	"github.com/ryfoo/orcha/internal/parser"
	"github.com/ryfoo/orcha/internal/runners"
)

// Serve reads exactly one Command from stdin, runs it, streams events to
// stdout as newline-delimited JSON, and returns. The single-shot model
// matches the spec: "read JSON command from stdin -> parse YAML -> execute
// pipeline -> write JSON events to stdout -> exit".
func Serve(stdin io.Reader, stdout io.Writer) error {
	dec := json.NewDecoder(bufio.NewReader(stdin))
	var cmd Command
	if err := dec.Decode(&cmd); err != nil {
		if err == io.EOF {
			return fmt.Errorf("no command received on stdin")
		}
		return fmt.Errorf("decode command: %w", err)
	}

	// Each event is encoded with a single Encoder protected by a mutex so
	// concurrent emits (future use) cannot interleave bytes.
	enc := json.NewEncoder(stdout)
	var emitMu sync.Mutex
	emit := func(ev engine.Event) {
		emitMu.Lock()
		defer emitMu.Unlock()
		_ = enc.Encode(ev)
	}

	switch cmd.Command {
	case "run":
		return runCommand(cmd, emit)
	default:
		emit(engine.Event{
			Type:  "task_fail",
			Index: -1,
			Error: fmt.Sprintf("unknown command %q", cmd.Command),
		})
		return nil
	}
}

func runCommand(cmd Command, emit func(engine.Event)) error {
	var doc *parser.Document
	var err error
	switch {
	case cmd.YAMLPath != "":
		doc, err = parser.Load(cmd.YAMLPath)
	case cmd.YAML != "":
		doc, err = parser.Parse([]byte(cmd.YAML))
	default:
		err = fmt.Errorf("command requires either yaml_path or yaml")
	}
	if err != nil {
		emit(engine.Event{Type: "task_fail", Index: -1, Error: err.Error()})
		return nil
	}
	if cmd.Pipeline == "" {
		emit(engine.Event{Type: "task_fail", Index: -1, Error: "pipeline name is required"})
		return nil
	}

	engine.Run(doc, cmd.Pipeline, cmd.Input, runners.Dispatch, emit)
	return nil
}
