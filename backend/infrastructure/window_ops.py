from __future__ import annotations

import ctypes
import sys
from typing import Any


_WINDOW_MAXIMIZED = False
_NATIVE_RESIZE_HOOKS: dict[int, tuple[Any, Any]] = {}


def _native_handle(native_window: Any) -> int | None:
    handle = getattr(native_window, "Handle", None)
    if handle is None:
        return None

    try:
        return int(handle.ToInt64())
    except AttributeError:
        try:
            return int(handle)
        except (TypeError, ValueError):
            return None


def install_native_window_resize(window: Any, border_size: int = 8) -> bool:
    """Restore edge resizing for pywebview's frameless WinForms window.

    WinForms removes the non-client area when ``FormBorderStyle`` is ``None``.
    Adding ``WS_THICKFRAME`` alone is not enough because Windows then receives
    client hit tests.  A small native subclass maps the edge pixels back to
    the standard resize hit-test values while keeping the custom title bar.
    """
    if sys.platform != "win32":
        return False

    native_window = getattr(window, "native", None)
    hwnd = _native_handle(native_window)
    if not hwnd or hwnd in _NATIVE_RESIZE_HOOKS:
        return False

    try:
        from ctypes import wintypes

        user32 = ctypes.WinDLL("user32", use_last_error=True)
        pointer_type = ctypes.c_ssize_t
        hwnd_type = wintypes.HWND

        get_window_long_ptr = user32.GetWindowLongPtrW
        get_window_long_ptr.argtypes = [hwnd_type, ctypes.c_int]
        get_window_long_ptr.restype = pointer_type

        set_window_long_ptr = user32.SetWindowLongPtrW
        set_window_long_ptr.argtypes = [hwnd_type, ctypes.c_int, pointer_type]
        set_window_long_ptr.restype = pointer_type

        set_window_pos = user32.SetWindowPos
        set_window_pos.argtypes = [
            hwnd_type,
            hwnd_type,
            ctypes.c_int,
            ctypes.c_int,
            ctypes.c_int,
            ctypes.c_int,
            ctypes.c_uint,
        ]
        set_window_pos.restype = wintypes.BOOL

        get_window_rect = user32.GetWindowRect
        get_window_rect.argtypes = [hwnd_type, ctypes.POINTER(wintypes.RECT)]
        get_window_rect.restype = wintypes.BOOL

        is_zoomed = user32.IsZoomed
        is_zoomed.argtypes = [hwnd_type]
        is_zoomed.restype = wintypes.BOOL

        call_window_proc = user32.CallWindowProcW
        call_window_proc.argtypes = [
            ctypes.c_void_p,
            hwnd_type,
            wintypes.UINT,
            wintypes.WPARAM,
            wintypes.LPARAM,
        ]
        call_window_proc.restype = pointer_type

        window_proc_type = ctypes.WINFUNCTYPE(
            pointer_type,
            hwnd_type,
            wintypes.UINT,
            wintypes.WPARAM,
            wintypes.LPARAM,
        )

        gwl_style = -16
        gwlp_wndproc = -4
        ws_thickframe = 0x00040000
        wm_nchittest = 0x0084
        ht_left = 10
        ht_right = 11
        ht_top = 12
        ht_top_left = 13
        ht_top_right = 14
        ht_bottom = 15
        ht_bottom_left = 16
        ht_bottom_right = 17
        swp_nosize = 0x0001
        swp_nomove = 0x0002
        swp_nozorder = 0x0004
        swp_noactivate = 0x0010
        swp_framechanged = 0x0020

        previous_proc = get_window_long_ptr(hwnd, gwlp_wndproc)
        if not previous_proc:
            return False

        style = get_window_long_ptr(hwnd, gwl_style)
        if not style & ws_thickframe:
            set_window_long_ptr(hwnd, gwl_style, style | ws_thickframe)
            set_window_pos(
                hwnd,
                None,
                0,
                0,
                0,
                0,
                swp_nosize
                | swp_nomove
                | swp_nozorder
                | swp_noactivate
                | swp_framechanged,
            )

        resize_border = max(6, int(border_size))

        def signed_word(value: int) -> int:
            value &= 0xFFFF
            return value - 0x10000 if value & 0x8000 else value

        @window_proc_type
        def window_proc(hwnd_value, message, w_param, l_param):
            if message == wm_nchittest and not is_zoomed(hwnd_value):
                rect = wintypes.RECT()
                if get_window_rect(hwnd_value, ctypes.byref(rect)):
                    cursor_x = signed_word(l_param)
                    cursor_y = signed_word(l_param >> 16)
                    left = cursor_x - rect.left < resize_border
                    right = rect.right - cursor_x <= resize_border
                    top = cursor_y - rect.top < resize_border
                    bottom = rect.bottom - cursor_y <= resize_border

                    if top and left:
                        return ht_top_left
                    if top and right:
                        return ht_top_right
                    if bottom and left:
                        return ht_bottom_left
                    if bottom and right:
                        return ht_bottom_right
                    if left:
                        return ht_left
                    if right:
                        return ht_right
                    if top:
                        return ht_top
                    if bottom:
                        return ht_bottom

            return call_window_proc(previous_proc, hwnd_value, message, w_param, l_param)

        set_window_long_ptr(hwnd, gwlp_wndproc, ctypes.cast(window_proc, ctypes.c_void_p).value)
        _NATIVE_RESIZE_HOOKS[hwnd] = (window_proc, previous_proc)
        return True
    except (AttributeError, OSError, TypeError, ValueError):
        return False


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
