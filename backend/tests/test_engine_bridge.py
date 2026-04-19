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

    def test_convert_batch_processes_multiple_payloads_in_order(self):
        app = create_app()
        source = Path("backend/testdata/simple.svg").resolve()

        app.save_settings(
            {
                "max_concurrency": 2,
                "output_prefix": "IF",
                "output_template": "{prefix}{basename}",
                "preserve_folder_structure": True,
                "conflict_strategy": "rename",
                "default_output_dir": "",
                "recent_input_dirs": [],
                "recent_output_dirs": [],
            }
        )

        with tempfile.TemporaryDirectory() as temp_dir:
            output_paths = [Path(temp_dir) / f"converted_{index}.png" for index in range(3)]
            payloads = [
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
                for output_path in output_paths
            ]

            results = app.ConvertBatch(payloads)

            self.assertEqual(len(results), len(payloads))
            self.assertTrue(all(result["success"] for result in results))
            self.assertEqual([result["output_path"] for result in results], [str(path) for path in output_paths])
            self.assertTrue(all(path.exists() for path in output_paths))


if __name__ == "__main__":
    unittest.main()
