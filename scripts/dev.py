"""ImageFlow dev launcher: starts Vite frontend + pywebview backend with one command.

Usage (project root):
    uv run python scripts/dev.py

Behavior mirrors scripts/dev.mjs:
- picks a free port starting at 5173
- spawns `npm run dev` in frontend/
- waits until the dev server responds
- spawns the Python host (`python -m backend.main`) with IMAGEFLOW_FRONTEND_URL set
- watches backend/**/*.py (excluding tests/engines/__pycache__) and restarts the host on change
"""

from __future__ import annotations

import http.client
import os
import socket
import subprocess
import sys
import time
from pathlib import Path

ROOT_DIR = Path(__file__).resolve().parents[1]
FRONTEND_DIR = ROOT_DIR / "frontend"
BACKEND_DIR = ROOT_DIR / "backend"
HOST = "127.0.0.1"
PORT_START = 5173
FRONTEND_URL = os.getenv("IMAGEFLOW_FRONTEND_URL") or f"http://{HOST}:{PORT_START}"
IS_WINDOWS = os.name == "nt"


def _print(*args: object) -> None:
    print("[dev]", *args, flush=True)


def _resolve_npm() -> str:
    from shutil import which

    found = which("npm")
    if not found and IS_WINDOWS:
        found = which("npm.cmd")
    if not found:
        _print("ERROR: npm not found in PATH. Install Node.js LTS first.")
        raise SystemExit(1)
    return str(Path(found).resolve())


def _find_free_port(start: int, attempts: int = 20) -> int:
    for offset in range(attempts):
        candidate = start + offset
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
            sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            try:
                sock.bind((HOST, candidate))
            except OSError:
                continue
            return candidate
    _print(f"ERROR: no free port from {start}")
    raise SystemExit(1)


def _wait_for_url(url: str, timeout_seconds: float = 30.0) -> bool:
    deadline = time.monotonic() + timeout_seconds
    # url like http://127.0.0.1:5173/
    stripped = url.split("://", 1)[-1]
    host_part, _, port_part = stripped.partition(":")
    port = int(port_part or 80)
    while time.monotonic() < deadline:
        try:
            conn = http.client.HTTPConnection(host_part, port, timeout=1.0)
            conn.request("GET", "/")
            response = conn.getresponse()
            response.read()
            conn.close()
            return True
        except OSError:
            time.sleep(0.3)
    return False


def _kill_tree(process: subprocess.Popen | None) -> None:
    if process is None or process.poll() is not None:
        return
    try:
        if IS_WINDOWS and process.pid:
            subprocess.run(
                ["taskkill", "/pid", str(process.pid), "/t", "/f"],
                capture_output=True,
                check=False,
            )
        else:
            process.terminate()
            try:
                process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                process.kill()
    except OSError:
        pass


class DevRunner:
    def __init__(self) -> None:
        self.frontend_url = FRONTEND_URL
        self.frontend_process: subprocess.Popen | None = None
        self.backend_process: subprocess.Popen | None = None
        self.stopping = False
        self.file_mtimes: dict[str, float] = {}
        self._restart_at: float = 0.0

    def start_frontend(self) -> None:
        npm = _resolve_npm()
        port = self.frontend_url.rsplit(":", 1)[-1]
        self.frontend_process = subprocess.Popen(
            [npm, "run", "dev", "--", "--host", HOST, "--port", port, "--strictPort"],
            cwd=str(FRONTEND_DIR),
            env={**os.environ},
        )
        _print(f"frontend pid={self.frontend_process.pid} -> {self.frontend_url}")

    def start_backend(self) -> None:
        env = {**os.environ, "IMAGEFLOW_FRONTEND_URL": self.frontend_url}
        self.backend_process = subprocess.Popen(
            [sys.executable, "-m", "backend.main"],
            cwd=str(ROOT_DIR),
            env=env,
        )
        _print(f"backend  pid={self.backend_process.pid}")

    def snapshot_backend_files(self) -> None:
        self.file_mtimes.clear()
        if not BACKEND_DIR.is_dir():
            return
        for path in BACKEND_DIR.rglob("*.py"):
            rel = path.relative_to(BACKEND_DIR).as_posix()
            if "__pycache__" in rel or rel.startswith("tests/") or rel.startswith("engines/"):
                continue
            try:
                self.file_mtimes[str(path)] = path.stat().st_mtime
            except OSError:
                continue

    def watch_backend(self) -> None:
        self.snapshot_backend_files()

    def pump_backend_watch(self) -> None:
        now = time.monotonic()
        if now < self._restart_at:
            return
        changed = False
        for file_path, old_mtime in list(self.file_mtimes.items()):
            try:
                new_mtime = os.stat(file_path).st_mtime
            except OSError:
                continue
            if new_mtime != old_mtime:
                changed = True
                break
        if changed:
            self.snapshot_backend_files()
            _print("backend change detected, restarting host...")
            _kill_tree(self.backend_process)
            self.backend_process = None
            self.start_backend()
        self._restart_at = now + 1.0  # poll once per second

    def stop_all(self, exit_code: int = 0) -> None:
        if self.stopping:
            return
        self.stopping = True
        _kill_tree(self.backend_process)
        _kill_tree(self.frontend_process)
        raise SystemExit(exit_code)


def main() -> None:
    if not FRONTEND_DIR.is_dir() or not (FRONTEND_DIR / "package.json").is_file():
        _print("ERROR: frontend/package.json missing. Run `cd frontend && npm install` first.")
        raise SystemExit(1)

    runner = DevRunner()
    port_start = int(FRONTEND_URL.rsplit(":", 1)[-1])
    free_port = _find_free_port(port_start)
    runner.frontend_url = f"http://{HOST}:{free_port}"

    runner.start_frontend()
    if not _wait_for_url(runner.frontend_url):
        _print(f"ERROR: frontend did not come up at {runner.frontend_url}")
        runner.stop_all(1)

    runner.watch_backend()
    runner.start_backend()
    _print("ready. Press Ctrl+C to stop.")

    try:
        while True:
            time.sleep(1)
            runner.pump_backend_watch()
            if runner.frontend_process is not None and runner.frontend_process.poll() is not None:
                _print("frontend exited, shutting down.")
                runner.stop_all(1)
            # User closed the app window => the session is over. Tear down the
            # whole tree (vite included) so no orphan processes linger.
            if runner.backend_process is not None and runner.backend_process.poll() is not None:
                _print("app window closed, shutting down.")
                runner.stop_all(0)
    except KeyboardInterrupt:
        runner.stop_all(0)


if __name__ == "__main__":
    main()
