"""Bounded, checksum-pinned input bundles for the trusted Workflow tools.

Only the source URL and original ZIP hash are authoritative. An edited expanded
manifest never authorizes new bytes. Contents are data and are never executed.
"""

import fcntl
import hashlib
import json
import os
import re
import selectors
import shutil
import stat
import subprocess
import tempfile
import time
import unicodedata
import urllib.parse
import urllib.request
import zipfile
from pathlib import Path

MAX_ARCHIVE = 16 * 1024 * 1024
MAX_FILE = 16 * 1024 * 1024
MAX_EXPANDED = 64 * 1024 * 1024
MAX_FILES = 256
TIMEOUT = 90
SHA = re.compile(r"[a-f0-9]{64}")


def _unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("Duplicate JSON field")
        result[key] = value
    return result


def validate_description(value, repository=None):
    try:
        description = json.loads(value, object_pairs_hook=_unique)
    except (TypeError, json.JSONDecodeError) as error:
        raise ValueError("Invalid material descriptor JSON") from error
    if (
        not isinstance(description, dict)
        or set(description) != {"url", "sha256", "version"}
        or not all(isinstance(v, str) for v in description.values())
        or not SHA.fullmatch(description["sha256"])
        or not description["version"].strip()
        or len(description["version"]) > 128
        or any(ord(c) < 32 for c in description["version"])
    ):
        raise ValueError("Material requires URL, SHA-256 and version")
    url = description["url"]
    asset = re.fullmatch(
        r"https://api\.github\.com/repos/([\w.-]+/[\w.-]+)/releases/assets/([1-9][0-9]*)",
        url,
        re.ASCII,
    )
    attachment = re.fullmatch(
        r"https://github\.com/user-attachments/files/[1-9][0-9]*/[A-Za-z0-9_.-]+\.zip",
        url,
    )
    if not asset and not attachment:
        raise ValueError("Unsupported material URL; use a pinned GitHub ZIP asset")
    if asset and repository is not None and asset[1] != repository:
        raise ValueError("Material Release Asset must belong to configured repository")
    return description


class _GitHubRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, message, headers, url):
        parsed = urllib.parse.urlsplit(url)
        if (
            parsed.scheme != "https"
            or parsed.username
            or parsed.password
            or parsed.port not in (None, 443)
            or not (
                parsed.hostname == "github.com"
                or parsed.hostname == "githubusercontent.com"
                or (parsed.hostname or "").endswith(".githubusercontent.com")
            )
        ):
            raise ValueError("Material redirect left approved GitHub download hosts")
        return super().redirect_request(request, fp, code, message, headers, url)


def _copy_bounded(source, target, limit):
    total = 0
    while block := source.read(65536):
        total += len(block)
        if total > limit:
            raise ValueError("Material download/file exceeds size limit")
        target.write(block)
    return total


def download(description, path):
    """Private assets use gh's authenticated API; Issue uploads carry no token."""
    url = description["url"]
    if url.startswith("https://api.github.com/"):
        with path.open("xb") as output, tempfile.TemporaryFile() as errors:
            process = subprocess.Popen(
                [
                    "gh",
                    "api",
                    "--hostname",
                    "github.com",
                    url.removeprefix("https://api.github.com/"),
                    "-H",
                    "Accept: application/octet-stream",
                ],
                stdout=subprocess.PIPE,
                stderr=errors,
            )
            deadline = time.monotonic() + TIMEOUT
            total = 0
            try:
                with selectors.DefaultSelector() as selector:
                    selector.register(process.stdout, selectors.EVENT_READ)
                    while True:
                        remaining = deadline - time.monotonic()
                        if remaining <= 0:
                            raise TimeoutError("GitHub material download timed out")
                        if not selector.select(remaining):
                            raise TimeoutError("GitHub material download timed out")
                        block = os.read(process.stdout.fileno(), 65536)
                        if not block:
                            break
                        total += len(block)
                        if total > MAX_ARCHIVE:
                            raise ValueError("Material download exceeds size limit")
                        output.write(block)
                if process.wait(timeout=max(0.01, deadline - time.monotonic())) != 0:
                    # gh stderr can contain signed URLs or credentials with debug enabled.
                    raise ValueError(
                        "GitHub material download failed; check asset access"
                    )
            finally:
                if process.poll() is None:
                    process.kill()
                process.wait()
                process.stdout.close()
    else:
        # Deliberately do not install an auth handler or copy GH_TOKEN to headers.
        opener = urllib.request.build_opener(_GitHubRedirect())
        with opener.open(url, timeout=TIMEOUT) as response, path.open("xb") as output:
            _copy_bounded(response, output, MAX_ARCHIVE)


def _path(name):
    if (
        not isinstance(name, str)
        or not name
        or len(name) > 512
        or name.startswith("/")
        or "\\" in name
        or ":" in name
        or any(ord(c) < 32 for c in name)
        or any(part in ("", ".", "..") for part in name.split("/"))
        or len(name.split("/")) > 16
    ):
        raise ValueError("Unsafe material path")
    return name


def _regular(path, base):
    try:
        relative = path.relative_to(base)
        current = base
        for part in relative.parts:
            current = current / part
            if current.is_symlink():
                raise ValueError("Material storage contains a symlink")
        info = path.stat()
        if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
            raise ValueError("Material input is not an independent regular file")
        return info
    except OSError as error:
        raise ValueError("Material input is missing or unreadable") from error


def _storage(workspace, run):
    run_id = run.get("run_id", "")
    if not re.fullmatch(r"[a-f0-9]{32}", run_id):
        raise ValueError("Material requires a valid Run ID")
    description = validate_description(run["parameters"]["material"])
    current = workspace
    for part in [".workflow-input", run_id, description["sha256"]]:
        current /= part
        if current.is_symlink() or (current.exists() and not current.is_dir()):
            raise ValueError("Material storage must be a private directory, not a link")
    return description, current


def _inspect(archive, description, destination=None):
    if (
        archive.stat().st_size > MAX_ARCHIVE
        or hashlib.sha256(archive.read_bytes()).hexdigest() != description["sha256"]
    ):
        raise ValueError("Material ZIP checksum or size mismatch")
    try:
        with zipfile.ZipFile(archive) as z:
            entries = {}
            folded = set()
            prefixes = {}
            directories = set()
            total = 0
            for info in z.infolist():
                name = _path(info.filename[:-1] if info.is_dir() else info.filename)
                portable = unicodedata.normalize("NFC", name).casefold()
                if portable in folded:
                    raise ValueError("Duplicate/colliding material archive path")
                folded.add(portable)
                parts = name.split("/")
                for count in range(1, len(parts) + 1):
                    prefix = "/".join(parts[:count])
                    identity = unicodedata.normalize("NFC", prefix).casefold()
                    if identity in prefixes and prefixes[identity] != prefix:
                        raise ValueError("Material directory/name prefixes collide")
                    prefixes[identity] = prefix
                mode = stat.S_IFMT(info.external_attr >> 16)
                if info.flag_bits & 1 or mode not in (0, stat.S_IFREG, stat.S_IFDIR):
                    raise ValueError(
                        "Material links, special files or encryption unsupported"
                    )
                if info.is_dir():
                    directories.add(name)
                    continue
                if mode == stat.S_IFDIR or info.file_size > MAX_FILE:
                    raise ValueError("Material file size/type invalid")
                entries[name] = info
                total += info.file_size
            if (
                len(entries) > MAX_FILES + 1
                or total > MAX_EXPANDED
                or "manifest.json" not in entries
            ):
                raise ValueError("Material count/expanded size/manifest invalid")
            if entries["manifest.json"].file_size > min(MAX_FILE, 1024 * 1024):
                raise ValueError("Material manifest exceeds limit")
            raw_manifest = z.read("manifest.json")
            manifest = json.loads(raw_manifest, object_pairs_hook=_unique)
            if (
                not isinstance(manifest, dict)
                or manifest.get("version") != description["version"]
            ):
                raise ValueError("Material manifest version mismatch")
            records = manifest.get("files")
            if not isinstance(records, list) or not records or len(records) > MAX_FILES:
                raise ValueError("Material manifest requires declared files")
            expected = {}
            for record in records:
                if not isinstance(record, dict) or set(record) != {
                    "path",
                    "bytes",
                    "sha256",
                }:
                    raise ValueError("Invalid material file record")
                name = _path(record["path"])
                if (
                    name == "manifest.json"
                    or name in expected
                    or not isinstance(record["sha256"], str)
                    or not SHA.fullmatch(record["sha256"])
                    or type(record["bytes"]) is not int
                    or not 0 <= record["bytes"] <= MAX_FILE
                ):
                    raise ValueError("Invalid material file identity/size/digest")
                expected[name] = record
            if set(entries) != {"manifest.json", *expected}:
                raise ValueError("Missing or undeclared material file")
            parent_names = {
                str(p) for name in expected for p in Path(name).parents if str(p) != "."
            }
            if not directories <= parent_names:
                raise ValueError("Unrelated material directory")
            if destination:
                destination.mkdir()
            for name, info in entries.items():
                with z.open(info) as source:
                    digest = hashlib.sha256()
                    size = 0
                    output = None
                    try:
                        if destination:
                            target = destination / name
                            target.parent.mkdir(parents=True, exist_ok=True)
                            output = target.open("xb")
                        while block := source.read(65536):
                            size += len(block)
                            if size > MAX_FILE:
                                raise ValueError("Expanded material file exceeds limit")
                            digest.update(block)
                            if output:
                                output.write(block)
                    finally:
                        if output:
                            output.close()
                    if name != "manifest.json" and (
                        size != expected[name]["bytes"]
                        or digest.hexdigest() != expected[name]["sha256"]
                    ):
                        raise ValueError("Material file checksum or size mismatch")
            return expected, raw_manifest
    except (
        zipfile.BadZipFile,
        json.JSONDecodeError,
        UnicodeError,
        OSError,
        RuntimeError,
    ) as error:
        raise ValueError("Invalid or unreadable material archive") from error


def _verify(workspace, directory, description):
    archive = directory / "archive.zip"
    _regular(archive, workspace)
    records, manifest = _inspect(archive, description)
    root = directory / "files"
    if root.is_symlink() or not root.is_dir():
        raise ValueError("Material expanded root missing or linked")
    actual = set()
    for parent, dirs, files in os.walk(root, followlinks=False):
        if any((Path(parent) / name).is_symlink() for name in dirs):
            raise ValueError("Material expanded directory linked")
        for name in files:
            path = Path(parent) / name
            info = _regular(path, workspace)
            relative = path.relative_to(root).as_posix()
            actual.add(relative)
            if relative == "manifest.json":
                if info.st_size > 1024 * 1024 or path.read_bytes() != manifest:
                    raise ValueError("Material expanded manifest changed")
            elif (
                relative not in records
                or info.st_size != records[relative]["bytes"]
                or hashlib.sha256(path.read_bytes()).hexdigest()
                != records[relative]["sha256"]
            ):
                raise ValueError("Material expanded input changed")
    if actual != {"manifest.json", *records}:
        raise ValueError("Material expanded inputs missing")
    return {
        "version": description["version"],
        "sha256": description["sha256"],
        "archive_path": archive.relative_to(workspace).as_posix(),
        "root": root.relative_to(workspace).as_posix(),
        "manifest_path": (root / "manifest.json").relative_to(workspace).as_posix(),
        "files": len(records),
    }


def install_material(workspace, run, repository):
    if not (run.get("parameters") or {}).get("material"):
        return None
    validate_description(run["parameters"]["material"], repository)
    description, directory = _storage(workspace, run)
    directory.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    # POSIX lock belongs to the Run. A killed process releases it, so retries
    # can distinguish orphan staging from an active download without guessing age.
    try:
        fd = os.open(
            directory.parent / ".lock", os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600
        )
    except OSError as error:
        raise ValueError("Material Run lock is unreadable or linked") from error
    with os.fdopen(fd, "a") as lock:
        if (
            not stat.S_ISREG(os.fstat(lock.fileno()).st_mode)
            or os.fstat(lock.fileno()).st_nlink != 1
        ):
            raise ValueError("Material Run lock must be an independent file")
        fcntl.flock(lock, fcntl.LOCK_EX)
        for orphan in directory.parent.glob(".pending-*"):
            if orphan.is_symlink() or not orphan.is_dir():
                orphan.unlink()
            else:
                shutil.rmtree(orphan)
        if directory.exists():
            return _verify(workspace, directory, description)
        with tempfile.TemporaryDirectory(
            prefix=".pending-", dir=directory.parent
        ) as temporary:
            pending = Path(temporary)
            download(description, pending / "archive.zip")
            _inspect(pending / "archive.zip", description, pending / "files")
            _verify(workspace, pending, description)
            pending.rename(directory)
        return _verify(workspace, directory, description)


def verify_material(workspace, run):
    if not (run.get("parameters") or {}).get("material"):
        return
    description, directory = _storage(workspace, run)
    _verify(workspace, directory, description)
