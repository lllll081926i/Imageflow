import os
import tempfile
import unittest
from pathlib import Path

from backend.packaging.inno import render_inno_script
from backend.packaging.release_builder import build_pyinstaller_command, ensure_frontend_dist
from backend.packaging.release_config import ReleasePaths, create_release_paths


class ReleaseBuilderTests(unittest.TestCase):
    def test_create_release_paths_builds_expected_artifact_names(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            paths = create_release_paths(project_root=root, version="1.0.11")

            self.assertEqual(paths.portable_archive.name, "ImageFlow-portable-1.0.11-windows-amd64.zip")
            self.assertEqual(paths.installer_exe.name, "ImageFlow-setup-1.0.11-windows-amd64.exe")
            self.assertEqual(paths.inno_script.name, "ImageFlow-setup.iss")
            self.assertEqual(paths.pyinstaller_dist_dir.name, "ImageFlow")

    def test_ensure_frontend_dist_raises_when_build_output_is_missing(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            paths = create_release_paths(project_root=root, version="1.0.11")

            with self.assertRaisesRegex(FileNotFoundError, "frontend dist"):
                ensure_frontend_dist(paths)

    def test_build_pyinstaller_command_includes_required_assets(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            frontend_dist = root / "frontend" / "dist"
            frontend_dist.mkdir(parents=True, exist_ok=True)
            (frontend_dist / "index.html").write_text("<html></html>", encoding="utf-8")
            (root / "ico.png").write_bytes(b"png")
            icon_path = root / "build" / "release" / "ImageFlow.ico"
            icon_path.parent.mkdir(parents=True, exist_ok=True)
            icon_path.write_bytes(b"ico")
            paths = create_release_paths(project_root=root, version="1.0.11")

            command = build_pyinstaller_command(paths, icon_path)

            self.assertIn("--onedir", command)
            self.assertIn("--windowed", command)
            self.assertIn("--name", command)
            self.assertIn("ImageFlow", command)
            self.assertIn(f"{frontend_dist}{os.pathsep}frontend/dist", command)
            self.assertIn(f"{root / 'ico.png'}{os.pathsep}.", command)
            self.assertIn(str(icon_path), command)

    def test_render_inno_script_uses_release_paths(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            paths = ReleasePaths(
                project_root=root,
                app_name="ImageFlow",
                version="1.0.11",
                build_root=root / "build" / "release",
                artifacts_dir=root / "artifacts" / "release",
                frontend_dist_dir=root / "frontend" / "dist",
                pyinstaller_work_dir=root / "build" / "release" / "pyinstaller",
                pyinstaller_dist_root=root / "build" / "release" / "dist",
                pyinstaller_spec_dir=root / "build" / "release" / "spec",
                pyinstaller_dist_dir=root / "build" / "release" / "dist" / "ImageFlow",
                portable_archive=root / "artifacts" / "release" / "ImageFlow-portable-1.0.11-windows-amd64.zip",
                installer_exe=root / "artifacts" / "release" / "ImageFlow-setup-1.0.11-windows-amd64.exe",
                inno_script=root / "build" / "release" / "ImageFlow-setup.iss",
            )

            script = render_inno_script(paths)

            self.assertIn('AppName=ImageFlow', script)
            self.assertIn('AppVersion=1.0.11', script)
            self.assertIn(f'SourceDir={paths.pyinstaller_dist_dir.as_posix()}', script)
            self.assertIn('OutputBaseFilename=ImageFlow-setup-1.0.11-windows-amd64', script)


if __name__ == "__main__":
    unittest.main()
