import os
import sys
import tempfile
import unittest

from PIL import Image
from PIL import PngImagePlugin

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

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

    def test_gif_metadata(self):
        img = Image.new("RGB", (16, 16), (128, 128, 0))
        path = self._path("sample.gif")
        img.save(path, format="GIF", save_all=True, duration=123, loop=2, comment=b"UnitTest")

        info = InfoViewer().get_info(path)
        self.assertTrue(info.get("success"))

        self.assertEqual(info.get("format"), "GIF")
        self.assertEqual(info.get("width"), 16)
        self.assertEqual(info.get("height"), 16)

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


if __name__ == "__main__":
    unittest.main()
