import os
import sys
import tempfile
import unittest
from pathlib import Path

from PIL import Image

ENGINE_DIR = Path(__file__).resolve().parents[2] / "engines"
if str(ENGINE_DIR) not in sys.path:
    sys.path.insert(0, str(ENGINE_DIR))

from watermark import WatermarkApplier, process as watermark_process


class WatermarkTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name):
        return os.path.join(self.temp_dir.name, name)

    def _svg_path(self):
        return os.path.abspath(
            os.path.join(os.path.dirname(__file__), "..", "..", "testdata", "simple.svg")
        )

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

    def test_image_watermark_requires_existing_watermark_path(self):
        src = self._path("base3.png")
        Image.new("RGB", (24, 24), (40, 80, 120)).save(src, format="PNG")

        out = self._path("missing_out.png")
        applier = WatermarkApplier()
        result = applier.apply(
            watermark_type="image",
            input_path=src,
            output_path=out,
            watermark_path=self._path("missing.png"),
        )

        self.assertFalse(result.get("success"))
        self.assertIn("File not found:", result.get("error", ""))
        self.assertFalse(os.path.exists(out))

    def test_text_watermark_accepts_svg_input(self):
        out = self._path("svg_watermark.png")
        applier = WatermarkApplier()
        result = applier.apply(
            watermark_type="text",
            input_path=self._svg_path(),
            output_path=out,
            text="SVG",
            font_color="#FFFFFF",
        )

        self.assertTrue(result.get("success"))
        with Image.open(out) as img:
            self.assertEqual(img.format, "PNG")
            self.assertGreater(img.size[0], 0)
            self.assertGreater(img.size[1], 0)

    def test_watermark_uses_output_extension_for_jpeg(self):
        src = self._path("alpha.png")
        Image.new("RGBA", (24, 24), (120, 80, 40, 128)).save(src, format="PNG")

        out = self._path("watermark.jpg")
        applier = WatermarkApplier()
        result = applier.apply(
            watermark_type="text",
            input_path=src,
            output_path=out,
            text="JPEG",
            opacity=0.5,
        )

        self.assertTrue(result.get("success"))
        with Image.open(out) as img:
            self.assertEqual(img.format, "JPEG")

    def test_process_accepts_frontend_watermark_payload_keys(self):
        src = self._path("frontend_base.png")
        Image.new("RGB", (32, 32), (80, 120, 160)).save(src, format="PNG")

        out = self._path("frontend_watermark.png")
        result = watermark_process(
            {
                "input_path": src,
                "output_path": out,
                "watermark_type": "text",
                "text": "ImageFlow",
                "font_name": "arial",
                "scale": 0.4,
                "opacity": 0.7,
                "position": "center",
            }
        )

        self.assertTrue(result.get("success"))
        self.assertTrue(os.path.exists(out))


if __name__ == "__main__":
    unittest.main()
