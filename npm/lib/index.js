'use strict';

// Programmatic JS/TS API for the orcha-dev npm package.
//
// Spawns the Go binary as a subprocess, sends one JSON command on stdin, and
// streams JSON-line events back from stdout. The protocol is one-shot per
// invocation: every run() call is a fresh process. The wire format is
// shared with the Python SDK so the binary stays single-source-of-truth.

const fs = require('node:fs');
const path = require('node:path');
const { spawn } = require('node:child_process');

const { resolveBinary } = require('./downloader');
const { OrchaError, TaskFailed } = require('./errors');

class Orcha {
  // Use Orcha.create() instead — resolving the binary involves I/O, so the
  // factory is async. The bare constructor is kept for callers that already
  // have an absolute binary path (tests, monorepo dev loops).
  constructor(yamlPath, binaryPath) {
    if (!yamlPath) throw new OrchaError('yamlPath is required');
    this.yamlPath = path.resolve(yamlPath);
    if (!fs.existsSync(this.yamlPath)) {
      throw new OrchaError(`yaml file not found: ${this.yamlPath}`);
    }
    if (!binaryPath) {
      throw new OrchaError('binaryPath is required (use Orcha.create() to auto-resolve)');
    }
    this.binary = path.resolve(binaryPath);
  }

  static async create(yamlPath, options = {}) {
    const binaryPath = options.binaryPath
      ? path.resolve(options.binaryPath)
      : await resolveBinary();
    return new Orcha(yamlPath, binaryPath);
  }

  // run() returns an async iterable that yields one event per JSON line the
  // binary emits. The returned iterator is consumable with `for await`. If
  // the engine exits non-zero, the iterator throws OrchaError after the last
  // event; per-task failures arrive as `task_fail` events first.
  run(target, input) {
    if (input instanceof Buffer || input instanceof Uint8Array) {
      input = Buffer.from(input).toString('utf8');
    }
    const cmd = {
      command: 'run',
      pipeline: target,
      yaml_path: this.yamlPath,
      input: input === undefined ? null : input,
    };
    return this._spawn(cmd);
  }

  // runToCompletion() drains the event stream and returns the final pipeline
  // output. Throws TaskFailed if any task fails so callers can use plain
  // try/catch. Equivalent to Python's run_sync().
  async runToCompletion(target, input) {
    let final;
    for await (const ev of this.run(target, input)) {
      if (ev.type === 'task_fail') {
        throw new TaskFailed(ev.task, ev.index, ev.error || 'unknown error');
      }
      if (ev.type === 'pipeline_complete') {
        final = ev.output;
      } else if (ev.type === 'task_complete') {
        // Track latest in case the binary doesn't emit pipeline_complete
        // (it always does in v1, but defensiveness here is cheap).
        final = ev.output;
      }
    }
    return final;
  }

  _spawn(cmd) {
    const proc = spawn(this.binary, [], {
      stdio: ['pipe', 'pipe', 'pipe'],
    });

    // Drain stderr eagerly so a full pipe buffer never blocks the engine
    // mid-run; surface the captured text only if the process exits non-zero.
    let stderrBuf = '';
    proc.stderr.setEncoding('utf8');
    proc.stderr.on('data', (chunk) => { stderrBuf += chunk; });

    let writeError = null;
    try {
      proc.stdin.write(JSON.stringify(cmd) + '\n');
      proc.stdin.end();
    } catch (e) {
      writeError = e;
    }

    return iterateLines(proc.stdout, () => stderrBuf, proc, writeError);
  }
}

// iterateLines consumes a Readable byte stream and yields one parsed JSON
// object per newline. Trailing partial lines are buffered until the next
// chunk arrives. When the stream ends, we await the child's exit code and
// throw if it's non-zero.
function iterateLines(stdout, getStderr, proc, initialError) {
  return {
    async *[Symbol.asyncIterator]() {
      if (initialError) {
        proc.kill();
        await new Promise((resolve) => proc.once('exit', resolve));
        throw new OrchaError(
          `failed to send command to engine: ${initialError.message}; stderr=${getStderr()}`,
        );
      }

      stdout.setEncoding('utf8');
      let buf = '';
      try {
        for await (const chunk of stdout) {
          buf += chunk;
          let nl;
          while ((nl = buf.indexOf('\n')) !== -1) {
            const line = buf.slice(0, nl).trim();
            buf = buf.slice(nl + 1);
            if (!line) continue;
            let raw;
            try {
              raw = JSON.parse(line);
            } catch (_) {
              continue;
            }
            yield raw;
          }
        }
        const tail = buf.trim();
        if (tail) {
          try {
            yield JSON.parse(tail);
          } catch (_) {
            // Ignore; matches Python which silently drops malformed lines.
          }
        }
      } finally {
        const code = await new Promise((resolve) => {
          if (proc.exitCode !== null) resolve(proc.exitCode);
          else proc.once('exit', (c) => resolve(c));
        });
        if (code !== 0) {
          const stderr = (getStderr() || '').trim();
          throw new OrchaError(
            `orcha engine exited ${code}` + (stderr ? `: ${stderr}` : ''),
          );
        }
      }
    },
  };
}

module.exports = { Orcha, OrchaError, TaskFailed };
