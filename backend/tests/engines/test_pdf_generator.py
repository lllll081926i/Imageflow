import os
import sys
import tempfile
import unittest
from pathlib import Path

ENGINE_DIR = Path(__file__).resolve().parents[2] / "engines"
if str(ENGINE_DIR) not in sys.path:
    sys.path.insert(0, str(ENGINE_DIR))

from pdf_generator import process as pdf_process


class PDFGenerationTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name):
        return os.path.join(self.temp_dir.name, name)

    def test_pdf_accepts_svg_input(self):
        svg_path = os.path.abspath(
            os.path.join(os.path.dirname(__file__), "..", "..", "testdata", "simple.svg")
        )
        out = self._path("svg.pdf")

        result = pdf_process(
            {
                "images": [svg_path],
                "output_path": out,
                "page_size": "A4",
                "layout": "single",
                "portrait": True,
            }
        )

        self.assertTrue(result.get("success"))
        self.assertTrue(os.path.exists(out))
        self.assertGreater(os.path.getsize(out), 0)

    def test_pdf_accepts_frontend_image_paths_and_orientation_alias(self):
        svg_path = os.path.abspath(
            os.path.join(os.path.dirname(__file__), "..", "..", "testdata", "simple.svg")
        )
        out = self._path("frontend-payload.pdf")

        result = pdf_process(
            {
                "image_paths": [svg_path],
                "output_path": out,
                "page_size": "A4",
                "layout": "landscape",
            }
        )

        self.assertTrue(result.get("success"))
        self.assertTrue(os.path.exists(out))
        self.assertGreater(os.path.getsize(out), 0)


if __name__ == "__main__":
    unittest.main()
