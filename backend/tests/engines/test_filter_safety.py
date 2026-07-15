import sys
import unittest
from pathlib import Path

ENGINE_DIR = Path(__file__).resolve().parents[2] / "engines"
if str(ENGINE_DIR) not in sys.path:
    sys.path.insert(0, str(ENGINE_DIR))

import filter as filter_engine  # noqa: E402
from PIL import Image  # noqa: E402


class FilterSafetyTests(unittest.TestCase):
    def test_slow_noise_fallback_is_disabled(self):
        tool = filter_engine.ImageFilterApplier()
        with Image.new("RGB", (8, 8), (20, 20, 20)) as img:
            with self.assertRaises(RuntimeError):
                tool._add_noise_slow(img, 0.5)

    def test_slow_vignette_fallback_is_disabled(self):
        tool = filter_engine.ImageFilterApplier()
        with Image.new("RGB", (8, 8), (20, 20, 20)) as img:
            with self.assertRaises(RuntimeError):
                tool._add_vignette_slow(img, 0.5)


if __name__ == "__main__":
    unittest.main()
