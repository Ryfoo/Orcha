"""Exception types raised by the Orcha SDK."""


class OrchaError(Exception):
    """Base class for SDK errors (binary download, subprocess, protocol)."""


class TaskFailed(OrchaError):
    """Raised by run_sync() when the pipeline emits a task_fail event."""

    def __init__(self, task: str, index: int, message: str):
        super().__init__(f"task '{task}' (step {index}) failed: {message}")
        self.task = task
        self.index = index
        self.message = message
