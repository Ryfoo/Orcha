"""Resolve and (lazily) download the orcha Go binary for the current platform.

The binary lives at ``~/.orcha/bin/orcha-<os>-<arch>[.exe]``. On first call we:

1. Fetch a ``manifest.json`` from the GitHub release that matches this Python
   package's version.
2. Look up the entry for our platform — it gives us a binary URL and the
   expected sha256.
3. Download the binary, verify the hash, mark it executable, and cache it.

Subsequent runs find the cached file and skip the network entirely.

Override hooks (set in env):

* ``ORCHA_BINARY_PATH`` — absolute path to a pre-built binary; useful for
  development against an in-tree build. No download or hash check is run.
* ``ORCHA_BINARY_VERSION`` — pin a specific release tag instead of using the
  package version.
"""

from __future__ import annotations

import hashlib
import json
import os
import platform
import shutil
import stat
import tempfile
import urllib.error
import urllib.request
from pathlib import Path
from typing import Optional

from . import __version__
from .errors import OrchaError

MANIFEST_URL_TEMPLATE = (
    "https://github.com/ryfoo/orcha/releases/download/v{version}/manifest.json"
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
    """Return the path to the orcha binary, downloading + verifying if needed."""
    override = os.environ.get("ORCHA_BINARY_PATH")
    if override:
        path = Path(override)
        if not path.exists():
            raise OrchaError(f"ORCHA_BINARY_PATH points to missing file: {path}")
        return path

    version = version or os.environ.get("ORCHA_BINARY_VERSION") or __version__
    os_name, arch = _detect_platform()
    name = binary_filename(os_name, arch)
    target = install_dir() / name
    if target.exists():
        return target

    target.parent.mkdir(parents=True, exist_ok=True)

    manifest = _fetch_manifest(version)
    platform_key = f"{os_name}-{arch}"
    entry = manifest.get("binaries", {}).get(platform_key)
    if not entry:
        raise OrchaError(
            f"release v{version} has no binary for {platform_key} "
            f"(available: {sorted(manifest.get('binaries', {}).keys())})"
        )
    url = entry["url"]
    expected_sha = entry["sha256"]

    _download_to(url, target)
    actual_sha = _sha256(target)
    if actual_sha != expected_sha:
        # Don't keep a tampered binary on disk.
        try:
            target.unlink()
        except OSError:
            pass
        raise OrchaError(
            f"sha256 mismatch for {name}: expected {expected_sha}, got {actual_sha}"
        )
    target.chmod(target.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    return target


def _fetch_manifest(version: str) -> dict:
    url = MANIFEST_URL_TEMPLATE.format(version=version)
    try:
        with urllib.request.urlopen(url, timeout=30) as resp:
            data = resp.read()
    except urllib.error.HTTPError as e:
        if e.code == 404:
            raise OrchaError(
                f"orcha release v{version} not found at {url}. "
                f"Either the release hasn't been published yet, or "
                f"ORCHA_BINARY_VERSION points at a tag that doesn't exist."
            ) from e
        raise OrchaError(f"failed to fetch manifest at {url}: HTTP {e.code}") from e
    except urllib.error.URLError as e:
        raise OrchaError(f"failed to fetch manifest at {url}: {e.reason}") from e
    try:
        return json.loads(data)
    except json.JSONDecodeError as e:
        raise OrchaError(f"manifest at {url} was not valid JSON: {e}") from e


def _download_to(url: str, dest: Path) -> None:
    tmp_fd, tmp_path = tempfile.mkstemp(prefix="orcha-", dir=str(dest.parent))
    os.close(tmp_fd)
    try:
        try:
            with urllib.request.urlopen(url, timeout=120) as src, open(tmp_path, "wb") as dst:
                shutil.copyfileobj(src, dst)
        except urllib.error.HTTPError as e:
            raise OrchaError(f"download failed ({url}): HTTP {e.code}") from e
        except urllib.error.URLError as e:
            raise OrchaError(f"download failed ({url}): {e.reason}") from e
        os.replace(tmp_path, dest)
    finally:
        if os.path.exists(tmp_path):
            try:
                os.remove(tmp_path)
            except OSError:
                pass


def _sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


# Public alias retained for any external caller.
sha256 = _sha256
