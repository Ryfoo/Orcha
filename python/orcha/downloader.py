"""Resolve and (lazily) download the orcha Go binary for the current platform.

The binary lives at ``~/.orcha/bin/orcha-<os>-<arch>[.exe]``. On first call we
download it from the GitHub release that matches this Python package version.
Subsequent runs find the cached file and skip the network round-trip.

Two override hooks are supported:

* ``ORCHA_BINARY_PATH`` — absolute path to a pre-built binary; useful for
  development against an in-tree build.
* ``ORCHA_BINARY_VERSION`` — pin a specific release tag instead of using the
  package version.
"""

from __future__ import annotations

import hashlib
import os
import platform
import shutil
import stat
import tempfile
import urllib.error
import urllib.request
from pathlib import Path
from typing import Optional

from .errors import OrchaError

DEFAULT_VERSION = "0.1.0"
RELEASE_URL_TEMPLATE = (
    "https://github.com/ryfoo/orcha/releases/download/v{version}/{name}"
)


def _detect_platform() -> tuple[str, str]:
    system = platform.system()
    machine = platform.machine().lower()

    os_map = {"Linux": "linux", "Darwin": "darwin", "Windows": "windows"}
    if system not in os_map:
        raise OrchaError(f"unsupported OS: {system}")

    arch_map = {
        "x86_64": "amd64",
        "amd64": "amd64",
        "arm64": "arm64",
        "aarch64": "arm64",
    }
    if machine not in arch_map:
        raise OrchaError(f"unsupported architecture: {machine}")

    return os_map[system], arch_map[machine]


def binary_filename(os_name: str, arch: str) -> str:
    name = f"orcha-{os_name}-{arch}"
    if os_name == "windows":
        name += ".exe"
    return name


def install_dir() -> Path:
    return Path(os.path.expanduser("~/.orcha/bin"))


def resolve_binary(version: Optional[str] = None) -> Path:
    """Return the path to the orcha binary, downloading if necessary."""
    override = os.environ.get("ORCHA_BINARY_PATH")
    if override:
        path = Path(override)
        if not path.exists():
            raise OrchaError(f"ORCHA_BINARY_PATH points to missing file: {path}")
        return path

    version = version or os.environ.get("ORCHA_BINARY_VERSION") or DEFAULT_VERSION
    os_name, arch = _detect_platform()
    name = binary_filename(os_name, arch)
    target = install_dir() / name
    if target.exists():
        return target

    target.parent.mkdir(parents=True, exist_ok=True)
    url = RELEASE_URL_TEMPLATE.format(version=version, name=name)
    _download_to(url, target)
    target.chmod(target.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    return target


def _download_to(url: str, dest: Path) -> None:
    tmp_fd, tmp_path = tempfile.mkstemp(prefix="orcha-", dir=str(dest.parent))
    os.close(tmp_fd)
    try:
        try:
            with urllib.request.urlopen(url, timeout=60) as src, open(tmp_path, "wb") as dst:
                shutil.copyfileobj(src, dst)
        except urllib.error.HTTPError as e:
            raise OrchaError(
                f"failed to download orcha binary from {url}: HTTP {e.code}"
            ) from e
        except urllib.error.URLError as e:
            raise OrchaError(
                f"failed to download orcha binary from {url}: {e.reason}"
            ) from e
        os.replace(tmp_path, dest)
    finally:
        if os.path.exists(tmp_path):
            try:
                os.remove(tmp_path)
            except OSError:
                pass


def sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()
