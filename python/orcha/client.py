"""Spawns the Go binary as a subprocess, sends one JSON command on stdin, and
streams JSON-line events back from stdout. The protocol is one-shot per
invocation: every ``run()`` call is a fresh process.
"""

from __future__ import annotations

import json
import os
import subprocess
import threading
from pathlib import Path
from typing import Any, Iterator, Optional

from .downloader import resolve_binary
from .errors import OrchaError, TaskFailed
from .events import OrchaEvent


class Orcha:
    """Entry point for executing pipelines defined in an ``orcha.yaml``.

    Args:
        yaml_path: path to the workflow file. Resolved to an absolute path so
            it survives the subprocess ``cwd``.
        binary_path: optional override for the engine binary. Pass an absolute
            path during development; otherwise the constructor resolves the
            cached binary in ``~/.orcha/bin``.
    """

    def __init__(self, yaml_path: str | os.PathLike, binary_path: Optional[str | os.PathLike] = None):
        self._yaml_path = str(Path(yaml_path).resolve())
        if not os.path.exists(self._yaml_path):
            raise OrchaError(f"yaml file not found: {self._yaml_path}")
        if binary_path:
            self._binary = str(Path(binary_path).resolve())
        else:
            self._binary = str(resolve_binary())

    @property
    def yaml_path(self) -> str:
        return self._yaml_path

    @property
    def binary(self) -> str:
        return self._binary

    def run(self, target: str, input_value: Any = None) -> Iterator[OrchaEvent]:
        """Run a pipeline (or single task) named ``target`` and yield events.

        ``input_value`` may be a string, bytes (decoded as UTF-8), dict, or
        list. The Go side coerces it into the first task's expected input type.
        """
        if isinstance(input_value, bytes):
            input_value = input_value.decode("utf-8", errors="replace")

        cmd = {
            "command": "run",
            "pipeline": target,
            "yaml_path": self._yaml_path,
            "input": input_value,
        }
        return self._spawn(cmd)

    def run_sync(self, target: str, input_value: Any = None) -> Any:
        """Run a pipeline and return the final output. Raises TaskFailed on
        failure so callers can keep using normal try/except control flow.
        """
        final: Any = None
        for event in self.run(target, input_value):
            if event.type == "task_fail":
                raise TaskFailed(event.task, event.index, event.error or "unknown error")
            if event.type == "pipeline_complete":
                final = event.output
            elif event.type == "task_complete":
                # Track latest in case pipeline has only one step or the
                # binary doesn't emit pipeline_complete (it always does in v1,
                # but defensiveness here is cheap).
                final = event.output
        return final

    def _spawn(self, cmd: dict) -> Iterator[OrchaEvent]:
        proc = subprocess.Popen(
            [self._binary],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )
        if proc.stdin is None or proc.stdout is None or proc.stderr is None:
            raise OrchaError("subprocess pipes are not connected")

        try:
            proc.stdin.write(json.dumps(cmd) + "\n")
            proc.stdin.flush()
            proc.stdin.close()
        except (BrokenPipeError, OSError) as e:
            proc.kill()
            proc.wait()
            stderr = proc.stderr.read() if proc.stderr else ""
            raise OrchaError(f"failed to send command to engine: {e}; stderr={stderr}") from e

        # Drain stderr in a background thread so it never blocks on a full
        # pipe buffer while we're streaming stdout. The captured text is only
        # surfaced if the process exits non-zero.
        stderr_buf: list[str] = []

        def _drain():
            assert proc.stderr is not None
            for line in proc.stderr:
                stderr_buf.append(line)

        t = threading.Thread(target=_drain, daemon=True)
        t.start()

        try:
            for line in proc.stdout:
                line = line.strip()
                if not line:
                    continue
                try:
                    raw = json.loads(line)
                except json.JSONDecodeError:
                    continue
                yield OrchaEvent.from_dict(raw)
        finally:
            proc.stdout.close()
            code = proc.wait()
            t.join(timeout=1.0)
            if code != 0:
                stderr = "".join(stderr_buf).strip()
                raise OrchaError(
                    f"orcha engine exited {code}"
                    + (f": {stderr}" if stderr else "")
                )
