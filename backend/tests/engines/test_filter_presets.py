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
import filter as filter_engine
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

    def test_filter_closes_replaced_source_image(self):
        class FakeImage:
            format = "PNG"
            mode = "RGB"
            size = (24, 24)

            def __init__(self):
                self.closed = False

            def save(self, path, **_kwargs):
                with open(path, "wb") as handle:
                    handle.write(b"fake image")

            def close(self):
                self.closed = True

        source = FakeImage()
        filtered = FakeImage()
        original_open = filter_engine.open_image_with_svg_support
        original_apply_basic = ImageFilterApplier._apply_basic_filter

        def fake_open(*_args, **_kwargs):
            return source

        def fake_basic(_applier, img, filter_name, intensity):
            self.assertIs(img, source)
            self.assertEqual(filter_name, "grayscale")
            return filtered

        try:
            filter_engine.open_image_with_svg_support = fake_open
            ImageFilterApplier._apply_basic_filter = fake_basic

            out = self._path("fake_filter.png")
            result = ImageFilterApplier().apply(
                input_path="virtual-input.png",
                output_path=out,
                filter_name="grayscale",
                intensity=0.5,
            )

            self.assertTrue(result.get("success"))
            self.assertTrue(os.path.exists(out))
            self.assertTrue(source.closed)
            self.assertTrue(filtered.closed)
        finally:
            filter_engine.open_image_with_svg_support = original_open
            ImageFilterApplier._apply_basic_filter = original_apply_basic

    def test_motion_blur_closes_intermediate_images(self):
        created_filters = []

        class FakeImage:
            def __init__(self, name):
                self.name = name
                self.closed = False

            def filter(self, _filter):
                filtered = FakeImage(f"{self.name}-filtered")
                created_filters.append(filtered)
                return filtered

            def close(self):
                self.closed = True

        source = FakeImage("source")
        blended = FakeImage("blended")
        original_blend = filter_engine.Image.blend

        def fake_blend(_left, _right, _alpha):
            return blended

        try:
            filter_engine.Image.blend = fake_blend

            result = ImageFilterApplier()._apply_advanced_filter(
                source,
                "blur_motion",
                intensity=1.0,
                blur_radius=2.0,
                sharpen_factor=2.0,
                noise_level=0.1,
                vignette_strength=0.5,
                color_offset_x=5,
                color_offset_y=5,
            )

            self.assertIs(result, blended)
            self.assertFalse(source.closed)
            self.assertEqual(len(created_filters), 2)
            self.assertTrue(all(image.closed for image in created_filters))
            self.assertFalse(blended.closed)
        finally:
            filter_engine.Image.blend = original_blend

    def test_color_offset_closes_split_channels(self):
        class FakeChannel:
            def __init__(self, name):
                self.name = name
                self.closed = False

            def close(self):
                self.closed = True

        class FakeImage:
            def split(self):
                return source_channels

        source_channels = (FakeChannel("r"), FakeChannel("g"), FakeChannel("b"))
        shifted_r = FakeChannel("shifted-r")
        shifted_b = FakeChannel("shifted-b")
        merged = FakeChannel("merged")
        original_offset = filter_engine.ImageChops.offset
        original_merge = filter_engine.Image.merge

        def fake_offset(channel, x, y):
            if channel is source_channels[0]:
                return shifted_r
            if channel is source_channels[2]:
                return shifted_b
            raise AssertionError("unexpected channel passed to offset")

        def fake_merge(mode, channels):
            self.assertEqual(mode, "RGB")
            self.assertEqual(channels, (shifted_r, source_channels[1], shifted_b))
            return merged

        try:
            filter_engine.ImageChops.offset = fake_offset
            filter_engine.Image.merge = fake_merge

            result = ImageFilterApplier()._add_color_offset(FakeImage(), 3, 2)

            self.assertIs(result, merged)
            for channel in (*source_channels, shifted_r, shifted_b):
                self.assertTrue(channel.closed)
            self.assertFalse(merged.closed)
        finally:
            filter_engine.ImageChops.offset = original_offset
            filter_engine.Image.merge = original_merge


if __name__ == "__main__":
    unittest.main()
