import os
from pathlib import Path

SUPPORTED_EXTENSIONS = {
    ".jpg",
    ".jpeg",
    ".png",
    ".webp",
    ".gif",
    ".bmp",
    ".tiff",
    ".tif",
    ".heic",
    ".heif",
    ".svg",
}


def _has_leading_parent_traversal(path_value: str) -> bool:
    path = Path(path_value)
    if path.is_absolute():
        return False
    normalized = path_value.replace("\\", "/")
    return normalized == ".." or normalized.startswith("../")


def validate_user_supplied_path(path_value: str, allow_empty: bool = False) -> None:
    trimmed = path_value.strip()
    if not trimmed:
        if allow_empty:
            return
        raise ValueError("路径不能为空")
    if "\x00" in trimmed:
        raise ValueError("路径包含非法空字符")
    cleaned = os.path.normpath(trimmed)
    if _has_leading_parent_traversal(cleaned):
        raise ValueError("不允许使用父级目录跳转路径")


def normalize_user_supplied_path(path_value: str) -> str:
    validate_user_supplied_path(path_value, allow_empty=False)
    return str(Path(path_value.strip()).resolve())


def normalize_optional_user_supplied_path(path_value: str) -> str:
    validate_user_supplied_path(path_value, allow_empty=True)
    trimmed = path_value.strip()
    if not trimmed:
        return ""
    return str(Path(trimmed).resolve())


def _is_image_file(path_value: Path) -> bool:
    return path_value.suffix.lower() in SUPPORTED_EXTENSIONS


def expand_input_paths(paths: list[str]) -> dict:
    files: list[dict] = []
    has_directory = False

    for raw_path in paths:
        if not str(raw_path).strip():
            continue
        normalized = Path(normalize_user_supplied_path(str(raw_path)))
        try:
            info = normalized.stat()
        except OSError:
            continue
        if normalized.is_file():
            if _is_image_file(normalized):
                files.append(
                    {
                        "input_path": str(normalized),
                        "source_root": str(normalized.parent),
                        "relative_path": normalized.name,
                        "is_from_dir_drop": False,
                        "size": info.st_size,
                        "mod_time": int(info.st_mtime),
                    }
                )
            continue

        has_directory = True
        for current in normalized.rglob("*"):
            if not current.is_file() or not _is_image_file(current):
                continue
            try:
                current_info = current.stat()
            except OSError:
                continue
            files.append(
                {
                    "input_path": str(current),
                    "source_root": str(normalized),
                    "relative_path": current.relative_to(normalized).as_posix(),
                    "is_from_dir_drop": True,
                    "size": current_info.st_size,
                    "mod_time": int(current_info.st_mtime),
                }
            )

    files.sort(key=lambda item: str(item["input_path"]).lower())
    return {"files": files, "has_directory": has_directory}


def resolve_output_path(base_path: str, reserved: list[str] | None = None) -> str:
    if not base_path.strip():
        raise ValueError("base path is empty")

    base = Path(normalize_user_supplied_path(base_path))
    reserved_set = {str(Path(normalize_optional_user_supplied_path(item))) for item in (reserved or []) if str(item).strip()}
    if not base.exists() and str(base) not in reserved_set:
        return str(base)

    stem = base.stem or "output"
    suffix = base.suffix
    parent = base.parent
    for index in range(1, 10000):
        candidate = parent / f"{stem}_{index:02d}{suffix}"
        if not candidate.exists() and str(candidate) not in reserved_set:
            return str(candidate)
    raise RuntimeError("failed to resolve unique output path")


def list_system_fonts() -> list[str]:
    if os.name != "nt":
        return []

    fonts_dir = Path(os.environ.get("WINDIR", r"C:\Windows")) / "Fonts"
    if not fonts_dir.exists():
        return []

    allowed = {".ttf", ".otf", ".ttc"}
    fonts = [str(path) for path in fonts_dir.iterdir() if path.is_file() and path.suffix.lower() in allowed]
    fonts.sort(key=lambda item: item.lower())
    return fonts
