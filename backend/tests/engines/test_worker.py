import os
import sys
import tempfile
import unittest
from pathlib import Path

from PIL import Image

ENGINE_DIR = Path(__file__).resolve().parents[2] / "engines"
if str(ENGINE_DIR) not in sys.path:
    sys.path.insert(0, str(ENGINE_DIR))

from worker import process_command


class WorkerPathValidationTests(unittest.TestCase):
    def test_process_command_rejects_parent_traversal_path(self):
        result = process_command({
            "script": "converter.py",
            "input": {
                "input_path": "../secret.png",
                "output_path": "out.png",
                "format": "png",
            },
        })

        self.assertFalse(result.get("success"))
        self.assertIn("[BAD_INPUT]", result.get("error", ""))
        self.assertIn("父级目录", result.get("error", ""))

    def test_process_command_normalizes_safe_relative_paths(self):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)

        src = os.path.join(temp_dir.name, "input.jpg")
        Image.new("RGB", (8, 8), (120, 30, 40)).save(src, format="JPEG")

        cwd = os.getcwd()
        os.chdir(temp_dir.name)
        self.addCleanup(lambda: os.chdir(cwd))

        result = process_command({
            "script": "converter.py",
            "input": {
                "input_path": "input.jpg",
                "output_path": "output.png",
                "format": "png",
            },
        })

        self.assertTrue(result.get("success"))
        self.assertTrue(os.path.isabs(result.get("input_path", "")))
        self.assertTrue(os.path.isabs(result.get("output_path", "")))

    def test_process_command_does_not_expose_traceback_by_default(self):
        result = process_command({
            "script": "missing_script.py",
            "input": {
                "input_path": "input.png",
            },
        })

        self.assertFalse(result.get("success"))
        self.assertNotIn("traceback", result)


if __name__ == "__main__":
    unittest.main()
