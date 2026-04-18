import os
import sys
import tempfile
import unittest
from pathlib import Path

from PIL import Image, ImageChops

ENGINE_DIR = Path(__file__).resolve().parents[2] / "engines"
if str(ENGINE_DIR) not in sys.path:
    sys.path.insert(0, str(ENGINE_DIR))

from adjuster import ImageAdjuster
from filter import ImageFilterApplier


class FilterPresetTests(unittest.TestCase):
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

    def _make_base(self, name):
        path = self._path(name)
        img = Image.new("RGB", (32, 32), (64, 128, 192))
        img.save(path, format="PNG")
        return path

    def _assert_same(self, p1, p2):
        with Image.open(p1) as a, Image.open(p2) as b:
            diff = ImageChops.difference(a.convert("RGB"), b.convert("RGB"))
            self.assertIsNone(diff.getbbox())

    def _assert_diff(self, p1, p2):
        with Image.open(p1) as a, Image.open(p2) as b:
            diff = ImageChops.difference(a.convert("RGB"), b.convert("RGB"))
            self.assertIsNotNone(diff.getbbox())

    def test_film_intensity_zero_no_change(self):
        src = self._make_base("film_base.png")
        out = self._path("film_out.png")
        applier = ImageFilterApplier()
        result = applier.apply(
            input_path=src,
            output_path=out,
            filter_name="film",
            intensity=0.0,
        )
        self.assertTrue(result.get("success"))
        self._assert_same(src, out)

    def test_polaroid_intensity_zero_no_change(self):
        src = self._make_base("polaroid_base.png")
        out = self._path("polaroid_out.png")
        applier = ImageFilterApplier()
        result = applier.apply(
            input_path=src,
            output_path=out,
            filter_name="polaroid",
            intensity=0.0,
        )
        self.assertTrue(result.get("success"))
        self._assert_same(src, out)

    def test_grain_changes_image(self):
        src = self._make_base("grain_base.png")
        out = self._path("grain_out.png")
        applier = ImageFilterApplier()
        result = applier.apply(
            input_path=src,
            output_path=out,
            filter_name="none",
            intensity=0.0,
            grain=0.6,
        )
        self.assertTrue(result.get("success"))
        self._assert_diff(src, out)

    def test_vignette_changes_image(self):
        src = self._make_base("vignette_base.png")
        out = self._path("vignette_out.png")
        applier = ImageFilterApplier()
        result = applier.apply(
            input_path=src,
            output_path=out,
            filter_name="none",
            intensity=0.0,
            vignette=0.6,
        )
        self.assertTrue(result.get("success"))
        self._assert_diff(src, out)

    def test_filter_respects_output_extension_format(self):
        src = self._path("filter_source.jpg")
        Image.new("RGB", (32, 24), (180, 120, 60)).save(src, format="JPEG")

        out = self._path("filter_out.png")
        applier = ImageFilterApplier()
        result = applier.apply(
            input_path=src,
            output_path=out,
            filter_name="warm",
            intensity=0.5,
        )

        self.assertTrue(result.get("success"))
        with Image.open(out) as img:
            self.assertEqual(img.format, "PNG")

    def test_adjuster_respects_output_extension_format(self):
        src = self._path("adjust_source.jpg")
        Image.new("RGB", (24, 32), (50, 100, 150)).save(src, format="JPEG")

        out = self._path("adjust_out.png")
        adjuster = ImageAdjuster()
        result = adjuster.adjust(
            input_path=src,
            output_path=out,
            rotate=90,
        )

        self.assertTrue(result.get("success"))
        with Image.open(out) as img:
            self.assertEqual(img.format, "PNG")

    def test_adjuster_accepts_svg_input(self):
        out = self._path("adjust_svg.png")
        adjuster = ImageAdjuster()
        result = adjuster.adjust(
            input_path=self._svg_path(),
            output_path=out,
            rotate=90,
        )

        self.assertTrue(result.get("success"))
        with Image.open(out) as img:
            self.assertEqual(img.format, "PNG")
            self.assertGreater(img.size[0], 0)
            self.assertGreater(img.size[1], 0)

    def test_filter_accepts_svg_input(self):
        out = self._path("filter_svg.png")
        applier = ImageFilterApplier()
        result = applier.apply(
            input_path=self._svg_path(),
            output_path=out,
            filter_name="bw",
            intensity=1.0,
        )

        self.assertTrue(result.get("success"))
        with Image.open(out) as img:
            self.assertEqual(img.format, "PNG")
            self.assertGreater(img.size[0], 0)
            self.assertGreater(img.size[1], 0)


if __name__ == "__main__":
    unittest.main()
