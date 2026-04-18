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


if __name__ == "__main__":
    unittest.main()
