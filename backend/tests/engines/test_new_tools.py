import json
import os
import sys
import tempfile
import unittest
from pathlib import Path

from PIL import Image

ENGINE_DIR = Path(__file__).resolve().parents[2] / "engines"
if str(ENGINE_DIR) not in sys.path:
    sys.path.insert(0, str(ENGINE_DIR))

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import image_splitter
import resizer
import palette as palette_engine
import stitcher
import richtext
import watermark


def _solid(path, size, color):
    Image.new("RGB", size, color).save(path)


class SplitterTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.tmp.cleanup()

    def p(self, name):
        return os.path.join(self.tmp.name, name)

    def test_square_mode_discards_excess_and_makes_square_tiles(self):
        src = self.p("img.png")
        # 100x60 with 2x2 square: tile=30 -> usable 60x60 center-cropped
        img = Image.new("RGB", (100, 60))
        for y in range(60):
            for x in range(100):
                img.putpixel((x, y), (x % 250, y % 250, 7))
        img.save(src)
        out = self.p("out")
        result = image_splitter.process({"input_path": src, "output_dir": out, "rows": 2, "cols": 2, "mode": "square"})
        self.assertTrue(result["success"], result)
        self.assertEqual(result["tile_count"], 4)
        self.assertEqual(result["tile_width"], 30)
        self.assertEqual(result["tile_height"], 30)
        names = sorted(os.listdir(out))
        self.assertEqual(names[0], "img_01.png")
        self.assertEqual(len(names), 4)

    def test_full_mode_covers_every_pixel(self):
        src = self.p("w.png")
        _solid(src, (105, 65), (3, 3, 3))  # not divisible by 2
        out = self.p("outf")
        result = image_splitter.process({"input_path": src, "output_dir": out, "rows": 2, "cols": 2, "mode": "full"})
        self.assertTrue(result["success"], result)
        total = 0
        for name in os.listdir(out):
            with Image.open(os.path.join(out, name)) as tile:
                width, height = tile.size
            total += width * height
        self.assertGreaterEqual(total, 105 * 65)

    def test_invalid_input(self):
        result = image_splitter.process({"input_path": self.p("no.png"), "output_dir": self.p("o")})
        self.assertFalse(result["success"])


class ResizerTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.tmp.cleanup()

    def p(self, name):
        return os.path.join(self.tmp.name, name)

    def test_modes(self):
        src = self.p("a.png")
        _solid(src, (200, 100), (9, 9, 9))

        r = resizer.process({"input_path": src, "output_path": self.p("pc.png"), "mode": "percent", "value": 50})
        self.assertEqual((r["width"], r["height"]), (100, 50))

        r = resizer.process({"input_path": src, "output_path": self.p("le.png"), "mode": "long_edge", "value": 400})
        self.assertEqual((r["width"], r["height"]), (400, 200))

        r = resizer.process({"input_path": src, "output_path": self.p("se.png"), "mode": "short_edge", "value": 50})
        self.assertEqual((r["width"], r["height"]), (100, 50))


class PaletteTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.tmp.cleanup()

    def test_extract_returns_theme_and_ten_colors(self):
        src = os.path.join(self.tmp.name, "p.png")
        img = Image.new("RGB", (80, 80))
        for y in range(80):
            for x in range(80):
                img.putpixel((x, y), (10, 20, 30) if x < 40 else (220, 180, 90))
        img.save(src)
        result = palette_engine.process({"action": "extract", "input_path": src})
        self.assertTrue(result["success"])
        self.assertTrue(result["theme_color"].startswith("#"))
        self.assertEqual(len(result["colors"]), 10)

    def test_export_combined_appends_strip(self):
        src = os.path.join(self.tmp.name, "c.png")
        _solid(src, (60, 40), (5, 5, 5))
        out = os.path.join(self.tmp.name, "combined.png")
        result = palette_engine.process({"action": "export_combined", "input_path": src, "output_path": out, "swatch_height": 24})
        self.assertTrue(result["success"], result)
        with Image.open(out) as img:
            self.assertEqual(img.size, (60, 64))


class StitcherTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.tmp.cleanup()

    def p(self, name):
        return os.path.join(self.tmp.name, name)

    def test_vertical_stitch_scales_to_common_width(self):
        a = self.p("a.png"); b = self.p("b.png")
        _solid(a, (100, 50), (255, 0, 0))
        _solid(b, (100, 70), (0, 128, 255))
        out = self.p("long.png")
        result = stitcher.process({"input_paths": [a, b], "output_path": out, "direction": "vertical", "gap": 4})
        self.assertTrue(result["success"], result)
        self.assertEqual(result["width"], 100)
        self.assertEqual(result["height"], 50 + 70 + 4)

    def test_frame_mode_adds_border(self):
        a = self.p("one.png")
        _solid(a, (60, 40), (1, 2, 3))
        out = self.p("frame.png")
        result = stitcher.process({"input_paths": [a], "output_path": out, "border_mode": True, "border": 10})
        self.assertTrue(result["success"], result)
        with Image.open(out) as img:
            self.assertEqual(img.size, (80, 60))

    def test_rejects_oversized_output(self):
        paths = []
        for i in range(6):
            p = self.p(f"big{i}.png")
            _solid(p, (8000, 8000), (i, 0, 0))
            paths.append(p)
        result = stitcher.process({"input_paths": paths, "output_path": self.p("huge.png")})
        self.assertFalse(result["success"])
        self.assertEqual(result.get("error_code"), "STITCH_TOO_LARGE")


class RichTextTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.tmp.cleanup()

    def test_render_blocks_to_exact_height_image(self):
        blocks = [
            {"type": "heading", "text": "标题一", "level": 1},
            {"type": "paragraph", "text": "这是一段正文内容。"},
            {"type": "list", "ordered": True, "items": ["第一项", "第二项"]},
            {"type": "quote", "text": "引用内容"},
        ]
        out = os.path.join(self.tmp.name, "rt.png")
        result = richtext.process({"blocks": blocks, "output_path": out, "width": 600, "preview": True})
        self.assertTrue(result["success"], result)
        self.assertTrue(os.path.exists(out))
        self.assertGreater(result["height"], 100)
        self.assertTrue(result["data_url"].startswith("data:image/png;base64,"))


class WatermarkTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.tmp.cleanup()

    def test_center_position_writes_image_and_dimensions(self):
        src = os.path.join(self.tmp.name, "source.png")
        out = os.path.join(self.tmp.name, "watermarked.png")
        _solid(src, (160, 100), (240, 240, 240))
        result = watermark.process({
            "action": "apply",
            "input_path": src,
            "output_path": out,
            "text": "中心",
            "position": "c",
        })
        self.assertTrue(result["success"], result)
        self.assertEqual((result["width"], result["height"]), (160, 100))
        with Image.open(out) as image:
            self.assertEqual(image.size, (160, 100))


if __name__ == "__main__":
    unittest.main()
