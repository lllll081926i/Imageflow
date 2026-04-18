from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
from pathlib import Path
from zipfile import ZIP_DEFLATED, ZipFile

from PIL import Image

from backend.packaging.inno import find_iscc, write_inno_script
from backend.packaging.release_config import ReleasePaths, create_release_paths, validate_windows_build_host


BACKEND_RUNTIME_PACKAGES = [
    "backend.api",
    "backend.application",
    "backend.contracts",
    "backend.domain",
    "backend.engines",
    "backend.host",
    "backend.infrastructure",
]


def _assert_inside_project(path: Path, project_root: Path) -> None:
    path.resolve().relative_to(project_root.resolve())


def _remove_tree(path: Path, project_root: Path) -> None:
    if not path.exists():
        return
    _assert_inside_project(path, project_root)
    shutil.rmtree(path)


def ensure_frontend_dist(paths: ReleasePaths) -> Path:
    index_html = paths.frontend_dist_dir / "index.html"
    if not index_html.exists():
        raise FileNotFoundError("frontend dist is missing. Run `npm --prefix frontend run build` first.")
    return paths.frontend_dist_dir


def prepare_icon(paths: ReleasePaths) -> Path | None:
    source = paths.project_root / "ico.png"
    if not source.exists():
        return None

    icon_path = paths.build_root / f"{paths.app_name}.ico"
    icon_path.parent.mkdir(parents=True, exist_ok=True)
    with Image.open(source) as image:
        image.save(icon_path, sizes=[(16, 16), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)])
    return icon_path


def _add_data_arg(source: Path, target: str) -> str:
    return f"{source}{os.pathsep}{target}"


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
        "webview.platforms.edgechromium",
        "--hidden-import",
        "webview.platforms.mshtml",
    ]
    for package_name in BACKEND_RUNTIME_PACKAGES:
        command.extend(["--collect-submodules", package_name])
    command.extend(["--collect-submodules", "webview"])
    logo_path = paths.project_root / "ico.png"
    if logo_path.exists():
        command.extend(["--add-data", _add_data_arg(logo_path, ".")])
    if icon_path is not None:
        command.extend(["--icon", str(icon_path)])
    command.append(entrypoint)
    return command


def run_pyinstaller(paths: ReleasePaths) -> Path:
    validate_windows_build_host()
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

    exe_path = paths.pyinstaller_dist_dir / f"{paths.app_name}.exe"
    if not exe_path.exists():
        raise FileNotFoundError(f"PyInstaller output missing: {exe_path}")
    return paths.pyinstaller_dist_dir


def create_portable_archive(paths: ReleasePaths) -> Path:
    if not paths.pyinstaller_dist_dir.exists():
        raise FileNotFoundError(f"PyInstaller dist directory missing: {paths.pyinstaller_dist_dir}")

    paths.artifacts_dir.mkdir(parents=True, exist_ok=True)
    if paths.portable_archive.exists():
        paths.portable_archive.unlink()

    with ZipFile(paths.portable_archive, "w", compression=ZIP_DEFLATED, compresslevel=9) as archive:
        for file_path in paths.pyinstaller_dist_dir.rglob("*"):
            if file_path.is_file():
                archive.write(file_path, file_path.relative_to(paths.pyinstaller_dist_root))

    return paths.portable_archive


def build_installer(paths: ReleasePaths) -> Path:
    validate_windows_build_host()
    if not paths.pyinstaller_dist_dir.exists():
        raise FileNotFoundError(f"PyInstaller dist directory missing: {paths.pyinstaller_dist_dir}")

    icon_path = prepare_icon(paths)
    script_path = write_inno_script(paths, icon_path)
    compiler = find_iscc()
    command = [str(compiler), str(script_path)]
    print(f"[release] Running Inno Setup: {' '.join(command)}")
    subprocess.run(command, cwd=paths.project_root, check=True)

    if not paths.installer_exe.exists():
        raise FileNotFoundError(f"Inno Setup output missing: {paths.installer_exe}")
    return paths.installer_exe


def build_release(target: str) -> list[Path]:
    paths = create_release_paths()
    outputs: list[Path] = []

    if target in {"portable", "installer", "all"}:
        run_pyinstaller(paths)

    if target in {"portable", "all"}:
        outputs.append(create_portable_archive(paths))

    if target in {"installer", "all"}:
        outputs.append(build_installer(paths))

    return outputs


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Build ImageFlow Windows release packages.")
    parser.add_argument("target", choices=["portable", "installer", "all"], nargs="?", default="all")
    args = parser.parse_args(argv)

    outputs = build_release(args.target)
    for output in outputs:
        print(f"[release] Created {output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
