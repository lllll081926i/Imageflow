from __future__ import annotations


_WINDOW_MAXIMIZED = False


def _get_first_window():
    import webview

    if not webview.windows:
        return None
    return webview.windows[0]


def runtime_quit() -> None:
    window = _get_first_window()
    if window is not None:
        window.destroy()


def runtime_window_minimise() -> None:
    window = _get_first_window()
    if window is not None:
        window.minimize()


def runtime_window_toggle_maximise() -> None:
    window = _get_first_window()
    if window is not None:
        if _WINDOW_MAXIMIZED:
            window.restore()
            set_window_maximized(False)
        else:
            window.maximize()
            set_window_maximized(True)


def set_window_maximized(is_maximized: bool) -> None:
    global _WINDOW_MAXIMIZED
    _WINDOW_MAXIMIZED = bool(is_maximized)


def is_window_maximized() -> bool:
    return _WINDOW_MAXIMIZED
