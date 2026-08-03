from dataclasses import dataclass, field
import os


def default_max_concurrency() -> int:
    """Prefer a conservative default on Windows where process spawn is expensive."""
    env = str(os.getenv("IMAGEFLOW_MAX_CONCURRENCY", "") or "").strip()
    if env:
        try:
            return max(1, min(32, int(env)))
        except ValueError:
            pass
    if os.name == "nt":
        cpu = os.cpu_count() or 4
        return max(1, min(4, max(2, cpu // 2)))
    cpu = os.cpu_count() or 4
    return max(1, min(8, cpu))


@dataclass(slots=True)
class AppSettings:
    max_concurrency: int = field(default_factory=default_max_concurrency)
    output_prefix: str = "IF"
    output_template: str = "{prefix}{basename}"
    preserve_folder_structure: bool = True
    conflict_strategy: str = "rename"
    default_output_dir: str = ""
    recent_input_dirs: list[str] = field(default_factory=list)
    recent_output_dirs: list[str] = field(default_factory=list)
    auto_check_updates: bool = True
    open_output_folder_on_complete: bool = False
    remember_window_state: bool = True


def default_app_settings() -> AppSettings:
    return AppSettings()
