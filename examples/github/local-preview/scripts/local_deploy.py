#!/usr/bin/env python3
"""Trusted, same-machine static release deployment; no user data is copied or deleted."""

import argparse
import contextlib
import fcntl
import hashlib
import http.server
import io
import json
import mimetypes
import os
import re
import shutil
import subprocess
import tarfile
import tempfile
import time
import urllib.parse
import urllib.request
from pathlib import Path


def git(repo, *args):
    return subprocess.check_output(["git", "-C", str(repo), *args], text=True).strip()


def save(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
    temporary.replace(path)


def fingerprint(path):
    digest = hashlib.sha256()
    for file in sorted(path.rglob("*")):
        if file.is_symlink():
            raise ValueError("release cannot contain symlinks")
        if file.is_file() and file.name != ".release.json":
            digest.update(file.relative_to(path).as_posix().encode() + b"\0")
            digest.update(file.read_bytes() + b"\0")
    return digest.hexdigest()


def declare(repo, impact, notes, migration_plan="", compatible_from=None):
    if (
        impact not in ("none", "migration", "destructive", "unknown")
        or not notes.strip()
    ):
        raise ValueError("explicit data impact and notes are required")
    if compatible_from is None:
        result = subprocess.run(
            ["git", "-C", str(repo), "rev-parse", "--verify", "HEAD"],
            capture_output=True,
            text=True,
        )
        compatible_from = result.stdout.strip() if result.returncode == 0 else ""
    save(
        repo / "deploy/release.json",
        {
            "app_sha256": fingerprint(repo / "app"),
            "compatible_from": compatible_from,
            "data_change": impact,
            "notes": notes,
            "migration_plan": migration_plan,
        },
    )


def current(root):
    link = root / "current"
    if not link.is_symlink():
        return None
    target = link.resolve()
    if target.parent != (root / "releases").resolve():
        raise ValueError("current points outside release storage")
    return target.name


@contextlib.contextmanager
def locked(root):
    root.mkdir(parents=True, exist_ok=True)
    with (root / "deploy.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        yield


def extract(repo, sha, directory):
    raw = subprocess.check_output(["git", "-C", str(repo), "archive", sha])
    with tarfile.open(fileobj=io.BytesIO(raw)) as archive:
        for member in archive.getmembers():
            if member.name.split("/")[0] not in ("app", "deploy", "tests"):
                continue
            if not (member.isfile() or member.isdir()):
                raise ValueError("release input contains a link or special file")
            archive.extract(member, directory, filter="data")


def prepare(root, repo, sha, force_review=False):
    if not re.fullmatch(r"[0-9a-f]{40}", sha):
        raise ValueError("a full commit SHA is required")
    with locked(root), tempfile.TemporaryDirectory(dir=root) as temporary:
        source = Path(temporary).resolve()
        extract(repo, sha, source)
        app = source / "app"
        if not all(
            (app / name).is_file() for name in ("index.html", "app.js", "styles.css")
        ):
            raise ValueError("unsupported application: expected static app entry files")
        for file in app.rglob("*.js"):
            subprocess.run(
                ["node", "--check", str(file)], check=True, capture_output=True
            )
        tests = sorted((source / "tests").glob("*.test.cjs"))
        if tests:
            subprocess.run(
                ["node", "--test", *map(str, tests)], check=True, capture_output=True
            )
        checksum = fingerprint(app)
        previous = current(root)
        try:
            declaration = json.loads((source / "deploy/release.json").read_text())
        except (FileNotFoundError, ValueError):
            declaration = {}
        reasons = []
        if declaration.get("app_sha256") != checksum:
            reasons.append("发布声明缺失或未覆盖这份前端代码，数据兼容性未确认。")
        impact = declaration.get("data_change", "unknown")
        if impact != "none":
            reasons.append("数据影响：" + str(impact))
        if not declaration.get("notes", "").strip():
            reasons.append("没有数据兼容性说明。")
        baseline = declaration.get("compatible_from", "")
        if previous and previous != sha:
            matches = False
            if re.fullmatch(r"[0-9a-f]{40}", baseline):
                try:
                    matches = git(repo, "rev-parse", baseline + ":app") == git(
                        repo, "rev-parse", previous + ":app"
                    )
                except subprocess.CalledProcessError:
                    pass
            if not matches:
                reasons.append(
                    "兼容性声明的基线与当前已部署 app 不一致或无法核实，需要检查完整数据变化。"
                )
        migration_text = ""
        if impact in ("migration", "destructive"):
            migration = (source / declaration.get("migration_plan", "")).resolve()
            if (
                not migration.is_relative_to(source / "deploy")
                or not migration.is_file()
            ):
                raise ValueError(
                    "migration/destructive release requires a deploy/ plan with backup, validation and recovery instructions"
                )
            migration_text = migration.read_text()
            if not migration_text.strip():
                raise ValueError("migration plan is empty")
        if previous and previous != sha:
            changed = git(repo, "diff", "--name-only", previous, sha).splitlines()
            if any(
                not name.startswith(("app/", "docs/", "tests/", "deploy/"))
                and name not in ("README.md", "AGENTS.md", ".gitignore")
                for name in changed
            ):
                reasons.append("部署配置、运行依赖或应用架构有变化，需要人工检查。")
        if force_review:
            reasons.append("本次由操作者主动要求部署审批。")
        release = root / "releases" / sha
        release.parent.mkdir(parents=True, exist_ok=True)
        if release.exists():
            if fingerprint(release) != checksum:
                raise ValueError("prepared release changed; refusing to overwrite")
        else:
            shutil.copytree(app, release)
            save(release / ".release.json", {"sha": sha, "prepared_at": time.time()})
        return {
            "sha": sha,
            "from_sha": previous,
            "app_sha256": checksum,
            "requires_approval": bool(reasons),
            "reasons": reasons,
            "data_notes": declaration.get("notes", ""),
            "compatible_from": baseline,
            "migration_plan": migration_text,
        }


def point(root, name, sha):
    link = root / (name + ".next")
    link.unlink(missing_ok=True)
    link.symlink_to(Path("releases") / sha)
    link.replace(root / name)


def activate(root, plan, approved=False, health=None):
    with locked(root):
        sha = plan["sha"]
        if not re.fullmatch(r"[0-9a-f]{40}", sha):
            raise ValueError("invalid release SHA")
        if plan["requires_approval"] and not approved:
            raise ValueError("deployment approval required")
        if fingerprint(root / "releases" / sha) != plan["app_sha256"]:
            raise ValueError("prepared release changed")
        old = current(root)
        if old == sha:
            return
        if old != plan["from_sha"]:
            raise ValueError("stale plan: current release changed; prepare again")
        if old:
            save(root / "published" / (old + ".json"), {"sha": old})
        point(root, "current", sha)
        try:
            if health is not None and not health():
                raise RuntimeError("deployment health check failed")
        except BaseException:
            if old:
                point(root, "current", old)
            else:
                (root / "current").unlink(missing_ok=True)
            raise
        if old:
            point(root, "previous", old)
        save(root / "published" / (sha + ".json"), {"sha": sha})
        save(
            root / "deployed.json",
            {"sha": sha, "previous_sha": old, "deployed_at": time.time()},
        )


def preview_settings(root):
    path = root / "preview.json"
    return json.loads(path.read_text()) if path.exists() else {}


def server(root, port=5533):
    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            path = urllib.parse.unquote(urllib.parse.urlsplit(self.path).path)
            sha = current(root)
            if not sha:
                self.send_error(503, "No release is deployed")
                return
            if path == "/__deployment.json":
                body = json.dumps(
                    {
                        "sha": sha,
                        "url": (
                            "https://github.com/"
                            + preview_settings(root)["repository"]
                            + "/commit/"
                            + sha
                        )
                        if preview_settings(root).get("repository")
                        else "",
                    }
                ).encode()
                content_type = "application/json"
            else:
                relative = path.lstrip("/") or "index.html"
                if relative.startswith("_releases/"):
                    _, version, *parts = relative.split("/")
                    if not re.fullmatch(r"[0-9a-f]{40}", version):
                        self.send_error(404)
                        return
                    if (
                        version != sha
                        and not (root / "published" / (version + ".json")).is_file()
                    ):
                        self.send_error(404)
                        return
                    sha, relative = version, "/".join(parts) or "index.html"
                release = (root / "releases" / sha).resolve()
                file = (release / relative).resolve()
                if (
                    not file.is_relative_to(release)
                    or any(part.startswith(".") for part in Path(relative).parts)
                    or not file.is_file()
                ):
                    self.send_error(404)
                    return
                body = file.read_bytes()
                content_type = (
                    mimetypes.guess_type(file.name)[0] or "application/octet-stream"
                )
                if file.name == "index.html":
                    body = body.replace(
                        b"<head>", f'<head><base href="/_releases/{sha}/">'.encode(), 1
                    )
            self.send_response(200)
            self.send_header(
                "Content-Type",
                content_type
                + ("; charset=utf-8" if content_type.startswith("text/") else ""),
            )
            self.send_header("Content-Length", str(len(body)))
            self.send_header("Cache-Control", "no-store")
            self.send_header("X-Content-Type-Options", "nosniff")
            self.end_headers()
            self.wfile.write(body)

    return http.server.ThreadingHTTPServer(("127.0.0.1", port), Handler)


def healthcheck(sha, port=5533):
    try:
        with urllib.request.urlopen(
            f"http://127.0.0.1:{port}/__deployment.json", timeout=5
        ) as response:
            if json.load(response)["sha"] != sha:
                return False
        with urllib.request.urlopen(f"http://127.0.0.1:{port}/", timeout=5) as response:
            return b"<html" in response.read().lower()
    except (OSError, ValueError):
        return False


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=["declare", "prepare", "activate", "serve"])
    parser.add_argument("--root", type=Path)
    parser.add_argument("--repo", type=Path, default=Path("."))
    parser.add_argument("--sha")
    parser.add_argument("--plan", type=Path)
    parser.add_argument("--impact", default="none")
    parser.add_argument("--notes", default="")
    parser.add_argument("--migration-plan", default="")
    parser.add_argument(
        "--compatible-from",
        help="Commit whose stored data was checked for compatibility; defaults to HEAD",
    )
    parser.add_argument("--review", action="store_true")
    parser.add_argument("--approved", action="store_true")
    args = parser.parse_args()
    if args.command == "declare":
        declare(
            args.repo.resolve(),
            args.impact,
            args.notes,
            args.migration_plan,
            args.compatible_from,
        )
    elif args.command == "serve":
        root = args.root.resolve()
        server(root, preview_settings(root).get("port", 5533)).serve_forever()
    else:
        root = args.root.resolve()
        repo = root / "repository"
        port = preview_settings(root).get("port", 5533)
        git(repo, "fetch", "origin", "main")
        latest = git(repo, "rev-parse", "refs/remotes/origin/main")
        if args.command == "prepare":
            if args.sha != latest:
                raise ValueError(
                    "superseded: request must deploy the current main commit"
                )
            plan = prepare(root, repo, args.sha, args.review)
            save(args.plan, plan)
            if os.environ.get("GITHUB_OUTPUT"):
                with open(os.environ["GITHUB_OUTPUT"], "a") as output:
                    output.write(
                        "review=" + str(plan["requires_approval"]).lower() + "\n"
                    )
            if os.environ.get("GITHUB_STEP_SUMMARY"):
                with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
                    summary.write(
                        f"## 本机 {port} 部署\n\n目标版本：`{plan['sha']}`\n\n当前版本：`{plan['from_sha']}`\n\n"
                        + "\n".join("- " + reason for reason in plan["reasons"])
                        + "\n\n"
                        + plan["data_notes"]
                        + "\n\n"
                        + plan["migration_plan"]
                    )
            print(json.dumps(plan, ensure_ascii=False))
        else:
            plan = json.loads(args.plan.read_text())
            if plan["sha"] != latest:
                raise ValueError(
                    "superseded: main changed while awaiting deployment; prepare a new run"
                )
            activate(
                root,
                plan,
                approved=args.approved,
                health=lambda: healthcheck(plan["sha"], port),
            )
            print("Deployed " + plan["sha"] + f" at http://127.0.0.1:{port}")


if __name__ == "__main__":
    main()
