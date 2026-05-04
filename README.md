# Orcha

Unix pipes for AI workflows. Define reusable tasks and linear pipelines in one
`orcha.yaml` file; execute them with one command.

Orcha is a single-binary Go program, that helps developers, create their own automated DAG workflow, with ease. Managing multiple tasks with one call!
say bye to repetitve: 
- file operation.
- API calls. 
- HTTP requests. 

Instead design the workflow as a chad, run one command.


```yaml
# orcha.yaml
tasks:
  read-article:
    type: file
    path: "{{$input}}"

  summarize:
    type: ai
    provider: openai
    prompt: "Summarize in three bullet points:\n\n{{$input}}"

  save:
    type: file
    operation: write
    path: "./summary.txt"
    content: "{{$input}}"

pipelines:
  summarize-article:
    steps:
      - task: read-article
      - task: summarize
      - task: save
```

```bash
$ export OPENAI_API_KEY=sk-...
$ orcha run summarize-article -f ./article.txt
> read-article ...
+ read-article (1ms)
> summarize ...
+ summarize (920ms)
> save ...
+ save (0ms)
-- done in 921ms
./summary.txt
```

## Quick Start

### Installation

```bash
pip install orcha-dev
```

The first time you run `orcha`, it downloads a small Go binary to
`~/.orcha/bin/` and verifies its sha256. Subsequent runs are zero-network.

### Hello, world

Create a file called `orcha.yaml`:

```yaml
tasks:
  greet:
    type: ai
    provider: openai
    prompt: "Say hello to {{$input}} in one sentence."

pipelines:
  hi:
    steps:
      - task: greet
```

Then:

```bash
export OPENAI_API_KEY=sk-...
orcha run hi -i "the world"
```

## Usage

### From the shell

```bash
# Run a pipeline; pretty progress on stderr, final result on stdout.
orcha run <pipeline> [-y orcha.yaml] [-i STR | -f FILE] [--json]

# Print version.
orcha version
```

Input precedence: `-i` (inline string) → `-f` (file content) → stdin.

`--json` swaps pretty progress for a JSON-line event stream — handy for piping
into `jq` or another tool.

### From Python

```python
from orcha import Orcha

o = Orcha("./orcha.yaml")

# Stream events as the pipeline runs.
for event in o.run("summarize-article", "./article.txt"):
    print(event.type, event.task, event.elapsed_ms)

# Or get just the final output.
result = o.run_sync("summarize-article", "./article.txt")
```

### Examples

`examples/orcha.yaml` defines a tiny three-step pipeline that reads a file,
summarizes it with OpenAI, and writes the result to disk:

```bash
cd examples
export OPENAI_API_KEY=sk-...
orcha run summarize-and-translate -i topic.txt
```

## Project structure

```
orcha/
├── cmd/
│   ├── orcha/         # production IPC binary (consumed by the Python SDK)
│   ├── orcha-run/     # user-facing Go CLI driver
│   └── orcha-debug/   # developer debug entry point
├── pkg/
│   ├── provider.go    # Provider plugin interface (Name, DefaultModel, Complete)
│   ├── registry.go    # global provider registry + env-var key resolution
│   └── openai/        # built-in OpenAI provider (REST, no SDK dependency)
├── internal/
│   ├── parser/        # YAML loader, schema validation, type-flow check
│   ├── engine/        # executor, value types, $input/$env interpolation
│   ├── runners/       # ai, http, file task runners
│   └── ipc/           # JSON-line stdin/stdout protocol
├── python/
│   └── orcha/         # Python SDK + `orcha` shell command
├── examples/          # sample workflows and inputs
└── tools/             # build helpers (manifest generation, etc.)
```

## Configuration

### API keys

Resolved from environment variables by convention:

| Provider | Variable               |
|----------|------------------------|
| openai   | `OPENAI_API_KEY`       |
| custom   | `ORCHA_<NAME>_API_KEY` |

Providers never read env vars directly — the registry passes the key into the
provider's `Complete()` call. Roll your own by implementing the `Provider`
interface and calling `providers.Register()`.

### Task types

| Type   | Required fields    | Defaulted fields                                                          |
|--------|--------------------|---------------------------------------------------------------------------|
| `ai`   | `provider, prompt` | `model` → provider's default; `system` → omitted; `output_type` → `text`  |
| `http` | `url`              | `method` → `GET`; `output_type` → `text`                                  |
| `file` | `path`             | `operation` → `read`; `output_type` → `text` (read) / `filepath` (write)  |

### Type compatibility

Outputs of step *N* must be compatible with the input of step *N+1*. The check
runs at parse time:

|                  | → text | → json | → filepath | → list |
|------------------|:-----:|:-----:|:---------:|:-----:|
| **text →**       | OK    | --    | OK        | --    |
| **json →**       | OK    | OK    | --        | --    |
| **filepath →**   | OK    | --    | OK        | --    |
| **list →**       | OK    | --    | --        | OK    |

## Contributing

The codebase is small on purpose. The Go engine is ~1k lines, the Python SDK
is ~300 lines, and the wire protocol between them is one JSON object on
stdin and one JSON object per line on stdout.

Run the full test suite:

```bash
go test ./...
( cd python && python -m pytest tests/ )
```

Open issues and PRs at https://github.com/ryfoo/orcha.

## License & Credits

Orcha is released under the MIT License. See `LICENSE` for the full text.

Built with [`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3) and the
Go standard library.
