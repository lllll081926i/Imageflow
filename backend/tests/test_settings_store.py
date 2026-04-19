import json
import unittest
import os
import tempfile
from pathlib import Path

from backend.contracts.settings import AppSettings
from backend.infrastructure.settings_store import load_settings, normalize_settings


class SettingsStoreTests(unittest.TestCase):
    def test_normalize_settings_clamps_concurrency_and_paths(self):
        raw = AppSettings(
            max_concurrency=99,
            default_output_dir="C:/tmp///",
            recent_input_dirs=["C:/One///", "c:/one", "", "D:/Two"],
        )

        normalized = normalize_settings(raw)

        self.assertEqual(normalized.max_concurrency, 32)
        self.assertEqual(normalized.default_output_dir, "C:/tmp")
        self.assertEqual(normalized.recent_input_dirs, ["C:/One", "D:/Two"])

    def test_load_settings_falls_back_to_defaults_for_invalid_json(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            settings_file = Path(temp_dir) / "settings.json"
            settings_file.write_text("{ invalid json", encoding="utf-8")
            os.environ["IMAGEFLOW_SETTINGS_FILE"] = str(settings_file)
            self.addCleanup(lambda: os.environ.pop("IMAGEFLOW_SETTINGS_FILE", None))

            loaded = load_settings()

            self.assertEqual(loaded.max_concurrency, 8)
            self.assertEqual(loaded.output_prefix, "IF")

    def test_load_settings_preserves_known_values_when_file_has_unknown_or_string_fields(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            settings_file = Path(temp_dir) / "settings.json"
            settings_file.write_text(
                json.dumps(
                    {
                        "max_concurrency": "12",
                        "output_prefix": "KEEP",
                        "output_template": "{basename}",
                        "preserve_folder_structure": "false",
                        "conflict_strategy": "rename",
                        "default_output_dir": "D:/Out///",
                        "recent_input_dirs": [r"D:\Input", "d:/input///", "E:/Other"],
                        "recent_output_dirs": [],
                        "future_field": "ignored",
                    }
                ),
                encoding="utf-8",
            )
            os.environ["IMAGEFLOW_SETTINGS_FILE"] = str(settings_file)
            self.addCleanup(lambda: os.environ.pop("IMAGEFLOW_SETTINGS_FILE", None))

            loaded = load_settings()

            self.assertEqual(loaded.max_concurrency, 12)
            self.assertEqual(loaded.output_prefix, "KEEP")
            self.assertFalse(loaded.preserve_folder_structure)
            self.assertEqual(loaded.default_output_dir, "D:/Out")
            self.assertEqual(loaded.recent_input_dirs, [r"D:\Input", "E:/Other"])


if __name__ == "__main__":
    unittest.main()
