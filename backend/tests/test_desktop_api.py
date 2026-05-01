import os
import tempfile
import threading
import time
import unittest
from pathlib import Path

from PIL import Image

from backend.api import desktop_api
from backend.app import create_app
from backend.infrastructure import dialogs


class DesktopAPITests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.settings_file = os.path.join(self.temp_dir.name, "settings.json")
        os.environ["IMAGEFLOW_SETTINGS_FILE"] = self.settings_file

    def tearDown(self):
        os.environ.pop("IMAGEFLOW_SETTINGS_FILE", None)
        os.environ.pop("IMAGEFLOW_PREVIEW_MAX_BYTES", None)
        self.temp_dir.cleanup()

    def test_ping_and_settings_roundtrip(self):
        app = create_app()

        self.assertEqual(app.ping(), "pong")

        settings = app.get_settings()
        self.assertEqual(settings["max_concurrency"], 8)
        self.assertEqual(settings["output_prefix"], "IF")

        saved = app.save_settings(
            {
                "max_concurrency": 12,
                "output_prefix": "TEST",
                "output_template": "{basename}",
                "preserve_folder_structure": True,
                "conflict_strategy": "rename",
                "default_output_dir": "C:/Output///",
                "recent_input_dirs": ["C:/Input///", "c:/input"],
                "recent_output_dirs": [],
            }
        )
        self.assertEqual(saved["max_concurrency"], 12)
        self.assertEqual(saved["default_output_dir"], "C:/Output")

        reloaded = app.get_settings()
        self.assertEqual(reloaded["output_prefix"], "TEST")
        self.assertEqual(reloaded["recent_input_dirs"], ["C:/Input"])

    def test_save_settings_ignores_unknown_fields_and_normalizes_types(self):
        app = create_app()

        saved = app.save_settings(
            {
                "max_concurrency": "16",
                "output_prefix": "IF",
                "output_template": "{prefix}{basename}",
                "preserve_folder_structure": "false",
                "conflict_strategy": "rename",
                "default_output_dir": "",
                "recent_input_dirs": [],
                "recent_output_dirs": [],
                "future_field": "ignored",
            }
        )

        self.assertEqual(saved["max_concurrency"], 16)
        self.assertFalse(saved["preserve_folder_structure"])

    def test_update_recent_paths_deduplicates_and_limits_entries(self):
        app = create_app()

        app.save_settings(
            {
                "max_concurrency": 8,
                "output_prefix": "IF",
                "output_template": "{prefix}{basename}",
                "preserve_folder_structure": True,
                "conflict_strategy": "rename",
                "default_output_dir": "",
                "recent_input_dirs": ["D:/One", "D:/Two", "D:/Three", "D:/Four"],
                "recent_output_dirs": ["E:/One"],
            }
        )

        updated = app.update_recent_paths({"input_dir": "d:/two///", "output_dir": "E:/Two///"})

        self.assertEqual(updated["recent_input_dirs"], ["d:/two", "D:/One", "D:/Three", "D:/Four"])
        self.assertEqual(updated["recent_output_dirs"], ["E:/Two", "E:/One"])

    def test_update_recent_paths_deduplicates_slash_variants(self):
        app = create_app()

        app.save_settings(
            {
                "max_concurrency": 8,
                "output_prefix": "IF",
                "output_template": "{prefix}{basename}",
                "preserve_folder_structure": True,
                "conflict_strategy": "rename",
                "default_output_dir": "",
                "recent_input_dirs": [r"D:\Input"],
                "recent_output_dirs": [],
            }
        )

        updated = app.update_recent_paths({"input_dir": "d:/input///", "output_dir": ""})

        self.assertEqual(updated["recent_input_dirs"], ["d:/input"])

    def test_expand_dropped_paths_filters_non_image_files(self):
        app = create_app()
        root = Path(self.temp_dir.name) / "drop"
        nested = root / "nested"
        nested.mkdir(parents=True)
        (nested / "image.png").write_bytes(b"png")
        (nested / "notes.txt").write_text("skip", encoding="utf-8")

        result = app.expand_dropped_paths([str(root)])

        self.assertTrue(result["has_directory"])
        self.assertEqual([item["relative_path"] for item in result["files"]], ["nested/image.png"])

    def test_expand_dropped_paths_ignores_missing_paths(self):
        app = create_app()
        root = Path(self.temp_dir.name) / "drop2"
        root.mkdir(parents=True)
        image = root / "image.png"
        image.write_bytes(b"png")

        result = app.expand_dropped_paths([str(root / "missing.png"), str(image)])

        self.assertFalse(result["has_directory"])
        self.assertEqual([item["input_path"] for item in result["files"]], [str(image.resolve())])

    def test_get_image_preview_returns_data_url(self):
        app = create_app()
        preview = app.get_image_preview(
            {
                "input_path": str(Path("backend/testdata/simple.svg").resolve()),
            }
        )

        self.assertTrue(preview["success"])
        self.assertTrue(str(preview["data_url"]).startswith("data:image/jpeg;base64,"))

    def test_get_image_preview_skips_large_input_when_limit_exceeded(self):
        app = create_app()
        large_file = Path(self.temp_dir.name) / "large.png"
        large_file.write_bytes(b"x" * 32)
        os.environ["IMAGEFLOW_PREVIEW_MAX_BYTES"] = "4"

        preview = app.get_image_preview({"input_path": str(large_file)})

        self.assertFalse(preview["success"])
        self.assertEqual(preview["error"], "PREVIEW_SKIPPED")

    def test_get_image_preview_ignores_invalid_max_bytes_env(self):
        app = create_app()
        preview_path = Path("backend/testdata/simple.svg").resolve()
        os.environ["IMAGEFLOW_PREVIEW_MAX_BYTES"] = "not-a-number"

        preview = app.get_image_preview({"input_path": str(preview_path)})

        self.assertTrue(preview["success"])
        self.assertTrue(str(preview["data_url"]).startswith("data:image/jpeg;base64,"))

    def test_get_image_preview_returns_error_payload_for_invalid_image(self):
        app = create_app()
        invalid_image = Path(self.temp_dir.name) / "broken.png"
        invalid_image.write_text("not an image", encoding="utf-8")

        preview = app.get_image_preview({"input_path": str(invalid_image)})

        self.assertFalse(preview["success"])
        self.assertIn("error", preview)

    def test_resolve_output_path_returns_error_for_empty_base_path(self):
        app = create_app()

        result = app.resolve_output_path({"base_path": ""})

        self.assertFalse(result["success"])
        self.assertIn("路径不能为空", result["error"])

    def test_resolve_file_paths_extracts_absolute_paths_from_runtime_payloads(self):
        app = create_app()
        first = str((Path(self.temp_dir.name) / "first.png").resolve())
        second = str((Path(self.temp_dir.name) / "second.png").resolve())

        resolved = app.resolve_file_paths(
            [
                {"path": first},
                {"pywebviewFullPath": second},
            ]
        )

        self.assertEqual(resolved, [first, second])
        self.assertTrue(app.can_resolve_file_paths())

    def test_resolve_file_paths_skips_items_that_cannot_be_resolved(self):
        app = create_app()
        first = str((Path(self.temp_dir.name) / "first.png").resolve())

        class DummyFile:
            def __init__(self, path: str):
                self.path = path

        resolved = app.resolve_file_paths(
            [
                {"path": first},
                DummyFile("relative-only.png"),
            ]
        )

        self.assertEqual(resolved, [first])

    def test_probe_animated_paths_returns_frame_counts_without_spawning_engine_processes(self):
        app = create_app()
        static_png = Path(self.temp_dir.name) / "static.png"
        animated_gif = Path(self.temp_dir.name) / "animated.gif"

        Image.new("RGBA", (16, 16), (255, 0, 0, 255)).save(static_png, format="PNG")
        frames = [
            Image.new("RGBA", (16, 16), (255, 0, 0, 255)),
            Image.new("RGBA", (16, 16), (0, 255, 0, 255)),
        ]
        frames[0].save(
            animated_gif,
            format="GIF",
            save_all=True,
            append_images=frames[1:],
            duration=[100, 100],
            loop=0,
        )

        result = app.probe_animated_paths([str(static_png), str(animated_gif), str(Path(self.temp_dir.name) / "missing.webp")])

        self.assertEqual(result[0]["input_path"], str(static_png.resolve()))
        self.assertEqual(result[0]["frame_count"], 1)
        self.assertFalse(result[0]["is_animated"])

        self.assertEqual(result[1]["input_path"], str(animated_gif.resolve()))
        self.assertEqual(result[1]["frame_count"], 2)
        self.assertTrue(result[1]["is_animated"])

        self.assertEqual(result[2]["frame_count"], 0)
        self.assertFalse(result[2]["is_animated"])
        self.assertIn("error", result[2])

    def test_probe_animated_paths_returns_error_payload_for_invalid_path(self):
        app = create_app()

        result = app.probe_animated_paths(["../outside.png"])

        self.assertEqual(len(result), 1)
        self.assertEqual(result[0]["input_path"], "../outside.png")
        self.assertEqual(result[0]["frame_count"], 0)
        self.assertFalse(result[0]["is_animated"])
        self.assertIn("不允许使用父级目录跳转路径", result[0]["error"])

    def test_get_info_cancels_previous_inflight_info_request(self):
        app = create_app()
        first = Path(self.temp_dir.name) / "first.png"
        second = Path(self.temp_dir.name) / "second.png"
        first.write_bytes(b"first")
        second.write_bytes(b"second")

        original_execute_engine = desktop_api.execute_engine
        first_started = threading.Event()
        first_cancelled = threading.Event()
        first_result: dict[str, object] = {}

        def fake_execute_engine(_module_name, payload, task_manager, task_id=None):
            input_name = Path(str(payload["input_path"])).name
            if input_name == "first.png":
                first_started.set()
                deadline = time.time() + 2.0
                while time.time() < deadline:
                    if task_manager.is_cancelled(task_id):
                        first_cancelled.set()
                        return {"success": False, "error": "[PY_CANCELLED] operation cancelled"}
                    time.sleep(0.01)
                return {"success": True, "input_path": payload["input_path"]}
            return {"success": True, "input_path": payload["input_path"]}

        def run_first_request():
            first_result["value"] = app.get_info({"input_path": str(first)})

        worker = threading.Thread(target=run_first_request)
        try:
            desktop_api.execute_engine = fake_execute_engine
            worker.start()
            self.assertTrue(first_started.wait(timeout=2.0))

            second_result = app.get_info({"input_path": str(second)})

            worker.join(timeout=2.0)
            self.assertFalse(worker.is_alive())
            self.assertTrue(first_cancelled.is_set())
            self.assertEqual(first_result["value"], {"success": False, "error": "[PY_CANCELLED] operation cancelled"})
            self.assertEqual(second_result, {"success": True, "input_path": str(second.resolve())})
        finally:
            desktop_api.execute_engine = original_execute_engine
            if worker.is_alive():
                worker.join(timeout=0.5)

    def test_get_info_does_not_replace_current_operation_task(self):
        task_manager = desktop_api.TaskManager()
        app = desktop_api.DesktopAPI(task_manager)
        operation_task_id = task_manager.begin_task("operation")
        image_path = Path(self.temp_dir.name) / "sample.png"
        image_path.write_bytes(b"sample")

        original_execute_engine = desktop_api.execute_engine
        try:
            desktop_api.execute_engine = lambda _module_name, _payload, _task_manager, task_id=None: {
                "success": True,
                "info_task_id": task_id,
                "current_task_id": _task_manager.current_task_id,
            }

            result = app.get_info({"input_path": str(image_path)})

            self.assertTrue(result["success"])
            self.assertIsNotNone(result["info_task_id"])
            self.assertNotEqual(result["info_task_id"], operation_task_id)
            self.assertEqual(result["current_task_id"], operation_task_id)
            self.assertEqual(task_manager.current_task_id, operation_task_id)
            self.assertTrue(task_manager.cancel_current_task())
            self.assertTrue(task_manager.is_cancelled(operation_task_id))
        finally:
            desktop_api.execute_engine = original_execute_engine
            task_manager.finish_task(operation_task_id)

    def test_get_info_registers_new_active_task_atomically(self):
        task_manager = desktop_api.TaskManager()
        app = desktop_api.DesktopAPI(task_manager)
        first = Path(self.temp_dir.name) / "first.png"
        second = Path(self.temp_dir.name) / "second.png"
        first.write_bytes(b"first")
        second.write_bytes(b"second")

        original_begin_task = task_manager.begin_task
        original_execute_engine = desktop_api.execute_engine
        first_begin_entered = threading.Event()
        first_cancelled = threading.Event()
        first_result: dict[str, object] = {}
        info_begin_count = 0

        def delayed_begin_task(kind: str, set_current: bool = True):
            nonlocal info_begin_count
            if kind == "info":
                info_begin_count += 1
                if info_begin_count == 1:
                    first_begin_entered.set()
                    time.sleep(0.2)
            return original_begin_task(kind, set_current=set_current)

        def fake_execute_engine(_module_name, payload, current_task_manager, task_id=None):
            input_name = Path(str(payload["input_path"])).name
            if input_name == "first.png":
                deadline = time.time() + 2.0
                while time.time() < deadline:
                    if current_task_manager.is_cancelled(task_id):
                        first_cancelled.set()
                        return {"success": False, "error": "[PY_CANCELLED] operation cancelled"}
                    time.sleep(0.01)
                return {"success": True, "input_path": payload["input_path"]}
            return {"success": True, "input_path": payload["input_path"]}

        def run_first_request():
            first_result["value"] = app.get_info({"input_path": str(first)})

        worker = threading.Thread(target=run_first_request)
        try:
            task_manager.begin_task = delayed_begin_task  # type: ignore[method-assign]
            desktop_api.execute_engine = fake_execute_engine
            worker.start()
            self.assertTrue(first_begin_entered.wait(timeout=2.0))

            second_result = app.get_info({"input_path": str(second)})

            worker.join(timeout=2.0)
            self.assertFalse(worker.is_alive())
            self.assertTrue(first_cancelled.is_set())
            self.assertEqual(first_result["value"], {"success": False, "error": "[PY_CANCELLED] operation cancelled"})
            self.assertEqual(second_result, {"success": True, "input_path": str(second.resolve())})
        finally:
            task_manager.begin_task = original_begin_task  # type: ignore[method-assign]
            desktop_api.execute_engine = original_execute_engine
            if worker.is_alive():
                worker.join(timeout=0.5)


class DialogInfrastructureTests(unittest.TestCase):
    def test_ensure_dialog_thread_raises_when_worker_startup_fails(self):
        original_worker = dialogs._dialog_worker
        original_started = dialogs._dialog_started

        result: dict[str, object] = {}

        def failing_worker():
            raise RuntimeError("tk init failed")

        def invoke():
            try:
                dialogs._ensure_dialog_thread()
                result["value"] = "ok"
            except Exception as exc:
                result["error"] = exc

        try:
            dialogs._dialog_worker = failing_worker
            dialogs._dialog_started = threading.Event()

            worker = threading.Thread(target=invoke, daemon=True)
            worker.start()
            worker.join(timeout=1.0)

            self.assertFalse(worker.is_alive(), "dialog startup should fail fast instead of hanging forever")
            self.assertIsInstance(result.get("error"), RuntimeError)
            self.assertIn("tk init failed", str(result.get("error")))
        finally:
            dialogs._dialog_worker = original_worker
            dialogs._dialog_started = original_started


if __name__ == "__main__":
    unittest.main()
