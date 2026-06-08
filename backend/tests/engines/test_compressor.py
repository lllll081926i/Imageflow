import os
import sys
import tempfile
import unittest
from io import BytesIO
from pathlib import Path

from PIL import Image

ENGINE_DIR = Path(__file__).resolve().parents[2] / "engines"
if str(ENGINE_DIR) not in sys.path:
    sys.path.insert(0, str(ENGINE_DIR))

import compressor
from compressor import CompressionLevel, ImageCompressor
from compressor import _copy_jpeg_without_metadata


class BoundedReadBytesIO(BytesIO):
    def read(self, size=-1):
        if size is None or size < 0:
            raise AssertionError("JPEG metadata stripping must not read the entire file at once")
        return super().read(size)


class CompressorTests(unittest.TestCase):
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

    def test_jpeg_metadata_stripping_streams_without_unbounded_read(self):
        # SOI + APP1(EXIF) + COM + APP0(kept) + SOS + image bytes + EOI.
        jpeg_bytes = (
            b"\xff\xd8"
            b"\xff\xe1\x00\x08Exifxx"
            b"\xff\xfe\x00\x07note!"
            b"\xff\xe0\x00\x06JFIF"
            b"\xff\xda\x00\x04AB"
            b"\x11\x22\xff\xd9"
        )
        src = BoundedReadBytesIO(jpeg_bytes)
        dst = BytesIO()

        _copy_jpeg_without_metadata(src, dst)

        stripped = dst.getvalue()
        self.assertTrue(stripped.startswith(b"\xff\xd8"))
        self.assertNotIn(b"Exifxx", stripped)
        self.assertNotIn(b"note!", stripped)
        self.assertIn(b"JFIF", stripped)
        self.assertIn(b"\xff\xda\x00\x04AB\x11\x22\xff\xd9", stripped)

    def test_compress_closes_open_image_when_engine_errors(self):
        src = self._path("input.png")
        with open(src, "wb") as handle:
            handle.write(b"fake png")

        class FakeImage:
            format = "PNG"
            mode = "RGB"

            def __init__(self):
                self.closed = False

            def close(self):
                self.closed = True

        opened_image = FakeImage()
        original_open = compressor.Image.open
        original_compress_png = ImageCompressor._compress_png
        original_logger_disabled = compressor.logger.disabled

        def fail_compress_png(self, *_args, **_kwargs):
            raise RuntimeError("forced compression failure")

        try:
            compressor.Image.open = lambda _path: opened_image
            ImageCompressor._compress_png = fail_compress_png
            compressor.logger.disabled = True

            result = ImageCompressor().compress(
                input_path=src,
                output_path=self._path("out.png"),
                level=CompressionLevel.MEDIUM,
            )

            self.assertFalse(result.get("success"))
            self.assertTrue(opened_image.closed)
        finally:
            compressor.Image.open = original_open
            ImageCompressor._compress_png = original_compress_png
            compressor.logger.disabled = original_logger_disabled

    def test_compress_rejects_svg_input_with_unsupported_format(self):
        src = self._path("vector.svg")
        with open(src, "w", encoding="utf-8") as handle:
            handle.write(
                '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16">'
                '<rect width="16" height="16" fill="red"/></svg>'
            )

        result = ImageCompressor().compress(
            input_path=src,
            output_path=self._path("vector.svg"),
            level=CompressionLevel.MEDIUM,
        )

        self.assertFalse(result.get("success"))
        self.assertTrue(str(result.get("error", "")).startswith("[UNSUPPORTED_FORMAT]"))

    def test_compress_rejects_gif_input_with_unsupported_format(self):
        src = self._path("animated.gif")
        Image.new("RGB", (8, 8), (255, 0, 0)).save(src, format="GIF")

        result = ImageCompressor().compress(
            input_path=src,
            output_path=self._path("animated.gif"),
            level=CompressionLevel.MEDIUM,
        )

        self.assertFalse(result.get("success"))
        self.assertTrue(str(result.get("error", "")).startswith("[UNSUPPORTED_FORMAT]"))

    def test_fallback_compression_keeps_original_when_result_is_larger(self):
        src = self._path("icon.ico")
        out = self._path("icon_compressed.ico")
        Image.new("RGBA", (64, 64), (255, 0, 0, 255)).save(
            src,
            format="ICO",
            sizes=[(16, 16), (32, 32), (64, 64)],
        )
        original_size = os.path.getsize(src)

        result = ImageCompressor().compress(
            input_path=src,
            output_path=out,
            level=CompressionLevel.MEDIUM,
        )

        self.assertTrue(result.get("success"))
        self.assertEqual(os.path.getsize(out), original_size)
        self.assertEqual(result.get("compressed_size"), original_size)
        self.assertEqual(result.get("compression_rate"), 0.0)
        self.assertIn("保留原文件", str(result.get("warning", "")))

    def test_fallback_compression_keeps_original_when_overwriting_larger_result(self):
        src = self._path("icon.ico")
        Image.new("RGBA", (64, 64), (255, 0, 0, 255)).save(
            src,
            format="ICO",
            sizes=[(16, 16), (32, 32), (64, 64)],
        )
        original_bytes = Path(src).read_bytes()

        result = ImageCompressor().compress(
            input_path=src,
            output_path=src,
            level=CompressionLevel.MEDIUM,
        )

        self.assertTrue(result.get("success"))
        self.assertEqual(Path(src).read_bytes(), original_bytes)
        self.assertEqual(result.get("compressed_size"), len(original_bytes))
        self.assertIn("保留原文件", str(result.get("warning", "")))

    def test_fallback_compression_keeps_original_when_strip_metadata_requested(self):
        src = self._path("icon.ico")
        out = self._path("icon_compressed.ico")
        Image.new("RGBA", (64, 64), (255, 0, 0, 255)).save(
            src,
            format="ICO",
            sizes=[(16, 16), (32, 32), (64, 64)],
        )
        original_bytes = Path(src).read_bytes()

        result = ImageCompressor().compress(
            input_path=src,
            output_path=out,
            level=CompressionLevel.MEDIUM,
            strip_metadata=True,
        )

        self.assertTrue(result.get("success"))
        self.assertEqual(Path(out).read_bytes(), original_bytes)
        self.assertEqual(result.get("compressed_size"), len(original_bytes))
        self.assertIn("保留原文件", str(result.get("warning", "")))

    def test_pngquant_rgb_path_closes_converted_and_quantized_images(self):
        class FakeImage:
            mode = "RGB"

            def __init__(self):
                self.converted = None

            def convert(self, mode):
                self.converted = FakeConvertedImage(mode)
                return self.converted

        class FakeConvertedImage:
            def __init__(self, mode):
                self.mode = mode
                self.closed = False

            def close(self):
                self.closed = True

        class FakeQuantizedImage:
            def __init__(self):
                self.closed = False

            def save(self, path, **_kwargs):
                with open(path, "wb") as handle:
                    handle.write(b"fake png")

            def close(self):
                self.closed = True

        source = FakeImage()
        quantized = FakeQuantizedImage()
        original_imagequant = getattr(compressor, "imagequant", None)
        had_imagequant = hasattr(compressor, "imagequant")

        class FakeImageQuant:
            @staticmethod
            def quantize_pil_image(*_args, **_kwargs):
                return quantized

        try:
            compressor.imagequant = FakeImageQuant()
            png_compressor = ImageCompressor()
            png_compressor.imagequant_available = True
            png_compressor.oxipng_available = False

            png_compressor._compress_png(
                source,
                input_path=self._path("input.png"),
                output_path=self._path("out.png"),
                level=CompressionLevel.MEDIUM,
                engine="pngquant",
            )

            self.assertIsNotNone(source.converted)
            self.assertTrue(source.converted.closed)
            self.assertTrue(quantized.closed)
        finally:
            if had_imagequant:
                compressor.imagequant = original_imagequant
            else:
                delattr(compressor, "imagequant")

    def test_pillow_png_quantize_path_closes_quantized_image(self):
        class FakeImage:
            mode = "RGBA"

            def __init__(self):
                self.quantized = None

            def quantize(self, **_kwargs):
                self.quantized = FakeQuantizedImage()
                return self.quantized

        class FakeQuantizedImage:
            def __init__(self):
                self.closed = False

            def save(self, path, **_kwargs):
                with open(path, "wb") as handle:
                    handle.write(b"fake png")

            def close(self):
                self.closed = True

        source = FakeImage()
        png_compressor = ImageCompressor()
        png_compressor.imagequant_available = False
        png_compressor.oxipng_available = False

        png_compressor._compress_png(
            source,
            input_path=self._path("input.png"),
            output_path=self._path("out.png"),
            level=CompressionLevel.MEDIUM,
            engine="pillow",
        )

        self.assertIsNotNone(source.quantized)
        self.assertTrue(source.quantized.closed)


if __name__ == "__main__":
    unittest.main()
