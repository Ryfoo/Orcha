# orcha-dev

[![npm version](https://img.shields.io/npm/v/orcha-dev.svg)](https://www.npmjs.com/package/orcha-dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Unix pipes for AI workflows. Define reusable tasks and linear pipelines in
one `orcha.yaml` file; execute them from Node, the shell, or both.

`orcha-dev` is a thin Node wrapper around the same single Go binary used by
the Python SDK. The binary is downloaded into `~/.orcha/bin/` on first run
and verified by sha256; later runs are zero-network.

Note: Linear pipelines (only) are supported
meaning that when designing a pipeline, know that the output of the N-th steps is the input of the N+1-th step.

## Install

```bash
npm install orcha-dev
```

Node 18+ is required. The first invocation downloads the engine binary that
matches the package version; pin a specific tag with `ORCHA_BINARY_VERSION`
or point at a local build with `ORCHA_BINARY_PATH=/abs/path/to/orcha`.

## CLI

```bash
# Run a pipeline; pretty progress on stderr, final output on stdout.
npx orcha run <pipeline> [-y orcha.yaml] [-i STR | -f FILE] [--json]

# Print version.
npx orcha version
```

Input precedence: `-i` (inline string) → `-f` (file content) → stdin.
`--json` swaps pretty progress for a JSON-line event stream.


## Usage

### orcha.yaml

Example:
```yaml
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
    content: "./results.txt"

pipelines:
  summarize-article:
    steps:
      - task: read-article
      - task: summarize
      - task: save
```

### Defined Formats

file operations:
```yaml
task:
  type: file
      operation: write/read     #read by default.
      path: "path/to/seeding/file"     
      content: "{{$input}}"     #only specified when the operation is write.

task:
  type: ai
  provider: openai/anthropic/deepseek                       #the default model will be chosen automatically.
  model:                                                    #optional if you need to specify a model.
  prompt: "Summarize in three bullet points:\n\n{{$input}}" #the prompt alongside the input is to be included.
```

## Programmatic API

```js
const { Orcha } = require('orcha-dev');

const orcha = await Orcha.create('./orcha.yaml');

// Stream events as the pipeline runs.
for await (const event of orcha.run('summarize-article', './article.txt')) {
  console.log(event.type, event.task, event.elapsed_ms);
}

// Or get just the final output.
const result = await orcha.runToCompletion('summarize-article', './article.txt');
```

TypeScript types ship with the package — `import { Orcha, OrchaEvent } from 'orcha-dev'` works out of the box.

## Providers

Set the matching environment variable for whichever provider your YAML uses:

| Provider   | Variable               |
|------------|------------------------|
| `openai`   | `OPENAI_API_KEY`       |
| `anthropic`| `ANTHROPIC_API_KEY`    |
| `deepseek` | `DEEPSEEK_API_KEY`     |
| custom     | `ORCHA_<NAME>_API_KEY` |

## Links

- Source: <https://github.com/ryfoo/orcha>
- Issues: <https://github.com/ryfoo/orcha/issues>
- Python SDK: <https://pypi.org/project/orcha-dev/>

MIT licensed.
