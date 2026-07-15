"""Detailed contract tests for DesktopAPI *Batch methods and helpers."""

from __future__ import annotations

import os
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from PIL import Image

from backend.api import desktop_api
from backend.api.desktop_api import DesktopAPI
from backend.app import create_app
from backend.contracts.settings import default_app_settings


class BatchContractTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.settings_file = os.path.join(self.temp_dir.name, "settings.json")
        os.environ["IMAGEFLOW_SETTINGS_FILE"] = self.settings_file
        self.app = create_app()
        self._original_execute_batch = desktop_api.execute_engine_batch
        self._original_execute = desktop_api.execute_engine

    def tearDown(self):
        desktop_api.execute_engine_batch = self._original_execute_batch
        desktop_api.execute_engine = self._original_execute
        os.environ.pop("IMAGEFLOW_SETTINGS_FILE", None)
        self.temp_dir.cleanup()

    def _payloads(self, count: int = 3, prefix: str = "item") -> list[dict]:
        items = []
        for index in range(count):
            items.append(
                {
                    "input_path": f"{prefix}-{index}.png",
                    "output_path": f"{prefix}-{index}-out.png",
                    "format": "png",
                }
            )
        return items

    def test_all_batch_methods_return_list_when_engine_raises(self):
        def boom(*_args, **_kwargs):
            raise RuntimeError("engine batch crash")

        desktop_api.execute_engine_batch = boom
        methods = [
            ("convert_batch", self._payloads()),
            ("compress_batch", self._payloads(prefix="c")),
            ("add_watermark_batch", self._payloads(prefix="w")),
            ("adjust_batch", self._payloads(prefix="a")),
            ("apply_filter_batch", self._payloads(prefix="f")),
        ]

        for method_name, payloads in methods:
            with self.subTest(method=method_name):
                result = getattr(self.app, method_name)(payloads)
                self.assertIsInstance(result, list, method_name)
                self.assertEqual(len(result), len(payloads), method_name)
                for index, item in enumerate(result):
                    self.assertFalse(item.get("success"), f"{method_name}[{index}]")
                    self.assertIn("engine batch crash", str(item.get("error") or ""))
                    self.assertTrue(str(item.get("input_path") or "").endswith(f"{index}.png"))

    def test_batch_methods_normalize_unexpected_dict_return(self):
        desktop_api.execute_engine_batch = lambda *_a, **_k: {"success": False, "error": "envelope"}
        result = self.app.convert_batch(self._payloads(2))
        self.assertIsInstance(result, list)
        self.assertEqual(len(result), 2)
        self.assertTrue(all(item.get("success") is False for item in result))
        self.assertTrue(all("envelope" in str(item.get("error") or "") for item in result))

    def test_batch_methods_normalize_non_list_non_dict_return(self):
        desktop_api.execute_engine_batch = lambda *_a, **_k: "not-a-list"
        result = self.app.compress_batch(self._payloads(2))
        self.assertEqual(len(result), 2)
        self.assertTrue(all(item.get("success") is False for item in result))
        self.assertTrue(all("格式异常" in str(item.get("error") or "") for item in result))

    def test_batch_methods_preserve_successful_list_order_and_payload(self):
        def fake_batch(module_name, payloads, settings, task_manager):
            self.assertEqual(module_name, "converter")
            self.assertIsInstance(settings.max_concurrency, int)
            self.assertGreaterEqual(settings.max_concurrency, 1)
            return [
                {"success": True, "input_path": item["input_path"], "index": index}
                for index, item in enumerate(payloads)
            ]

        desktop_api.execute_engine_batch = fake_batch
        payloads = self._payloads(4)
        # Keep absolute-looking paths so path normalization does not drop custom fields.
        for item in payloads:
            item["input_path"] = f"D:/images/{item['input_path']}"
            item["output_path"] = f"D:/out/{item['output_path']}"
        result = self.app.convert_batch(payloads)
        self.assertEqual(len(result), 4)
        self.assertEqual([item.get("index") for item in result], [0, 1, 2, 3])
        self.assertTrue(all(item.get("success") for item in result))

    def test_empty_batch_short_circuits_without_calling_engine(self):
        called = {"value": False}

        def should_not_run(*_args, **_kwargs):
            called["value"] = True
            raise AssertionError("engine should not run for empty batch")

        desktop_api.execute_engine_batch = should_not_run
        for method_name in (
            "convert_batch",
            "compress_batch",
            "add_watermark_batch",
            "adjust_batch",
            "apply_filter_batch",
        ):
            with self.subTest(method=method_name):
                self.assertEqual(getattr(self.app, method_name)([]), [])
        self.assertFalse(called["value"])

    def test_pascal_case_batch_aliases_match_snake_case_behavior(self):
        def boom(*_args, **_kwargs):
            raise RuntimeError("alias crash")

        desktop_api.execute_engine_batch = boom
        snake = self.app.convert_batch(self._payloads(2, prefix="s"))
        pascal = self.app.ConvertBatch(self._payloads(2, prefix="p"))
        self.assertIsInstance(snake, list)
        self.assertIsInstance(pascal, list)
        self.assertEqual(len(snake), 2)
        self.assertEqual(len(pascal), 2)
        self.assertIn("alias crash", str(snake[0].get("error") or ""))
        self.assertIn("alias crash", str(pascal[0].get("error") or ""))

    def test_single_operation_still_returns_error_dict_envelope(self):
        def boom(*_args, **_kwargs):
            raise RuntimeError("single fail")

        desktop_api.execute_engine = boom
        result = self.app.convert(
            {
                "input_path": "one.png",
                "output_path": "one-out.png",
                "format": "png",
            }
        )
        self.assertIsInstance(result, dict)
        self.assertFalse(result.get("success"))
        self.assertIn("single fail", str(result.get("error") or ""))

    def test_run_batch_operation_helper_attaches_and_finishes_task(self):
        api = DesktopAPI()
        manager = api._task_manager
        observed = {"begin": 0, "finish": []}

        original_begin = manager.begin_task
        original_finish = manager.finish_task

        def tracking_begin(kind, set_current=True):
            observed["begin"] += 1
            return original_begin(kind, set_current=set_current)

        def tracking_finish(task_id):
            observed["finish"].append(task_id)
            return original_finish(task_id)

        manager.begin_task = tracking_begin  # type: ignore[method-assign]
        manager.finish_task = tracking_finish  # type: ignore[method-assign]
        try:
            result = api._run_batch_operation(
                [{"input_path": "x.png"}],
                lambda: (_ for _ in ()).throw(RuntimeError("tracked")),
            )
        finally:
            manager.begin_task = original_begin  # type: ignore[method-assign]
            manager.finish_task = original_finish  # type: ignore[method-assign]

        self.assertEqual(observed["begin"], 1)
        self.assertEqual(len(observed["finish"]), 1)
        self.assertEqual(len(result), 1)
        self.assertIn("tracked", str(result[0].get("error") or ""))
        self.assertIsNone(manager.current_task_id)


class PreviewIsolationTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.settings_file = os.path.join(self.temp_dir.name, "settings.json")
        os.environ["IMAGEFLOW_SETTINGS_FILE"] = self.settings_file

    def tearDown(self):
        os.environ.pop("IMAGEFLOW_SETTINGS_FILE", None)
        os.environ.pop("IMAGEFLOW_PREVIEW_MAX_BYTES", None)
        self.temp_dir.cleanup()

    def _png(self, name: str = "preview.png", size=(64, 48), color=(12, 34, 56)) -> str:
        path = os.path.join(self.temp_dir.name, name)
        Image.new("RGB", size, color).save(path, format="PNG")
        return path

    def test_build_image_preview_returns_jpeg_data_url(self):
        from backend.application.preview import build_image_preview

        result = build_image_preview(self._png())
        self.assertTrue(result.get("success"), result)
        self.assertTrue(str(result.get("data_url") or "").startswith("data:image/jpeg;base64,"))

    def test_build_image_preview_skips_when_file_too_large(self):
        from backend.application.preview import build_image_preview

        os.environ["IMAGEFLOW_PREVIEW_MAX_BYTES"] = "32"
        path = self._png("big-name-only.png")
        # Force file size above threshold with padding bytes after a valid image write is hard;
        # write a larger raw file that still exists for size check before decode.
        with open(path, "wb") as handle:
            handle.write(b"0" * 128)
        result = build_image_preview(path)
        self.assertFalse(result.get("success"))
        self.assertEqual(result.get("error"), "PREVIEW_SKIPPED")

    def test_build_image_preview_smart_success_path(self):
        from backend.application.preview import build_image_preview_smart

        result = build_image_preview_smart(self._png("isolated.png"))
        self.assertTrue(result.get("success"), result)
        self.assertIn("data_url", result)

    def test_build_image_preview_smart_timeout_terminates_worker(self):
        from backend.application import preview as preview_module

        class HangingProcess:
            def __init__(self, *_args, **_kwargs):
                self._alive = False
                self.exitcode = None
                self.terminate_calls = 0
                self.kill_calls = 0

            def start(self):
                self._alive = True

            def join(self, timeout=None):
                # Stay alive for the initial timeout join, die after terminate.
                if self.terminate_calls:
                    self._alive = False
                    self.exitcode = -15

            def is_alive(self):
                return self._alive

            def terminate(self):
                self.terminate_calls += 1

            def kill(self):
                self.kill_calls += 1
                self._alive = False
                self.exitcode = -9

        class DummyQueue:
            def get_nowait(self):
                raise preview_module.Empty()

            def close(self):
                return None

            def join_thread(self):
                return None

        original_process = preview_module.Process
        original_queue = preview_module.Queue
        hanging = {"proc": None}

        def process_factory(*args, **kwargs):
            proc = HangingProcess(*args, **kwargs)
            hanging["proc"] = proc
            return proc

        try:
            preview_module.Process = process_factory  # type: ignore[assignment]
            preview_module.Queue = DummyQueue  # type: ignore[assignment]
            result = preview_module.build_image_preview_isolated("any.png", timeout=0.05)
        finally:
            preview_module.Process = original_process
            preview_module.Queue = original_queue

        self.assertFalse(result.get("success"))
        self.assertEqual(result.get("error"), "预览超时")
        self.assertIsNotNone(hanging["proc"])
        self.assertGreaterEqual(hanging["proc"].terminate_calls, 1)

    def test_desktop_api_get_image_preview_uses_isolated_builder(self):
        app = create_app()
        with mock.patch(
            "backend.application.preview.build_image_preview_smart",
            return_value={"success": True, "data_url": "data:image/jpeg;base64,xx"},
        ) as isolated:
            result = app.get_image_preview({"input_path": self._png("via-api.png")})
        self.assertTrue(result.get("success"))
        isolated.assert_called_once()
        called_path = isolated.call_args[0][0]
        self.assertTrue(str(called_path).endswith("via-api.png"))


class SvgSafetyUnitTests(unittest.TestCase):
    def test_assert_svg_render_safe_rejects_script_and_foreign_object(self):
        from backend.engines import converter as converter_module

        with tempfile.TemporaryDirectory() as temp_dir:
            for name, body in {
                "script.svg": '<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>',
                "foreign.svg": '<svg xmlns="http://www.w3.org/2000/svg"><foreignObject width="10" height="10"></foreignObject></svg>',
                "import.svg": '<svg xmlns="http://www.w3.org/2000/svg"><style>@import url("x.css");</style></svg>',
                "http_href.svg": '<svg xmlns="http://www.w3.org/2000/svg"><image href="https://example.com/a.png"/></svg>',
            }.items():
                path = Path(temp_dir) / name
                path.write_text(body, encoding="utf-8")
                with self.subTest(name=name):
                    with self.assertRaises(RuntimeError):
                        converter_module.assert_svg_render_safe(str(path))

    def test_assert_svg_render_safe_allows_plain_shapes(self):
        from backend.engines import converter as converter_module

        with tempfile.TemporaryDirectory() as temp_dir:
            path = Path(temp_dir) / "ok.svg"
            path.write_text(
                '<svg width="10" height="10" xmlns="http://www.w3.org/2000/svg">'
                '<rect width="10" height="10" fill="#f00"/></svg>',
                encoding="utf-8",
            )
            converter_module.assert_svg_render_safe(str(path))

    def test_clamp_svg_render_size_edge_and_pixel_caps(self):
        from backend.engines import converter as converter_module

        w, h = converter_module.clamp_svg_render_size(converter_module.MAX_SVG_EDGE * 4, 10)
        self.assertLessEqual(max(w, h), converter_module.MAX_SVG_EDGE)
        self.assertGreaterEqual(min(w, h), 1)

        w2, h2 = converter_module.clamp_svg_render_size(9000, 9000)
        self.assertLessEqual(w2 * h2, converter_module.MAX_SVG_PIXELS)


if __name__ == "__main__":
    unittest.main()
