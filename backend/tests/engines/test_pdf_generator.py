import os
import sys
import tempfile
import unittest
from pathlib import Path

from PIL import Image

ENGINE_DIR = Path(__file__).resolve().parents[2] / "engines"
if str(ENGINE_DIR) not in sys.path:
    sys.path.insert(0, str(ENGINE_DIR))

from pdf_generator import PDFGenerator, process as pdf_process


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

    def test_pdf_accepts_legacy_image_object_payloads(self):
        image_path = self._path("page.png")
        Image.new("RGB", (40, 30), (0, 128, 255)).save(image_path)
        out = self._path("legacy-object-payload.pdf")

        result = pdf_process(
            {
                "images": [{"path": image_path}],
                "output_path": out,
                "page_size": "A4",
                "layout": "single",
            }
        )

        self.assertTrue(result.get("success"))
        self.assertTrue(os.path.exists(out))
        self.assertGreater(os.path.getsize(out), 0)

    def test_compressed_flowable_uses_temp_file_instead_of_retained_bytesio(self):
        image_path = self._path("source.png")
        Image.new("RGBA", (40, 30), (255, 0, 0, 128)).save(image_path)

        generator = PDFGenerator()
        generator._validate_images([image_path])
        flowable = generator._create_image_flowable(
            image_path,
            available_size=(200, 200),
            compression_level=1,
        )

        temp_path = getattr(flowable, "filename", None)
        self.assertIsInstance(temp_path, str)
        self.assertTrue(os.path.exists(temp_path))
        self.assertFalse(hasattr(flowable, "_image_buffer"))

        generator._cleanup_temp_images()
        self.assertFalse(os.path.exists(temp_path))

    def test_generate_cleans_compressed_temp_images_after_build(self):
        image_path = self._path("source.png")
        out = self._path("compressed.pdf")
        Image.new("RGB", (40, 30), (0, 128, 255)).save(image_path)

        generator = PDFGenerator()
        created_paths: list[str] = []
        original_create_temp = generator._create_temp_image_file

        def record_temp_file(*args, **kwargs):
            temp_path = original_create_temp(*args, **kwargs)
            created_paths.append(temp_path)
            return temp_path

        generator._create_temp_image_file = record_temp_file

        result = generator.generate(
            images=[image_path],
            output_path=out,
            page_size="A4",
            layout="single",
            compression_level=1,
        )

        self.assertTrue(result.get("success"))
        self.assertTrue(os.path.exists(out))
        self.assertGreater(os.path.getsize(out), 0)
        self.assertGreater(len(created_paths), 0)
        self.assertEqual(generator._temp_image_paths, [])
        self.assertTrue(all(not os.path.exists(path) for path in created_paths))


if __name__ == "__main__":
    unittest.main()
