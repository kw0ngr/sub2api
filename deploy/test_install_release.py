"""Offline regression checks; no real network, systemd or install directory access."""

import hashlib
import io
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("install.sh").read_text()
FUNCTIONS = "\n".join(
    re.search(rf"^{name}\(\) \{{\n.*?^\}}", SCRIPT, re.M | re.S).group()
    for name in (
        "download_release",
        "install_downloaded_release",
        "upgrade",
        "install_version",
    )
)

STUBS = r"""
msg() { echo "$1"; }
print_info() { :; }
print_success() { :; }
print_warning() { :; }
print_error() { echo "$*" >&2; }
wait_for_release_asset() { :; }
get_latest_version() { LATEST_VERSION=v0.1.337; }
get_current_version() { echo 0.1.336; }
validate_version() { echo "$1"; }
sha256_file() {
    [ "$SCENARIO" != no_hasher ] || return 1
    shasum -a 256 "$1" | awk '{print $1}'
}
curl() {
    local url="" out=""
    while [ "$#" -gt 0 ]; do
        case "$1" in
            https://*) url="$1";;
            -o) shift; out="$1";;
        esac
        shift
    done
    case "$url" in
        */checksums.txt)
            [ "$SCENARIO" != missing_checksum ] || return 1
            cp "$FIXTURES/checksums.txt" "$out";;
        *)
            [ "$SCENARIO" != download_failure ] || return 1
            cp "$FIXTURES/release.tar.gz" "$out";;
    esac
}
systemctl() {
    echo "$*" >> "$FIXTURES/events"
    if [ "$1" = stop ]; then
        [ -f "$TEMP_DIR/sub2api" ] || exit 90
        [ "$(cat "$INSTALL_DIR/sub2api")" = "$(cat "$FIXTURES/old")" ] || exit 91
    fi
}
chown() { :; }
"""


class InstallReleaseTests(unittest.TestCase):
    def run_upgrade(self, command, scenario):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            install = root / "install"
            install.mkdir()
            old = b"#!/bin/sh\necho 0.1.336\n"
            (root / "old").write_bytes(old)
            binary = install / "sub2api"
            binary.write_bytes(old)
            binary.chmod(0o755)
            payload = b"new release"
            with tarfile.open(root / "release.tar.gz", "w:gz") as archive:
                entry = tarfile.TarInfo(
                    "not-the-binary" if scenario == "missing_binary" else "sub2api"
                )
                entry.size = len(payload)
                archive.addfile(entry, io.BytesIO(payload))
            digest = hashlib.sha256((root / "release.tar.gz").read_bytes()).hexdigest()
            name = "sub2api_0.1.337_linux_amd64.tar.gz"
            line = f"{digest}  {name}\n"
            if scenario == "mismatch":
                line = f"{'0' * 64}  {name}\n"
            elif scenario == "wrong_name":
                line = f"{digest}  prefix-{name}\n"
            elif scenario == "duplicate":
                line += line
            (root / "checksums.txt").write_text(line)
            env = dict(
                os.environ,
                FIXTURES=directory,
                INSTALL_DIR=str(install),
                OS="linux",
                ARCH="amd64",
                GITHUB_REPO="example/test",
                SERVICE_USER="test",
                SCENARIO=scenario,
            )
            run = subprocess.run(
                ["bash", "-c", "set -e\n" + STUBS + FUNCTIONS + "\n" + command],
                env=env,
                capture_output=True,
                text=True,
                timeout=15,
            )
            events = (root / "events").read_text() if (root / "events").exists() else ""
            if scenario == "success":
                self.assertEqual(run.returncode, 0, run.stderr)
                self.assertEqual(binary.read_bytes(), payload)
                self.assertIn("stop sub2api", events)
                self.assertIn("start sub2api", events)
            else:
                self.assertNotEqual(run.returncode, 0, scenario)
                self.assertEqual(binary.read_bytes(), old)
                self.assertNotIn("stop sub2api", events)
                self.assertNotIn("start sub2api", events)

    def test_verified_release_before_stop(self):
        for command in ("upgrade", "install_version v0.1.337"):
            for scenario in (
                "success",
                "missing_checksum",
                "download_failure",
                "mismatch",
                "wrong_name",
                "duplicate",
                "no_hasher",
                "missing_binary",
            ):
                with self.subTest(command=command, scenario=scenario):
                    self.run_upgrade(command, scenario)


if __name__ == "__main__":
    unittest.main()
