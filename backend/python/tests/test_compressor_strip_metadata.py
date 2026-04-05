import os
import sys
import tempfile
import unittest

from PIL import Image

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from compressor import CompressionLevel, ImageCompressor
from metadata_tool import process
from worker import process_command


class CompressorStripMetadataTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name):
        return os.path.join(self.temp_dir.name, name)

    def test_strip_metadata_flag_respected(self):
        src = self._path("input.jpg")
        img = Image.new("RGB", (16, 16), (255, 0, 0))
        exif = Image.Exif()
        exif[0x010F] = "UnitTestMake"
        img.save(src, format="JPEG", exif=exif)

        compressor = ImageCompressor()

        out_keep = self._path("out_keep.jpg")
        result = compressor.compress(
            input_path=src,
            output_path=out_keep,
            level=CompressionLevel.LOSSLESS,
            engine="pillow",
            strip_metadata=False,
        )
        self.assertTrue(result.get("success"))
        with Image.open(out_keep) as out_img:
            exif_out = out_img.getexif()
            self.assertEqual(exif_out.get(0x010F), "UnitTestMake")

        out_strip = self._path("out_strip.jpg")
        result = compressor.compress(
            input_path=src,
            output_path=out_strip,
            level=CompressionLevel.LOSSLESS,
            engine="pillow",
            strip_metadata=True,
        )
        self.assertTrue(result.get("success"))
        with Image.open(out_strip) as out_img:
            exif_out = out_img.getexif()
            self.assertIsNone(exif_out.get(0x010F))


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


class WorkerPathValidationTests(unittest.TestCase):
    def test_process_command_rejects_parent_traversal_path(self):
        result = process_command({
            "script": "converter.py",
            "input": {
                "input_path": "../secret.png",
                "output_path": "out.png",
                "format": "png",
            },
        })

        self.assertFalse(result.get("success"))
        self.assertIn("[BAD_INPUT]", result.get("error", ""))
        self.assertIn("父级目录", result.get("error", ""))

    def test_process_command_normalizes_safe_relative_paths(self):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)

        src = os.path.join(temp_dir.name, "input.jpg")
        Image.new("RGB", (8, 8), (120, 30, 40)).save(src, format="JPEG")

        cwd = os.getcwd()
        os.chdir(temp_dir.name)
        self.addCleanup(lambda: os.chdir(cwd))

        result = process_command({
            "script": "converter.py",
            "input": {
                "input_path": "input.jpg",
                "output_path": "output.png",
                "format": "png",
            },
        })

        self.assertTrue(result.get("success"))
        self.assertTrue(os.path.isabs(result.get("input_path", "")))
        self.assertTrue(os.path.isabs(result.get("output_path", "")))

    def test_process_command_does_not_expose_traceback_by_default(self):
        result = process_command({
            "script": "missing_script.py",
            "input": {
                "input_path": "input.png",
            },
        })

        self.assertFalse(result.get("success"))
        self.assertNotIn("traceback", result)


if __name__ == "__main__":
    unittest.main()
