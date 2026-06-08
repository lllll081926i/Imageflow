import os
import sys
import tempfile
import unittest
import xml.etree.ElementTree as stdlib_et
from pathlib import Path

from PIL import Image

ENGINE_DIR = Path(__file__).resolve().parents[2] / "engines"
if str(ENGINE_DIR) not in sys.path:
    sys.path.insert(0, str(ENGINE_DIR))

import converter
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

    def test_svg_intrinsic_size_parser_does_not_require_xml_parser(self):
        original_fromstring = stdlib_et.fromstring

        def fail_if_xml_parser_is_used(_data):
            raise AssertionError("XML parser should not be used for SVG dimensions")

        try:
            stdlib_et.fromstring = fail_if_xml_parser_is_used

            size = converter.parse_svg_intrinsic_size_from_bytes(
                b'<svg width="120" height="80" viewBox="0 0 120 80" xmlns="http://www.w3.org/2000/svg"></svg>'
            )

            self.assertEqual(size, (120, 80))
        finally:
            stdlib_et.fromstring = original_fromstring

    def test_svg_intrinsic_size_probe_does_not_read_entire_file(self):
        svg_path = self._path("large.svg")
        with open(svg_path, "w", encoding="utf-8") as handle:
            handle.write('<svg width="120" height="80" xmlns="http://www.w3.org/2000/svg">')
            handle.write(" " * 1024 * 1024)
            handle.write("</svg>")

        original_read_bytes = Path.read_bytes

        def fail_if_full_file_is_read(_path):
            raise AssertionError("SVG size probe should not read the whole file")

        try:
            Path.read_bytes = fail_if_full_file_is_read
            size = converter.ImageConverter()._parse_svg_intrinsic_size(svg_path)
            self.assertEqual(size, (120, 80))
        finally:
            Path.read_bytes = original_read_bytes


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

    def test_single_size_ico_adds_size_suffix_to_output_path(self):
        src = self._path("source.png")
        Image.new("RGBA", (64, 64), (10, 120, 220, 255)).save(src, format="PNG")
        requested_out = self._path("icon.ico")
        expected_out = self._path("icon_ico16.ico")

        result = convert_process(
            {
                "input_path": src,
                "output_path": requested_out,
                "format": "ico",
                "ico_sizes": [16],
            }
        )

        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("output_path"), expected_out)
        self.assertFalse(os.path.exists(requested_out))
        with Image.open(expected_out) as img:
            self.assertEqual(sorted(img.info.get("sizes", [])), [(16, 16)])

    def test_single_size_ico_does_not_duplicate_existing_size_suffix(self):
        src = self._path("source.png")
        Image.new("RGBA", (64, 64), (10, 120, 220, 255)).save(src, format="PNG")
        requested_out = self._path("icon_ico32.ico")

        result = convert_process(
            {
                "input_path": src,
                "output_path": requested_out,
                "format": "ico",
                "ico_sizes": [32],
            }
        )

        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("output_path"), requested_out)
        self.assertTrue(os.path.exists(requested_out))
        self.assertFalse(os.path.exists(self._path("icon_ico32_ico32.ico")))

    def test_single_size_ico_replaces_mismatched_existing_size_suffix(self):
        src = self._path("source.png")
        Image.new("RGBA", (64, 64), (10, 120, 220, 255)).save(src, format="PNG")
        requested_out = self._path("icon_ico32.ico")
        expected_out = self._path("icon_ico16.ico")

        result = convert_process(
            {
                "input_path": src,
                "output_path": requested_out,
                "format": "ico",
                "ico_sizes": [16],
            }
        )

        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("output_path"), expected_out)
        self.assertTrue(os.path.exists(expected_out))
        self.assertFalse(os.path.exists(requested_out))


class ConversionResourceTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name):
        return os.path.join(self.temp_dir.name, name)

    def test_resize_conversion_closes_replaced_source_image(self):
        class FakeImage:
            def __init__(self, size=(100, 50)):
                self.size = size
                self.mode = "RGB"
                self.info = {}
                self.closed = False
                self.resized = None

            def load(self):
                return None

            def resize(self, size, _resample):
                self.resized = FakeImage(size)
                return self.resized

            def save(self, path, **_kwargs):
                with open(path, "wb") as handle:
                    handle.write(b"fake image")

            def close(self):
                self.closed = True

        source = FakeImage()
        original_open = converter.Image.open

        def fake_open(_path):
            return source

        try:
            converter.Image.open = fake_open
            output_path = self._path("out.png")

            result = converter.ImageConverter().convert(
                input_path="virtual-input.png",
                output_path=output_path,
                format_type="png",
                width=50,
                height=0,
                maintain_ar=True,
            )

            self.assertTrue(result.get("success"))
            self.assertTrue(os.path.exists(output_path))
            self.assertTrue(source.closed)
            self.assertIsNotNone(source.resized)
            self.assertTrue(source.resized.closed)
        finally:
            converter.Image.open = original_open


if __name__ == "__main__":
    unittest.main()
