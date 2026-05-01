package engine

import (
	"fmt"
	"time"

	"github.com/ryfoo/orcha/internal/parser"
)

// Event is one notification emitted by the executor as the pipeline runs.
// These are serialized to JSON and streamed over stdout to the Python SDK.
type Event struct {
	Type       string            `json:"type"`
	Task       string            `json:"task,omitempty"`
	Index      int               `json:"index"`
	Output     any               `json:"output,omitempty"`
	OutputType parser.OutputType `json:"output_type,omitempty"`
	Error      string            `json:"error,omitempty"`
	ElapsedMs  int64             `json:"elapsed_ms"`
}

// Runner executes a single task. The engine supplies the input and expects a
// Value whose Type matches task.OutputType — runners that can't satisfy that
// must return an error.
type Runner func(t *parser.Task, input Value) (Value, error)

// Emitter is the channel by which events leave the engine. The IPC layer wires
// this to JSON-line writes; tests can capture events into a slice.
type Emitter func(Event)

// Run executes the pipeline (or single task) named `target` against `userInput`
// and streams events through emit. It always emits a terminal event:
// task_fail (and stops) on the first error, or pipeline_complete on success.
func Run(doc *parser.Document, target string, userInput any, runner Runner, emit Emitter) {
	pipeline, err := resolveTarget(doc, target)
	if err != nil {
		emit(Event{Type: "task_fail", Index: -1, Error: err.Error()})
		return
	}

	firstTask := doc.Tasks[pipeline.Steps[0].Task]
	current := CoerceUserInput(userInput, firstTask.InputType())

	pipelineStart := time.Now()
	for i, step := range pipeline.Steps {
		task := doc.Tasks[step.Task]
		emit(Event{Type: "task_start", Task: task.Name, Index: i})

		stepStart := time.Now()
		out, err := runner(task, current)
		elapsed := time.Since(stepStart).Milliseconds()
		if err != nil {
			emit(Event{
				Type:      "task_fail",
				Task:      task.Name,
				Index:     i,
				Error:     err.Error(),
				ElapsedMs: elapsed,
			})
			return
		}
		if out.Type != task.OutputType {
			emit(Event{
				Type:      "task_fail",
				Task:      task.Name,
				Index:     i,
				Error:     fmt.Sprintf("runner returned type %q but task declared output_type %q", out.Type, task.OutputType),
				ElapsedMs: elapsed,
			})
			return
		}
		emit(Event{
			Type:       "task_complete",
			Task:       task.Name,
			Index:      i,
			Output:     out.Raw,
			OutputType: out.Type,
			ElapsedMs:  elapsed,
		})
		current = out
	}

	emit(Event{
		Type:       "pipeline_complete",
		Index:      len(pipeline.Steps) - 1,
		Output:     current.Raw,
		OutputType: current.Type,
		ElapsedMs:  time.Since(pipelineStart).Milliseconds(),
	})
}

// resolveTarget accepts either a pipeline name or a task name. Single tasks
// run as one-step pipelines so the rest of the engine doesn't branch.
func resolveTarget(doc *parser.Document, target string) (*parser.Pipeline, error) {
	if p, ok := doc.Pipelines[target]; ok {
		return p, nil
	}
	if _, ok := doc.Tasks[target]; ok {
		return &parser.Pipeline{
			Name:  target,
			Steps: []parser.Step{{Task: target}},
		}, nil
	}
	return nil, fmt.Errorf("no pipeline or task named %q", target)
}
