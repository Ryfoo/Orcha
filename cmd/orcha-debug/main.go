// orcha-debug is a developer-only entry point for exercising the Go engine
// without the Python SDK or the JSON-on-stdin protocol. It is NOT shipped to
// end users — it exists so a contributor can:
//
//  1. Run any pipeline with one shell command.
//  2. Step through the executor / runners under `dlv exec`.
//
// Example:
//
//	go run ./cmd/orcha-debug -yaml ./examples/orcha.yaml \
//	    -pipeline summarize-article -input ./examples/article.txt -pretty
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ryfoo/orcha/internal/engine"
	"github.com/ryfoo/orcha/internal/parser"
	"github.com/ryfoo/orcha/internal/runners"

	_ "github.com/ryfoo/orcha/pkg/anthropic"
	_ "github.com/ryfoo/orcha/pkg/deepseek"
	_ "github.com/ryfoo/orcha/pkg/openai"
)

func main() {
	var (
		yamlPath  = flag.String("yaml", "", "path to orcha.yaml (required)")
		pipeline  = flag.String("pipeline", "", "pipeline or task name to run (required)")
		input     = flag.String("input", "", "input string passed to the first task")
		inputFile = flag.String("input-file", "", "read input from a file instead of -input")
		pretty    = flag.Bool("pretty", false, "pretty-print events as indented JSON")
	)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "orcha-debug — run an orcha.yaml pipeline directly through the Go engine.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  orcha-debug -yaml FILE -pipeline NAME [-input STR | -input-file FILE] [-pretty]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *yamlPath == "" || *pipeline == "" {
		flag.Usage()
		os.Exit(2)
	}

	doc, err := parser.Load(*yamlPath)
	if err != nil {
		fail(err)
	}

	var inputVal any = *input
	if *inputFile != "" {
		raw, err := os.ReadFile(*inputFile)
		if err != nil {
			fail(err)
		}
		inputVal = string(raw)
	}

	enc := json.NewEncoder(os.Stdout)
	if *pretty {
		enc.SetIndent("", "  ")
	}

	exitCode := 0
	emit := func(ev engine.Event) {
		if ev.Type == "task_fail" {
			exitCode = 1
		}
		_ = enc.Encode(ev)
	}

	engine.Run(doc, *pipeline, inputVal, runners.Dispatch, emit)
	os.Exit(exitCode)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "orcha-debug:", err)
	os.Exit(1)
}
