import json
import os
import sys
from pathlib import Path

from backend.api import DesktopAPI
from backend.infrastructure.window_ops import set_window_maximized


def build_window_api() -> DesktopAPI:
    return DesktopAPI()


def _dispatch_file_drop(window, event: dict) -> None:
    files = event.get("dataTransfer", {}).get("files", [])
    paths: list[str] = []
    for item in files:
        candidate = str(item.get("pywebviewFullPath") or "").strip()
        if candidate:
            paths.append(candidate)

    if not paths:
        return

    payload = {
        "x": int(event.get("clientX") or 0),
        "y": int(event.get("clientY") or 0),
        "paths": paths,
    }
    window.evaluate_js(
        """
        window.dispatchEvent(
            new CustomEvent("__imageflow_file_drop__", {
                detail: %s
            })
        );
        """
        % json.dumps(payload, ensure_ascii=False)
    )


def configure_window(window) -> None:
    from webview.dom import DOMEventHandler

    def handle_loaded() -> None:
        document = window.dom.document
        document.events.dragover += DOMEventHandler(lambda _event: None, prevent_default=True)
        document.events.drop += DOMEventHandler(
            lambda event: _dispatch_file_drop(window, event),
            prevent_default=True,
        )

    window.events.loaded += handle_loaded
    window.events.maximized += lambda: set_window_maximized(True)
    window.events.restored += lambda: set_window_maximized(False)
    window.events.minimized += lambda: set_window_maximized(False)


def resolve_frontend_entry(project_root: Path | None = None, frontend_url: str | None = None) -> str:
    if frontend_url:
        return frontend_url

    env_url = os.getenv("IMAGEFLOW_FRONTEND_URL", "").strip()
    if env_url:
        return env_url

    if project_root is not None:
        return str(project_root / "frontend" / "dist" / "index.html")

    roots: list[Path] = []
    bundled_root = getattr(sys, "_MEIPASS", None)
    if bundled_root:
        roots.append(Path(bundled_root))

    roots.append(Path(__file__).resolve().parents[2])

    candidates = [root / "frontend" / "dist" / "index.html" for root in roots]
    for candidate in candidates:
        if candidate.exists():
            return str(candidate)
    return str(candidates[0])
