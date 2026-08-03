"""Resolve the running ImageFlow version and GitHub update metadata."""
from __future__ import annotations

import json
import os
import re
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any

GITHUB_REPO = os.getenv("IMAGEFLOW_GITHUB_REPO", "lllll081926i/Imageflow").strip() or "lllll081926i/Imageflow"
GITHUB_API = f"https://api.github.com/repos/{GITHUB_REPO}/releases/latest"
USER_AGENT = "ImageFlow-UpdateCheck/1.0"


def read_local_version() -> str:
    """Read version from env, embedded VERSION file, or pyproject.toml."""
    env = str(os.getenv("IMAGEFLOW_VERSION", "") or "").strip()
    if env:
        return env.lstrip("vV")

    candidates: list[Path] = []
    meipass = getattr(__import__("sys"), "_MEIPASS", None)
    if meipass:
        candidates.append(Path(meipass) / "VERSION")
    try:
        import sys

        if getattr(sys, "frozen", False) and sys.executable:
            candidates.append(Path(sys.executable).resolve().parent / "VERSION")
    except Exception:
        pass
    candidates.append(Path(__file__).resolve().parents[2] / "VERSION")
    for candidate in candidates:
        try:
            raw = candidate.read_text(encoding="utf-8").strip()
        except OSError:
            continue
        if raw:
            return raw.lstrip("vV")

    root = Path(__file__).resolve().parents[2]
    pyproject = root / "pyproject.toml"
    try:
        text = pyproject.read_text(encoding="utf-8")
    except OSError:
        return "0.0.0"
    match = re.search(r'(?m)^version\s*=\s*["\']([^"\']+)["\']', text)
    if match:
        return match.group(1).strip().lstrip("vV")
    return "0.0.0"


def _parse_version_tuple(version: str) -> tuple[int, ...]:
    cleaned = str(version or "").strip().lstrip("vV")
    parts: list[int] = []
    for token in re.split(r"[^\d]+", cleaned):
        if not token:
            continue
        try:
            parts.append(int(token))
        except ValueError:
            parts.append(0)
    return tuple(parts or [0])


def is_remote_newer(local: str, remote: str) -> bool:
    return _parse_version_tuple(remote) > _parse_version_tuple(local)


def check_for_updates(timeout_seconds: float = 8.0) -> dict[str, Any]:
    """Query GitHub latest release. Network failures return success with update=false."""
    local = read_local_version()
    result: dict[str, Any] = {
        "success": True,
        "current_version": local,
        "latest_version": local,
        "update_available": False,
        "release_url": f"https://github.com/{GITHUB_REPO}/releases",
        "release_notes": "",
        "checked": False,
        "error": "",
    }
    try:
        request = urllib.request.Request(
            GITHUB_API,
            headers={
                "Accept": "application/vnd.github+json",
                "User-Agent": USER_AGENT,
            },
            method="GET",
        )
        with urllib.request.urlopen(request, timeout=max(2.0, float(timeout_seconds))) as response:
            payload = json.loads(response.read().decode("utf-8", errors="replace"))
    except (urllib.error.URLError, urllib.error.HTTPError, TimeoutError, json.JSONDecodeError, OSError, ValueError) as exc:
        result["error"] = str(exc)
        result["success"] = True  # soft-fail: app still usable offline
        return result

    if not isinstance(payload, dict):
        result["error"] = "unexpected release payload"
        return result

    tag = str(payload.get("tag_name") or payload.get("name") or "").strip()
    remote = tag.lstrip("vV") or local
    html_url = str(payload.get("html_url") or result["release_url"]).strip()
    notes = str(payload.get("body") or "").strip()
    if len(notes) > 4000:
        notes = notes[:4000] + "\n…"

    result["latest_version"] = remote
    result["release_url"] = html_url or result["release_url"]
    result["release_notes"] = notes
    result["update_available"] = is_remote_newer(local, remote)
    result["checked"] = True
    return result
