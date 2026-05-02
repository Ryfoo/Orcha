// orcha is the v1 binary that the Python SDK spawns. It reads a single JSON
// command from stdin, executes it, streams JSON-line events to stdout, and
// exits. Errors that prevent even reading the command are written to stderr
// and exit non-zero.
package main

import (
	"fmt"
	"os"

	"github.com/ryfoo/orcha/internal/ipc"

	// Side-effect import: registers the OpenAI provider in the global registry.
	_ "github.com/ryfoo/orcha/pkg/openai"
)

const version = "0.1.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v", "version":
			fmt.Println(version)
			return
		}
	}
	if err := ipc.Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "orcha:", err)
		os.Exit(1)
	}
}
