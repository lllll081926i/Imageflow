import sys
import tempfile
import types
import unittest
from pathlib import Path

import backend.main as main_module
from backend.host.window import resolve_frontend_entry
from backend.infrastructure import window_ops


class WindowConfigTests(unittest.TestCase):
    def tearDown(self):
        import os

        os.environ.pop("IMAGEFLOW_FRONTEND_URL", None)
        window_ops.set_window_maximized(False)

    def test_prefers_explicit_frontend_url(self):
        entry = resolve_frontend_entry(frontend_url="http://127.0.0.1:5173")
        self.assertEqual(entry, "http://127.0.0.1:5173")

    def test_uses_environment_frontend_url_when_present(self):
        import os

        os.environ["IMAGEFLOW_FRONTEND_URL"] = "http://127.0.0.1:4173"

        entry = resolve_frontend_entry()

        self.assertEqual(entry, "http://127.0.0.1:4173")

    def test_falls_back_to_frontend_dist_index(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            dist_index = root / "frontend" / "dist" / "index.html"
            dist_index.parent.mkdir(parents=True, exist_ok=True)
            dist_index.write_text("<html></html>", encoding="utf-8")

            entry = resolve_frontend_entry(project_root=root)

            self.assertEqual(entry, str(dist_index))

    def test_returns_frontend_dist_target_when_build_is_missing(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            dist_index = root / "frontend" / "dist" / "index.html"

            entry = resolve_frontend_entry(project_root=root)

            self.assertEqual(entry, str(dist_index))

    def test_prefers_packaged_frontend_dist_when_running_from_pyinstaller(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            runtime_root = Path(temp_dir)
            dist_index = runtime_root / "frontend" / "dist" / "index.html"
            dist_index.parent.mkdir(parents=True, exist_ok=True)
            dist_index.write_text("<html></html>", encoding="utf-8")

            original_meipass = getattr(sys, "_MEIPASS", None)
            sys._MEIPASS = str(runtime_root)
            try:
                entry = resolve_frontend_entry()
            finally:
                if original_meipass is None:
                    delattr(sys, "_MEIPASS")
                else:
                    sys._MEIPASS = original_meipass

            self.assertEqual(entry, str(dist_index))

    def test_window_toggle_maximise_updates_local_state_without_host_events(self):
        class DummyWindow:
            def __init__(self):
                self.calls: list[str] = []

            def maximize(self):
                self.calls.append("maximize")

            def restore(self):
                self.calls.append("restore")

        dummy_window = DummyWindow()
        previous_webview = sys.modules.get("webview")
        sys.modules["webview"] = types.SimpleNamespace(windows=[dummy_window])
        try:
            window_ops.set_window_maximized(False)

            window_ops.runtime_window_toggle_maximise()
            self.assertEqual(dummy_window.calls, ["maximize"])
            self.assertTrue(window_ops.is_window_maximized())

            window_ops.runtime_window_toggle_maximise()
            self.assertEqual(dummy_window.calls, ["maximize", "restore"])
            self.assertFalse(window_ops.is_window_maximized())
        finally:
            if previous_webview is None:
                sys.modules.pop("webview", None)
            else:
                sys.modules["webview"] = previous_webview

    def test_main_creates_frameless_window_for_custom_titlebar(self):
        captured: dict[str, object] = {}
        fake_window = object()

        def fake_create_window(**kwargs):
            captured.update(kwargs)
            return fake_window

        def fake_start():
            captured["started"] = True

        previous_webview = sys.modules.get("webview")
        original_build_window_api = main_module.build_window_api
        original_configure_window = main_module.configure_window
        original_resolve_frontend_entry = main_module.resolve_frontend_entry
        sys.modules["webview"] = types.SimpleNamespace(create_window=fake_create_window, start=fake_start)

        try:
            main_module.build_window_api = lambda: "api"
            main_module.configure_window = lambda window: captured.setdefault("configured_window", window)
            main_module.resolve_frontend_entry = lambda: "http://127.0.0.1:5173"

            main_module.main()

            self.assertTrue(captured["frameless"])
            self.assertFalse(captured["easy_drag"])
            self.assertEqual(captured["configured_window"], fake_window)
            self.assertTrue(captured["started"])
        finally:
            main_module.build_window_api = original_build_window_api
            main_module.configure_window = original_configure_window
            main_module.resolve_frontend_entry = original_resolve_frontend_entry
            if previous_webview is None:
                sys.modules.pop("webview", None)
            else:
                sys.modules["webview"] = previous_webview


if __name__ == "__main__":
    unittest.main()
