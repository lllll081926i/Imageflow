import os
import plistlib
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from backend.packaging.inno import render_inno_script
from backend.packaging.release_builder import (
    APPIMAGE_BUILD_ASSETS,
    build_pyinstaller_command,
    ensure_frontend_dist,
    finalize_bundle_metadata,
)
from backend.packaging.release_config import ReleasePaths, create_release_paths, validate_windows_build_host


class ReleaseBuilderTests(unittest.TestCase):
    def test_create_release_paths_builds_expected_artifact_names(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            paths = create_release_paths(
                project_root=root,
                version="1.0.11",
                system_name="Windows",
                machine="AMD64",
            )

            self.assertEqual(paths.portable_archive.name, "ImageFlow-portable-1.0.11-windows-amd64.zip")
            self.assertEqual(paths.installer_exe.name, "ImageFlow-setup-1.0.11-windows-amd64.exe")
            self.assertEqual(paths.inno_script.name, "ImageFlow-setup.iss")
            self.assertEqual(paths.pyinstaller_dist_dir.name, "ImageFlow")

    def test_create_release_paths_covers_all_native_package_formats(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            macos = create_release_paths(root, "2.3.0", system_name="Darwin", machine="arm64")
            linux = create_release_paths(root, "2.3.0", system_name="Linux", machine="aarch64")
            windows_arm = create_release_paths(root, "2.3.0", system_name="Windows", machine="ARM64")

            self.assertEqual(macos.portable_archive.name, "ImageFlow-portable-2.3.0-macos-arm64.zip")
            self.assertEqual(macos.installer_exe.name, "ImageFlow-setup-2.3.0-macos-arm64.pkg")
            self.assertEqual(macos.pyinstaller_dist_dir.name, "ImageFlow.app")
            self.assertEqual(linux.portable_archive.name, "ImageFlow-portable-2.3.0-linux-arm64.tar.gz")
            self.assertEqual(linux.installer_exe.name, "ImageFlow-setup-2.3.0-linux-arm64.deb")
            self.assertEqual(linux.linux_rpm.name, "ImageFlow-setup-2.3.0-linux-arm64.rpm")
            self.assertEqual(linux.linux_appimage.name, "ImageFlow-portable-2.3.0-linux-arm64.AppImage")
            self.assertEqual(windows_arm.installer_exe.name, "ImageFlow-setup-2.3.0-windows-arm64.exe")
            self.assertEqual(len(APPIMAGE_BUILD_ASSETS["arm64"]["tool_sha256"]), 64)

    def test_ensure_frontend_dist_raises_when_build_output_is_missing(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            paths = create_release_paths(project_root=root, version="1.0.11")

            with self.assertRaisesRegex(FileNotFoundError, "frontend dist"):
                ensure_frontend_dist(paths)

    def test_ensure_frontend_dist_raises_when_build_output_is_stale(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            frontend_root = root / "frontend"
            frontend_dist = frontend_root / "dist"
            frontend_dist.mkdir(parents=True, exist_ok=True)
            index_html = frontend_dist / "index.html"
            index_html.write_text("<html></html>", encoding="utf-8")
            bundle = frontend_dist / "assets.js"
            bundle.write_text("console.log('dist');", encoding="utf-8")
            source_file = frontend_root / "App.tsx"
            source_file.write_text("export const app = 1;", encoding="utf-8")
            older = 1_700_000_000
            newer = older + 60
            os.utime(index_html, (older, older))
            os.utime(bundle, (older, older))
            os.utime(source_file, (newer, newer))
            paths = create_release_paths(project_root=root, version="1.0.11")

            with self.assertRaisesRegex(RuntimeError, "frontend dist is stale"):
                ensure_frontend_dist(paths)

    def test_ensure_frontend_dist_accepts_fresh_build_output(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            frontend_root = root / "frontend"
            frontend_dist = frontend_root / "dist"
            frontend_dist.mkdir(parents=True, exist_ok=True)
            source_file = frontend_root / "App.tsx"
            source_file.write_text("export const app = 1;", encoding="utf-8")
            index_html = frontend_dist / "index.html"
            index_html.write_text("<html></html>", encoding="utf-8")
            bundle = frontend_dist / "assets.js"
            bundle.write_text("console.log('dist');", encoding="utf-8")
            older = 1_700_000_000
            newer = older + 60
            os.utime(source_file, (older, older))
            os.utime(index_html, (newer, newer))
            os.utime(bundle, (newer, newer))
            paths = create_release_paths(project_root=root, version="1.0.11")

            self.assertEqual(ensure_frontend_dist(paths).resolve(), frontend_dist.resolve())

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
            self.assertIn(f"{paths.frontend_dist_dir}{os.pathsep}frontend/dist", command)
            self.assertIn(f"{paths.project_root / 'ico.png'}{os.pathsep}.", command)
            self.assertIn(str(icon_path), command)

    def test_windows_arm64_skips_native_packages_without_arm_wheels(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            frontend_dist = root / "frontend" / "dist"
            frontend_dist.mkdir(parents=True)
            (frontend_dist / "index.html").write_text("<html></html>", encoding="utf-8")
            (root / "VERSION").write_text("2.3.0\n", encoding="utf-8")
            paths = create_release_paths(root, "2.3.0", system_name="Windows", machine="ARM64")

            command_text = " ".join(build_pyinstaller_command(paths, None))

            self.assertNotIn("--collect-all mozjpeg_lossless_optimization", command_text)
            self.assertNotIn("--collect-all oxipng", command_text)
            self.assertIn("--collect-all imagequant", command_text)

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
            self.assertIn('AppVerName=ImageFlow', script)
            self.assertIn('AppPublisher=lllll081926i', script)
            self.assertIn('UninstallDisplayName=ImageFlow', script)
            self.assertIn('VersionInfoProductName=ImageFlow', script)
            self.assertIn('AppCopyright=Copyright (c) 2026 lllll081926i', script)
            self.assertIn(f'SourceDir={paths.pyinstaller_dist_dir.resolve().as_posix()}', script)
            self.assertIn('OutputBaseFilename=ImageFlow-setup-1.0.11-windows-amd64', script)

    def test_finalize_macos_bundle_metadata_populates_system_fields(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            paths = create_release_paths(root, "2.3.0", system_name="Darwin", machine="arm64")
            plist_path = paths.pyinstaller_dist_dir / "Contents" / "Info.plist"
            plist_path.parent.mkdir(parents=True)
            with plist_path.open("wb") as file:
                plistlib.dump({"CFBundleExecutable": "ImageFlow"}, file)

            finalize_bundle_metadata(paths)

            with plist_path.open("rb") as file:
                metadata = plistlib.load(file)
            self.assertEqual(metadata["CFBundleDisplayName"], "ImageFlow")
            self.assertEqual(metadata["CFBundleShortVersionString"], "2.3.0")
            self.assertEqual(metadata["CFBundleIdentifier"], "io.github.lllll081926i.imageflow")
            self.assertIn("lllll081926i", metadata["NSHumanReadableCopyright"])

    def test_validate_windows_build_host_rejects_non_windows(self):
        with mock.patch("backend.packaging.release_config.platform.system", return_value="Linux"):
            with self.assertRaisesRegex(RuntimeError, "built on Windows"):
                validate_windows_build_host()

    def test_validate_windows_build_host_rejects_non_x64_runtime(self):
        with mock.patch("backend.packaging.release_config.platform.system", return_value="Windows"):
            with mock.patch("backend.packaging.release_config.platform.machine", return_value="ARM64"):
                with self.assertRaisesRegex(RuntimeError, "x64 Python runtime"):
                    validate_windows_build_host()


class ReleaseWorkflowTests(unittest.TestCase):
    def test_ci_publishes_all_native_assets_from_a_version_tag(self):
        workflow_path = Path(__file__).resolve().parents[2] / ".github" / "workflows" / "ci.yml"
        workflow = workflow_path.read_text(encoding="utf-8")

        for platform_name in (
            "windows-amd64", "windows-arm64", "macos-amd64", "macos-arm64", "linux-amd64", "linux-arm64"
        ):
            self.assertIn(f"platform: {platform_name}", workflow)
        for extension in ("*.zip", "*.tar.gz", "*.AppImage", "*.exe", "*.pkg", "*.deb", "*.rpm"):
            self.assertIn(extension, workflow)
        self.assertIn("generate_release_notes: true", workflow)
        self.assertIn("SHA256SUMS-*.txt", workflow)
        self.assertIn('if [ "${#assets[@]}" -ne 22 ]', workflow)
        self.assertIn("sha256sum --check SHA256SUMS-*.txt", workflow)
        self.assertIn("draft: true", workflow)
        self.assertIn('gh release edit "$RELEASE_TAG" --draft=false --latest', workflow)

    def test_ci_limits_release_write_permission_to_publish_job(self):
        workflow_path = Path(__file__).resolve().parents[2] / ".github" / "workflows" / "ci.yml"
        workflow = workflow_path.read_text(encoding="utf-8")

        self.assertIn(
            "  test-and-build:\n"
            "    name: Test and build (${{ matrix.platform }})\n"
            "    runs-on: ${{ matrix.runner }}\n"
            "    permissions:\n"
            "      contents: read",
            workflow,
        )
        self.assertIn("  publish-release-assets:\n    needs: test-and-build", workflow)
        self.assertIn(
            "  publish-release-assets:\n"
            "    needs: test-and-build\n"
            "    if: startsWith(github.ref, 'refs/tags/v')",
            workflow,
        )
        self.assertIn(
            "    permissions:\n"
            "      contents: write\n"
            "      actions: read",
            workflow,
        )
        self.assertIn("actions/download-artifact", workflow)


if __name__ == "__main__":
    unittest.main()
