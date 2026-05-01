package engine_test

// End-to-end test for the Go engine using only file tasks so it runs without
// any API key. To step through the engine:
//
//	go test -v ./internal/engine/...
//	dlv test ./internal/engine/...

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ryfoo/orcha/internal/engine"
	"github.com/ryfoo/orcha/internal/parser"
	"github.com/ryfoo/orcha/internal/runners"
)

func TestPipelineEndToEnd(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "in.txt")
	out := filepath.Join(tmp, "out.txt")
	if err := os.WriteFile(src, []byte("hello, world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OUT_PATH", out)

	doc, err := parser.Load("testdata/pipeline.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var events []engine.Event
	engine.Run(doc, "copy", src, runners.Dispatch, func(ev engine.Event) {
		events = append(events, ev)
	})

	wantTypes := []string{
		"task_start", "task_complete",
		"task_start", "task_complete",
		"pipeline_complete",
	}
	if len(events) != len(wantTypes) {
		t.Fatalf("got %d events, want %d:\n%+v", len(events), len(wantTypes), events)
	}
	for i, want := range wantTypes {
		if events[i].Type != want {
			t.Errorf("event[%d].Type = %q, want %q", i, events[i].Type, want)
		}
	}

	if got := events[1].Output; got != "hello, world\n" {
		t.Errorf("step 0 output = %q, want %q", got, "hello, world\n")
	}
	if got := events[3].Output; got != out {
		t.Errorf("step 1 output = %q, want %q", got, out)
	}

	contents, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "hello, world\n" {
		t.Errorf("on-disk content = %q, want %q", contents, "hello, world\n")
	}
}

func TestPipelineFailFast(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "does-not-exist.txt")
	t.Setenv("OUT_PATH", filepath.Join(tmp, "never-written.txt"))

	doc, err := parser.Load("testdata/pipeline.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var events []engine.Event
	engine.Run(doc, "copy", missing, runners.Dispatch, func(ev engine.Event) {
		events = append(events, ev)
	})

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2: %+v", len(events), events)
	}
	if events[0].Type != "task_start" || events[1].Type != "task_fail" {
		t.Fatalf("unexpected sequence: %+v", events)
	}
	if events[1].Task != "read-source" {
		t.Errorf("task_fail.Task = %q, want read-source", events[1].Task)
	}
	if events[1].Error == "" {
		t.Error("task_fail missing error message")
	}
}
