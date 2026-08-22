import os
import tempfile
import unittest
from pathlib import Path

from PIL import Image

from backend.engines.grid_cutter import process as grid_process


class GridCutterTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name):
        return os.path.join(self.temp_dir.name, name)

    def _make_image(self, name, size, color_fn):
        path = self._path(name)
        img = Image.new("RGB", size)
        for y in range(size[1]):
            for x in range(size[0]):
                img.putpixel((x, y), color_fn(x, y))
        img.save(path)
        return path

    def test_cut_3x3_produces_ordered_tiles_with_expected_colors(self):
        # 60x60: each 20x20 block gets a unique color.
        def color_fn(x, y):
            row, col = y // 20, x // 20
            return (row * 80 + 10, col * 80 + 10, 5)

        src = self._make_image("blocks.png", (60, 60), color_fn)
        out_dir = self._path("tiles3")
        result = grid_process({
            "input_path": src,
            "output_dir": out_dir,
            "grid_size": 3,
        })

        self.assertTrue(result.get("success"), result)
        self.assertEqual(result["tile_count"], 9)
        self.assertEqual(result["tile_width"], 20)
        paths = result["output_paths"]
        self.assertEqual(len(paths), 9)
        expected_names = [f"blocks_{i:02d}.png" for i in range(1, 10)]
        self.assertEqual([os.path.basename(p) for p in paths], expected_names)
        for p in paths:
            self.assertTrue(os.path.exists(p))

        # Row-major order check: tile 1 is top-left block (color of x=0..20,y=0..20).
        with Image.open(paths[0]) as tile:
            self.assertEqual(tile.size, (20, 20))
            self.assertEqual(tile.getpixel((10, 10)), (10, 10, 5))
        with Image.open(paths[8]) as tile:
            self.assertEqual(tile.getpixel((10, 10)), (170, 170, 5))
        with Image.open(paths[4]) as tile:
            self.assertEqual(tile.getpixel((10, 10)), (90, 90, 5))

    def test_cut_2x2_and_4x4_tile_counts(self):
        def flat(_x, _y):
            return (1, 2, 3)

        src = self._make_image("flat.png", (80, 80), flat)

        out2 = self._path("tiles2")
        r2 = grid_process({"input_path": src, "output_dir": out2, "grid_size": "2x2"})
        self.assertTrue(r2.get("success"), r2)
        self.assertEqual(r2["tile_count"], 4)
        self.assertEqual(r2["tile_width"], 40)

        out4 = self._path("tiles4")
        r4 = grid_process({"input_path": src, "output_dir": out4, "grid_size": "十六宫格"})
        self.assertTrue(r4.get("success"), r4)
        self.assertEqual(r4["tile_count"], 16)
        self.assertEqual([os.path.basename(p) for p in r4["output_paths"]][0], "flat_01.png")
        self.assertEqual([os.path.basename(p) for p in r4["output_paths"]][-1], "flat_16.png")

    def test_non_square_input_is_center_cropped_to_square(self):
        # 100x60 -> square 60x60 crop centered -> 3x3 tiles of 20px.
        def flat(_x, _y):
            return (9, 9, 9)

        src = self._make_image("wide.png", (100, 60), flat)
        out_dir = self._path("tiles_wide")
        result = grid_process({"input_path": src, "output_dir": out_dir, "grid_size": 3})

        self.assertTrue(result.get("success"), result)
        with Image.open(result["output_paths"][0]) as tile:
            self.assertEqual(tile.size, (20, 20))

    def test_jpg_output_flattens_alpha(self):
        rgba_path = self._path("alpha.png")
        Image.new("RGBA", (40, 40), (200, 10, 10, 120)).save(rgba_path)
        out_dir = self._path("tiles_jpg")
        result = grid_process({
            "input_path": rgba_path,
            "output_dir": out_dir,
            "grid_size": 2,
            "output_format": "jpg",
        })

        self.assertTrue(result.get("success"), result)
        for tile_path in result["output_paths"]:
            self.assertTrue(tile_path.endswith(".jpg"))
            with Image.open(tile_path) as tile:
                self.assertEqual(tile.mode, "RGB")

    def test_invalid_grid_size_rejected(self):
        def flat(_x, _y):
            return (0, 0, 0)

        src = self._make_image("tiny.png", (30, 30), flat)
        result = grid_process({
            "input_path": src,
            "output_dir": self._path("tiles_bad"),
            "grid_size": 5,
        })
        self.assertFalse(result.get("success"))
        self.assertEqual(result.get("error_code"), "GRID_SIZE_INVALID")

    def test_missing_input_rejected(self):
        result = grid_process({
            "input_path": os.path.join(self.temp_dir.name, "nope.png"),
            "output_dir": self._path("tiles_none"),
            "grid_size": 3,
        })
        self.assertFalse(result.get("success"))
        self.assertEqual(result.get("error_code"), "GRID_INPUT_NOT_FOUND")


if __name__ == "__main__":
    unittest.main()
