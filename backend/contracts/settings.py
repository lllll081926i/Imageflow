from dataclasses import dataclass, field


@dataclass(slots=True)
class AppSettings:
    max_concurrency: int = 8
    output_prefix: str = "IF"
    output_template: str = "{prefix}{basename}"
    preserve_folder_structure: bool = True
    conflict_strategy: str = "rename"
    default_output_dir: str = ""
    recent_input_dirs: list[str] = field(default_factory=list)
    recent_output_dirs: list[str] = field(default_factory=list)


def default_app_settings() -> AppSettings:
    return AppSettings()
