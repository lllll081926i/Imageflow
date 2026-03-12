import os
import sys
import tempfile
import unittest

from PIL import Image, ImageDraw

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

import subtitle_stitcher


class SubtitleStitcherTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name: str) -> str:
        return os.path.join(self.temp_dir.name, name)

    def _make_frame(self, name: str, body_color, subtitle_color, size=(100, 100), subtitle_height=20):
        path = self._path(name)
        image = Image.new("RGB", size, body_color)
        draw = ImageDraw.Draw(image)
        y0 = size[1] - subtitle_height
        draw.rectangle((0, y0, size[0], size[1]), fill=subtitle_color)
        image.save(path, format="PNG")
        return path

    def _make_pattern_frame(self, name: str, body_color, stripe_specs, size=(100, 100), subtitle_height=20):
        path = self._path(name)
        image = Image.new("RGB", size, body_color)
        draw = ImageDraw.Draw(image)
        y0 = size[1] - subtitle_height
        draw.rectangle((0, y0, size[0], size[1]), fill=(12, 12, 12))
        for x0, x1, color in stripe_specs:
            draw.rectangle((x0, y0, x1, size[1]), fill=color)
        image.save(path, format="PNG")
        return path

    def test_stitch_keeps_first_frame_and_crops_rest(self):
        frame1 = self._make_frame("frame1.png", (240, 30, 30), (255, 255, 255))
        frame2 = self._make_frame("frame2.png", (20, 220, 20), (250, 230, 80))
        frame3 = self._make_frame("frame3.png", (20, 20, 220), (80, 230, 250))
        output_path = self._path("dialogue.png")

        result = subtitle_stitcher.process(
            {
                "action": "subtitle_stitch",
                "input_paths": [frame1, frame2, frame3],
                "output_path": output_path,
                "subtitle_crop_ratio": 0.20,
                "header_keep_full": True,
                "dedup_enabled": False,
                "minimum_strip_height": 10,
            }
        )

        self.assertTrue(result.get("success"), result)
        self.assertEqual(result.get("input_count"), 3)
        self.assertEqual(result.get("kept_count"), 3)
        self.assertEqual(result.get("strip_height"), 20)
        self.assertTrue(os.path.exists(output_path))

        with Image.open(output_path) as out:
            self.assertEqual(out.size, (100, 140))
            self.assertEqual(out.getpixel((10, 10)), (240, 30, 30, 255))
            self.assertEqual(out.getpixel((10, 105)), (250, 230, 80, 255))
            self.assertEqual(out.getpixel((10, 130)), (80, 230, 250, 255))

    def test_dedup_skips_neighbor_duplicate_subtitle_strip(self):
        frame1 = self._make_pattern_frame(
            "a.png",
            (120, 40, 40),
            [(0, 20, (255, 255, 255)), (35, 55, (255, 255, 255)), (70, 90, (255, 255, 255))],
        )
        frame2 = self._make_pattern_frame(
            "b.png",
            (10, 200, 80),
            [(10, 28, (250, 220, 120)), (52, 74, (250, 220, 120))],
        )
        frame3 = self._make_pattern_frame(
            "c.png",
            (80, 80, 200),
            [(10, 28, (250, 220, 120)), (52, 74, (250, 220, 120))],
        )
        output_path = self._path("dedup.png")

        result = subtitle_stitcher.process(
            {
                "action": "subtitle_stitch",
                "input_paths": [frame1, frame2, frame3],
                "output_path": output_path,
                "subtitle_crop_ratio": 0.20,
                "header_keep_full": True,
                "dedup_enabled": True,
                "dedup_threshold": 0,
                "minimum_strip_height": 10,
            }
        )

        self.assertTrue(result.get("success"), result)
        self.assertEqual(result.get("skipped_count"), 1)
        self.assertEqual(result.get("kept_count"), 2)

        with Image.open(output_path) as out:
            self.assertEqual(out.size, (100, 120))

    def test_dedup_skips_duplicate_subtitle_right_after_full_header(self):
        frame1 = self._make_pattern_frame(
            "header.png",
            (120, 40, 40),
            [(8, 24, (255, 255, 255)), (48, 68, (255, 255, 255))],
        )
        frame2 = self._make_pattern_frame(
            "repeat.png",
            (10, 200, 80),
            [(8, 24, (255, 255, 255)), (48, 68, (255, 255, 255))],
        )
        frame3 = self._make_pattern_frame(
            "next.png",
            (80, 80, 200),
            [(16, 38, (255, 220, 80)), (60, 88, (255, 220, 80))],
        )
        output_path = self._path("dedup_after_header.png")

        result = subtitle_stitcher.process(
            {
                "action": "subtitle_stitch",
                "input_paths": [frame1, frame2, frame3],
                "output_path": output_path,
                "subtitle_crop_ratio": 0.20,
                "header_keep_full": True,
                "dedup_enabled": True,
                "dedup_threshold": 0,
                "minimum_strip_height": 10,
            }
        )

        self.assertTrue(result.get("success"), result)
        self.assertEqual(result.get("skipped_count"), 1)
        self.assertEqual(result.get("kept_count"), 2)

        with Image.open(output_path) as out:
            self.assertEqual(out.size, (100, 120))


if __name__ == "__main__":
    unittest.main()
