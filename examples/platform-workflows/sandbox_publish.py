#!/usr/bin/env python3
"""Publish the Agent's reviewed files and report through the repository boundary."""

import argparse
import json
import os
import re
import subprocess
import urllib.parse
import urllib.request
from pathlib import Path


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise ValueError("GitHub redirected an authenticated request")


def publish(repository, report):
    root = Path.cwd().resolve()
    value = json.loads(report.read_text())
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository):
        raise ValueError("invalid repository")
    title, body, files = value["title"], value["body"], value["files"]
    if (
        not re.fullmatch(
            r"【(?:产品功能|产品修复|产品测试|Harness验证|Harness修复|交付维护)】[^\r\n]{1,180}",
            title,
        )
        or not isinstance(body, str)
        or not body.strip()
        or len(body) > 60000
    ):
        raise ValueError("report must describe the actual change and evidence")
    if not isinstance(files, list) or not 1 <= len(files) <= 100:
        raise ValueError("provide reviewed repository files")
    for name in files:
        if (
            not isinstance(name, str)
            or name.startswith((".", "/"))
            or ".." in Path(name).parts
        ):
            raise ValueError("invalid reviewed file")
        path = root / name
        if (
            not path.is_file()
            or any(
                part.is_symlink()
                for part in [path, *path.parents]
                if part != root.parent
            )
            or not path.resolve().is_relative_to(root)
        ):
            raise ValueError("reviewed file is not a regular workspace file")

    def git(*args):
        return subprocess.check_output(
            ["git", "-c", "core.hooksPath=/dev/null", *args],
            cwd=root,
            text=True,
            timeout=60,
        ).strip()

    branch = git("branch", "--show-current")
    if not re.fullmatch(r"codex/run-[0-9a-f]{12}", branch):
        raise ValueError("publish only the Run-owned branch")
    if (
        git("remote", "get-url", "origin")
        != "https://github.com/" + repository + ".git"
    ):
        raise ValueError("repository differs from prepared origin")
    identity = json.loads((root / ".git/agent-platform-prepared.json").read_text())
    if identity["repository"] != repository or not re.fullmatch(
        r"[0-9a-f]{40}", identity["commit"]
    ):
        raise ValueError("publication lacks pinned repository identity")
    subprocess.run(
        ["git", "merge-base", "--is-ancestor", identity["commit"], "HEAD"],
        cwd=root,
        check=True,
        timeout=10,
    )
    # Local rewrites, separate push URLs, credential helpers and hooks must not
    # redirect the fixed authenticated operation selected by this workflow.
    unsafe = subprocess.run(
        [
            "git",
            "config",
            "--local",
            "--get-regexp",
            r"^(url\.|http\.|credential\.|include|remote\.origin\.pushurl)",
        ],
        cwd=root,
        capture_output=True,
        timeout=10,
    )
    if unsafe.returncode != 1:
        raise ValueError(
            "repository contains unsupported authentication or destination overrides"
        )
    # Validate both selected files and fixed delivery outputs before any write
    # or index mutation. Respect Git's tracked-file and ignore semantics.
    delivery = Path("docs/harness") / ("run-" + identity["run_id"][:12])
    publication_paths = files + [
        str(delivery / name) for name in ("qa.md", "report.md")
    ]
    ignored = subprocess.run(
        ["git", "check-ignore", "-z", "--stdin"],
        input=("\0".join(publication_paths) + "\0").encode(),
        cwd=root,
        capture_output=True,
        timeout=10,
    )
    if ignored.returncode == 0:
        raise ValueError(
            "ignored publication path: select repository artifacts, not runtime inputs; "
            "QA and report body are copied to docs/harness/run-* automatically"
        )
    if ignored.returncode != 1:
        raise ValueError("cannot verify publication paths against Git ignore rules")
    from repository import check_publish_blob, check_publish_files

    qa = root / "harness-output/qa.md"
    if not qa.is_file() or qa.is_symlink() or qa.parent.is_symlink():
        raise ValueError("independent QA report is unavailable")
    qa_raw = qa.read_bytes()
    check_publish_blob("qa.md", qa_raw)
    if delivery.is_symlink() or any(
        parent.is_symlink() for parent in (root / delivery).parents
    ):
        raise ValueError("delivery path is not owned")
    delivery_files = {
        str(delivery / "qa.md"): qa_raw,
        str(delivery / "report.md"): body.encode(),
    }
    for name, content in delivery_files.items():
        destination = root / name
        if destination.is_symlink() or (
            destination.exists() and destination.read_bytes() != content
        ):
            raise ValueError("delivery artifact differs from prior publication")
    # A prior attempt may already have staged or committed these exact outputs.
    # Include them before both history and tracked-change scope validation.
    files = list(dict.fromkeys([*files, *delivery_files]))
    check_publish_files(root)
    commits = git("rev-list", "--reverse", identity["commit"] + "..HEAD").splitlines()
    if len(commits) > 200:
        raise ValueError("publication history needs separate review")
    for commit in commits:
        parents = git("show", "-s", "--format=%P", commit).split()
        if len(parents) != 1:
            raise ValueError("merge history needs separate review before publication")
        paths = (
            subprocess.check_output(
                [
                    "git",
                    "diff-tree",
                    "--no-commit-id",
                    "--name-only",
                    "-r",
                    "-z",
                    commit,
                ],
                cwd=root,
                timeout=30,
            )
            .decode()
            .rstrip("\0")
            .split("\0")
        )
        for name in filter(None, paths):
            if name not in files:
                raise ValueError(
                    "unreviewed committed file remains in publication history"
                )
            blob = commit + ":" + name
            present = subprocess.run(
                ["git", "cat-file", "-e", blob],
                cwd=root,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                timeout=5,
            )
            if present.returncode:
                continue  # deletion; its earlier version is checked in its own commit
            size = int(git("cat-file", "-s", blob))
            if size > 5 * 1024 * 1024:
                raise ValueError("publication history blob exceeds size limit")
            raw = subprocess.check_output(
                ["git", "cat-file", "blob", blob], cwd=root, timeout=10
            )
            if name in delivery_files and raw != delivery_files[name]:
                raise ValueError("delivery artifact differs in publication history")
            check_publish_blob(name, raw)
    changed = set(git("diff", "--name-only", identity["commit"]).splitlines())
    if not changed.issubset(set(files)):
        raise ValueError("unreviewed tracked changes remain")
    (root / delivery).mkdir(parents=True, exist_ok=True)
    for name, content in delivery_files.items():
        (root / name).write_bytes(content)
    subprocess.run(["git", "add", "--", *files], cwd=root, check=True, timeout=30)
    staged = (
        subprocess.check_output(
            ["git", "diff", "--cached", "--name-only", "-z", identity["commit"]],
            cwd=root,
            timeout=30,
        )
        .decode()
        .rstrip("\0")
        .split("\0")
    )
    for name in filter(None, staged):
        if name not in files:
            raise ValueError("unreviewed staged file would be committed")
        blob = ":" + name
        size = int(git("cat-file", "-s", blob))
        if size > 5 * 1024 * 1024:
            raise ValueError("staged blob exceeds publication limit")
        check_publish_blob(
            name,
            subprocess.check_output(
                ["git", "cat-file", "blob", blob], cwd=root, timeout=10
            ),
        )
    if git("diff", "--cached", "--name-only"):
        subprocess.run(
            [
                "git",
                "-c",
                "user.name=agent-platform",
                "-c",
                "user.email=agent-platform@users.noreply.github.com",
                "-c",
                "core.hooksPath=/dev/null",
                "commit",
                "-m",
                title,
            ],
            cwd=root,
            check=True,
            timeout=60,
        )
    head = git("rev-parse", "HEAD")
    # Use the same credential-free askpass source as preparation, never a token URL.
    import tempfile

    with tempfile.TemporaryDirectory(prefix="agent-platform-publish-") as directory:
        helper = Path(directory) / "askpass.py"
        helper.write_text(
            '#!/usr/bin/env python3\nimport os,sys\nprint("x-access-token" if "username" in sys.argv[1].lower() else os.environ["GITHUB_TOKEN"])\n'
        )
        helper.chmod(0o700)
        env = dict(
            os.environ,
            GIT_ASKPASS=str(helper),
            GIT_TERMINAL_PROMPT="0",
            GIT_CONFIG_GLOBAL="/dev/null",
            GIT_CONFIG_NOSYSTEM="1",
        )
        subprocess.run(
            [
                "git",
                "-c",
                "credential.helper=",
                "-c",
                "http.followRedirects=false",
                "-c",
                "core.hooksPath=/dev/null",
                "push",
                "https://github.com/" + repository + ".git",
                branch,
            ],
            env=env,
            cwd=root,
            check=True,
            timeout=120,
        )
    http = urllib.request.build_opener(NoRedirect())

    def api(path, value=None):
        request = urllib.request.Request(
            "https://api.github.com/repos/" + repository + path,
            data=None if value is None else json.dumps(value).encode(),
            headers={
                "Authorization": "Bearer " + os.environ["GITHUB_TOKEN"],
                "Accept": "application/vnd.github+json",
                "Content-Type": "application/json",
            },
        )
        with http.open(request, timeout=30) as response:
            return json.load(response)

    existing = api(
        "/pulls?state=all&head="
        + urllib.parse.quote(repository.split("/")[0] + ":" + branch, safe="")
        + "&base=main"
    )
    if len(existing) > 1:
        raise ValueError("multiple matching pull requests; reconcile")
    pr = (
        existing[0]
        if existing
        else api(
            "/pulls",
            {
                "title": title,
                "body": "由本 Issue 的 Pipeline 生成并经过独立 QA；以下为原始交付报告。\n\n"
                + body,
                "head": branch,
                "base": "main",
                "draft": True,
            },
        )
    )
    if pr["head"]["sha"] != head:
        raise ValueError("remote PR does not match reviewed commit")
    print(
        json.dumps(
            {
                "published": True,
                "head": head,
                "pr": pr["html_url"],
                "draft": pr["draft"],
            }
        )
    )


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--repository", required=True)
    parser.add_argument(
        "--report", type=Path, default=Path("harness-output/report.json")
    )
    args = parser.parse_args()
    publish(args.repository, args.report)
