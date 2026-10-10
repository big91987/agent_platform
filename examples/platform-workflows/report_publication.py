#!/usr/bin/env python3
"""Publish same-Run report assets using a fixed private repository capability."""

import argparse
import fcntl
import hashlib
import io
import json
import os
import re
import stat
import subprocess
import sys
import urllib.parse
import uuid
import zipfile
from pathlib import Path

LIMIT = 64 * 1024 * 1024
SECRET = re.compile(
    rb"(?:sk-[A-Za-z0-9_-]{16,}|gh[pousr]_[A-Za-z0-9]{20,}|-----BEGIN [A-Z ]*PRIVATE KEY-----)"
)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def directory(path, create=False):
    """Open every absolute directory through no-follow descriptors."""
    path = Path(path).absolute()
    fd = os.open("/", os.O_RDONLY | os.O_DIRECTORY)
    try:
        for name in path.parts[1:]:
            if name in (".", ".."):
                raise ValueError("unsafe directory")
            if create:
                try:
                    os.mkdir(name, mode=0o700, dir_fd=fd)
                except FileExistsError:
                    pass
            child = os.open(
                name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd
            )
            os.close(fd)
            fd = child
        return fd
    except BaseException:
        os.close(fd)
        raise


def read_file(root, relative):
    parts = Path(relative).parts
    if (
        not parts
        or Path(relative).is_absolute()
        or any(x in (".", "..") for x in parts)
    ):
        raise ValueError("unsafe publication path")
    fd = directory(root)
    try:
        for name in parts[:-1]:
            child = os.open(
                name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd
            )
            os.close(fd)
            fd = child
        source = os.open(
            parts[-1], os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=fd
        )
        with os.fdopen(source, "rb") as file:
            info = os.fstat(file.fileno())
            if not stat.S_ISREG(info.st_mode) or info.st_size > LIMIT:
                raise ValueError("publication file must be regular and bounded")
            data = file.read(LIMIT + 1)
            if len(data) > LIMIT:
                raise ValueError("publication file grew beyond limit")
            return data
    finally:
        os.close(fd)


def safe_path(value, prefix):
    if not isinstance(value, str) or not value.startswith(prefix):
        raise ValueError("publication files must belong to this Run")
    parts = Path(value).parts
    if any(
        not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,180}", part) for part in parts
    ):
        raise ValueError("unsafe publication file name")
    lower = value.lower()
    if lower.endswith((".har", ".pem", ".key")) or "trace.zip" in lower:
        raise ValueError("private traces or credentials cannot be published")
    return value


def verified(root, item, prefix):
    path = safe_path(item.get("path"), prefix)
    if (
        type(item.get("bytes")) is not int
        or not 0 <= item["bytes"] <= LIMIT
        or not re.fullmatch(r"[a-f0-9]{64}", str(item.get("sha256", "")))
    ):
        raise ValueError("bytes and SHA256 required for each publication file")
    data = read_file(root, path)
    if len(data) != item["bytes"] or digest(data) != item["sha256"]:
        raise ValueError("publication source digest changed")
    if SECRET.search(data):
        raise ValueError("publication file contains a credential pattern")
    return path, data


def compressed(data, path):
    return Path(path).suffix.lower() in (
        ".zip",
        ".gz",
        ".tar",
        ".bz2",
        ".xz",
        ".7z",
        ".rar",
    ) or data.startswith(
        (
            b"PK\x03\x04",
            b"PK\x05\x06",
            b"\x1f\x8b",
            b"BZh",
            b"\xfd7zXZ",
            b"7z\xbc\xaf\x27\x1c",
            b"Rar!",
        )
    )


def prepared_head(run, current):
    for step in reversed(run.get("previous_results", [])):
        if step.get("node_id") != "prepare":
            continue
        receipt = step.get("connector_receipt", {})
        if step.get("status") != "completed" or receipt.get("exit_code") != 0:
            raise ValueError("latest prepare receipt is failed or uncertain")
        try:
            head = json.loads(receipt.get("output", "")).get("head")
        except (ValueError, TypeError, AttributeError):
            raise ValueError("prepare receipt must identify the fixed head") from None
        if not re.fullmatch(r"[a-f0-9]{40}", str(head)) or head != current:
            raise ValueError("current head differs from the trusted prepare receipt")
        return head
    raise ValueError("successful prepare receipt required before report publication")


def package(root, run, head):
    prefix = "docs/workflow/runs/" + run + "/"
    raw = read_file(root, prefix + "report-publication.json")
    description = json.loads(raw)
    if description.get("product_head") != head:
        raise ValueError("publication product head differs from fixed workspace")
    title = description.get("title", "")
    if (
        not isinstance(title, str)
        or not re.fullmatch(r"【[^\n\r】]+】[^\n\r]+", title)
        or len(title.encode()) > 200
    ):
        raise ValueError("publication title needs a category and concrete goal")
    if SECRET.search(title.encode()):
        raise ValueError("publication title contains a credential pattern")
    items = description.get("files")
    if not isinstance(items, list) or not 1 <= len(items) <= 16:
        raise ValueError("publication needs 1–16 white-listed attachments")
    files = {}
    names = set()
    for item in items:
        path, data = verified(root, item, prefix)
        name = Path(path).name
        if (
            path in files
            or name in names
            or Path(path).suffix.lower()
            not in (".md", ".html", ".json", ".zip", ".txt", ".csv", ".png")
        ):
            raise ValueError("duplicate or unsupported attachment")
        if path != description.get("archive") and compressed(data, path):
            raise ValueError("only the designated evidence archive may be compressed")
        files[path] = data
        names.add(name)
    if sum(map(len, files.values())) > 256 * 1024 * 1024:
        raise ValueError("publication attachments exceed total limit")
    for key in ("body_file", "dashboard", "whitelist", "archive"):
        if description.get(key) not in files:
            raise ValueError(
                "report, dashboard, whitelist and archive must be attachments"
            )
    body = files[description["body_file"]].decode("utf-8")
    if len(body.encode()) > 100000:
        raise ValueError("report body too large")
    whitelist = json.loads(files[description["whitelist"]]).get("files", [])
    if not isinstance(whitelist, list) or not 1 <= len(whitelist) <= 2000:
        raise ValueError("bounded evidence whitelist required")
    expected = {}
    total = 0
    for item in whitelist:
        path, data = verified(root, item, prefix)
        if compressed(data, path):
            raise ValueError("nested evidence archives are not supported")
        if path in expected:
            raise ValueError("duplicate evidence whitelist path")
        expected[path] = (len(data), digest(data))
        total += len(data)
        if total > 512 * 1024 * 1024:
            raise ValueError("evidence whitelist exceeds total limit")
    with zipfile.ZipFile(io.BytesIO(files[description["archive"]])) as archive:
        members = archive.infolist()
        if len(members) != len(expected) or {x.filename for x in members} != set(
            expected
        ):
            raise ValueError("archive does not match evidence whitelist")
        for member in members:
            if (
                member.is_dir()
                or member.file_size != expected[member.filename][0]
                or member.flag_bits & 1
            ):
                raise ValueError("archive member differs from whitelist")
            data = archive.read(member)
            if digest(data) != expected[member.filename][1] or SECRET.search(data):
                raise ValueError("archive digest or credential check failed")
    return description, files, body, digest(raw)


def save_state(path, value):
    temporary = path.with_name(path.name + "." + uuid.uuid4().hex + ".tmp")
    fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "w") as file:
        json.dump(value, file)
        file.flush()
        os.fsync(file.fileno())
    os.replace(temporary, path)
    fd = directory(path.parent)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


class GitHub:
    def call(self, method, path, body=None, data_file=None):
        command = [
            "gh",
            "api",
            "--hostname",
            "github.com",
            "--include",
            "--method",
            method,
            path,
        ]
        if body is not None:
            command += ["--input", "-"]
        if data_file is not None:
            command += [
                "--header",
                "Content-Type: application/octet-stream",
                "--input",
                str(data_file),
            ]
        try:
            result = subprocess.run(
                command,
                input=None if body is None else json.dumps(body).encode(),
                capture_output=True,
                timeout=90,
            )
        except (subprocess.TimeoutExpired, OSError):
            raise RuntimeError(
                "GitHub transport interrupted; inspect receipt before retry"
            ) from None
        output = result.stdout.decode("utf-8")
        header, separator, payload = output.replace("\r\n", "\n").partition("\n\n")
        status = re.search(r"^HTTP/\S+ (\d+)", header)
        if method == "GET" and status and status[1] == "404":
            return None
        if (
            result.returncode
            or not separator
            or not status
            or not 200 <= int(status[1]) < 300
        ):
            raise RuntimeError(
                "GitHub request failed or response unknown; no raw remote content exported"
            )
        return json.loads(payload)


def publish(root, run, repository, head, state_root, remote):
    if (
        not re.fullmatch(r"[a-f0-9]{32}", run)
        or not re.fullmatch(r"[a-f0-9]{40}", head)
        or not re.fullmatch(r"[\w.-]+/[\w.-]+", repository)
    ):
        raise ValueError("valid Run, repository and product head required")
    root = Path(root).resolve()
    state_root = Path(state_root).resolve()
    if state_root.is_relative_to(root):
        raise ValueError("publication state must be outside Agent workspace")
    description, files, report_body, binding = package(root, run, head)
    state_dir = state_root / run
    fd = directory(state_dir, create=True)
    os.close(fd)
    lock = os.open(state_dir / "lock", os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        return publish_locked(
            state_dir,
            description,
            files,
            report_body,
            binding,
            run,
            repository,
            head,
            remote,
        )
    finally:
        os.close(lock)


def publish_locked(
    state_dir, description, files, report_body, binding, run, repository, head, remote
):
    state_path = state_dir / "state.json"
    state = (
        json.loads(read_file(state_dir, "state.json"))
        if state_path.exists()
        else {
            "binding": binding,
            "repository": repository,
            "create_attempted": False,
            "assets_attempted": [],
        }
    )
    if state.get("binding") != binding or state.get("repository") != repository:
        raise ValueError(
            "publication binding changed after an attempt; preserve original state"
        )
    repo = remote.call("GET", "repos/" + repository)
    if not repo or repo.get("private") is not True:
        raise ValueError("this publisher requires the authorized private repository")
    tag = "workflow-report-" + run
    marker = "<!-- agent-platform-report:" + run + ":" + binding + " -->"
    body = (
        "报告与下载附件的发布回执；不改变原测试结论，不代表产品验收或部署。Dashboard为下载HTML，请按报告说明解压证据包查看。\n\n"
        + report_body
        + "\n\n"
        + marker
    )
    release = remote.call("GET", "repos/" + repository + "/releases/tags/" + tag)
    if release is None:
        if state["create_attempted"]:
            raise ValueError("release creation remains unknown; no POST replay")
        state["create_attempted"] = True
        save_state(state_path, state)
        release = remote.call(
            "POST",
            "repos/" + repository + "/releases",
            body={
                "tag_name": tag,
                "target_commitish": head,
                "name": description["title"],
                "body": body,
                "draft": False,
                "prerelease": False,
                "make_latest": "false",
            },
        )
    if (
        release.get("body") != body
        or release.get("target_commitish") != head
        or release.get("tag_name") != tag
        or release.get("name") != description["title"]
        or release.get("draft")
    ):
        raise ValueError("existing release differs from immutable publication binding")
    release_id = release.get("id")
    if type(release_id) is not int or release_id <= 0:
        raise ValueError("invalid release receipt")
    assets = remote.call(
        "GET", f"repos/{repository}/releases/{release_id}/assets?per_page=100"
    )
    if (
        not isinstance(assets, list)
        or len(assets) > 16
        or len({x.get("name") for x in assets}) != len(assets)
    ):
        raise ValueError("invalid or duplicate remote assets")
    expected_names = {Path(path).name for path in files}
    if any(x.get("name") not in expected_names for x in assets):
        raise ValueError("unexpected remote assets; do not overwrite or delete")
    receipts = []
    for path, data in files.items():
        name = Path(path).name
        asset = next((x for x in assets if x.get("name") == name), None)
        if asset is None:
            if name in state["assets_attempted"]:
                raise ValueError("asset upload remains unknown; no POST replay")
            snapshots = state_dir / "assets"
            fd = directory(snapshots, create=True)
            os.close(fd)
            snapshot = snapshots / name
            if snapshot.exists():
                if read_file(snapshots, name) != data:
                    raise ValueError("publication snapshot changed")
            else:
                fd = os.open(
                    snapshot,
                    os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
                    0o600,
                )
                with os.fdopen(fd, "wb") as file:
                    file.write(data)
                    file.flush()
                    os.fsync(file.fileno())
            state["assets_attempted"].append(name)
            save_state(state_path, state)
            endpoint = (
                f"https://uploads.github.com/repos/{repository}/releases/{release_id}/assets?name="
                + urllib.parse.quote(name, safe="")
            )
            remote.call("POST", endpoint, data_file=snapshot)
            actual = remote.call(
                "GET", f"repos/{repository}/releases/{release_id}/assets?per_page=100"
            )
            asset = next((x for x in actual if x.get("name") == name), None)
        if (
            not asset
            or asset.get("state") != "uploaded"
            or asset.get("size") != len(data)
            or asset.get("digest") != "sha256:" + digest(data)
        ):
            raise ValueError("remote asset digest or size differs; no overwrite")
        url = asset.get("browser_download_url", "")
        if not url.startswith(
            "https://github.com/" + repository + "/releases/download/"
        ):
            raise ValueError("foreign asset receipt URL")
        receipts.append(
            {"name": name, "bytes": len(data), "sha256": digest(data), "url": url}
        )
    url = release.get("html_url", "")
    if url != "https://github.com/" + repository + "/releases/tag/" + tag:
        raise ValueError("foreign release receipt URL")
    return {
        "release_url": url,
        "release_id": release_id,
        "product_head": head,
        "remote_digest_verified": True,
        "dashboard_delivery": "download attachment",
        "assets": receipts,
    }


def write_receipt(root, run, result):
    relative = "docs/workflow/runs/" + run
    fd = directory(Path(root) / relative)
    temporary = "report-publication-receipt.md." + uuid.uuid4().hex + ".tmp"
    text = "**测试报告与下载附件已发布，产品结论以原报告为准。**\n\n"
    text += "[查看完整报告与附件](" + result["release_url"] + ")\n\n"
    text += "Dashboard为下载HTML附件，不是在线站点；附件大小及SHA256已逐项核验。\n\n"
    text += (
        "\n".join(
            "- [" + item["name"] + "](" + item["url"] + ")" for item in result["assets"]
        )
        + "\n"
    )
    try:
        output = os.open(
            temporary,
            os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
            0o600,
            dir_fd=fd,
        )
        with os.fdopen(output, "w") as file:
            file.write(text)
            file.flush()
            os.fsync(file.fileno())
        os.replace(
            temporary, "report-publication-receipt.md", src_dir_fd=fd, dst_dir_fd=fd
        )
        os.fsync(fd)
    finally:
        os.close(fd)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--repository", required=True)
    parser.add_argument("--state-root", type=Path, required=True)
    args = parser.parse_args()
    run = json.load(sys.stdin)
    root = Path(run["workspace_path"]).resolve()
    if root != Path.cwd().resolve():
        raise ValueError("Run workspace and command directory differ")
    from repository import git, remote_repository

    if remote_repository(git(root, "remote", "get-url", "origin")) != args.repository:
        raise ValueError("workspace origin differs from configured repository")
    result = publish(
        root,
        run["run_id"],
        args.repository,
        prepared_head(run, git(root, "rev-parse", "HEAD")),
        args.state_root,
        GitHub(),
    )
    write_receipt(root, run["run_id"], result)
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except (
        ValueError,
        RuntimeError,
        OSError,
        zipfile.BadZipFile,
        subprocess.SubprocessError,
    ) as error:
        # Errors contain only protocol facts, not file contents or GitHub bodies.
        print(
            str(error)
            if isinstance(error, (ValueError, RuntimeError))
            else type(error).__name__
            + ": report publication stopped; inspect private command evidence",
            file=sys.stderr,
        )
        raise SystemExit(1)
