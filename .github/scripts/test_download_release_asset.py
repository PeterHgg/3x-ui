import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import urllib.error
import urllib.request
import zipfile

spec = importlib.util.spec_from_file_location("downloader", Path(__file__).with_name("download_release_asset.py"))
downloader = importlib.util.module_from_spec(spec)
spec.loader.exec_module(downloader)


class Response(io.BytesIO):
    def __init__(self, data, content_type="application/octet-stream", length=None):
        super().__init__(data)
        self.headers = {"Content-Type": content_type, "Content-Length": str(len(data) if length is None else length)}


class DownloadTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.output = Path(self.directory.name) / "geoip.dat"
        self.sleep = patch.object(downloader.time, "sleep").start()
        self.addCleanup(patch.stopall)

    def test_direct_success_does_not_use_api(self):
        with patch.object(downloader.OPENER, "open", return_value=Response(b"binary-rules")) as open_url:
            downloader.download_release_asset("owner/repo", "latest", "geoip.dat", self.output)
        self.assertEqual(self.output.read_bytes(), b"binary-rules")
        self.assertEqual(open_url.call_count, 1)
        self.assertEqual(open_url.call_args.args[0].full_url, "https://github.com/owner/repo/releases/latest/download/geoip.dat")
        self.assertNotIn(".part", " ".join(p.name for p in self.output.parent.iterdir()))

    def test_504_falls_back_to_official_asset_api(self):
        data = b"verified-rules"
        metadata = {"assets": [{"id": 123, "name": "geoip.dat", "size": len(data),
                                "digest": "sha256:" + hashlib.sha256(data).hexdigest()}]}
        failure = urllib.error.HTTPError("https://github.com/owner/repo", 504, "timeout", {}, None)
        with patch.object(downloader.OPENER, "open", side_effect=[failure, failure, failure,
                          Response(json.dumps(metadata).encode(), "application/json"), Response(data)]) as open_url:
            downloader.download_release_asset("owner/repo", "v1.0", "geoip.dat", self.output)
        self.assertEqual(self.output.read_bytes(), data)
        urls = [call.args[0].full_url for call in open_url.call_args_list]
        self.assertEqual(urls[3], "https://api.github.com/repos/owner/repo/releases/tags/v1.0")
        self.assertEqual(urls[4], "https://api.github.com/repos/owner/repo/releases/assets/123")
        self.assertEqual([call.args[0] for call in self.sleep.call_args_list], [15, 45])

    def test_failures_preserve_existing_file_and_remove_partial(self):
        self.output.write_bytes(b"old-rules")
        cases = [
            (Response(b"<html>gateway error</html>", "text/html"), {}),
            (Response(b"<html>gateway error</html>"), {}),
            (Response(b'{"message":"error"}'), {}),
            (Response(b""), {}),
            (Response(b"short", length=100), {}),
            (Response(b"binary"), {"size": 100}),
            (Response(b"binary"), {"digest": "sha256:" + "0" * 64}),
        ]
        for response, options in cases:
            with self.subTest(options=options), patch.object(downloader.OPENER, "open", return_value=response):
                with self.assertRaises(ValueError):
                    downloader.download("https://github.com/asset", self.output, **options)
                self.assertEqual(self.output.read_bytes(), b"old-rules")
                self.assertEqual(list(self.output.parent.iterdir()), [self.output])

    def test_zip_integrity_is_checked(self):
        self.output = self.output.with_suffix(".zip")
        with patch.object(downloader.OPENER, "open", return_value=Response(b"not-a-zip")):
            with self.assertRaises(zipfile.BadZipFile):
                downloader.download("https://github.com/asset", self.output)
        archive = io.BytesIO()
        with zipfile.ZipFile(archive, "w") as zf:
            zf.writestr("xray", b"dummy-binary")
        with patch.object(downloader.OPENER, "open", return_value=Response(archive.getvalue())):
            downloader.download("https://github.com/asset", self.output)
        self.assertTrue(zipfile.is_zipfile(self.output))

    def test_both_paths_fail_without_publishing_file(self):
        metadata = {"assets": [{"id": 1, "name": "geoip.dat", "size": 6}]}
        failure = urllib.error.HTTPError("https://github.com/asset", 504, "timeout", {}, None)
        with patch.object(downloader.OPENER, "open", side_effect=[failure] * 3 +
                          [Response(json.dumps(metadata).encode(), "application/json")] + [failure] * 3):
            with self.assertRaises(RuntimeError):
                downloader.download_release_asset("owner/repo", "latest", "geoip.dat", self.output)
        self.assertEqual(list(self.output.parent.iterdir()), [])

    def test_missing_asset_fails_without_output(self):
        failure = urllib.error.HTTPError("https://github.com/asset", 504, "timeout", {}, None)
        with patch.object(downloader.OPENER, "open", side_effect=[failure] * 3 +
                          [Response(b'{"assets":[]}', "application/json")]):
            with self.assertRaisesRegex(RuntimeError, "Required asset missing"):
                downloader.download_release_asset("owner/repo", "latest", "geoip.dat", self.output)
        self.assertFalse(self.output.exists())

    def test_interrupted_response_is_retried(self):
        with patch.object(downloader.OPENER, "open", side_effect=[
            downloader.http.client.IncompleteRead(b"partial"), Response(b"complete")
        ]):
            downloader.retry(lambda: downloader.download("https://github.com/asset", self.output), "asset")
        self.assertEqual(self.output.read_bytes(), b"complete")
        self.assertEqual(list(self.output.parent.iterdir()), [self.output])

    def test_token_is_only_sent_to_api_and_removed_on_redirect(self):
        with patch.dict(downloader.os.environ, {"GH_TOKEN": "test-secret"}):
            api_request = downloader.request("https://api.github.com/repos/owner/repo/releases/assets/1", "application/octet-stream")
            direct_request = downloader.request("https://github.com/owner/repo/releases/download/v1/file", "application/octet-stream")
        self.assertEqual(api_request.get_header("Authorization"), "Bearer test-secret")
        self.assertIsNone(direct_request.get_header("Authorization"))
        redirected = downloader.SafeRedirect().redirect_request(api_request, None, 302, "Found", {},
                                                               "https://release-assets.githubusercontent.com/asset")
        self.assertIsNone(redirected.get_header("Authorization"))


if __name__ == "__main__":
    unittest.main()
