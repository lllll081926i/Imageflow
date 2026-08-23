import os
import tempfile
import threading
import time
import unittest
from pathlib import Path
from unittest import mock

from PIL import Image

from backend.api import desktop_api
from backend.app import create_app
from backend.contracts.settings import default_app_settings
from backend.infrastructure import dialogs


class DesktopAPITests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.settings_file = os.path.join(self.temp_dir.name, "settings.json")
        self.settings_env = mock.patch.dict(
            os.environ,
            {"IMAGEFLOW_SETTINGS_FILE": self.settings_file},
        )
        self.settings_env.start()

    def tearDown(self):
        self.settings_env.stop()
        self.temp_dir.cleanup()

    def test_ping_and_settings_roundtrip(self):
        app = create_app()

        self.assertEqual(app.ping(), "pong")

        settings = app.get_settings()
        self.assertEqual(settings["max_concurrency"], default_app_settings().max_concurrency)
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

    def test_select_input_files_forwards_dialog_filters(self):
        app = create_app()
        captured_options = {}
        filters = [{"DisplayName": "Compressible bitmaps", "Pattern": "*.jpg;*.png"}]
        original_open_file_dialog = desktop_api.open_file_dialog

        def fake_open_file_dialog(options=None):
            captured_options.update(options or {})
            return ["D:/input/a.png"]

        try:
            desktop_api.open_file_dialog = fake_open_file_dialog

            result = app.select_input_files({"filters": filters})

            self.assertEqual(result, ["D:/input/a.png"])
            self.assertEqual(captured_options.get("filters"), filters)
            self.assertTrue(captured_options.get("allowsMultipleSelection"))
        finally:
            desktop_api.open_file_dialog = original_open_file_dialog

    def test_select_input_files_preserves_single_selection_option(self):
        app = create_app()
        captured_options = {}
        original_open_file_dialog = desktop_api.open_file_dialog

        def fake_open_file_dialog(options=None):
            captured_options.update(options or {})
            return "D:/input/a.png"

        try:
            desktop_api.open_file_dialog = fake_open_file_dialog

            result = app.select_input_files({"allowsMultipleSelection": False})

            self.assertEqual(result, ["D:/input/a.png"])
            self.assertFalse(captured_options.get("allowsMultipleSelection"))
        finally:
            desktop_api.open_file_dialog = original_open_file_dialog

    def test_resolve_output_path_returns_error_for_empty_base_path(self):
        app = create_app()

        result = app.resolve_output_path({"base_path": ""})

        self.assertFalse(result["success"])
        self.assertIn("路径不能为空", result["error"])

    def test_get_thumbnail_returns_data_url_and_caches(self):
        from PIL import Image as PILImage

        app = create_app()
        image_path = str((Path(self.temp_dir.name) / "thumb.png").resolve())
        PILImage.new("RGBA", (64, 32), (10, 200, 30, 255)).save(image_path, format="PNG")

        result = app.get_thumbnail({"path": image_path, "max_edge": 48})

        self.assertTrue(result["success"], result)
        self.assertTrue(result["data_url"].startswith("data:image/png;base64,"))
        self.assertLessEqual(max(result["width"], result["height"]), 48)

        cached = app.get_thumbnail({"path": image_path, "max_edge": 48})
        self.assertEqual(cached["data_url"], result["data_url"])

        missing = app.get_thumbnail({"path": str(Path(self.temp_dir.name) / "missing.png")})
        self.assertFalse(missing["success"])
        self.assertIn("NOT_FOUND", missing["error"])

    def test_get_thumbnail_rejects_unsupported_extensions(self):
        app = create_app()
        text_path = (Path(self.temp_dir.name) / "note.txt").resolve()
        text_path.write_text("hello", encoding="utf-8")

        result = app.get_thumbnail({"path": str(text_path)})

        self.assertFalse(result["success"])
        self.assertIn("UNSUPPORTED_FORMAT", result["error"])

    def test_resolve_output_path_strategies(self):
        app = create_app()
        temp_path = Path(self.temp_dir.name).resolve()
        existing = temp_path / "exists.png"
        existing.write_bytes(b"x")
        missing = temp_path / "missing.png"

        overwrite = app.resolve_output_path(
            {"base_path": str(existing), "strategy": "overwrite"}
        )
        self.assertTrue(overwrite["success"])
        self.assertEqual(Path(overwrite["output_path"]).resolve(), existing)
        self.assertTrue(overwrite["exists"])

        skip_missing = app.resolve_output_path({"base_path": str(missing), "strategy": "skip"})
        self.assertTrue(skip_missing["success"])
        self.assertFalse(skip_missing["exists"])

        skip_existing = app.resolve_output_path({"base_path": str(existing), "strategy": "skip"})
        self.assertTrue(skip_existing["success"])
        self.assertTrue(skip_existing["exists"])

        skip_reserved = app.resolve_output_path(
            {
                "base_path": str(missing),
                "strategy": "skip",
                "reserved": [str(missing)],
            }
        )
        self.assertTrue(skip_reserved["success"])
        self.assertTrue(skip_reserved["exists"])

        renamed = app.resolve_output_path({"base_path": str(existing)})
        self.assertTrue(renamed["success"])
        self.assertNotEqual(Path(renamed["output_path"]).resolve(), existing)
        self.assertTrue(Path(renamed["output_path"]).exists() is False)

    def test_resolve_output_paths_batch_reports_exists_flags(self):
        app = create_app()
        temp_path = Path(self.temp_dir.name).resolve()
        existing = temp_path / "batch-exists.png"
        existing.write_bytes(b"x")
        missing = temp_path / "batch-missing.png"

        result = app.resolve_output_paths(
            {
                "items": [str(existing), str(missing)],
                "strategy": "skip",
            }
        )

        self.assertTrue(result["success"])
        self.assertEqual([str(Path(p).resolve()) for p in result["paths"]], [str(existing), str(missing)])
        self.assertEqual(result["exists"], [True, False])

        duplicate = app.resolve_output_paths(
            {
                "items": [str(missing), str(missing)],
                "strategy": "skip",
            }
        )
        self.assertTrue(duplicate["success"])
        self.assertEqual(duplicate["exists"], [False, True])

        reserved = app.resolve_output_paths(
            {
                "items": [str(missing)],
                "strategy": "skip",
                "reserved": [str(missing)],
            }
        )
        self.assertTrue(reserved["success"])
        self.assertEqual(reserved["exists"], [True])

        renamed = app.resolve_output_paths({"items": [str(existing)]})
        self.assertTrue(renamed["success"])
        self.assertNotIn("exists", renamed)
        self.assertNotEqual(Path(renamed["paths"][0]).resolve(), existing)

    def test_generate_pdf_preserves_image_object_payloads_when_normalizing_paths(self):
        app = create_app()
        captured_payload = {}
        original_execute_engine = desktop_api.execute_engine

        image_path = str((Path(self.temp_dir.name) / "page.png").resolve())

        def fake_execute_engine(module_name, payload, *_args, **_kwargs):
            captured_payload["module_name"] = module_name
            captured_payload["payload"] = payload
            return {"success": True}

        try:
            desktop_api.execute_engine = fake_execute_engine

            result = app.generate_pdf(
                {
                    "images": [{"path": image_path}],
                    "output_path": str((Path(self.temp_dir.name) / "out.pdf").resolve()),
                }
            )

            self.assertTrue(result["success"])
            self.assertEqual(captured_payload["module_name"], "pdf_generator")
            self.assertEqual(captured_payload["payload"]["images"], [{"path": image_path}])
        finally:
            desktop_api.execute_engine = original_execute_engine

    def test_rich_text_block_paths_are_normalized_and_reject_parent_traversal(self):
        image_path = str((Path(self.temp_dir.name) / "inline.png").resolve())

        normalized = desktop_api._normalize_payload_paths(
            {"blocks": [{"type": "image", "path": image_path}]}
        )
        self.assertEqual(normalized["blocks"][0]["path"], image_path)

        with self.assertRaisesRegex(ValueError, "父级目录"):
            desktop_api._normalize_payload_paths(
                {"blocks": [{"type": "image", "path": "../outside.png"}]}
            )

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



    def test_default_task_manager_respects_module_level_replacement(self):
        calls: list[str] = []

        class FakeTaskManager:
            def __init__(self):
                calls.append("created")

            def cancel_current_task(self):
                calls.append("cancelled")
                return True

        sentinel = object()
        original_task_manager = desktop_api.__dict__.get("TaskManager", sentinel)
        try:
            desktop_api.TaskManager = FakeTaskManager
            app = desktop_api.DesktopAPI()

            self.assertTrue(app.cancel_processing())

            self.assertEqual(calls, ["created", "cancelled"])
        finally:
            if original_task_manager is sentinel:
                desktop_api.__dict__.pop("TaskManager", None)
            else:
                desktop_api.TaskManager = original_task_manager


    def test_convert_batch_always_returns_list_on_engine_failure(self):
        app = create_app()
        original = desktop_api.execute_engine_batch

        def boom(*_args, **_kwargs):
            raise RuntimeError("worker exploded")

        try:
            desktop_api.execute_engine_batch = boom
            results = app.convert_batch(
                [
                    {"input_path": "a.png", "output_path": "a-out.png", "format": "png"},
                    {"input_path": "b.png", "output_path": "b-out.png", "format": "png"},
                ]
            )
        finally:
            desktop_api.execute_engine_batch = original

        self.assertIsInstance(results, list)
        self.assertEqual(len(results), 2)
        self.assertFalse(results[0].get("success"))
        self.assertIn("worker exploded", str(results[0].get("error") or ""))
        self.assertTrue(str(results[0].get("input_path") or "").endswith("a.png"))
        self.assertTrue(str(results[1].get("input_path") or "").endswith("b.png"))

    def test_compress_batch_empty_payload_returns_empty_list(self):
        app = create_app()
        self.assertEqual(app.compress_batch([]), [])


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
