import os
import sys
import tempfile
import unittest
from pathlib import Path

from PIL import Image

ENGINE_DIR = Path(__file__).resolve().parents[2] / "engines"
if str(ENGINE_DIR) not in sys.path:
    sys.path.insert(0, str(ENGINE_DIR))

from metadata_tool import process


class MetadataToolStripTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name):
        return os.path.join(self.temp_dir.name, name)

    def _create_jpeg_with_exif(self, path, make="UnitTestMake"):
        img = Image.new("RGB", (16, 16), (20, 40, 60))
        exif = Image.Exif()
        exif[0x010F] = make
        img.save(path, format="JPEG", exif=exif)

    def test_process_creates_stripped_copy_when_overwrite_disabled(self):
        src = self._path("input.jpg")
        out = self._path("output.jpg")
        self._create_jpeg_with_exif(src)

        result = process({
            "action": "strip_metadata",
            "input_path": src,
            "output_path": out,
            "overwrite": False,
        })

        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("input_path"), src)
        self.assertEqual(result.get("output_path"), out)

        with Image.open(src) as src_img:
            self.assertEqual(src_img.getexif().get(0x010F), "UnitTestMake")
        with Image.open(out) as out_img:
            self.assertIsNone(out_img.getexif().get(0x010F))

    def test_process_overwrite_true_allows_in_place_strip_without_output_path(self):
        src = self._path("overwrite.jpg")
        self._create_jpeg_with_exif(src, make="OverwriteMake")

        result = process({
            "action": "strip_metadata",
            "input_path": src,
            "overwrite": True,
        })

        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("input_path"), src)
        self.assertEqual(result.get("output_path"), src)

        with Image.open(src) as stripped:
            self.assertIsNone(stripped.getexif().get(0x010F))

    def test_process_requires_output_path_when_not_overwriting(self):
        src = self._path("missing-output.jpg")
        self._create_jpeg_with_exif(src)

        result = process({
            "action": "strip_metadata",
            "input_path": src,
            "overwrite": False,
        })

        self.assertFalse(result.get("success"))
        self.assertEqual(result.get("error"), "[BAD_INPUT] missing output_path")


if __name__ == "__main__":
    unittest.main()
