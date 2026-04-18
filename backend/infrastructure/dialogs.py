from __future__ import annotations

import threading
from typing import Any


def _run_in_tk_thread(callback):
    result: dict[str, Any] = {}
    error: dict[str, Exception] = {}
    event = threading.Event()

    def runner():
        try:
            import tkinter as tk

            root = tk.Tk()
            root.withdraw()
            root.attributes("-topmost", True)
            try:
                result["value"] = callback(root)
            finally:
                root.destroy()
        except Exception as exc:
            error["value"] = exc
        finally:
            event.set()

    threading.Thread(target=runner, daemon=True).start()
    event.wait()
    if "value" in error:
        raise error["value"]
    return result.get("value")


def _build_filetypes(options: dict[str, Any]) -> list[tuple[str, str]]:
    filters = options.get("filters")
    if not isinstance(filters, list) or not filters:
        return [("Images", "*.jpg *.jpeg *.png *.webp *.gif *.bmp *.tiff *.tif *.heic *.heif *.svg")]

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

    return filetypes or [("Images", "*.jpg *.jpeg *.png *.webp *.gif *.bmp *.tiff *.tif *.heic *.heif *.svg")]


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
