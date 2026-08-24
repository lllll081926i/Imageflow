import hashlib
import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from backend.infrastructure import app_version
from backend.infrastructure.app_version import (
    _download_verified_asset,
    _parse_version_tuple,
    _select_installer_asset,
    check_for_updates,
    download_and_install_update,
    is_remote_newer,
    read_distribution_info,
    read_local_version,
)


class _Response:
    def __init__(self, value: bytes):
        self._value = value
        self._offset = 0

    def read(self, size: int = -1) -> bytes:
        if self._offset >= len(self._value):
            return b""
        end = len(self._value) if size < 0 else min(len(self._value), self._offset + size)
        chunk = self._value[self._offset:end]
        self._offset = end
        return chunk

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        return False


class _Opener:
    def __init__(self, value: bytes):
        self.value = value

    def open(self, *_args, **_kwargs):
        return _Response(self.value)

class AppVersionTests(unittest.TestCase):
    def test_parse_and_compare(self):
        self.assertEqual(_parse_version_tuple("v2.1.1"), (2, 1, 1))
        self.assertTrue(is_remote_newer("2.1.0", "2.1.1"))
        self.assertFalse(is_remote_newer("2.1.1", "2.1.1"))
        self.assertFalse(is_remote_newer("2.2.0", "2.1.9"))

    def test_local_version_readable(self):
        v = read_local_version()
        self.assertTrue(v)
        self.assertRegex(v, r"^\d+")

    def test_distribution_kind_can_be_declared_for_portable_build(self):
        with mock.patch.dict("os.environ", {"IMAGEFLOW_DISTRIBUTION_KIND": "portable", "IMAGEFLOW_PLATFORM_TAG": "linux-arm64"}):
            self.assertEqual(read_distribution_info(), {"kind": "portable", "platform": "linux-arm64"})

    def test_select_installer_uses_exact_platform_asset(self):
        payload = {"assets": [
            {"name": "ImageFlow-setup-2.3.0-windows-amd64.exe"},
            {"name": "ImageFlow-setup-2.3.0-windows-arm64.exe"},
        ]}
        selected = _select_installer_asset(payload, "2.3.0", "windows-arm64")
        self.assertEqual(selected["name"], "ImageFlow-setup-2.3.0-windows-arm64.exe")

    def test_auto_check_reuses_fresh_cache_without_network_request(self):
        cached = {"fetched_at": 1_000.0, "etag": "etag", "payload": {"tag_name": "v2.3.0"}}
        with mock.patch.object(app_version, "_read_release_cache", return_value=cached), \
             mock.patch.object(app_version.time, "time", return_value=1_100.0), \
             mock.patch.object(app_version, "_proxy_opener") as proxy_opener:
            payload, proxy_used, cache_used = app_version._fetch_latest_release(8.0)
        self.assertEqual(payload["tag_name"], "v2.3.0")
        self.assertFalse(proxy_used)
        self.assertTrue(cache_used)
        proxy_opener.assert_not_called()

    def test_manual_check_forces_release_refresh(self):
        payload = {"tag_name": "v9.0.0", "html_url": "https://github.com/lllll081926i/Imageflow/releases/tag/v9.0.0", "assets": []}
        with mock.patch.object(app_version, "read_local_version", return_value="2.2.2"), \
             mock.patch.object(app_version, "read_distribution_info", return_value={"kind": "installed", "platform": "windows-amd64"}), \
             mock.patch.object(app_version, "_fetch_latest_release", return_value=(payload, True, False)) as fetch:
            result = check_for_updates(force=True)
        fetch.assert_called_once_with(8.0, force=True)
        self.assertTrue(result["update_available"])
        self.assertTrue(result["proxy_used"])

    def test_download_verifies_declared_size_and_sha256(self):
        value = b"trusted installer bytes"
        digest = hashlib.sha256(value).hexdigest()
        asset = {
            "name": "ImageFlow-setup-2.3.0-windows-amd64.exe",
            "browser_download_url": "https://github.com/lllll081926i/Imageflow/releases/download/v2.3.0/setup.exe",
            "size": len(value),
            "digest": f"sha256:{digest}",
        }
        with tempfile.TemporaryDirectory() as temp_dir, \
             mock.patch.object(app_version, "_updates_dir", return_value=Path(temp_dir)), \
             mock.patch.object(app_version, "_proxy_opener", return_value=(_Opener(value), True)):
            output, proxy_used = _download_verified_asset(asset, 30.0)
            self.assertEqual(output.read_bytes(), value)
        self.assertTrue(proxy_used)

    def test_portable_update_only_opens_release_page(self):
        payload = {"tag_name": "v9.0.0", "html_url": "https://github.com/lllll081926i/Imageflow/releases/tag/v9.0.0", "assets": []}
        with mock.patch.object(app_version, "_fetch_latest_release", return_value=(payload, False, False)), \
             mock.patch.object(app_version, "read_distribution_info", return_value={"kind": "portable", "platform": "linux-amd64"}), \
             mock.patch.object(app_version, "open_release_page", return_value={"success": True, "action": "release_page"}) as open_page:
            result = download_and_install_update(expected_version="9.0.0")
        open_page.assert_called_once()
        self.assertTrue(result["portable"])
        self.assertEqual(result["action"], "release_page")

if __name__ == "__main__":
    unittest.main()
