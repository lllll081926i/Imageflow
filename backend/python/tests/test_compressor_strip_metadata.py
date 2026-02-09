import os
import sys
import tempfile
import unittest

from PIL import Image

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from compressor import CompressionLevel, ImageCompressor


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


if __name__ == "__main__":
    unittest.main()
