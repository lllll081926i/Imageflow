"""
Regression tests for the merged image_processor engine.
Verifies that adjust and filter capabilities from the old adjuster+filter
engines still work correctly through ImageProcessor.
"""
import os
import sys
import tempfile
import unittest
from pathlib import Path

from PIL import Image

from backend.engines.image_processor import ImageProcessor
from backend.infrastructure.engine_loader import ensure_engine_scripts_path
ensure_engine_scripts_path()


class ImageProcessorAdjustTests(unittest.TestCase):
    """Adjustment tests migrated from test_adjuster."""

    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name):
        return os.path.join(self.temp_dir.name, name)

    def _create_image(self, path, size=(32, 32), mode="RGB"):
        img = Image.new(mode, size, (100, 150, 200))
        img.save(path)
        img.close()

    def test_rotate_90_degrees(self):
        src = self._path("rotate.jpg")
        out = self._path("out.jpg")
        self._create_image(src, (40, 20))
        proc = ImageProcessor()
        result = proc.process(src, out, rotate=90)
        self.assertTrue(result.get("success"))
        with Image.open(out) as img:
            self.assertEqual(img.size, (20, 40))

    def test_flip_horizontal(self):
        src = self._path("flip_h.jpg")
        out = self._path("out.jpg")
        self._create_image(src, (16, 16))
        proc = ImageProcessor()
        result = proc.process(src, out, flip_h=True)
        self.assertTrue(result.get("success"))

    def test_flip_vertical(self):
        src = self._path("flip_v.jpg")
        out = self._path("out.jpg")
        self._create_image(src, (16, 16))
        proc = ImageProcessor()
        result = proc.process(src, out, flip_v=True)
        self.assertTrue(result.get("success"))

    def test_brightness_contrast(self):
        src = self._path("bright.jpg")
        out = self._path("out.jpg")
        self._create_image(src)
        proc = ImageProcessor()
        result = proc.process(src, out, brightness=30, contrast=20)
        self.assertTrue(result.get("success"))

    def test_saturation_hue(self):
        src = self._path("color.jpg")
        out = self._path("out.jpg")
        self._create_image(src)
        proc = ImageProcessor()
        result = proc.process(src, out, saturation=20, hue=45)
        self.assertTrue(result.get("success"))

    def test_crop_16_9(self):
        src = self._path("crop.jpg")
        out = self._path("out.jpg")
        self._create_image(src, (80, 40))
        proc = ImageProcessor()
        result = proc.process(src, out, crop_ratio="16:9")
        self.assertTrue(result.get("success"))
        with Image.open(out) as img:
            w, h = img.size
            self.assertAlmostEqual(w / float(h), 16.0 / 9.0, delta=0.05)


class ImageProcessorFilterTests(unittest.TestCase):
    """Filter tests migrated from test_filter_presets / test_filter_safety."""

    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name):
        return os.path.join(self.temp_dir.name, name)

    def _create_image(self, path, size=(32, 32)):
        img = Image.new("RGB", size, (100, 150, 200))
        img.save(path)
        img.close()

    def test_preset_vivid(self):
        src = self._path("vivid.jpg")
        out = self._path("out.jpg")
        self._create_image(src)
        proc = ImageProcessor()
        result = proc.process(src, out, filter_name="vivid", intensity=0.8)
        self.assertTrue(result.get("success"))

    def test_preset_bw(self):
        src = self._path("bw.jpg")
        out = self._path("out.jpg")
        self._create_image(src)
        proc = ImageProcessor()
        result = proc.process(src, out, filter_name="bw", intensity=1.0)
        self.assertTrue(result.get("success"))

    def test_preset_retro(self):
        src = self._path("retro.jpg")
        out = self._path("out.jpg")
        self._create_image(src)
        proc = ImageProcessor()
        result = proc.process(src, out, filter_name="retro", intensity=0.7)
        self.assertTrue(result.get("success"))

    def test_advanced_blur(self):
        src = self._path("blur.jpg")
        out = self._path("out.jpg")
        self._create_image(src)
        proc = ImageProcessor()
        result = proc.process(src, out, filter_name="blur_gaussian", blur_radius=3.0)
        self.assertTrue(result.get("success"))

    def test_advanced_sharpen(self):
        src = self._path("sharpen.jpg")
        out = self._path("out.jpg")
        self._create_image(src)
        proc = ImageProcessor()
        result = proc.process(src, out, filter_name="sharpen", sharpen_factor=3.0)
        self.assertTrue(result.get("success"))

    def test_advanced_noise(self):
        src = self._path("noise.jpg")
        out = self._path("out.jpg")
        self._create_image(src)
        proc = ImageProcessor()
        result = proc.process(src, out, filter_name="noise", noise_level=0.3)
        self.assertTrue(result.get("success"))

    def test_advanced_vignette(self):
        src = self._path("vignette.jpg")
        out = self._path("out.jpg")
        self._create_image(src)
        proc = ImageProcessor()
        result = proc.process(src, out, filter_name="vignette", vignette_strength=0.6)
        self.assertTrue(result.get("success"))

    def test_advanced_color_offset(self):
        src = self._path("offset.jpg")
        out = self._path("out.jpg")
        self._create_image(src)
        proc = ImageProcessor()
        result = proc.process(src, out, filter_name="color_offset", color_offset_x=5, color_offset_y=3)
        self.assertTrue(result.get("success"))

    def test_unknown_filter_returns_error(self):
        src = self._path("unknown.jpg")
        out = self._path("out.jpg")
        self._create_image(src)
        proc = ImageProcessor()
        result = proc.process(src, out, filter_name="nonexistent_filter_xyz")
        self.assertFalse(result.get("success"))
        self.assertIn("Unknown filter", result.get("error", ""))

    def test_extra_grain_on_top(self):
        src = self._path("grain.jpg")
        out = self._path("out.jpg")
        self._create_image(src)
        proc = ImageProcessor()
        result = proc.process(src, out, filter_name="vivid", intensity=0.8, grain=0.3)
        self.assertTrue(result.get("success"))

    def test_extra_vignette_on_top(self):
        src = self._path("vignette_extra.jpg")
        out = self._path("out.jpg")
        self._create_image(src)
        proc = ImageProcessor()
        result = proc.process(src, out, filter_name="vivid", intensity=0.5, vignette=0.4)
        self.assertTrue(result.get("success"))


class ImageProcessorCombinedTests(unittest.TestCase):
    """Test that adjustments + filter work together."""

    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name):
        return os.path.join(self.temp_dir.name, name)

    def test_adjust_then_filter(self):
        src = self._path("combined.jpg")
        out = self._path("out.jpg")
        img = Image.new("RGB", (64, 64), (100, 150, 200))
        img.save(src)
        img.close()
        proc = ImageProcessor()
        result = proc.process(src, out, rotate=90, contrast=15, filter_name="vivid", intensity=0.7)
        self.assertTrue(result.get("success"))
        with Image.open(out) as loaded:
            self.assertEqual(loaded.size, (64, 64))


if __name__ == "__main__":
    unittest.main()