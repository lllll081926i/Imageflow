import os
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from PIL import Image

from backend.app import create_app
from backend.application import preview as preview_module


class PerfPathTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        os.environ["IMAGEFLOW_SETTINGS_FILE"] = os.path.join(self.temp_dir.name, "settings.json")

    def tearDown(self):
        os.environ.pop("IMAGEFLOW_SETTINGS_FILE", None)
        os.environ.pop("IMAGEFLOW_PREVIEW_ISOLATE", None)
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

    def test_preview_smart_uses_in_process_for_png(self):
        path = self._png("preview.png")
        with mock.patch.object(preview_module, "build_image_preview_isolated") as isolated:
            result = preview_module.build_image_preview_smart(path)
            isolated.assert_not_called()
        self.assertTrue(result.get("success"), result)
        self.assertTrue(str(result.get("data_url") or "").startswith("data:image/jpeg;base64,"))

    def test_preview_cache_hits_second_call(self):
        path = self._png("cache.png")
        first = preview_module.build_image_preview(path)
        second = preview_module.build_image_preview(path)
        self.assertEqual(first.get("data_url"), second.get("data_url"))

    def test_preview_smart_isolates_svg(self):
        svg = Path(self.temp_dir.name) / "x.svg"
        svg.write_text('<svg width="10" height="10" xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10" fill="#0f0"/></svg>', encoding="utf-8")
        with mock.patch.object(preview_module, "build_image_preview_isolated", return_value={"success": True, "data_url": "data:image/jpeg;base64,abc"}) as isolated:
            result = preview_module.build_image_preview_smart(str(svg))
            isolated.assert_called_once()
        self.assertTrue(result.get("success"))


if __name__ == "__main__":
    unittest.main()
