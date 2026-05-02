"""Smoke tests for the Python SDK against a locally built binary.

These do not hit any AI provider — they exercise file/http tasks plus the
streaming protocol. Set ORCHA_BINARY_PATH to point at the in-tree build.
"""

import json
import os
import sys
import textwrap
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "python"))

from orcha import Orcha, OrchaEvent, TaskFailed  # noqa: E402


BIN = ROOT / "dist" / "orcha"


def write_yaml(tmp_path, body: str) -> str:
    p = tmp_path / "orcha.yaml"
    p.write_text(textwrap.dedent(body))
    return str(p)


def test_file_write_pipeline(tmp_path):
    out_file = tmp_path / "out.txt"
    yaml_path = write_yaml(
        tmp_path,
        f"""
        tasks:
          greet:
            type: file
            operation: write
            path: "{out_file}"
            content: "hello {{{{$input}}}}"
            output_type: filepath
        pipelines:
          smoke:
            steps:
              - task: greet
        """,
    )
    o = Orcha(yaml_path, binary_path=str(BIN))
    events = list(o.run("smoke", "world"))
    types = [e.type for e in events]
    assert types == ["task_start", "task_complete", "pipeline_complete"]
    assert events[-1].output == str(out_file)
    assert out_file.read_text() == "hello world"


def test_run_sync_returns_final(tmp_path):
    out_file = tmp_path / "out.txt"
    yaml_path = write_yaml(
        tmp_path,
        f"""
        tasks:
          greet:
            type: file
            operation: write
            path: "{out_file}"
            content: "{{{{$input}}}}"
            output_type: filepath
        pipelines:
          smoke:
            steps:
              - task: greet
        """,
    )
    o = Orcha(yaml_path, binary_path=str(BIN))
    result = o.run_sync("smoke", "synced")
    assert result == str(out_file)
    assert out_file.read_text() == "synced"


def test_two_step_chain(tmp_path):
    """write a file, then read it back through a second task."""
    f1 = tmp_path / "step1.txt"
    yaml_path = write_yaml(
        tmp_path,
        f"""
        tasks:
          write1:
            type: file
            operation: write
            path: "{f1}"
            content: "{{{{$input}}}}"
            output_type: filepath
          read1:
            type: file
            operation: read
            path: "{{{{$input}}}}"
            output_type: text
        pipelines:
          chain:
            steps:
              - task: write1
              - task: read1
        """,
    )
    o = Orcha(yaml_path, binary_path=str(BIN))
    result = o.run_sync("chain", "round-tripped content")
    assert result == "round-tripped content"


def test_task_fail_event(tmp_path):
    """reading a missing file should emit task_fail and run_sync should raise."""
    yaml_path = write_yaml(
        tmp_path,
        f"""
        tasks:
          read-missing:
            type: file
            operation: read
            path: "{tmp_path / 'does-not-exist.txt'}"
            output_type: text
        pipelines:
          bad:
            steps:
              - task: read-missing
        """,
    )
    o = Orcha(yaml_path, binary_path=str(BIN))
    fail_seen = False
    for e in o.run("bad"):
        if e.type == "task_fail":
            fail_seen = True
            assert "no such file" in (e.error or "").lower() or "cannot find" in (e.error or "").lower()
    assert fail_seen

    try:
        o.run_sync("bad")
    except TaskFailed as exc:
        assert exc.task == "read-missing"
    else:
        raise AssertionError("expected TaskFailed")


def test_typecheck_failure(tmp_path):
    """text -> json should be rejected at parse time."""
    yaml_path = write_yaml(
        tmp_path,
        f"""
        tasks:
          plain:
            type: file
            operation: write
            path: "{tmp_path / 'x.txt'}"
            content: "hi"
            output_type: text
          want-json:
            type: http
            method: POST
            url: "http://localhost:0/"
            body: "{{{{$input}}}}"
            output_type: json
        pipelines:
          bad:
            steps:
              - task: plain
              - task: want-json
        """,
    )
    # want-json's input type is text (HTTP), and plain outputs text — actually compatible.
    # That's fine; this should parse. Let's instead test list -> filepath which is incompatible.
    yaml_path2 = write_yaml(
        tmp_path,
        f"""
        tasks:
          listy:
            type: file
            operation: read
            path: "{tmp_path / 'x.txt'}"
            output_type: list
          read-second:
            type: file
            operation: read
            path: "{{{{$input}}}}"
            output_type: text
        pipelines:
          bad:
            steps:
              - task: listy
              - task: read-second
        """,
    )
    (tmp_path / "x.txt").write_text("a\nb\nc")
    o = Orcha(yaml_path2, binary_path=str(BIN))
    fail_seen = False
    for e in o.run("bad"):
        if e.type == "task_fail":
            fail_seen = True
            assert "type mismatch" in (e.error or "").lower()
    assert fail_seen
