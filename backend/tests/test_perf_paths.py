import os
import tempfile
import unittest
from pathlib import Path

from PIL import Image

from backend.app import create_app


class PerfPathTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        os.environ["IMAGEFLOW_SETTINGS_FILE"] = os.path.join(self.temp_dir.name, "settings.json")

    def tearDown(self):
        os.environ.pop("IMAGEFLOW_SETTINGS_FILE", None)
        self.temp_dir.cleanup()

    def _png(self, name="a.png"):
        path = Path(self.temp_dir.name) / name
        Image.new("RGB", (32, 24), (1, 2, 3)).save(path)
        return str(path)

    def test_resolve_output_paths_batch(self):
        app = create_app()
        base = Path(self.temp_dir.name)
        first = str(base / "out.png")
        # create conflict for first
        Path(first).write_bytes(b"x")
        result = app.resolve_output_paths({
            "items": [first, str(base / "second.png")],
            "reserved": [],
        })
        self.assertTrue(result.get("success"), result)
        paths = result.get("paths") or []
        self.assertEqual(len(paths), 2)
        self.assertNotEqual(paths[0], first)
        self.assertTrue(paths[0].endswith("out_01.png") or "out_" in paths[0])
        self.assertTrue(str(paths[1]).endswith("second.png"))


if __name__ == "__main__":
    unittest.main()
