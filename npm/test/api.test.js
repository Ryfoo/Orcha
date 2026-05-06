'use strict';

// Integration test for the JS API. Requires the engine binary at
// dist/orcha (run `make build` from the repo root first); skips otherwise.
// Avoids the network — never touches resolveBinary().

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const { Orcha, TaskFailed } = require('../lib/index');

const repoRoot = path.resolve(__dirname, '..', '..');
const binaryPath = path.join(repoRoot, 'dist', 'orcha');
const haveBinary = fs.existsSync(binaryPath);

function tmpYaml(contents) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'orcha-npm-'));
  const file = path.join(dir, 'orcha.yaml');
  fs.writeFileSync(file, contents);
  return file;
}

test('runToCompletion returns the final output', { skip: !haveBinary }, async () => {
  const yaml = tmpYaml(
    `tasks:
  echo:
    type: file
    operation: write
    path: ./out.txt
    content: "{{$input}}"

pipelines:
  go:
    steps:
      - task: echo
`,
  );
  const orcha = new Orcha(yaml, binaryPath);
  const result = await orcha.runToCompletion('go', 'hello');
  assert.equal(typeof result, 'string');
  assert.match(result, /out\.txt$/);
  fs.rmSync(path.dirname(yaml), { recursive: true, force: true });
});

test('run() yields task events in order', { skip: !haveBinary }, async () => {
  const yaml = tmpYaml(
    `tasks:
  noop:
    type: file
    operation: write
    path: ./out2.txt
    content: hi

pipelines:
  go:
    steps:
      - task: noop
`,
  );
  const orcha = new Orcha(yaml, binaryPath);
  const types = [];
  for await (const ev of orcha.run('go', null)) {
    types.push(ev.type);
  }
  assert.deepEqual(types, ['task_start', 'task_complete', 'pipeline_complete']);
  fs.rmSync(path.dirname(yaml), { recursive: true, force: true });
});

test('TaskFailed is thrown on parse error', { skip: !haveBinary }, async () => {
  const yaml = tmpYaml('not: a: valid: yaml: pipeline');
  const orcha = new Orcha(yaml, binaryPath);
  await assert.rejects(() => orcha.runToCompletion('go', null), TaskFailed);
  fs.rmSync(path.dirname(yaml), { recursive: true, force: true });
});
