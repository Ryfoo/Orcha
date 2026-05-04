"""Orcha — Unix pipes for AI workflows.

Public API:

    from orcha import Orcha, OrchaEvent, OrchaError

    o = Orcha("./orcha.yaml")
    for event in o.run("pipeline-name", input_value):
        ...

    final = o.run_sync("pipeline-name", input_value)
"""

# Defined first because downloader.py imports it at module-load time. Keeping
# the assignment a single line lets the release Makefile rewrite it via sed.
__version__ = "0.1.0"

from .events import OrchaEvent
from .errors import OrchaError, TaskFailed
from .client import Orcha

__all__ = ["Orcha", "OrchaEvent", "OrchaError", "TaskFailed"]
