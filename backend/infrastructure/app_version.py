"""Resolve the running ImageFlow version and GitHub update metadata."""
from __future__ import annotations

import json
import hashlib
import os
import platform
import re
import shutil
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
import webbrowser
from pathlib import Path
from typing import Any

GITHUB_REPO = os.getenv("IMAGEFLOW_GITHUB_REPO", "lllll081926i/Imageflow").strip() or "lllll081926i/Imageflow"
GITHUB_API = f"https://api.github.com/repos/{GITHUB_REPO}/releases/latest"
USER_AGENT = "ImageFlow-UpdateCheck/1.0"
MAX_UPDATE_BYTES = 1024 * 1024 * 1024
AUTO_UPDATE_CACHE_SECONDS = 6 * 60 * 60
_UPDATE_LOCK = threading.Lock()
_RELEASE_CACHE_LOCK = threading.Lock()


def _platform_tag() -> str:
    system = platform.system().lower()
    system_name = {"windows": "windows", "darwin": "macos", "linux": "linux"}.get(system, system)
    machine = platform.machine().lower()
    architecture = "arm64" if machine in {"arm64", "aarch64"} else "amd64"
    return f"{system_name}-{architecture}"


def read_distribution_info() -> dict[str, str]:
    env_kind = str(os.getenv("IMAGEFLOW_DISTRIBUTION_KIND", "") or "").strip().lower()
    env_platform = str(os.getenv("IMAGEFLOW_PLATFORM_TAG", "") or "").strip().lower()
    marker_candidates: list[Path] = []
    if getattr(sys, "frozen", False) and sys.executable:
        marker_candidates.append(Path(sys.executable).resolve().parent / "DISTRIBUTION.json")
    meipass = getattr(sys, "_MEIPASS", None)
    if meipass:
        marker_candidates.append(Path(meipass) / "DISTRIBUTION.json")

    marker: dict[str, Any] = {}
    for candidate in marker_candidates:
        try:
            value = json.loads(candidate.read_text(encoding="utf-8"))
        except (OSError, ValueError, TypeError):
            continue
        if isinstance(value, dict):
            marker = value
            break

    kind = env_kind or str(marker.get("kind") or "").strip().lower()
    if kind not in {"portable", "installed", "development"}:
        kind = "installed" if getattr(sys, "frozen", False) else "development"
    return {
        "kind": kind,
        "platform": env_platform or str(marker.get("platform") or "").strip().lower() or _platform_tag(),
    }


def _proxy_opener() -> tuple[Any, bool]:
    proxies = urllib.request.getproxies()
    return urllib.request.build_opener(urllib.request.ProxyHandler(proxies)), bool(proxies)


def _read_release_cache() -> dict[str, Any]:
    try:
        value = json.loads((_updates_dir() / "release-cache.json").read_text(encoding="utf-8"))
    except (OSError, ValueError, TypeError):
        return {}
    return value if isinstance(value, dict) and isinstance(value.get("payload"), dict) else {}


def _write_release_cache(payload: dict[str, Any], etag: str = "") -> None:
    cache_file = _updates_dir() / "release-cache.json"
    partial = cache_file.with_suffix(".json.part")
    value = {"fetched_at": time.time(), "etag": etag, "payload": payload}
    with _RELEASE_CACHE_LOCK:
        partial.write_text(json.dumps(value, ensure_ascii=False), encoding="utf-8")
        os.replace(partial, cache_file)


def _fetch_latest_release(timeout_seconds: float, *, force: bool = False) -> tuple[dict[str, Any], bool, bool]:
    cache = _read_release_cache()
    fetched_at = float(cache.get("fetched_at") or 0.0)
    cached_payload = cache.get("payload") if isinstance(cache.get("payload"), dict) else None
    if not force and cached_payload and time.time() - fetched_at < AUTO_UPDATE_CACHE_SECONDS:
        return cached_payload, False, True

    headers = {"Accept": "application/vnd.github+json", "User-Agent": USER_AGENT}
    if cache.get("etag"):
        headers["If-None-Match"] = str(cache["etag"])
    request = urllib.request.Request(
        GITHUB_API,
        headers=headers,
        method="GET",
    )
    opener, proxy_used = _proxy_opener()
    try:
        with opener.open(request, timeout=max(2.0, float(timeout_seconds))) as response:
            payload = json.loads(response.read().decode("utf-8", errors="replace"))
            etag = str(response.headers.get("ETag") or "")
    except urllib.error.HTTPError as exc:
        if exc.code == 304 and cached_payload:
            _write_release_cache(cached_payload, str(cache.get("etag") or ""))
            return cached_payload, proxy_used, True
        if exc.code in {403, 429} and cached_payload:
            return cached_payload, proxy_used, True
        raise
    if not isinstance(payload, dict):
        raise ValueError("unexpected release payload")
    _write_release_cache(payload, etag)
    return payload, proxy_used, False


def _select_installer_asset(payload: dict[str, Any], version: str, target_platform: str) -> dict[str, Any] | None:
    expected_suffix = {"windows": ".exe", "macos": ".pkg", "linux": _linux_installer_suffix()}.get(
        target_platform.split("-", 1)[0]
    )
    if not expected_suffix:
        return None
    expected_name = f"ImageFlow-setup-{version}-{target_platform}{expected_suffix}".lower()
    assets = payload.get("assets")
    if not isinstance(assets, list):
        return None
    for asset in assets:
        if isinstance(asset, dict) and str(asset.get("name") or "").lower() == expected_name:
            return asset
    return None


def _linux_installer_suffix() -> str:
    if platform.system().lower() != "linux":
        return ".deb"
    try:
        os_release = Path("/etc/os-release").read_text(encoding="utf-8", errors="replace").lower()
    except OSError:
        return ".deb"
    rpm_markers = ("fedora", "rhel", "centos", "rocky", "almalinux", "suse", "opensuse", "amzn")
    return ".rpm" if any(marker in os_release for marker in rpm_markers) else ".deb"


def _release_url(payload: dict[str, Any]) -> str:
    fallback = f"https://github.com/{GITHUB_REPO}/releases"
    url = str(payload.get("html_url") or fallback).strip()
    parsed = urllib.parse.urlparse(url)
    return url if parsed.scheme == "https" and parsed.hostname == "github.com" else fallback


def read_local_version() -> str:
    """Read version from env, embedded VERSION file, or pyproject.toml."""
    env = str(os.getenv("IMAGEFLOW_VERSION", "") or "").strip()
    if env:
        return env.lstrip("vV")

    candidates: list[Path] = []
    meipass = getattr(__import__("sys"), "_MEIPASS", None)
    if meipass:
        candidates.append(Path(meipass) / "VERSION")
    try:
        import sys

        if getattr(sys, "frozen", False) and sys.executable:
            candidates.append(Path(sys.executable).resolve().parent / "VERSION")
    except Exception:
        pass
    candidates.append(Path(__file__).resolve().parents[2] / "VERSION")
    for candidate in candidates:
        try:
            raw = candidate.read_text(encoding="utf-8").strip()
        except OSError:
            continue
        if raw:
            return raw.lstrip("vV")

    root = Path(__file__).resolve().parents[2]
    pyproject = root / "pyproject.toml"
    try:
        text = pyproject.read_text(encoding="utf-8")
    except OSError:
        return "0.0.0"
    match = re.search(r'(?m)^version\s*=\s*["\']([^"\']+)["\']', text)
    if match:
        return match.group(1).strip().lstrip("vV")
    return "0.0.0"


def _parse_version_tuple(version: str) -> tuple[int, ...]:
    cleaned = str(version or "").strip().lstrip("vV")
    parts: list[int] = []
    for token in re.split(r"[^\d]+", cleaned):
        if not token:
            continue
        try:
            parts.append(int(token))
        except ValueError:
            parts.append(0)
    return tuple(parts or [0])


def is_remote_newer(local: str, remote: str) -> bool:
    return _parse_version_tuple(remote) > _parse_version_tuple(local)


def check_for_updates(timeout_seconds: float = 8.0, *, force: bool = False) -> dict[str, Any]:
    """Query GitHub latest release. Network failures return success with update=false."""
    local = read_local_version()
    distribution = read_distribution_info()
    result: dict[str, Any] = {
        "success": True,
        "current_version": local,
        "latest_version": local,
        "update_available": False,
        "release_url": f"https://github.com/{GITHUB_REPO}/releases",
        "release_notes": "",
        "checked": False,
        "error": "",
        "distribution_kind": distribution["kind"],
        "platform": distribution["platform"],
        "update_action": "open_release",
        "installer_available": False,
        "proxy_used": False,
        "cached": False,
    }
    try:
        payload, proxy_used, cached = _fetch_latest_release(timeout_seconds, force=force)
    except (urllib.error.URLError, urllib.error.HTTPError, TimeoutError, json.JSONDecodeError, OSError, ValueError) as exc:
        result["error"] = str(exc)
        result["success"] = True  # soft-fail: app still usable offline
        return result

    tag = str(payload.get("tag_name") or payload.get("name") or "").strip()
    remote = tag.lstrip("vV") or local
    html_url = _release_url(payload)
    notes = str(payload.get("body") or "").strip()
    if len(notes) > 4000:
        notes = notes[:4000] + "\n…"

    result["latest_version"] = remote
    result["release_url"] = html_url or result["release_url"]
    result["release_notes"] = notes
    result["update_available"] = is_remote_newer(local, remote)
    result["checked"] = True
    result["proxy_used"] = proxy_used
    result["cached"] = cached
    asset = _select_installer_asset(payload, remote, distribution["platform"])
    digest = str(asset.get("digest") or "") if asset else ""
    if distribution["kind"] == "installed" and asset and digest.lower().startswith("sha256:"):
        result["update_action"] = "install"
        result["installer_available"] = True
        result["asset_name"] = str(asset.get("name") or "")
        result["asset_size"] = int(asset.get("size") or 0)
    return result


def open_release_page(url: str | None = None) -> dict[str, Any]:
    target = str(url or f"https://github.com/{GITHUB_REPO}/releases").strip()
    parsed = urllib.parse.urlparse(target)
    if parsed.scheme != "https" or parsed.hostname != "github.com" or not parsed.path.startswith(f"/{GITHUB_REPO}/releases"):
        target = f"https://github.com/{GITHUB_REPO}/releases"
    return {"success": bool(webbrowser.open(target, new=2)), "action": "release_page", "release_url": target}


def _updates_dir() -> Path:
    if sys.platform == "win32":
        root = Path(os.getenv("LOCALAPPDATA") or Path.home() / "AppData" / "Local") / "ImageFlow" / "updates"
    elif sys.platform == "darwin":
        root = Path.home() / "Library" / "Caches" / "ImageFlow" / "updates"
    else:
        root = Path(os.getenv("XDG_CACHE_HOME") or Path.home() / ".cache") / "imageflow" / "updates"
    root.mkdir(parents=True, exist_ok=True)
    return root


def _download_verified_asset(asset: dict[str, Any], timeout_seconds: float) -> tuple[Path, bool]:
    name = Path(str(asset.get("name") or "")).name
    url = str(asset.get("browser_download_url") or "").strip()
    size = int(asset.get("size") or 0)
    digest_label = str(asset.get("digest") or "").strip().lower()
    if not name or not url or not digest_label.startswith("sha256:"):
        raise ValueError("release asset is missing a trusted SHA-256 digest")
    if size <= 0 or size > MAX_UPDATE_BYTES:
        raise ValueError("release asset size is invalid")
    parsed = urllib.parse.urlparse(url)
    if parsed.scheme != "https" or parsed.hostname != "github.com":
        raise ValueError("release asset URL is not trusted")

    expected_digest = digest_label.split(":", 1)[1]
    target = _updates_dir() / name
    partial = target.with_suffix(target.suffix + ".part")
    request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT}, method="GET")
    opener, proxy_used = _proxy_opener()
    actual_size = 0
    digest = hashlib.sha256()
    try:
        with opener.open(request, timeout=max(10.0, float(timeout_seconds))) as response, partial.open("wb") as output:
            for chunk in iter(lambda: response.read(1024 * 1024), b""):
                actual_size += len(chunk)
                if actual_size > size or actual_size > MAX_UPDATE_BYTES:
                    raise ValueError("downloaded update exceeds its declared size")
                digest.update(chunk)
                output.write(chunk)
        if actual_size != size:
            raise ValueError("downloaded update size does not match release metadata")
        if digest.hexdigest().lower() != expected_digest:
            raise ValueError("downloaded update failed SHA-256 verification")
        os.replace(partial, target)
    finally:
        try:
            partial.unlink(missing_ok=True)
        except OSError:
            pass
    return target, proxy_used


def _launch_installer(path: Path) -> None:
    if sys.platform == "win32":
        subprocess.Popen([str(path), "/SILENT", "/CLOSEAPPLICATIONS", "/RESTARTAPPLICATIONS"], close_fds=True)
    elif sys.platform == "darwin":
        subprocess.Popen(["open", str(path)], close_fds=True)
    elif path.suffix.lower() == ".rpm" and shutil.which("pkexec"):
        package_manager = shutil.which("dnf") or shutil.which("yum") or shutil.which("zypper")
        if package_manager:
            command = ["pkexec", package_manager]
            command.extend(["install", "-y", str(path)] if Path(package_manager).name != "zypper" else ["--non-interactive", "install", str(path)])
            subprocess.Popen(command, close_fds=True)
        else:
            subprocess.Popen(["xdg-open", str(path)], close_fds=True)
    elif shutil.which("pkexec") and shutil.which("apt-get"):
        subprocess.Popen(["pkexec", "apt-get", "install", "-y", str(path)], close_fds=True)
    else:
        subprocess.Popen(["xdg-open", str(path)], close_fds=True)


def download_and_install_update(expected_version: str = "", timeout_seconds: float = 120.0) -> dict[str, Any]:
    if not _UPDATE_LOCK.acquire(blocking=False):
        return {"success": False, "error": "已有更新任务正在进行"}
    try:
        payload, metadata_proxy_used, _cached = _fetch_latest_release(min(timeout_seconds, 30.0), force=True)
        remote = str(payload.get("tag_name") or payload.get("name") or "").strip().lstrip("vV")
        release_url = _release_url(payload)
        distribution = read_distribution_info()
        if expected_version and remote != str(expected_version).strip().lstrip("vV"):
            return {"success": False, "error": "发布版本已变化，请重新检查更新"}
        if distribution["kind"] != "installed":
            opened = open_release_page(release_url)
            return {**opened, "portable": True, "version": remote}
        if not is_remote_newer(read_local_version(), remote):
            return {"success": True, "action": "none", "version": remote, "message": "当前已是最新版本"}

        asset = _select_installer_asset(payload, remote, distribution["platform"])
        if not asset or not str(asset.get("digest") or "").lower().startswith("sha256:"):
            opened = open_release_page(release_url)
            return {**opened, "success": False, "error": "没有找到经过校验的本平台安装包"}
        installer, download_proxy_used = _download_verified_asset(asset, timeout_seconds)
        _launch_installer(installer)
        return {
            "success": True,
            "action": "installer_started",
            "version": remote,
            "installer_path": str(installer),
            "proxy_used": bool(metadata_proxy_used or download_proxy_used),
        }
    except (urllib.error.URLError, urllib.error.HTTPError, TimeoutError, OSError, ValueError, subprocess.SubprocessError) as exc:
        return {"success": False, "error": str(exc)}
    finally:
        _UPDATE_LOCK.release()
