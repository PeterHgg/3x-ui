#!/usr/bin/env python3
"""从 GitHub 官方直链下载 Release 文件，失败后使用官方资产 API。"""

import argparse
import hashlib
import http.client
import json
import os
from pathlib import Path
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile


class SafeRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        redirected = super().redirect_request(req, fp, code, msg, headers, newurl)
        if redirected is not None and urllib.parse.urlsplit(newurl).netloc != "api.github.com":
            redirected.remove_header("Authorization")
        return redirected


OPENER = urllib.request.build_opener(SafeRedirect())
DELAYS = (0, 15, 45)


def request(url, accept):
    headers = {"User-Agent": "3x-ui-release-builder", "Accept": accept}
    if urllib.parse.urlsplit(url).netloc == "api.github.com":
        headers["X-GitHub-Api-Version"] = "2022-11-28"
        token = os.environ.get("GH_TOKEN")
        if token:
            headers["Authorization"] = "Bearer " + token
    return urllib.request.Request(url, headers=headers)


def retry(operation, label):
    for attempt, delay in enumerate(DELAYS, 1):
        if delay:
            time.sleep(delay)
        try:
            return operation()
        except (OSError, ValueError, http.client.HTTPException, zipfile.BadZipFile) as exc:
            # 不打印请求头、Token 或带签名的重定向地址。
            reason = f"HTTP {exc.code}" if isinstance(exc, urllib.error.HTTPError) else type(exc).__name__
            print(f"{label}: attempt {attempt}/{len(DELAYS)} failed ({reason})", flush=True)
    raise RuntimeError(f"{label}: all attempts failed")


def download(url, output, size=None, digest=None):
    output = Path(output)
    fd, name = tempfile.mkstemp(prefix=output.name + ".", suffix=".part", dir=output.parent)
    temporary = Path(name)
    try:
        hasher = hashlib.sha256()
        count = 0
        with os.fdopen(fd, "wb") as target:
            with OPENER.open(request(url, "application/octet-stream"), timeout=120) as response:
                content_type = response.headers.get("Content-Type", "").lower()
                if "text/html" in content_type or "json" in content_type:
                    raise ValueError("response is not a binary asset")
                declared = response.headers.get("Content-Length")
                while chunk := response.read(1024 * 1024):
                    count += len(chunk)
                    hasher.update(chunk)
                    target.write(chunk)
        if not count or (declared is not None and count != int(declared)):
            raise ValueError("empty or truncated asset")
        if size is not None and count != size:
            raise ValueError("asset size differs from release metadata")
        if digest:
            algorithm, _, expected = digest.partition(":")
            if algorithm == "sha256" and hasher.hexdigest() != expected:
                raise ValueError("asset SHA256 differs from release metadata")
        with temporary.open("rb") as stream:
            prefix = stream.read(256).lstrip().lower()
        if prefix.startswith((b"<!doctype html", b"<html", b"{", b"[")):
            raise ValueError("asset contains an error page or JSON")
        if output.suffix.lower() == ".zip":
            with zipfile.ZipFile(temporary) as archive:
                if not archive.namelist() or archive.testzip() is not None:
                    raise ValueError("invalid ZIP contents")
        temporary.replace(output)
        print(f"Downloaded {output.name}: {count} bytes", flush=True)
    finally:
        temporary.unlink(missing_ok=True)


def download_release_asset(repo, release, asset_name, output):
    if len(repo.split("/")) != 2 or any(not part for part in repo.split("/")):
        raise ValueError("repository must be owner/name")
    repo_path = "/".join(urllib.parse.quote(part, safe="") for part in repo.split("/"))
    asset_path = urllib.parse.quote(asset_name, safe="")
    release_path = "latest/download" if release == "latest" else "download/" + urllib.parse.quote(release, safe="")
    direct = f"https://github.com/{repo_path}/releases/{release_path}/{asset_path}"
    try:
        retry(lambda: download(direct, output), "Release direct download")
        return
    except RuntimeError:
        print("Direct download exhausted; trying GitHub official asset API", flush=True)

    endpoint = "latest" if release == "latest" else "tags/" + urllib.parse.quote(release, safe="")
    metadata_url = f"https://api.github.com/repos/{repo_path}/releases/{endpoint}"

    def metadata():
        with OPENER.open(request(metadata_url, "application/vnd.github+json"), timeout=60) as response:
            return json.load(response)

    release_data = retry(metadata, "Release metadata")
    asset = next((a for a in release_data["assets"] if a["name"] == asset_name), None)
    if asset is None:
        raise RuntimeError(f"Required asset missing: {repo}/{release}/{asset_name}")
    asset_url = f"https://api.github.com/repos/{repo_path}/releases/assets/{int(asset['id'])}"
    retry(lambda: download(asset_url, output, asset["size"], asset.get("digest")), "Release asset API")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("repo")
    parser.add_argument("release", help="版本标签或 latest")
    parser.add_argument("asset")
    parser.add_argument("output")
    args = parser.parse_args()
    download_release_asset(args.repo, args.release, args.asset, args.output)


if __name__ == "__main__":
    main()
