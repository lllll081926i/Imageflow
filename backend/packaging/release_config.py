from __future__ import annotations

import platform
import tomllib
from dataclasses import dataclass
from pathlib import Path


APP_NAME = "ImageFlow"
PLATFORM_TAG = "windows-amd64"


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


def create_release_paths(project_root: Path | None = None, version: str | None = None) -> ReleasePaths:
    root = (project_root or default_project_root()).resolve()
    release_version = version or read_project_version(root)
    build_root = root / "build" / "release"
    artifacts_dir = root / "artifacts" / "release"
    pyinstaller_dist_root = build_root / "dist"
    portable_name = f"{APP_NAME}-portable-{release_version}-{PLATFORM_TAG}.zip"
    installer_name = f"{APP_NAME}-setup-{release_version}-{PLATFORM_TAG}.exe"

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
        pyinstaller_dist_dir=pyinstaller_dist_root / APP_NAME,
        portable_archive=artifacts_dir / portable_name,
        installer_exe=artifacts_dir / installer_name,
        inno_script=build_root / f"{APP_NAME}-setup.iss",
    )

