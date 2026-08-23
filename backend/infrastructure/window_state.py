"""Persist main window geometry when remember_window_state is enabled."""
from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Any


MIN_WINDOW_WIDTH = 900
MIN_WINDOW_HEIGHT = 560
MAX_WINDOW_WIDTH = 8192
MAX_WINDOW_HEIGHT = 8192


def get_virtual_screen_bounds() -> tuple[int, int, int, int] | None:
    """Return the Windows virtual desktop as x, y, width, height."""
    if os.name != "nt":
        return None
    try:
        import ctypes

        user32 = ctypes.windll.user32
        bounds = tuple(int(user32.GetSystemMetrics(metric)) for metric in (76, 77, 78, 79))
        if bounds[2] <= 0 or bounds[3] <= 0:
            return None
        return bounds
    except (AttributeError, OSError, TypeError, ValueError):
        return None


def clamp_window_state(
    state: dict[str, Any],
    bounds: tuple[int, int, int, int] | None = None,
) -> dict[str, int]:
    """Sanitize persisted geometry and keep it inside the current desktop."""
    raw_width = int(state.get("width") or MIN_WINDOW_WIDTH)
    raw_height = int(state.get("height") or MIN_WINDOW_HEIGHT)
    width = max(MIN_WINDOW_WIDTH, min(raw_width, MAX_WINDOW_WIDTH))
    height = max(MIN_WINDOW_HEIGHT, min(raw_height, MAX_WINDOW_HEIGHT))
    active_bounds = get_virtual_screen_bounds() if bounds is None else bounds

    if active_bounds is None:
        result = {"width": width, "height": height}
        for key in ("x", "y"):
            if key in state:
                value = int(state[key])
                if abs(value) <= 100_000:
                    result[key] = value
        return result

    left, top, screen_width, screen_height = (int(value) for value in active_bounds)
    if screen_width <= 0 or screen_height <= 0:
        return {"width": width, "height": height}
    width = min(width, screen_width)
    height = min(height, screen_height)
    x = max(left, min(int(state.get("x", left)), left + screen_width - width))
    y = max(top, min(int(state.get("y", top)), top + screen_height - height))
    return {"width": width, "height": height, "x": x, "y": y}


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
