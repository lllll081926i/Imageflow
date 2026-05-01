from __future__ import annotations

import queue
import threading
from typing import Any

# Singleton dialog thread: owns one Tk root and processes dialog requests.
_dialog_queue: queue.Queue[tuple[callable, threading.Event, dict]] = queue.Queue()
_dialog_started = threading.Event()
_dialog_start_lock = threading.Lock()
_dialog_start_error: Exception | None = None
_dialog_thread: threading.Thread | None = None


def _dialog_worker():
    import tkinter as tk

    root = tk.Tk()
    root.withdraw()
    root.attributes("-topmost", True)
    _dialog_started.set()

    while True:
        try:
            callback, done_event, state = _dialog_queue.get()
        except queue.Empty:
            continue
        if callback is None:
            root.destroy()
            return
        try:
            state["value"] = callback(root)
        except Exception as exc:
            state["error"] = exc
        finally:
            done_event.set()


def _dialog_worker_entry():
    global _dialog_start_error
    try:
        _dialog_worker()
    except Exception as exc:
        _dialog_start_error = exc
        _dialog_started.set()


def _ensure_dialog_thread():
    global _dialog_start_error, _dialog_thread
    if _dialog_started.is_set() and _dialog_start_error is None:
        return

    with _dialog_start_lock:
        if _dialog_started.is_set() and _dialog_start_error is None:
            return
        if _dialog_thread is None or not _dialog_thread.is_alive() or _dialog_start_error is not None:
            _dialog_start_error = None
            _dialog_started.clear()
            _dialog_thread = threading.Thread(target=_dialog_worker_entry, daemon=True)
            _dialog_thread.start()

    if not _dialog_started.wait(timeout=5):
        raise RuntimeError("Timed out waiting for dialog thread startup")
    if _dialog_start_error is not None:
        raise RuntimeError(f"Failed to start dialog thread: {_dialog_start_error}") from _dialog_start_error


def _run_in_tk_thread(callback):
    _ensure_dialog_thread()

    done = threading.Event()
    state: dict[str, Any] = {}
    _dialog_queue.put((callback, done, state))
    done.wait()

    if "error" in state:
        raise state["error"]
    return state.get("value")


def _build_filetypes(options: dict[str, Any]) -> list[tuple[str, str]]:
    filters = options.get("filters")
    if not isinstance(filters, list) or not filters:
        return [("Images", "*.jpg *.jpeg *.png *.webp *.gif *.bmp *.avif *.ico *.tiff *.tif *.heic *.heif *.svg")]

    filetypes: list[tuple[str, str]] = []
    for item in filters:
        if not isinstance(item, dict):
            continue
        name = str(item.get("DisplayName") or item.get("displayName") or "Files").strip() or "Files"
        pattern = str(item.get("Pattern") or item.get("pattern") or "").strip()
        if not pattern:
            continue
        normalized = " ".join(part.strip() for part in pattern.replace(";", " ").split() if part.strip())
        if normalized:
            filetypes.append((name, normalized))

    return filetypes or [("Images", "*.jpg *.jpeg *.png *.webp *.gif *.bmp *.avif *.ico *.tiff *.tif *.heic *.heif *.svg")]


def open_file_dialog(options: dict[str, Any] | None = None):
    options = options or {}

    def run(_root):
        from tkinter import filedialog

        title = options.get("title") or "选择文件"
        can_choose_files = bool(options.get("canChooseFiles", True))
        can_choose_directories = bool(options.get("canChooseDirectories", False))
        multiple = bool(options.get("allowsMultipleSelection", False))
        filetypes = _build_filetypes(options)
        if can_choose_directories and not can_choose_files:
            return filedialog.askdirectory(title=title) or ""
        if not can_choose_files:
            return ""
        if multiple:
            return list(filedialog.askopenfilenames(title=title, filetypes=filetypes))
        return filedialog.askopenfilename(title=title, filetypes=filetypes) or ""

    return _run_in_tk_thread(run)


def open_directory_dialog(options: dict[str, Any] | None = None):
    options = options or {}

    def run(_root):
        from tkinter import filedialog

        title = options.get("title") or "选择文件夹"
        return filedialog.askdirectory(title=title) or ""

    return _run_in_tk_thread(run)
