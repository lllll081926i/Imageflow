"""Persist main window geometry when remember_window_state is enabled."""
from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Any


def _state_path() -> Path:
    override = os.getenv("IMAGEFLOW_WINDOW_STATE_FILE", "").strip()
    if override:
        return Path(override)
    appdata = os.getenv("APPDATA", "").strip()
    if appdata:
        return Path(appdata) / "imageflow" / "window_state.json"
    return Path.home() / ".imageflow" / "window_state.json"


def load_window_state() -> dict[str, Any]:
    path = _state_path()
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError, TypeError, ValueError):
        return {}
    if not isinstance(data, dict):
        return {}
    out: dict[str, Any] = {}
    for key in ("width", "height", "x", "y"):
        try:
            out[key] = int(data[key])
        except (KeyError, TypeError, ValueError):
            continue
    return out


def save_window_state(
    width: int,
    height: int,
    x: int | None = None,
    y: int | None = None,
) -> None:
    path = _state_path()
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        payload: dict[str, int] = {"width": int(width), "height": int(height)}
        if x is not None:
            payload["x"] = int(x)
        if y is not None:
            payload["y"] = int(y)
        path.write_text(json.dumps(payload, ensure_ascii=True, indent=2), encoding="utf-8")
    except OSError:
        pass
