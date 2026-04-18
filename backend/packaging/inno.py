from __future__ import annotations

import os
import shutil
from pathlib import Path

from backend.packaging.release_config import PLATFORM_TAG, ReleasePaths


def _inno_path(path: Path) -> str:
    return path.resolve().as_posix()


def render_inno_script(paths: ReleasePaths, setup_icon: Path | None = None) -> str:
    output_base = paths.installer_exe.stem
    setup_icon_line = ""
    if setup_icon is not None:
        setup_icon_line = f"SetupIconFile={_inno_path(setup_icon)}\n"

    return f"""[Setup]
AppId={{{{7C8D34F5-0D4C-4E50-9C5B-4F9F93E3A511}}}}
AppName={paths.app_name}
AppVersion={paths.version}
AppPublisher=ImageFlow
DefaultDirName={{autopf}}\\{paths.app_name}
DefaultGroupName={paths.app_name}
DisableProgramGroupPage=yes
OutputDir={_inno_path(paths.artifacts_dir)}
OutputBaseFilename={output_base}
SourceDir={_inno_path(paths.pyinstaller_dist_dir)}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
UninstallDisplayIcon={{app}}\\{paths.app_name}.exe
{setup_icon_line}
[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional icons:"; Flags: unchecked

[Files]
Source: "*"; DestDir: "{{app}}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{{autoprograms}}\\{paths.app_name}"; Filename: "{{app}}\\{paths.app_name}.exe"
Name: "{{autodesktop}}\\{paths.app_name}"; Filename: "{{app}}\\{paths.app_name}.exe"; Tasks: desktopicon

[Run]
Filename: "{{app}}\\{paths.app_name}.exe"; Description: "Launch {paths.app_name}"; Flags: nowait postinstall skipifsilent
"""


def write_inno_script(paths: ReleasePaths, setup_icon: Path | None = None) -> Path:
    paths.inno_script.parent.mkdir(parents=True, exist_ok=True)
    paths.inno_script.write_text(render_inno_script(paths, setup_icon), encoding="utf-8")
    return paths.inno_script


def find_iscc() -> Path:
    configured = os.getenv("INNO_SETUP_ISCC", "").strip()
    candidates = []
    if configured:
        candidates.append(Path(configured))

    found = shutil.which("ISCC.exe") or shutil.which("ISCC")
    if found:
        candidates.append(Path(found))

    candidates.extend(
        [
            Path("C:/Users/lllll/AppData/Local/Programs/Inno Setup 6/ISCC.exe"),
            Path("C:/Program Files (x86)/Inno Setup 6/ISCC.exe"),
            Path("C:/Program Files/Inno Setup 6/ISCC.exe"),
        ]
    )

    for candidate in candidates:
        if candidate.exists():
            return candidate

    raise FileNotFoundError("Inno Setup 6 compiler not found. Install Inno Setup 6 or set INNO_SETUP_ISCC.")


__all__ = ["PLATFORM_TAG", "find_iscc", "render_inno_script", "write_inno_script"]
