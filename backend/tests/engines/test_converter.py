import os
import sys
import tempfile
import unittest
from pathlib import Path

from PIL import Image

ENGINE_DIR = Path(__file__).resolve().parents[2] / "engines"
if str(ENGINE_DIR) not in sys.path:
    sys.path.insert(0, str(ENGINE_DIR))

from converter import process as convert_process


class SVGConversionTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name):
        return os.path.join(self.temp_dir.name, name)

    def test_svg_resize_mode_percent_affects_output_dimensions(self):
        svg_path = os.path.abspath(
            os.path.join(os.path.dirname(__file__), "..", "..", "testdata", "simple.svg")
        )
        output_path = self._path("simple.png")

        result = convert_process(
            {
                "input_path": svg_path,
                "output_path": output_path,
                "format": "png",
                "resize_mode": "percent",
                "scale_percent": 50,
                "maintain_ar": True,
            }
        )

        self.assertTrue(result.get("success"))
        with Image.open(output_path) as img:
            self.assertEqual(img.size, (64, 64))

    def test_svg_resize_modes_long_edge_and_fixed_are_applied(self):
        svg_path = os.path.abspath(
            os.path.join(os.path.dirname(__file__), "..", "..", "testdata", "simple.svg")
        )

        long_edge_output = self._path("long-edge.png")
        long_edge_result = convert_process(
            {
                "input_path": svg_path,
                "output_path": long_edge_output,
                "format": "png",
                "resize_mode": "long_edge",
                "long_edge": 256,
                "maintain_ar": True,
            }
        )
        self.assertTrue(long_edge_result.get("success"))
        with Image.open(long_edge_output) as img:
            self.assertEqual(img.size, (256, 256))

        fixed_output = self._path("fixed.png")
        fixed_result = convert_process(
            {
                "input_path": svg_path,
                "output_path": fixed_output,
                "format": "png",
                "resize_mode": "fixed",
                "width": 320,
                "height": 0,
                "maintain_ar": True,
            }
        )
        self.assertTrue(fixed_result.get("success"))
        with Image.open(fixed_output) as img:
            self.assertEqual(img.size, (320, 320))

    def test_svg_with_doctype_keeps_intrinsic_dimensions(self):
        svg_path = self._path("unsafe.svg")
        secret_path = self._path("secret.txt")
        output_path = self._path("unsafe.png")

        with open(secret_path, "w", encoding="utf-8") as handle:
            handle.write("SECRET")

        with open(svg_path, "w", encoding="utf-8") as handle:
            handle.write(
                f"""<?xml version="1.0"?>
<!DOCTYPE svg [
<!ENTITY ext SYSTEM "file:///{secret_path.replace(os.sep, '/')}">
]>
<svg width="120" height="80" xmlns="http://www.w3.org/2000/svg">
  <rect width="120" height="80" fill="#3366ff" />
  <text x="10" y="20">&ext;</text>
</svg>
"""
            )

        result = convert_process(
            {
                "input_path": svg_path,
                "output_path": output_path,
                "format": "png",
            }
        )

        self.assertTrue(result.get("success"))
        with Image.open(output_path) as img:
            self.assertEqual(img.size, (120, 80))

    def test_utf16_svg_with_external_entity_keeps_intrinsic_dimensions(self):
        svg_path = self._path("unsafe-utf16.svg")
        secret_path = self._path("secret.txt")
        output_path = self._path("unsafe-utf16.png")

        with open(secret_path, "w", encoding="utf-8") as handle:
            handle.write("SECRET")

        content = f"""<?xml version="1.0" encoding="UTF-16"?>
<!DOCTYPE svg [<!ENTITY ext SYSTEM "file:///{secret_path.replace(os.sep, '/')}">]>
<svg width="120" height="80" xmlns="http://www.w3.org/2000/svg">
  <rect width="120" height="80" fill="#3366ff" />
  <text x="10" y="20">&ext;</text>
</svg>
"""
        with open(svg_path, "w", encoding="utf-16") as handle:
            handle.write(content)

        result = convert_process(
            {
                "input_path": svg_path,
                "output_path": output_path,
                "format": "png",
            }
        )

        self.assertTrue(result.get("success"))
        with Image.open(output_path) as img:
            self.assertEqual(img.size, (120, 80))


class ICOConversionTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name):
        return os.path.join(self.temp_dir.name, name)

    def test_ico_defaults_to_multiple_common_sizes_for_large_source(self):
        src = self._path("source.png")
        Image.new("RGBA", (300, 180), (10, 120, 220, 255)).save(src, format="PNG")
        out = self._path("default.ico")

        result = convert_process(
            {
                "input_path": src,
                "output_path": out,
                "format": "ico",
            }
        )

        self.assertTrue(result.get("success"))
        with Image.open(out) as img:
            self.assertEqual(
                sorted(img.info.get("sizes", [])),
                [(16, 16), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)],
            )


if __name__ == "__main__":
    unittest.main()
