"""System tray support for background running.

The tray is created lazily the first time the window hides while
`background_running` is enabled. Menu: show window / quit.
"""

from __future__ import annotations

import threading
from typing import Callable

from threading import Lock

_tray_lock = Lock()
_tray_icon = None


def _make_icon_image():
    from PIL import Image, ImageDraw

    img = Image.new("RGBA", (64, 64), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)
    top = (0, 122, 255)
    bottom = (88, 86, 214)
    for y in range(64):
        t = y / 63
        color = (
            round(top[0] + (bottom[0] - top[0]) * t),
            round(top[1] + (bottom[1] - top[1]) * t),
            round(top[2] + (bottom[2] - top[2]) * t),
            255,
        )
        draw.line([(3, y), (60, y)], fill=color)
    mask = Image.new("L", (64, 64), 0)
    mask_draw = ImageDraw.Draw(mask)
    mask_draw.rounded_rectangle([0, 0, 63, 63], radius=14, fill=255)
    img.putalpha(mask)
    return img


def ensure_tray(on_show: Callable[[], None], on_quit: Callable[[], None]) -> bool:
    """Create the tray icon once; later calls are no-ops. Returns success."""
    global _tray_icon
    with _tray_lock:
        if _tray_icon is not None:
            return True
        try:
            import pystray

            def _show() -> None:
                try:
                    on_show()
                except Exception:
                    pass

            def _quit() -> None:
                try:
                    stop_tray()
                finally:
                    on_quit()

            icon = pystray.Icon(
                "ImageFlow",
                _make_icon_image(),
                "ImageFlow",
                pystray.Menu(
                    pystray.MenuItem("显示 ImageFlow", lambda *_: _show(), default=True),
                    pystray.MenuItem("退出", lambda *_: _quit()),
                ),
            )

            def _run() -> None:
                try:
                    icon.run()
                except Exception:
                    pass

            threading.Thread(target=_run, name="imageflow-tray", daemon=True).start()
            _tray_icon = icon
            return True
        except Exception:
            return False


def stop_tray() -> None:
    global _tray_icon
    with _tray_lock:
        if _tray_icon is not None:
            try:
                _tray_icon.stop()
            except Exception:
                pass
            _tray_icon = None
