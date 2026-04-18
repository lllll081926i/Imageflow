import os
import tempfile
import unittest
from pathlib import Path

from backend.app import create_app


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

    def test_resolve_file_paths_returns_empty_when_any_item_cannot_be_resolved(self):
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

        self.assertEqual(resolved, [])


if __name__ == "__main__":
    unittest.main()
