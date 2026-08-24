from __future__ import annotations

import platform
import sys
import tomllib
from dataclasses import dataclass
from pathlib import Path


APP_NAME = "ImageFlow"
APP_AUTHOR = "lllll081926i"
APP_DESCRIPTION = "Cross-platform image processing desktop application"
APP_IDENTIFIER = "io.github.lllll081926i.imageflow"
APP_HOMEPAGE = "https://github.com/lllll081926i/Imageflow"
APP_COPYRIGHT = "Copyright (c) 2026 lllll081926i"


def normalize_system(value: str | None = None) -> str:
    current = str(value or platform.system()).strip().lower()
    aliases = {"windows": "windows", "darwin": "macos", "macos": "macos", "linux": "linux"}
    if current not in aliases:
        raise RuntimeError(f"Unsupported release platform: {current or 'unknown'}")
    return aliases[current]


def normalize_architecture(value: str | None = None) -> str:
    current = str(value or platform.machine()).strip().lower()
    if current in {"amd64", "x86_64", "x64"}:
        return "amd64"
    if current in {"arm64", "aarch64"}:
        return "arm64"
    raise RuntimeError(f"Unsupported release architecture: {current or 'unknown'}")


def platform_tag(system_name: str | None = None, machine: str | None = None) -> str:
    return f"{normalize_system(system_name)}-{normalize_architecture(machine)}"


PLATFORM_TAG = platform_tag()


@dataclass(frozen=True)
class ReleasePaths:
    project_root: Path
    app_name: str
    version: str
    build_root: Path
    artifacts_dir: Path
    frontend_dist_dir: Path
    pyinstaller_work_dir: Path
    pyinstaller_dist_root: Path
    pyinstaller_spec_dir: Path
    pyinstaller_dist_dir: Path
    portable_archive: Path
    installer_exe: Path
    inno_script: Path
    system_name: str = "windows"
    architecture: str = "amd64"
    platform_tag: str = "windows-amd64"
    checksums_file: Path | None = None
    linux_rpm: Path | None = None
    linux_appimage: Path | None = None


def default_project_root() -> Path:
    return Path(__file__).resolve().parents[2]


def read_project_version(project_root: Path) -> str:
    pyproject_path = project_root / "pyproject.toml"
    with pyproject_path.open("rb") as file:
        data = tomllib.load(file)
    return str(data["project"]["version"])


def validate_windows_build_host() -> None:
    if platform.system().lower() != "windows":
        raise RuntimeError("Windows release packages must be built on Windows.")
    machine = platform.machine().lower()
    if machine not in {"amd64", "x86_64"}:
        raise RuntimeError(f"Windows release packages must be built with an x64 Python runtime, got: {platform.machine()}")
    if sys.maxsize <= 2**32:
        raise RuntimeError("Windows release packages require a 64-bit Python runtime.")


def validate_build_host(expected_system: str | None = None, expected_architecture: str | None = None) -> None:
    actual_system = normalize_system()
    actual_architecture = normalize_architecture()
    if expected_system and actual_system != normalize_system(expected_system):
        raise RuntimeError(f"Release target {expected_system} must be built on that operating system, got: {actual_system}")
    if expected_architecture and actual_architecture != normalize_architecture(expected_architecture):
        raise RuntimeError(
            f"Release target {expected_architecture} must be built with that architecture, got: {actual_architecture}"
        )
    if sys.maxsize <= 2**32:
        raise RuntimeError("ImageFlow release packages require a 64-bit Python runtime.")


def create_release_paths(
    project_root: Path | None = None,
    version: str | None = None,
    system_name: str | None = None,
    machine: str | None = None,
) -> ReleasePaths:
    root = (project_root or default_project_root()).resolve()
    release_version = version or read_project_version(root)
    release_system = normalize_system(system_name)
    release_architecture = normalize_architecture(machine)
    release_platform_tag = f"{release_system}-{release_architecture}"
    build_root = root / "build" / "release"
    artifacts_dir = root / "artifacts" / "release"
    pyinstaller_dist_root = build_root / "dist"
    portable_extension = ".tar.gz" if release_system == "linux" else ".zip"
    installer_extension = {"windows": ".exe", "macos": ".pkg", "linux": ".deb"}[release_system]
    portable_name = f"{APP_NAME}-portable-{release_version}-{release_platform_tag}{portable_extension}"
    installer_name = f"{APP_NAME}-setup-{release_version}-{release_platform_tag}{installer_extension}"
    pyinstaller_dist_dir = (
        pyinstaller_dist_root / f"{APP_NAME}.app"
        if release_system == "macos"
        else pyinstaller_dist_root / APP_NAME
    )

    return ReleasePaths(
        project_root=root,
        app_name=APP_NAME,
        version=release_version,
        build_root=build_root,
        artifacts_dir=artifacts_dir,
        frontend_dist_dir=root / "frontend" / "dist",
        pyinstaller_work_dir=build_root / "pyinstaller",
        pyinstaller_dist_root=pyinstaller_dist_root,
        pyinstaller_spec_dir=build_root / "spec",
        pyinstaller_dist_dir=pyinstaller_dist_dir,
        portable_archive=artifacts_dir / portable_name,
        installer_exe=artifacts_dir / installer_name,
        inno_script=build_root / f"{APP_NAME}-setup.iss",
        system_name=release_system,
        architecture=release_architecture,
        platform_tag=release_platform_tag,
        checksums_file=artifacts_dir / f"SHA256SUMS-{release_platform_tag}.txt",
        linux_rpm=(
            artifacts_dir / f"{APP_NAME}-setup-{release_version}-{release_platform_tag}.rpm"
            if release_system == "linux"
            else None
        ),
        linux_appimage=(
            artifacts_dir / f"{APP_NAME}-portable-{release_version}-{release_platform_tag}.AppImage"
            if release_system == "linux"
            else None
        ),
    )
