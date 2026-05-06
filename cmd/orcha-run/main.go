// orcha-run is the user-facing driver for executing an orcha.yaml pipeline
// from the command line, end-to-end, with real provider API calls.
//
// Usage:
//
//	orcha-run [flags] <pipeline-name>
//
// Input precedence (highest first):
//
//	-input STRING          inline string
//	-input-file PATH       read input from a file
//	stdin                  read all of stdin if neither flag is given
//
// By default, progress events are pretty-printed to stderr and the final
// pipeline output is printed to stdout (so you can pipe it). Pass -json to
// get the raw JSON-line event stream on stdout instead.
//
// Example:
//
//	export OPENAI_API_KEY=sk-...
//	orcha-run -yaml ./examples/orcha.yaml \
//	  -input-file ./examples/article.txt \
//	  summarize-article
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ryfoo/orcha/internal/engine"
	"github.com/ryfoo/orcha/internal/parser"
	"github.com/ryfoo/orcha/internal/runners"

	// Side-effect imports — register built-in providers so AI tasks with
	// matching `provider:` values work out of the box. Adding a new
	// provider is a matter of adding another blank import here.
	_ "github.com/ryfoo/orcha/pkg/anthropic"
	_ "github.com/ryfoo/orcha/pkg/deepseek"
	_ "github.com/ryfoo/orcha/pkg/openai"
)

func main() {
	var (
		yamlPath  = flag.String("yaml", "", "path to orcha.yaml (defaults to ./orcha.yaml in cwd)")
		input     = flag.String("input", "", "inline input string passed to the first task")
		inputFile = flag.String("input-file", "", "read input from this file")
		jsonOut   = flag.Bool("json", false, "stream raw JSON-line events to stdout instead of pretty progress")
	)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "orcha-run — execute an orcha.yaml pipeline.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  orcha-run [flags] <pipeline-name>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Environment:")
		fmt.Fprintln(os.Stderr, "  OPENAI_API_KEY     used by the openai provider")
		fmt.Fprintln(os.Stderr, "  ANTHROPIC_API_KEY  used by the anthropic provider")
		fmt.Fprintln(os.Stderr, "  DEEPSEEK_API_KEY   used by the deepseek provider")
	}
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		os.Exit(2)
	}
	pipeline := args[0]

	resolvedYAML, err := resolveYAMLPath(*yamlPath)
	if err != nil {
		fail(err)
	}
	doc, err := parser.Load(resolvedYAML)
	if err != nil {
		fail(err)
	}

	inputVal, err := readInput(*input, *inputFile)
	if err != nil {
		fail(err)
	}

	emit, finish := makeEmitter(*jsonOut)
	engine.Run(doc, pipeline, inputVal, runners.Dispatch, emit)
	os.Exit(finish())
}

// resolveYAMLPath honors -yaml when set, otherwise looks for ./orcha.yaml in
// the working directory. Returning an absolute path makes errors clearer.
func resolveYAMLPath(flagVal string) (string, error) {
	if flagVal != "" {
		abs, err := filepath.Abs(flagVal)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(cwd, "orcha.yaml")
	if _, err := os.Stat(candidate); err != nil {
		return "", fmt.Errorf("no -yaml flag and ./orcha.yaml not found in %s", cwd)
	}
	return candidate, nil
}

// readInput resolves the user's input following the precedence documented in
// the package comment. Returns an empty string when no input was supplied —
// pipelines whose first task doesn't read $input still run fine.
func readInput(inline, file string) (string, error) {
	switch {
	case inline != "" && file != "":
		return "", errors.New("pass either -input or -input-file, not both")
	case inline != "":
		return inline, nil
	case file != "":
		raw, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
	// If stdin is a pipe (i.e. has data attached), read it. If it's a TTY,
	// treat that as "no input" rather than blocking the user forever.
	if isStdinPiped() {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
	return "", nil
}

func isStdinPiped() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}

// makeEmitter returns the event handler the engine should call, plus a
// finish() hook that returns the desired exit code after the pipeline ends.
// Two output modes:
//
//   - jsonOut=true : raw JSON-line events to stdout (machine-friendly).
//   - jsonOut=false: human-readable progress to stderr, final output to stdout.
func makeEmitter(jsonOut bool) (engine.Emitter, func() int) {
	exitCode := 0
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		emit := func(ev engine.Event) {
			if ev.Type == "task_fail" {
				exitCode = 1
			}
			_ = enc.Encode(ev)
		}
		return emit, func() int { return exitCode }
	}

	var (
		startTimes = map[string]time.Time{}
		final      any
		finalType  parser.OutputType
	)
	emit := func(ev engine.Event) {
		switch ev.Type {
		case "task_start":
			startTimes[ev.Task] = time.Now()
			fmt.Fprintf(os.Stderr, "▸ %s ...\n", ev.Task)
		case "task_complete":
			fmt.Fprintf(os.Stderr, "✓ %s (%dms)\n", ev.Task, ev.ElapsedMs)
		case "task_fail":
			exitCode = 1
			fmt.Fprintf(os.Stderr, "✗ %s — %s\n", ev.Task, ev.Error)
		case "pipeline_complete":
			final = ev.Output
			finalType = ev.OutputType
			fmt.Fprintf(os.Stderr, "── done in %dms\n", ev.ElapsedMs)
		}
	}
	finish := func() int {
		if exitCode == 0 && final != nil {
			printFinal(final, finalType)
		}
		return exitCode
	}
	return emit, finish
}

// printFinal sends the pipeline's terminal output to stdout in a form that
// pipes well: text/filepath are printed as-is; json/list are JSON-encoded so
// downstream tools (jq, etc.) can consume them.
func printFinal(v any, t parser.OutputType) {
	switch t {
	case parser.TypeText, parser.TypeFilepath:
		if s, ok := v.(string); ok {
			fmt.Fprintln(os.Stdout, s)
			return
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "orcha-run:", err)
	os.Exit(1)
}
