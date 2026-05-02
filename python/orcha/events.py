"""OrchaEvent — the lightweight value yielded by Orcha.run() iterators."""

from dataclasses import dataclass
from typing import Any, Optional


@dataclass
class OrchaEvent:
    """One streaming event from a pipeline run.

    type:        "task_start" | "task_complete" | "task_fail" | "pipeline_complete"
    task:        name of the task (empty string for pipeline_complete in v1)
    index:       0-based position in the pipeline; -1 for parse-time errors
    output:      task output (str / dict / list) — None for task_start and task_fail
    output_type: "text" | "json" | "filepath" | "list" — empty for task_start/fail
    error:       error message — only set when type == "task_fail"
    elapsed_ms:  wall-clock duration of this step (0 for task_start)
    """

    type: str
    task: str = ""
    index: int = 0
    output: Any = None
    output_type: str = ""
    error: Optional[str] = None
    elapsed_ms: int = 0

    @classmethod
    def from_dict(cls, raw: dict) -> "OrchaEvent":
        return cls(
            type=raw.get("type", ""),
            task=raw.get("task", ""),
            index=int(raw.get("index", 0)),
            output=raw.get("output"),
            output_type=raw.get("output_type", ""),
            error=raw.get("error"),
            elapsed_ms=int(raw.get("elapsed_ms", 0)),
        )
