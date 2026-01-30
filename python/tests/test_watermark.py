import os
import sys
import tempfile
import unittest

from PIL import Image

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from watermark import WatermarkApplier


class WatermarkTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name):
        return os.path.join(self.temp_dir.name, name)

    def test_text_watermark_invalid_color(self):
        src = self._path("base.png")
        Image.new("RGB", (24, 24), (120, 120, 120)).save(src, format="PNG")

        out = self._path("text_out.png")
        applier = WatermarkApplier()
        result = applier.apply(
            watermark_type="text",
            input_path=src,
            output_path=out,
            text="Test",
            font_color="ZZZZZZ",
        )
        self.assertTrue(result.get("success"))
        self.assertTrue(os.path.exists(out))

    def test_image_watermark_scale_clamped(self):
        src = self._path("base2.png")
        Image.new("RGB", (40, 40), (200, 200, 200)).save(src, format="PNG")

        watermark_path = self._path("wm.png")
        Image.new("RGBA", (8, 8), (255, 0, 0, 128)).save(watermark_path, format="PNG")

        out = self._path("img_out.png")
        applier = WatermarkApplier()
        result = applier.apply(
            watermark_type="image",
            input_path=src,
            output_path=out,
            watermark_path=watermark_path,
            watermark_scale=0,
        )
        self.assertTrue(result.get("success"))
        self.assertTrue(os.path.exists(out))


if __name__ == "__main__":
    unittest.main()
