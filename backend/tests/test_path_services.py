import tempfile
import unittest
from pathlib import Path

from backend.domain.paths import expand_input_paths, resolve_output_path


class PathServicesTests(unittest.TestCase):
    def test_expand_input_paths_collects_files_and_directories(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            nested = root / "nested"
            nested.mkdir()
            image_path = nested / "sample.png"
            image_path.write_bytes(b"fake")

            result = expand_input_paths([str(root)])

            self.assertTrue(result["has_directory"])
            self.assertEqual(len(result["files"]), 1)
            self.assertEqual(result["files"][0]["relative_path"], "nested/sample.png")

    def test_expand_input_paths_includes_avif_and_ico_files(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            (root / "sample.avif").write_bytes(b"fake-avif")
            (root / "icon.ico").write_bytes(b"fake-ico")

            result = expand_input_paths([str(root)])

            self.assertEqual(
                [item["relative_path"] for item in result["files"]],
                ["icon.ico", "sample.avif"],
            )

    def test_resolve_output_path_appends_suffix_for_conflicts(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            output = root / "result.png"
            output.write_bytes(b"exists")

            resolved = resolve_output_path(str(output), reserved=[])

            self.assertEqual(Path(resolved).name, "result_01.png")

    def test_resolve_output_path_treats_reserved_paths_case_insensitively(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            output = root / "Logo.png"

            resolved = resolve_output_path(str(output), reserved=[str(root / "logo.png")])

            self.assertEqual(Path(resolved).name, "Logo_01.png")


if __name__ == "__main__":
    unittest.main()
