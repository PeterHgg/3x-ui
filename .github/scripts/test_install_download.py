"""仅测试安装器下载函数，不执行安装、停服务或写入系统目录。"""
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest


class InstallerDownloadTests(unittest.TestCase):
    def run_download(self, *, bad_digest=False, unsafe=False):
        root = Path(__file__).resolve().parents[2]
        text = (root / 'install.sh').read_text()
        function = text[text.index('download_install_package() {'):text.index('\ninstall_x-ui() {')]
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory)
            archive = work / 'source.tar.gz'
            with tarfile.open(archive, 'w:gz') as tar:
                for name, body in [('x-ui/x-ui', b'dummy-binary'), ('x-ui/x-ui.sh', b'#!/bin/bash\ntrue\n')]:
                    member = tarfile.TarInfo(name)
                    member.size = len(body)
                    tar.addfile(member, io.BytesIO(body))
                if unsafe:
                    member = tarfile.TarInfo('../escaped')
                    member.size = 1
                    tar.addfile(member, io.BytesIO(b'x'))
            digest = '0' * 64 if bad_digest else hashlib.sha256(archive.read_bytes()).hexdigest()
            (work / 'metadata.json').write_text(json.dumps({'assets': [{
                'name': 'x-ui-linux-amd64.tar.gz', 'id': 123, 'size': archive.stat().st_size,
                'digest': 'sha256:' + digest,
            }]}))
            script = '''set -u
arch() { printf amd64; }
curl() {
  local url='' output=''
  while [ "$#" -gt 0 ]; do
    case "$1" in
      https://*) url="$1"; shift ;;
      -o) output="$2"; shift 2 ;;
      *) shift ;;
    esac
  done
  case "$url" in
    */releases/tags/*) command cp "$TEST_WORK/metadata.json" "$output" ;;
    https://github.com/*) return 22 ;;
    */releases/assets/123) command cp "$TEST_WORK/source.tar.gz" "$output" ;;
    *) return 1 ;;
  esac
}
cp() {
  if [ "$2" != /usr/bin/x-ui-temp ]; then return 99; fi
  command cp "$1" "$TEST_WORK/cli"
}
xui_folder="$TEST_WORK/x-ui"
''' + function + '\ndownload_install_package v2.9.125\n'
            result = subprocess.run(['bash'], input=script, text=True, capture_output=True,
                                    env={**os.environ, 'TEST_WORK': str(work), 'TMPDIR': str(work)})
            package = work / 'x-ui-linux-amd64.tar.gz'
            if bad_digest or unsafe:
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertFalse(package.exists())
                self.assertFalse((work / 'cli').exists())
            else:
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(package.read_bytes(), archive.read_bytes())
                self.assertEqual((work / 'cli').read_bytes(), b'#!/bin/bash\ntrue\n')
            self.assertFalse((work.parent / 'escaped').exists())

    def test_direct_failure_uses_api_and_embedded_cli(self):
        self.run_download()

    def test_bad_digest_does_not_publish_files(self):
        self.run_download(bad_digest=True)

    def test_unsafe_archive_does_not_publish_files(self):
        self.run_download(unsafe=True)


if __name__ == '__main__':
    unittest.main()
