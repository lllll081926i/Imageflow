from __future__ import annotations

import argparse
import hashlib
import json
import os
import plistlib
import shutil
import subprocess
import sys
import tarfile
import urllib.request
from pathlib import Path
from zipfile import ZIP_DEFLATED, ZipFile

from PIL import Image

from backend.packaging.inno import find_iscc, write_inno_script
from backend.packaging.release_config import (
    APP_AUTHOR,
    APP_COPYRIGHT,
    APP_DESCRIPTION,
    APP_HOMEPAGE,
    APP_IDENTIFIER,
    ReleasePaths,
    create_release_paths,
    validate_build_host,
)


BACKEND_RUNTIME_PACKAGES = [
    "backend.api",
    "backend.application",
    "backend.contracts",
    "backend.domain",
    "backend.engines",
    "backend.host",
    "backend.infrastructure",
]

NATIVE_RUNTIME_PACKAGES = [
    "pillow_heif",
    "resvg_py",
    "mozjpeg_lossless_optimization",
    "imagequant",
    "oxipng",
    "pystray",
]

APPIMAGETOOL_VERSION = "1.9.1"
APPIMAGE_BUILD_ASSETS = {
    "amd64": {
        "tool_name": "appimagetool-x86_64.AppImage",
        "tool_sha256": "ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0",
        "runtime_url": "https://api.github.com/repos/AppImage/type2-runtime/releases/assets/456065460",
        "runtime_sha256": "1cc49bcf1e2ccd593c379adb17c9f85a36d619088296504de95b1d06215aebbf",
        "arch": "x86_64",
    },
    "arm64": {
        "tool_name": "appimagetool-aarch64.AppImage",
        "tool_sha256": "f0837e7448a0c1e4e650a93bb3e85802546e60654ef287576f46c71c126a9158",
        "runtime_url": "https://api.github.com/repos/AppImage/type2-runtime/releases/assets/456064894",
        "runtime_sha256": "7d5d772b7c32f0c84caf0a452a3072a5709027d7eac5856feb89a7a7a8881372",
        "arch": "aarch64",
    },
}


def _assert_inside_project(path: Path, project_root: Path) -> None:
    path.resolve().relative_to(project_root.resolve())


def _remove_tree(path: Path, project_root: Path) -> None:
    if not path.exists():
        return
    _assert_inside_project(path, project_root)
    shutil.rmtree(path)


def _iter_frontend_source_files(frontend_root: Path):
    for path in frontend_root.rglob("*"):
        if not path.is_file():
            continue
        if any(part in {"dist", "node_modules", ".git"} for part in path.parts):
            continue
        yield path


def _latest_mtime(paths: list[Path]) -> float:
    if not paths:
        return 0.0
    return max(path.stat().st_mtime for path in paths)


def ensure_frontend_dist(paths: ReleasePaths) -> Path:
    index_html = paths.frontend_dist_dir / "index.html"
    if not index_html.exists():
        raise FileNotFoundError("frontend dist is missing. Run `npm --prefix frontend run build` first.")
    frontend_root = paths.project_root / "frontend"
    source_files = list(_iter_frontend_source_files(frontend_root))
    dist_files = [path for path in paths.frontend_dist_dir.rglob("*") if path.is_file()]
    latest_source_mtime = _latest_mtime(source_files)
    latest_dist_mtime = _latest_mtime(dist_files)
    if latest_source_mtime > latest_dist_mtime:
        raise RuntimeError("frontend dist is stale. Run `npm --prefix frontend run build` before packaging.")
    return paths.frontend_dist_dir


def prepare_icon(paths: ReleasePaths) -> Path | None:
    source = paths.project_root / "ico.png"
    if not source.exists():
        return None

    suffix = {"windows": ".ico", "macos": ".icns", "linux": ".png"}[paths.system_name]
    icon_path = paths.build_root / f"{paths.app_name}{suffix}"
    icon_path.parent.mkdir(parents=True, exist_ok=True)
    with Image.open(source) as image:
        if paths.system_name == "windows":
            image.save(icon_path, sizes=[(16, 16), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)])
        elif paths.system_name == "macos":
            image.save(icon_path, format="ICNS")
        else:
            image.convert("RGBA").resize((256, 256), Image.Resampling.LANCZOS).save(icon_path, format="PNG")
    return icon_path


def _add_data_arg(source: Path, target: str) -> str:
    return f"{source}{os.pathsep}{target}"


def _version_tuple(version: str) -> tuple[int, int, int, int]:
    values: list[int] = []
    for part in version.split("."):
        try:
            values.append(max(0, int(part)))
        except ValueError:
            values.append(0)
    return tuple((values + [0, 0, 0, 0])[:4])  # type: ignore[return-value]


def write_windows_version_file(paths: ReleasePaths) -> Path:
    numeric_version = _version_tuple(paths.version)
    dotted_version = ", ".join(str(item) for item in numeric_version)
    version_file = paths.build_root / "ImageFlow-version-info.txt"
    version_file.parent.mkdir(parents=True, exist_ok=True)
    version_file.write_text(
        "VSVersionInfo(ffi=FixedFileInfo("
        f"filevers=({dotted_version}), prodvers=({dotted_version}), mask=0x3f, flags=0x0, "
        "OS=0x40004, fileType=0x1, subtype=0x0, date=(0, 0)), kids=["
        "StringFileInfo([StringTable('040904B0', ["
        f"StringStruct('CompanyName', {APP_AUTHOR!r}),"
        f"StringStruct('FileDescription', {APP_DESCRIPTION!r}),"
        f"StringStruct('FileVersion', {paths.version!r}),"
        f"StringStruct('InternalName', {paths.app_name!r}),"
        f"StringStruct('LegalCopyright', {APP_COPYRIGHT!r}),"
        f"StringStruct('OriginalFilename', {(paths.app_name + '.exe')!r}),"
        f"StringStruct('ProductName', {paths.app_name!r}),"
        f"StringStruct('ProductVersion', {paths.version!r})])]),"
        "VarFileInfo([VarStruct('Translation', [1033, 1200])])])",
        encoding="utf-8",
    )
    return version_file


def build_pyinstaller_command(paths: ReleasePaths, icon_path: Path | None) -> list[str]:
    entrypoint = str(paths.project_root / "backend" / "main.py")
    command = [
        sys.executable,
        "-m",
        "PyInstaller",
        "--noconfirm",
        "--clean",
        "--onedir",
        "--windowed",
        "--name",
        paths.app_name,
        "--distpath",
        str(paths.pyinstaller_dist_root),
        "--workpath",
        str(paths.pyinstaller_work_dir),
        "--specpath",
        str(paths.pyinstaller_spec_dir),
        "--add-data",
        _add_data_arg(paths.frontend_dist_dir, "frontend/dist"),
        "--hidden-import",
        "resvg_py",
        "--hidden-import",
        "pystray",
    ]
    platform_hidden_imports = {
        "windows": ["webview.platforms.edgechromium", "webview.platforms.mshtml", "pystray._win32"],
        "macos": ["webview.platforms.cocoa", "pystray._darwin"],
        "linux": ["webview.platforms.gtk", "pystray._xorg"],
    }[paths.system_name]
    for module_name in platform_hidden_imports:
        command.extend(["--hidden-import", module_name])
    for package_name in BACKEND_RUNTIME_PACKAGES:
        command.extend(["--collect-submodules", package_name])
    native_runtime_packages = list(NATIVE_RUNTIME_PACKAGES)
    if paths.system_name == "windows" and paths.architecture == "arm64":
        native_runtime_packages = [
            package_name
            for package_name in native_runtime_packages
            if package_name not in {"mozjpeg_lossless_optimization", "oxipng"}
        ]
    for package_name in native_runtime_packages:
        command.extend(["--collect-all", package_name])
    command.extend(["--collect-submodules", "webview"])
    logo_path = paths.project_root / "ico.png"
    if logo_path.exists():
        command.extend(["--add-data", _add_data_arg(logo_path, ".")])
    version_file = paths.project_root / "VERSION"
    if not version_file.exists():
        version_file.write_text(f"{paths.version}\n", encoding="utf-8")
    command.extend(["--add-data", _add_data_arg(version_file, ".")])
    if icon_path is not None:
        command.extend(["--icon", str(icon_path)])
    if paths.system_name == "macos":
        command.extend(["--osx-bundle-identifier", APP_IDENTIFIER])
    elif paths.system_name == "windows":
        command.extend(["--version-file", str(write_windows_version_file(paths))])
    command.append(entrypoint)
    return command


def run_pyinstaller(paths: ReleasePaths) -> Path:
    validate_build_host(paths.system_name, paths.architecture)
    ensure_frontend_dist(paths)
    paths.build_root.mkdir(parents=True, exist_ok=True)
    paths.artifacts_dir.mkdir(parents=True, exist_ok=True)
    _remove_tree(paths.pyinstaller_work_dir, paths.project_root)
    _remove_tree(paths.pyinstaller_dist_root, paths.project_root)
    _remove_tree(paths.pyinstaller_spec_dir, paths.project_root)

    icon_path = prepare_icon(paths)
    command = build_pyinstaller_command(paths, icon_path)
    print(f"[release] Running PyInstaller: {' '.join(command)}")
    subprocess.run(command, cwd=paths.project_root, check=True)

    exe_path = _packaged_executable(paths)
    if not exe_path.exists():
        raise FileNotFoundError(f"PyInstaller output missing: {exe_path}")
    finalize_bundle_metadata(paths)
    if paths.system_name == "macos":
        subprocess.run(
            ["codesign", "--force", "--deep", "--sign", "-", str(paths.pyinstaller_dist_dir)],
            cwd=paths.project_root,
            check=True,
        )
    return paths.pyinstaller_dist_dir


def finalize_bundle_metadata(paths: ReleasePaths) -> None:
    if paths.system_name != "macos":
        return
    plist_path = paths.pyinstaller_dist_dir / "Contents" / "Info.plist"
    try:
        with plist_path.open("rb") as file:
            metadata = plistlib.load(file)
    except (OSError, plistlib.InvalidFileException) as exc:
        raise RuntimeError(f"macOS app metadata is missing or invalid: {plist_path}") from exc
    metadata.update(
        {
            "CFBundleDisplayName": paths.app_name,
            "CFBundleName": paths.app_name,
            "CFBundleIdentifier": APP_IDENTIFIER,
            "CFBundleShortVersionString": paths.version,
            "CFBundleVersion": paths.version,
            "CFBundleGetInfoString": APP_DESCRIPTION,
            "NSHumanReadableCopyright": APP_COPYRIGHT,
            "LSApplicationCategoryType": "public.app-category.graphics-design",
        }
    )
    with plist_path.open("wb") as file:
        plistlib.dump(metadata, file, sort_keys=True)


def _packaged_executable(paths: ReleasePaths) -> Path:
    if paths.system_name == "windows":
        return paths.pyinstaller_dist_dir / f"{paths.app_name}.exe"
    if paths.system_name == "macos":
        return paths.pyinstaller_dist_dir / "Contents" / "MacOS" / paths.app_name
    return paths.pyinstaller_dist_dir / paths.app_name


def _distribution_marker(paths: ReleasePaths) -> Path:
    return _packaged_executable(paths).parent / "DISTRIBUTION.json"


def set_distribution_kind(paths: ReleasePaths, kind: str | None) -> None:
    marker = _distribution_marker(paths)
    if kind is None:
        if marker.exists():
            marker.unlink()
        return
    marker.write_text(
        json.dumps({"kind": kind, "platform": paths.platform_tag}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )


def create_portable_archive(paths: ReleasePaths) -> Path:
    if not paths.pyinstaller_dist_dir.exists():
        raise FileNotFoundError(f"PyInstaller dist directory missing: {paths.pyinstaller_dist_dir}")

    paths.artifacts_dir.mkdir(parents=True, exist_ok=True)
    if paths.portable_archive.exists():
        paths.portable_archive.unlink()

    set_distribution_kind(paths, "portable")
    if paths.system_name == "linux":
        with tarfile.open(paths.portable_archive, "w:gz", compresslevel=9) as archive:
            archive.add(paths.pyinstaller_dist_dir, arcname=paths.app_name, recursive=True)
    else:
        with ZipFile(paths.portable_archive, "w", compression=ZIP_DEFLATED, compresslevel=9) as archive:
            for file_path in paths.pyinstaller_dist_dir.rglob("*"):
                if file_path.is_file() or file_path.is_symlink():
                    archive.write(file_path, file_path.relative_to(paths.pyinstaller_dist_root))

    return paths.portable_archive


def _download_pinned_asset(url: str, destination: Path, expected_sha256: str) -> Path:
    destination.parent.mkdir(parents=True, exist_ok=True)
    partial = destination.with_suffix(destination.suffix + ".part")
    request = urllib.request.Request(
        url,
        headers={"Accept": "application/octet-stream", "User-Agent": "ImageFlow-ReleaseBuilder/1.0"},
    )
    digest = hashlib.sha256()
    try:
        with urllib.request.urlopen(request, timeout=120) as response, partial.open("wb") as output:
            for chunk in iter(lambda: response.read(1024 * 1024), b""):
                digest.update(chunk)
                output.write(chunk)
        if digest.hexdigest().lower() != expected_sha256.lower():
            raise ValueError(f"SHA-256 verification failed for build dependency: {destination.name}")
        os.replace(partial, destination)
    finally:
        partial.unlink(missing_ok=True)
    return destination


def build_linux_appimage(paths: ReleasePaths) -> Path:
    if paths.system_name != "linux" or paths.linux_appimage is None:
        raise RuntimeError("AppImage can only be built on Linux")
    assets = APPIMAGE_BUILD_ASSETS[paths.architecture]
    appdir = paths.build_root / "ImageFlow.AppDir"
    _remove_tree(appdir, paths.project_root)
    bundle_root = appdir / "usr" / "lib" / "imageflow"
    shutil.copytree(paths.pyinstaller_dist_dir, bundle_root, symlinks=True)

    app_run = appdir / "AppRun"
    app_run.write_text(
        '#!/bin/sh\nHERE="$(dirname "$(readlink -f "$0")")"\nexec "$HERE/usr/lib/imageflow/ImageFlow" "$@"\n',
        encoding="utf-8",
        newline="\n",
    )
    app_run.chmod(0o755)
    desktop_entry = (
        f"[Desktop Entry]\nType=Application\nName={paths.app_name}\nComment={APP_DESCRIPTION}\n"
        "Exec=ImageFlow\nIcon=imageflow\nCategories=Graphics;Utility;\nTerminal=false\nStartupWMClass=ImageFlow\n"
    )
    (appdir / "imageflow.desktop").write_text(desktop_entry, encoding="utf-8", newline="\n")
    applications_dir = appdir / "usr" / "share" / "applications"
    applications_dir.mkdir(parents=True, exist_ok=True)
    (applications_dir / "imageflow.desktop").write_text(desktop_entry, encoding="utf-8", newline="\n")
    icon_source = paths.project_root / "ico.png"
    if icon_source.exists():
        with Image.open(icon_source) as image:
            prepared_icon = image.convert("RGBA").resize((256, 256), Image.Resampling.LANCZOS)
            prepared_icon.save(appdir / "imageflow.png")
            icon_dir = appdir / "usr" / "share" / "icons" / "hicolor" / "256x256" / "apps"
            icon_dir.mkdir(parents=True, exist_ok=True)
            prepared_icon.save(icon_dir / "imageflow.png")

    tools_dir = paths.build_root / "appimage-tools"
    tool_name = str(assets["tool_name"])
    tool = _download_pinned_asset(
        f"https://github.com/AppImage/appimagetool/releases/download/{APPIMAGETOOL_VERSION}/{tool_name}",
        tools_dir / tool_name,
        str(assets["tool_sha256"]),
    )
    runtime = _download_pinned_asset(
        str(assets["runtime_url"]),
        tools_dir / f"runtime-{assets['arch']}",
        str(assets["runtime_sha256"]),
    )
    tool.chmod(0o755)
    environment = os.environ.copy()
    environment.update({"ARCH": str(assets["arch"]), "VERSION": paths.version, "APPIMAGE_EXTRACT_AND_RUN": "1"})
    paths.linux_appimage.unlink(missing_ok=True)
    subprocess.run(
        [str(tool), "--runtime-file", str(runtime), str(appdir), str(paths.linux_appimage)],
        cwd=paths.project_root,
        env=environment,
        check=True,
    )
    paths.linux_appimage.chmod(0o755)
    return paths.linux_appimage


def build_installer(paths: ReleasePaths) -> list[Path]:
    validate_build_host(paths.system_name, paths.architecture)
    if not paths.pyinstaller_dist_dir.exists():
        raise FileNotFoundError(f"PyInstaller dist directory missing: {paths.pyinstaller_dist_dir}")

    set_distribution_kind(paths, None)
    if paths.system_name == "windows":
        icon_path = prepare_icon(paths)
        script_path = write_inno_script(paths, icon_path)
        compiler = find_iscc()
        command = [str(compiler), str(script_path)]
        print(f"[release] Running Inno Setup: {' '.join(command)}")
        subprocess.run(command, cwd=paths.project_root, check=True)
    elif paths.system_name == "macos":
        command = [
            "pkgbuild",
            "--component", str(paths.pyinstaller_dist_dir),
            "--install-location", "/Applications",
            "--identifier", APP_IDENTIFIER,
            "--version", paths.version,
            str(paths.installer_exe),
        ]
        print(f"[release] Running pkgbuild: {' '.join(command)}")
        subprocess.run(command, cwd=paths.project_root, check=True)
    else:
        _build_linux_deb(paths)
        _build_linux_rpm(paths)

    if not paths.installer_exe.exists():
        raise FileNotFoundError(f"Inno Setup output missing: {paths.installer_exe}")
    outputs = [paths.installer_exe]
    if paths.system_name == "linux":
        if paths.linux_rpm is None or not paths.linux_rpm.exists():
            raise FileNotFoundError(f"RPM output missing: {paths.linux_rpm}")
        outputs.append(paths.linux_rpm)
    return outputs


def _build_linux_deb(paths: ReleasePaths) -> None:
    staging = paths.build_root / "deb-root"
    _remove_tree(staging, paths.project_root)
    install_root = staging / "opt" / "imageflow"
    shutil.copytree(paths.pyinstaller_dist_dir, install_root, symlinks=True)
    executable = install_root / paths.app_name
    executable.chmod(executable.stat().st_mode | 0o111)

    desktop_dir = staging / "usr" / "share" / "applications"
    desktop_dir.mkdir(parents=True, exist_ok=True)
    (desktop_dir / "imageflow.desktop").write_text(
        "[Desktop Entry]\nType=Application\nName=ImageFlow\n"
        f"Comment={APP_DESCRIPTION}\nExec=/opt/imageflow/ImageFlow\nIcon=imageflow\n"
        f"Categories=Graphics;Utility;\nTerminal=false\nStartupWMClass=ImageFlow\nX-AppImage-Name={paths.app_name}\n",
        encoding="utf-8",
    )
    icon_source = paths.project_root / "ico.png"
    if icon_source.exists():
        icon_dir = staging / "usr" / "share" / "icons" / "hicolor" / "256x256" / "apps"
        icon_dir.mkdir(parents=True, exist_ok=True)
        with Image.open(icon_source) as image:
            image.convert("RGBA").resize((256, 256), Image.Resampling.LANCZOS).save(icon_dir / "imageflow.png")

    control_dir = staging / "DEBIAN"
    control_dir.mkdir(parents=True, exist_ok=True)
    control_dir.joinpath("control").write_text(
        f"Package: imageflow\nVersion: {paths.version}\nArchitecture: {paths.architecture}\n"
        f"Maintainer: {APP_AUTHOR}\nHomepage: {APP_HOMEPAGE}\nSection: graphics\nPriority: optional\n"
        f"Description: {APP_DESCRIPTION}\n"
        "Depends: libgtk-3-0 | libgtk-3-0t64, libwebkit2gtk-4.1-0\n",
        encoding="utf-8",
    )
    subprocess.run(
        ["dpkg-deb", "--build", "--root-owner-group", str(staging), str(paths.installer_exe)],
        cwd=paths.project_root,
        check=True,
    )


def _build_linux_rpm(paths: ReleasePaths) -> None:
    if paths.linux_rpm is None:
        raise RuntimeError("Linux RPM output path is missing")
    rpm_root = paths.build_root / "rpm"
    _remove_tree(rpm_root, paths.project_root)
    for directory in ("BUILD", "BUILDROOT", "RPMS", "SOURCES", "SPECS", "SRPMS"):
        (rpm_root / directory).mkdir(parents=True, exist_ok=True)

    source_root = rpm_root / "SOURCES" / "ImageFlow"
    shutil.copytree(paths.pyinstaller_dist_dir, source_root, symlinks=True)
    icon_source = paths.project_root / "ico.png"
    if icon_source.exists():
        with Image.open(icon_source) as image:
            image.convert("RGBA").resize((256, 256), Image.Resampling.LANCZOS).save(rpm_root / "SOURCES" / "imageflow.png")

    rpm_architecture = "aarch64" if paths.architecture == "arm64" else "x86_64"
    icon_install = (
        "install -Dm644 %{_sourcedir}/imageflow.png %{buildroot}%{_datadir}/icons/hicolor/256x256/apps/imageflow.png\n"
        if icon_source.exists()
        else ""
    )
    icon_file = "%{_datadir}/icons/hicolor/256x256/apps/imageflow.png\n" if icon_source.exists() else ""
    spec_path = rpm_root / "SPECS" / "imageflow.spec"
    spec_path.write_text(
        f"Name: imageflow\nVersion: {paths.version}\nRelease: 1%{{?dist}}\nSummary: {APP_DESCRIPTION}\n"
        f"License: MIT\nURL: {APP_HOMEPAGE}\nBuildArch: {rpm_architecture}\nRequires: gtk3, webkit2gtk4.1\n\n"
        f"%description\n{APP_DESCRIPTION}\n\n%install\nmkdir -p %{{buildroot}}/opt/imageflow\n"
        "cp -a %{_sourcedir}/ImageFlow/. %{buildroot}/opt/imageflow/\n"
        "install -d %{buildroot}%{_datadir}/applications\n"
        "cat > %{buildroot}%{_datadir}/applications/imageflow.desktop <<'EOF'\n"
        f"[Desktop Entry]\nType=Application\nName={paths.app_name}\nComment={APP_DESCRIPTION}\n"
        "Exec=/opt/imageflow/ImageFlow\nIcon=imageflow\nCategories=Graphics;Utility;\nTerminal=false\nStartupWMClass=ImageFlow\nEOF\n"
        f"{icon_install}\n%files\n/opt/imageflow\n%{{_datadir}}/applications/imageflow.desktop\n{icon_file}"
        f"\n%changelog\n* Mon Aug 24 2026 {APP_AUTHOR} - {paths.version}-1\n- Native ImageFlow release\n",
        encoding="utf-8",
    )
    subprocess.run(
        ["rpmbuild", "-bb", "--define", f"_topdir {rpm_root}", str(spec_path)],
        cwd=paths.project_root,
        check=True,
    )
    candidates = list((rpm_root / "RPMS").rglob("*.rpm"))
    if len(candidates) != 1:
        raise FileNotFoundError(f"Expected one RPM output, got {len(candidates)}")
    shutil.copy2(candidates[0], paths.linux_rpm)


def create_checksum_manifest(paths: ReleasePaths, outputs: list[Path]) -> Path:
    manifest = paths.checksums_file or paths.artifacts_dir / f"SHA256SUMS-{paths.platform_tag}.txt"
    lines: list[str] = []
    for output in outputs:
        digest = hashlib.sha256()
        with output.open("rb") as file:
            for chunk in iter(lambda: file.read(1024 * 1024), b""):
                digest.update(chunk)
        lines.append(f"{digest.hexdigest()}  {output.name}")
    manifest.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return manifest


def build_release(target: str) -> list[Path]:
    paths = create_release_paths()
    outputs: list[Path] = []

    if target in {"portable", "installer", "all"}:
        run_pyinstaller(paths)

    if target in {"portable", "all"}:
        outputs.append(create_portable_archive(paths))
        if paths.system_name == "linux":
            outputs.append(build_linux_appimage(paths))

    if target in {"installer", "all"}:
        outputs.extend(build_installer(paths))

    if outputs:
        outputs.append(create_checksum_manifest(paths, list(outputs)))

    return outputs


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Build ImageFlow native release packages for the current platform.")
    parser.add_argument("target", choices=["portable", "installer", "all"], nargs="?", default="all")
    args = parser.parse_args(argv)

    outputs = build_release(args.target)
    for output in outputs:
        print(f"[release] Created {output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
