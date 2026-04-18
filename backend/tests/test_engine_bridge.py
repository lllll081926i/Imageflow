import tempfile
import unittest
from pathlib import Path

from backend.app import create_app


class EngineBridgeTests(unittest.TestCase):
    def test_get_info_uses_python_engine_module(self):
        app = create_app()
        source = Path("backend/testdata/simple.svg").resolve()

        result = app.GetInfo({"input_path": str(source)})

        self.assertTrue(result["success"])
        self.assertEqual(result["input_path"].lower().replace("\\", "/"), str(source).lower().replace("\\", "/"))

    def test_convert_uses_python_engine_module(self):
        app = create_app()
        source = Path("backend/testdata/simple.svg").resolve()

        with tempfile.TemporaryDirectory() as temp_dir:
            output_path = Path(temp_dir) / "converted.png"
            result = app.Convert(
                {
                    "input_path": str(source),
                    "output_path": str(output_path),
                    "format": "png",
                    "quality": 95,
                    "width": 0,
                    "height": 0,
                    "maintain_ar": True,
                    "resize_mode": "",
                    "scale_percent": 0,
                    "long_edge": 0,
                    "keep_metadata": False,
                    "compress_level": 6,
                    "ico_sizes": [],
                }
            )

            self.assertTrue(result["success"])
            self.assertTrue(output_path.exists())


if __name__ == "__main__":
    unittest.main()
