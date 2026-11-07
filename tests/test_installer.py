"""Exercise the real installer with fake GitHub downloads and an isolated HOME."""
import hashlib
import io
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest


INSTALLER = Path(__file__).resolve().parents[1] / "install.sh"


class InstallerTests(unittest.TestCase):
    def run_install(self, *, os_name="Darwin", arch="arm64", checksum="valid",
                    download_error=False, version=None, entry="bin/speedgrapher"):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            tools = root / "tools"
            tools.mkdir()
            destination = root / "installed"
            destination.mkdir()
            old = destination / "speedgrapher"
            old.write_text("original binary")
            payload = b'#!/bin/sh\nprintf "%s\\n" "$*" > "$TEST_CALLED"\n'
            archive = root / "archive.tar.gz"
            with tarfile.open(archive, "w:gz") as tar:
                info = tarfile.TarInfo(entry)
                info.mode = 0o755
                info.size = len(payload)
                tar.addfile(info, io.BytesIO(payload))
            mapped_os = os_name.lower()
            mapped_arch = "x64" if arch in ("amd64", "x86_64") else "arm64"
            asset = f"{mapped_os}.{mapped_arch}.speedgrapher.tar.gz"
            digest = hashlib.sha256(archive.read_bytes()).hexdigest()
            if checksum == "invalid":
                digest = "0" * 64
            rows = f"{digest}  {asset}\n"
            if checksum == "missing":
                rows = f"{digest}  unrelated.tar.gz\n"
            if checksum == "duplicate":
                rows *= 2
            (root / "checksums.txt").write_text(rows)
            scripts = {
                "go": '#!/bin/sh\ncase "$2" in GOBIN) printf "%s\\n" "$TEST_DEST";; GOPATH) exit 99;; esac\n',
                "uname": f'#!/bin/sh\ncase "$1" in -s) printf "%s\\n" "{os_name}";; -m) printf "%s\\n" "{arch}";; esac\n',
                "curl": r'''#!/bin/sh
output=""
url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o) output=$2; shift ;;
    -w|--connect-timeout|--max-time) shift ;;
    https://*) url=$1 ;;
  esac
  shift
done
printf '%s\n' "$url" >> "$TEST_URLS"
case "$url" in
  */releases/latest)
    printf 'https://github.com/KaiLepler/speedgrapher/releases/tag/v1.2.3' ;;
  */checksums.txt) cp "$TEST_ROOT/checksums.txt" "$output" ;;
  *.tar.gz)
    if [ "$TEST_DOWNLOAD_ERROR" = 1 ]; then exit 22; fi
    cp "$TEST_ROOT/archive.tar.gz" "$output" ;;
  *) exit 99 ;;
esac
''',
            }
            for name, script in scripts.items():
                path = tools / name
                path.write_text(script)
                path.chmod(0o755)
            env = os.environ.copy()
            env.update({
                "HOME": str(root), "PATH": f"{tools}:/usr/bin:/bin",
                "TEST_DEST": str(destination), "TEST_ROOT": str(root),
                "TEST_CALLED": str(root / "called"), "TEST_URLS": str(root / "urls"),
                "TEST_DOWNLOAD_ERROR": "1" if download_error else "0",
            })
            args = ["bash", str(INSTALLER), "--skills"]
            if version is not None:
                args += ["--version", version]
            result = subprocess.run(args, env=env, capture_output=True, text=True, timeout=10)
            return (result, old.read_bytes(), (root / "called").exists(),
                    (root / "urls").read_text() if (root / "urls").exists() else "")

    def test_supported_platforms(self):
        for platform, arch in (("Darwin", "arm64"), ("Darwin", "x86_64"),
                               ("Linux", "aarch64"), ("Linux", "amd64")):
            with self.subTest(platform=platform, arch=arch):
                result, binary, called, urls = self.run_install(os_name=platform, arch=arch)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertTrue(binary.startswith(b"#!/bin/sh"))
                self.assertTrue(called)
                self.assertNotIn("/latest/download/", urls)
                self.assertIn("/download/v1.2.3/", urls)

    def test_integrity_failures_preserve_installation(self):
        for checksum in ("invalid", "missing", "duplicate"):
            with self.subTest(checksum=checksum):
                result, binary, called, _ = self.run_install(checksum=checksum)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(binary, b"original binary")
                self.assertFalse(called)

    def test_download_failure_preserves_installation(self):
        result, binary, called, _ = self.run_install(download_error=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(binary, b"original binary")
        self.assertFalse(called)

    def test_pinned_version(self):
        result, _, called, urls = self.run_install(version="v1.2.3")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(called)
        self.assertNotIn("/releases/latest", urls)

    def test_unsupported_platform(self):
        result, binary, called, urls = self.run_install(os_name="Unknown")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(binary, b"original binary")
        self.assertFalse(called)
        self.assertEqual(urls, "")

    def test_missing_binary_in_archive(self):
        result, binary, called, _ = self.run_install(entry="unrelated")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(binary, b"original binary")
        self.assertFalse(called)


if __name__ == "__main__":
    unittest.main()
