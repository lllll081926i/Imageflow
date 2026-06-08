import os
import sys
import tempfile
import unittest
from pathlib import Path

from backend.infrastructure import engine_loader
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
            for result, output_path in zip(results, output_paths):
                self.assertTrue(os.path.samefile(result["output_path"], output_path))
            self.assertTrue(all(path.exists() for path in output_paths))

    def test_engine_loader_uses_packaged_engine_file_when_sys_path_has_shadow_module(self):
        real_converter = sys.modules.get("converter")
        original_sys_path = list(sys.path)
        engine_loader.load_engine_module.cache_clear()

        with tempfile.TemporaryDirectory() as temp_dir:
            shadow_module = Path(temp_dir) / "converter.py"
            shadow_module.write_text(
                "def process(payload):\n"
                "    return {'success': False, 'error': 'shadow module loaded'}\n",
                encoding="utf-8",
            )

            try:
                sys.modules.pop("converter", None)
                sys.path.insert(0, temp_dir)

                module = engine_loader.load_engine_module("converter")

                self.assertEqual(
                    Path(module.__file__).resolve(),
                    Path("backend/engines/converter.py").resolve(),
                )
            finally:
                engine_loader.load_engine_module.cache_clear()
                sys.path[:] = original_sys_path
                if real_converter is not None:
                    sys.modules["converter"] = real_converter
                else:
                    sys.modules.pop("converter", None)

    def test_engine_loader_does_not_preload_converter_for_independent_engine(self):
        real_compressor = sys.modules.get("compressor")
        real_converter = sys.modules.get("converter")
        engine_loader.load_engine_module.cache_clear()

        try:
            sys.modules.pop("compressor", None)
            sys.modules.pop("converter", None)

            module = engine_loader.load_engine_module("compressor")

            self.assertEqual(
                Path(module.__file__).resolve(),
                Path("backend/engines/compressor.py").resolve(),
            )
            self.assertNotIn("converter", sys.modules)
        finally:
            engine_loader.load_engine_module.cache_clear()
            if real_compressor is not None:
                sys.modules["compressor"] = real_compressor
            else:
                sys.modules.pop("compressor", None)
            if real_converter is not None:
                sys.modules["converter"] = real_converter
            else:
                sys.modules.pop("converter", None)


if __name__ == "__main__":
    unittest.main()
