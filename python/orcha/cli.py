"""The `orcha` shell command, installed by pip via [project.scripts].

Mirrors the UX of the Go `orcha-run` binary but uses the Python SDK so users
can rely on a single ``pip install orcha`` to get an executable on PATH.

Subcommands:

    orcha run <pipeline> [-y YAML] [-i STR | -f FILE] [--json]
    orcha version
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from typing import Optional

from . import __version__
from .client import Orcha
from .errors import OrchaError


def main(argv: Optional[list[str]] = None) -> int:
    parser = argparse.ArgumentParser(
        prog="orcha",
        description="Run an orcha.yaml pipeline from the command line.",
    )
    sub = parser.add_subparsers(dest="cmd", required=True)

    run = sub.add_parser("run", help="Execute a pipeline")
    run.add_argument("pipeline", help="Pipeline or task name defined in the YAML")
    run.add_argument(
        "-y", "--yaml",
        default=None,
        help="path to orcha.yaml (default: ./orcha.yaml in cwd)",
    )
    run.add_argument(
        "-i", "--input",
        default=None,
        help="inline input string passed to the first task",
    )
    run.add_argument(
        "-f", "--input-file",
        default=None,
        help="read input from this file",
    )
    run.add_argument(
        "--json",
        action="store_true",
        help="emit JSON-line events to stdout instead of pretty progress",
    )

    sub.add_parser("version", help="Print the installed orcha version")

    args = parser.parse_args(argv)

    if args.cmd == "version":
        print(__version__)
        return 0

    if args.cmd == "run":
        return _do_run(args)

    parser.error(f"unknown command: {args.cmd}")
    return 2  # unreachable; argparse raises SystemExit on .error()


def _resolve_yaml(arg: Optional[str]) -> str:
    if arg:
        return arg
    candidate = os.path.join(os.getcwd(), "orcha.yaml")
    if not os.path.exists(candidate):
        sys.stderr.write(
            f"orcha: no -y/--yaml flag and ./orcha.yaml not found in {os.getcwd()}\n"
        )
        sys.exit(2)
    return candidate


def _resolve_input(inline: Optional[str], file: Optional[str]):
    if inline is not None and file:
        sys.stderr.write("orcha: pass either -i/--input or -f/--input-file, not both\n")
        sys.exit(2)
    if inline is not None:
        return inline
    if file:
        with open(file) as f:
            return f.read()
    if not sys.stdin.isatty():
        return sys.stdin.read()
    return None


def _do_run(args) -> int:
    yaml_path = _resolve_yaml(args.yaml)
    input_value = _resolve_input(args.input, args.input_file)

    try:
        orcha = Orcha(yaml_path)
    except OrchaError as e:
        sys.stderr.write(f"orcha: {e}\n")
        return 1

    if args.json:
        return _run_json(orcha, args.pipeline, input_value)
    return _run_pretty(orcha, args.pipeline, input_value)


def _run_json(orcha: Orcha, pipeline: str, input_value) -> int:
    exit_code = 0
    try:
        for ev in orcha.run(pipeline, input_value):
            if ev.type == "task_fail":
                exit_code = 1
            print(json.dumps({
                "type": ev.type,
                "task": ev.task,
                "index": ev.index,
                "output": ev.output,
                "output_type": ev.output_type,
                "error": ev.error,
                "elapsed_ms": ev.elapsed_ms,
            }))
    except OrchaError as e:
        sys.stderr.write(f"orcha: {e}\n")
        return 1
    return exit_code


def _run_pretty(orcha: Orcha, pipeline: str, input_value) -> int:
    final = None
    final_type = ""
    try:
        for ev in orcha.run(pipeline, input_value):
            if ev.type == "task_start":
                sys.stderr.write(f"> {ev.task} ...\n")
            elif ev.type == "task_complete":
                sys.stderr.write(f"+ {ev.task} ({ev.elapsed_ms}ms)\n")
            elif ev.type == "task_fail":
                sys.stderr.write(f"x {ev.task} -- {ev.error}\n")
                return 1
            elif ev.type == "pipeline_complete":
                final = ev.output
                final_type = ev.output_type
                sys.stderr.write(f"-- done in {ev.elapsed_ms}ms\n")
    except OrchaError as e:
        sys.stderr.write(f"orcha: {e}\n")
        return 1

    if final is not None:
        if final_type in ("text", "filepath") and isinstance(final, str):
            print(final)
        else:
            print(json.dumps(final, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
