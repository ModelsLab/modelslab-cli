#!/usr/bin/env python3
"""Build platform wheels for the ModelsLab CLI.

One wheel per platform, each containing the prebuilt Go binary and a console
script that hands the process over to it.

Wheels are written directly rather than through setuptools. The binary is not
Python and there is nothing to compile, so a build backend would only add a
dependency and a temp directory to the same zip file. The wheel format is
specified well enough (PEP 427, PEP 425) to emit correctly, and doing it here
keeps the platform tag explicit instead of inferred from the build host.

Usage: python3 packaging/pypi/build.py <version> <artifacts-dir> [out-dir]
  <artifacts-dir> holds the goreleaser tarballs already extracted into
  <artifacts-dir>/<goos>_<goarch>/modelslab[.exe]
"""
from __future__ import annotations

import base64
import csv
import hashlib
import io
import os
import re
import stat
import sys
import zipfile
from pathlib import Path

NAME = "modelslab-cli"
DIST = "modelslab_cli"
SUMMARY = "ModelsLab CLI — AI generation and account management from the terminal"

# goreleaser target -> PEP 425 platform tag.
#
# manylinux2014 (glibc 2.17, CentOS 7) rather than a newer baseline: the binary
# is CGO_ENABLED=0, so it has no libc dependency at all and the tag only needs to
# be old enough that pip on any live distro will accept it.
TARGETS = [
    ("darwin_amd64", "macosx_10_12_x86_64"),
    ("darwin_arm64", "macosx_11_0_arm64"),
    ("linux_amd64", "manylinux2014_x86_64"),
    ("linux_arm64", "manylinux2014_aarch64"),
    ("windows_amd64", "win_amd64"),
    ("windows_arm64", "win_arm64"),
]

LAUNCHER = '''"""Console entry point for the ModelsLab CLI."""
import os
import sys
from pathlib import Path


def _binary() -> Path:
    exe = "modelslab.exe" if os.name == "nt" else "modelslab"
    return Path(__file__).resolve().parent / "bin" / exe


def main() -> int:
    binary = _binary()

    if not binary.exists():
        sys.stderr.write(
            "ModelsLab CLI binary is missing from the installed package.\\n"
            "Reinstall with: pip install --force-reinstall modelslab-cli\\n"
        )
        return 1

    argv = [str(binary), *sys.argv[1:]]

    if os.name == "nt":
        # Windows has no exec that replaces the process in a way cmd respects;
        # spawn and forward the exit code instead.
        import subprocess

        return subprocess.call(argv)

    # Replace this process so signals, exit codes and TTY behaviour are the
    # binary's own. A subprocess wrapper here would swallow SIGINT handling.
    os.execv(str(binary), argv)
    return 1  # unreachable; execv does not return


if __name__ == "__main__":
    raise SystemExit(main())
'''


def _read_description() -> str:
    readme = Path(__file__).resolve().parent / "README.md"
    return readme.read_text(encoding="utf-8") if readme.is_file() else SUMMARY


def _metadata(version: str) -> str:
    return "\n".join(
        [
            "Metadata-Version: 2.1",
            f"Name: {NAME}",
            f"Version: {version}",
            f"Summary: {SUMMARY}",
            "Home-page: https://modelslab.sh",
            "Author: ModelsLab",
            "Author-email: support@modelslab.com",
            "License: MIT",
            "Project-URL: Source, https://github.com/ModelsLab/modelslab-cli",
            "Project-URL: Documentation, https://docs.modelslab.com",
            "Classifier: Development Status :: 4 - Beta",
            "Classifier: Environment :: Console",
            "Classifier: Intended Audience :: Developers",
            "Classifier: License :: OSI Approved :: MIT License",
            "Classifier: Programming Language :: Go",
            "Classifier: Topic :: Scientific/Engineering :: Artificial Intelligence",
            "Requires-Python: >=3.8",
            "Description-Content-Type: text/markdown",
            "",
            _read_description(),
        ]
    )


def _wheel_tag(platform_tag: str) -> str:
    # py3-none-<platform>: pure launcher, no Python ABI, platform-specific payload.
    return f"py3-none-{platform_tag}"


def _urlsafe_digest(payload: bytes) -> str:
    digest = hashlib.sha256(payload).digest()
    return "sha256=" + base64.urlsafe_b64encode(digest).decode().rstrip("=")


def build_wheel(version: str, artifacts: Path, out_dir: Path, go_target: str, platform_tag: str) -> Path:
    exe = "modelslab.exe" if go_target.startswith("windows") else "modelslab"
    source = artifacts / go_target / exe

    if not source.is_file():
        raise SystemExit(f"missing binary for {go_target}: {source}")

    tag = _wheel_tag(platform_tag)
    dist_info = f"{DIST}-{version}.dist-info"
    out_dir.mkdir(parents=True, exist_ok=True)
    wheel_path = out_dir / f"{DIST}-{version}-{tag}.whl"

    records: list[tuple[str, str, int]] = []

    def add(zf: zipfile.ZipFile, arcname: str, payload: bytes, *, executable: bool = False) -> None:
        info = zipfile.ZipInfo(arcname, date_time=(1980, 1, 1, 0, 0, 0))
        # The mode must carry the file-TYPE bits, not just the permission bits.
        # pip decides whether to make an unpacked file executable with
        # `stat.S_ISREG(mode) and mode & 0o111`, so a bare 0o755 fails S_ISREG,
        # the binary lands as 0o644, and execv dies with EPERM at first run.
        # twine check does not catch this — only installing and running does.
        mode = stat.S_IFREG | (0o755 if executable else 0o644)
        info.external_attr = (mode << 16) | 0o600
        info.compress_type = zipfile.ZIP_DEFLATED
        zf.writestr(info, payload)
        records.append((arcname, _urlsafe_digest(payload), len(payload)))

    with zipfile.ZipFile(wheel_path, "w", zipfile.ZIP_DEFLATED) as zf:
        add(zf, f"{DIST}/__init__.py", b'"""ModelsLab CLI."""\n')
        add(zf, f"{DIST}/__main__.py", LAUNCHER.encode("utf-8"))
        add(zf, f"{DIST}/bin/{exe}", source.read_bytes(), executable=True)

        add(
            zf,
            f"{dist_info}/METADATA",
            _metadata(version).encode("utf-8"),
        )
        add(
            zf,
            f"{dist_info}/WHEEL",
            (
                "Wheel-Version: 1.0\n"
                "Generator: modelslab-cli packaging/pypi/build.py\n"
                "Root-Is-Purelib: false\n"
                f"Tag: {tag}\n"
            ).encode("utf-8"),
        )
        add(
            zf,
            f"{dist_info}/entry_points.txt",
            f"[console_scripts]\nmodelslab = {DIST}.__main__:main\n".encode("utf-8"),
        )
        add(zf, f"{dist_info}/top_level.txt", f"{DIST}\n".encode("utf-8"))

        # RECORD lists every file including itself, with its own hash left blank.
        buffer = io.StringIO()
        writer = csv.writer(buffer, lineterminator="\n")
        for arcname, digest, size in records:
            writer.writerow([arcname, digest, size])
        writer.writerow([f"{dist_info}/RECORD", "", ""])

        record_info = zipfile.ZipInfo(f"{dist_info}/RECORD", date_time=(1980, 1, 1, 0, 0, 0))
        record_info.external_attr = ((stat.S_IFREG | 0o644) << 16) | 0o600
        record_info.compress_type = zipfile.ZIP_DEFLATED
        zf.writestr(record_info, buffer.getvalue())

    return wheel_path


def normalise_version(raw: str) -> str:
    """Turn a git tag into a PEP 440 version.

    Tags are semver (`v1.2.3`, `v1.2.3-rc1`); PEP 440 is not. The hyphen matters
    more than it looks: it is the field separator in a wheel filename, so
    `1.2.3-rc1` produces `modelslab_cli-1.2.3-rc1-py3-none-*.whl`, which pip
    parses as version `1.2.3` with a build tag of `rc1` — and build tags must
    start with a digit, so the file is simply invalid.
    """
    version = raw.lstrip("v").strip()

    match = re.fullmatch(
        r"(\d+\.\d+\.\d+)"
        r"(?:[-.]?(a|b|rc|alpha|beta)\.?(\d+))?",
        version,
        re.IGNORECASE,
    )

    if not match:
        raise SystemExit(
            f'refusing to build: "{raw}" is not a version this can express in PEP 440'
        )

    release, phase, number = match.groups()

    if not phase:
        return release

    canonical = {"alpha": "a", "beta": "b", "a": "a", "b": "b", "rc": "rc"}[phase.lower()]

    return f"{release}{canonical}{number}"


def main() -> int:
    if len(sys.argv) < 3:
        sys.stderr.write("usage: build.py <version> <artifacts-dir> [out-dir]\n")
        return 1

    version = normalise_version(sys.argv[1])
    artifacts = Path(sys.argv[2])
    out_dir = Path(sys.argv[3]) if len(sys.argv) > 3 else Path("dist/pypi")

    for go_target, platform_tag in TARGETS:
        path = build_wheel(version, artifacts, out_dir, go_target, platform_tag)
        print(f"built {path.name}")

    print(f"output: {out_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
