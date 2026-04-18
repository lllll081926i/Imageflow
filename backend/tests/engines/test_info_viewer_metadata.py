import os
import sys
import tempfile
import unittest
from pathlib import Path

from PIL import Image
from PIL import PngImagePlugin

ENGINE_DIR = Path(__file__).resolve().parents[2] / "engines"
if str(ENGINE_DIR) not in sys.path:
    sys.path.insert(0, str(ENGINE_DIR))

from info_viewer import InfoViewer


class InfoViewerMetadataTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name):
        return os.path.join(self.temp_dir.name, name)

    def test_jpeg_exif_metadata(self):
        img = Image.new("RGB", (16, 16), (255, 0, 0))
        exif = Image.Exif()
        exif[0x010F] = "UnitTestMake"
        exif[0x0131] = "UnitTestSoftware"

        path = self._path("sample.jpg")
        img.save(path, format="JPEG", exif=exif)

        info = InfoViewer().get_info(path)
        self.assertTrue(info.get("success"))

        meta = info.get("metadata", {})
        piexif_meta = meta.get("piexif", {})
        self.assertEqual(piexif_meta.get("0th:Make"), "UnitTestMake")

        exifread_meta = meta.get("exifread", {})
        self.assertTrue(any("Make" in k and "UnitTestMake" in v for k, v in exifread_meta.items()))
        basic = info.get("basic", {})
        self.assertEqual(basic.get("format"), "JPEG")
        self.assertEqual(basic.get("width"), 16)
        self.assertEqual(basic.get("height"), 16)
        fields = info.get("fields", [])
        self.assertTrue(any(field.get("key") == "basic.format" and field.get("value") == "JPEG" for field in fields))
        self.assertTrue(any(field.get("source") == "piexif" and field.get("value") == "UnitTestMake" for field in fields))
        self.assertIsInstance(info.get("warnings", []), list)

    def test_png_text_metadata(self):
        img = Image.new("RGB", (16, 16), (0, 255, 0))
        pnginfo = PngImagePlugin.PngInfo()
        pnginfo.add_text("Author", "UnitTest")

        path = self._path("sample.png")
        img.save(path, format="PNG", pnginfo=pnginfo, dpi=(300, 300))

        info = InfoViewer().get_info(path)
        self.assertTrue(info.get("success"))

        extra = info.get("metadata", {}).get("extra", {})
        self.assertEqual(extra.get("PNG:Text:Author"), "UnitTest")
        self.assertIn("PNG:DPI", extra)
        basic = info.get("basic", {})
        self.assertEqual(basic.get("format"), "PNG")
        self.assertEqual(basic.get("width"), 16)
        self.assertEqual(basic.get("height"), 16)
        format_details = info.get("format_details", {})
        self.assertIn("png.color_type", format_details)
        self.assertTrue(any(field.get("group") == "png_text" and field.get("value") == "UnitTest" for field in info.get("fields", [])))

    def test_tiff_tags(self):
        img = Image.new("RGB", (16, 16), (0, 0, 255))
        tiffinfo = {
            270: "UnitTestDescription",
            305: "UnitTestSoftware",
        }

        path = self._path("sample.tiff")
        img.save(path, format="TIFF", tiffinfo=tiffinfo)

        info = InfoViewer().get_info(path)
        self.assertTrue(info.get("success"))

        flat = info.get("exif", {})
        self.assertIn("UnitTestDescription", flat.values())
        self.assertIn("UnitTestSoftware", flat.values())
        basic = info.get("basic", {})
        self.assertEqual(basic.get("format"), "TIFF")

    def test_gif_metadata(self):
        img = Image.new("RGB", (16, 16), (128, 128, 0))
        path = self._path("sample.gif")
        img.save(path, format="GIF", save_all=True, duration=123, loop=2, comment=b"UnitTest")

        info = InfoViewer().get_info(path)
        self.assertTrue(info.get("success"))

        self.assertEqual(info.get("format"), "GIF")
        self.assertEqual(info.get("width"), 16)
        self.assertEqual(info.get("height"), 16)
        basic = info.get("basic", {})
        self.assertTrue(basic.get("is_animated"))
        self.assertGreaterEqual(basic.get("frame_count", 0), 1)
        format_details = info.get("format_details", {})
        self.assertEqual(format_details.get("gif.loop_count"), "2")
        self.assertTrue(any(field.get("group") == "gif" and field.get("key") == "gif.comment" for field in info.get("fields", [])))

    def test_svg_metadata(self):
        path = self._path("sample.svg")
        with open(path, "w", encoding="utf-8") as f:
            f.write(
                """<svg xmlns="http://www.w3.org/2000/svg" width="120" height="80" viewBox="0 0 120 80">
                <title>Unit SVG</title>
                <desc>Structured metadata</desc>
                <rect width="120" height="80" fill="red" />
                </svg>"""
            )

        info = InfoViewer().get_info(path)
        self.assertTrue(info.get("success"))
        basic = info.get("basic", {})
        self.assertEqual(basic.get("format"), "SVG")
        self.assertEqual(basic.get("width"), 120)
        self.assertEqual(basic.get("height"), 80)
        format_details = info.get("format_details", {})
        self.assertEqual(format_details.get("svg.view_box"), "0 0 120 80")
        self.assertTrue(any(field.get("group") == "svg" and field.get("key") == "svg.title" for field in info.get("fields", [])))

    def test_svg_rejects_unsafe_doctype_but_keeps_basic_dimensions(self):
        path = self._path("unsafe.svg")
        with open(path, "w", encoding="utf-8") as f:
            f.write(
                """<?xml version="1.0" encoding="UTF-8"?>
                <!DOCTYPE svg [
                    <!ENTITY xxe "boom">
                ]>
                <svg xmlns="http://www.w3.org/2000/svg" width="120" height="80" viewBox="0 0 120 80">
                    <title>&xxe;</title>
                    <rect width="120" height="80" fill="red" />
                </svg>"""
            )

        info = InfoViewer().get_info(path)
        self.assertTrue(info.get("success"))

        basic = info.get("basic", {})
        self.assertEqual(basic.get("format"), "SVG")
        self.assertEqual(basic.get("width"), 120)
        self.assertEqual(basic.get("height"), 80)

        warnings = info.get("warnings", [])
        self.assertTrue(any(item.get("code") == "SVG_UNSAFE_XML" for item in warnings))

    def test_edit_metadata(self):
        img = Image.new("RGB", (16, 16), (10, 20, 30))
        exif = Image.Exif()
        exif[0x010F] = "OriginalMake"
        src = self._path("edit_src.jpg")
        out = self._path("edit_out.jpg")
        img.save(src, format="JPEG", exif=exif)

        viewer = InfoViewer()
        result = viewer.edit_exif(src, out, {"0th:Make": "NewMake"}, overwrite=False)
        self.assertTrue(result.get("success"))

        info = viewer.get_info(out)
        self.assertTrue(info.get("success"))
        piexif_meta = info.get("metadata", {}).get("piexif", {})
        self.assertEqual(piexif_meta.get("0th:Make"), "NewMake")

    def test_avif_brand_is_normalized_to_avif_format(self):
        sample = (
            Path(__file__).resolve().parents[3]
            / "test"
            / "avif"
            / "animated-avif-12-frames-hq-lossy.avif"
        )
        if not sample.exists():
            self.skipTest("AVIF sample not found")

        info = InfoViewer().get_info(str(sample))
        self.assertTrue(info.get("success"))
        self.assertEqual(info.get("format"), "AVIF")

        basic = info.get("basic", {})
        self.assertEqual(basic.get("format"), "AVIF")
        self.assertTrue(basic.get("is_animated"))

        format_details = info.get("format_details", {})
        self.assertEqual(format_details.get("avif.major_brand"), "avis")
        self.assertEqual(format_details.get("avif.primary_size"), "480x360")


if __name__ == "__main__":
    unittest.main()
