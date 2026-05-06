#!/usr/bin/env node
'use strict';

// The `orcha` shell command shipped via the `orcha-dev` npm package. Mirrors
// the UX of the Go orcha-run binary and the Python `orcha` script so users
// who pick npm get the same subcommands and flags as everyone else.
//
//   orcha run <pipeline> [-y YAML] [-i STR | -f FILE] [--json]
//   orcha version

const fs = require('node:fs');
const path = require('node:path');

const { Orcha } = require('../lib/index');
const { OrchaError } = require('../lib/errors');
const pkg = require('../package.json');

function printUsage(stream = process.stderr) {
  stream.write(
    'orcha — run an orcha.yaml pipeline from the command line.\n' +
    '\n' +
    'Usage:\n' +
    '  orcha run <pipeline> [-y YAML] [-i STR | -f FILE] [--json]\n' +
    '  orcha version\n' +
    '\n' +
    'Flags:\n' +
    '  -y, --yaml PATH         path to orcha.yaml (default: ./orcha.yaml)\n' +
    '  -i, --input STR         inline input string passed to the first task\n' +
    '  -f, --input-file PATH   read input from this file\n' +
    '      --json              emit JSON-line events to stdout instead of pretty progress\n',
  );
}

function parseRunArgs(argv) {
  const out = { pipeline: null, yaml: null, input: null, inputFile: null, json: false };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    const next = () => {
      const v = argv[++i];
      if (v === undefined) {
        process.stderr.write(`orcha: ${a} requires a value\n`);
        process.exit(2);
      }
      return v;
    };
    if (a === '-y' || a === '--yaml') out.yaml = next();
    else if (a === '-i' || a === '--input') out.input = next();
    else if (a === '-f' || a === '--input-file') out.inputFile = next();
    else if (a === '--json') out.json = true;
    else if (a === '-h' || a === '--help') { printUsage(process.stdout); process.exit(0); }
    else if (a.startsWith('-')) { process.stderr.write(`orcha: unknown flag ${a}\n`); process.exit(2); }
    else if (out.pipeline === null) out.pipeline = a;
    else { process.stderr.write(`orcha: unexpected argument ${a}\n`); process.exit(2); }
  }
  if (out.pipeline === null) { printUsage(); process.exit(2); }
  if (out.input !== null && out.inputFile !== null) {
    process.stderr.write('orcha: pass either -i/--input or -f/--input-file, not both\n');
    process.exit(2);
  }
  return out;
}

function resolveYaml(arg) {
  if (arg) return arg;
  const candidate = path.join(process.cwd(), 'orcha.yaml');
  if (!fs.existsSync(candidate)) {
    process.stderr.write(
      `orcha: no -y/--yaml flag and ./orcha.yaml not found in ${process.cwd()}\n`,
    );
    process.exit(2);
  }
  return candidate;
}

async function readStdinAll() {
  const chunks = [];
  for await (const chunk of process.stdin) chunks.push(chunk);
  return Buffer.concat(chunks).toString('utf8');
}

async function resolveInput(args) {
  if (args.input !== null) return args.input;
  if (args.inputFile) return fs.readFileSync(args.inputFile, 'utf8');
  if (!process.stdin.isTTY) return readStdinAll();
  return null;
}

async function runJson(orcha, pipeline, input) {
  let exitCode = 0;
  try {
    for await (const ev of orcha.run(pipeline, input)) {
      if (ev.type === 'task_fail') exitCode = 1;
      process.stdout.write(JSON.stringify(ev) + '\n');
    }
  } catch (e) {
    if (e instanceof OrchaError) {
      process.stderr.write(`orcha: ${e.message}\n`);
      return 1;
    }
    throw e;
  }
  return exitCode;
}

async function runPretty(orcha, pipeline, input) {
  let final = null;
  let finalType = '';
  try {
    for await (const ev of orcha.run(pipeline, input)) {
      if (ev.type === 'task_start') {
        process.stderr.write(`> ${ev.task} ...\n`);
      } else if (ev.type === 'task_complete') {
        process.stderr.write(`+ ${ev.task} (${ev.elapsed_ms}ms)\n`);
      } else if (ev.type === 'task_fail') {
        process.stderr.write(`x ${ev.task} -- ${ev.error}\n`);
        return 1;
      } else if (ev.type === 'pipeline_complete') {
        final = ev.output;
        finalType = ev.output_type;
        process.stderr.write(`-- done in ${ev.elapsed_ms}ms\n`);
      }
    }
  } catch (e) {
    if (e instanceof OrchaError) {
      process.stderr.write(`orcha: ${e.message}\n`);
      return 1;
    }
    throw e;
  }

  if (final !== null && final !== undefined) {
    if ((finalType === 'text' || finalType === 'filepath') && typeof final === 'string') {
      process.stdout.write(final + '\n');
    } else {
      process.stdout.write(JSON.stringify(final, null, 2) + '\n');
    }
  }
  return 0;
}

async function main(argv) {
  const sub = argv[0];
  if (!sub || sub === '-h' || sub === '--help') {
    printUsage(sub ? process.stdout : process.stderr);
    return sub ? 0 : 2;
  }
  if (sub === 'version' || sub === '--version' || sub === '-v') {
    process.stdout.write(pkg.version + '\n');
    return 0;
  }
  if (sub !== 'run') {
    process.stderr.write(`orcha: unknown command: ${sub}\n`);
    return 2;
  }

  const args = parseRunArgs(argv.slice(1));
  const yamlPath = resolveYaml(args.yaml);
  const input = await resolveInput(args);

  let orcha;
  try {
    orcha = await Orcha.create(yamlPath);
  } catch (e) {
    if (e instanceof OrchaError) {
      process.stderr.write(`orcha: ${e.message}\n`);
      return 1;
    }
    throw e;
  }

  return args.json ? runJson(orcha, args.pipeline, input) : runPretty(orcha, args.pipeline, input);
}

main(process.argv.slice(2)).then(
  (code) => process.exit(code || 0),
  (err) => {
    process.stderr.write(`orcha: ${err && err.message ? err.message : err}\n`);
    process.exit(1);
  },
);
