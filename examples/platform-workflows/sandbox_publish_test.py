import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from sandbox_publish import publish


class PublicationBoundaryTest(unittest.TestCase):
    def test_preflight_rejects_staged_only_paths_and_generated_private_report(self):
        for kind in ("staged-only", "report-secret"):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                git, report = self.fixture(root)
                if kind == "staged-only":
                    (root / ".env").write_text("staged fixture")
                    git("add", ".env")
                    (root / ".env").unlink()
                    error = "unreviewed staged file"
                else:
                    (root / ".git/info/exclude").write_text("report.json\n")
                    value = json.loads(report.read_text())
                    value["body"] = "github_pat_" + "a" * 36
                    report.write_text(json.dumps(value))
                    error = "private credential"
                index = (root / ".git/index").read_bytes()
                with patch("sandbox_publish.Path.cwd", return_value=root):
                    with self.assertRaisesRegex(ValueError, error):
                        publish("owner/project", report, check_only=True)
                self.assertEqual(index, (root / ".git/index").read_bytes())
                self.assertFalse((root / "docs/harness").exists())

    def test_preflight_checks_real_selection_without_git_or_network_writes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root)
            (root / "reviewed.txt").write_text("reviewed change")
            before = {
                name: (root / name).read_bytes() for name in (".git/index", ".git/HEAD")
            }
            head = git("rev-parse", "HEAD")
            with patch("sandbox_publish.Path.cwd", return_value=root):
                publish("owner/project", report, check_only=True)
                (root / "unreviewed.txt").write_text("not in selected files")
                with self.assertRaisesRegex(ValueError, "unreviewed tracked changes"):
                    publish("owner/project", report, check_only=True)
            self.assertEqual(head, git("rev-parse", "HEAD"))
            for name, raw in before.items():
                self.assertEqual(raw, (root / name).read_bytes())
            self.assertFalse((root / "docs/harness").exists())

    def fixture(self, root, project_files=()):
        def git(*args):
            return subprocess.check_output(
                ["git", "-C", str(root), *args], stderr=subprocess.DEVNULL, text=True
            ).strip()

        git("init", "--quiet")
        git("config", "user.name", "Fixture")
        git("config", "user.email", "fixture@example.invalid")
        (root / "reviewed.txt").write_text("base")
        (root / "unreviewed.txt").write_text("base")
        for name in project_files:
            target = root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text("base project rules\n")
        git("add", ".")
        git("commit", "--quiet", "-m", "base")
        head = git("rev-parse", "HEAD")
        git("checkout", "--quiet", "-b", "codex/run-" + "a" * 12)
        git("remote", "add", "origin", "https://github.com/owner/project.git")
        (root / ".git/agent-platform-prepared.json").write_text(
            json.dumps(
                {"repository": "owner/project", "commit": head, "run_id": "a" * 32}
            )
        )
        (root / "harness-output").mkdir()
        (root / "harness-output/qa.md").write_text("Independent QA fixture")
        report = root / "report.json"
        report.write_text(
            json.dumps(
                {
                    "title": "【Harness验证】验证共享文件",
                    "body": "Actual evidence",
                    "files": ["reviewed.txt"],
                }
            )
        )
        return git, report

    def test_reviewed_project_dotfiles_are_published_with_exact_bytes(self):
        for name in (".gitignore", ".project/contracts.md"):
            with self.subTest(path=name), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                git, report = self.fixture(root, (name,))
                target = root / name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text("Reviewed project rules\n")
                value = json.loads(report.read_text())
                value["files"] = [name]
                report.write_text(json.dumps(value))
                original = subprocess.run

                def stop_before_network(args, **kwargs):
                    if "push" in args:
                        raise RuntimeError("fixture network boundary")
                    return original(args, **kwargs)

                with (
                    patch("sandbox_publish.Path.cwd", return_value=root),
                    patch(
                        "sandbox_publish.subprocess.run",
                        side_effect=stop_before_network,
                    ),
                ):
                    with self.assertRaisesRegex(
                        RuntimeError, "fixture network boundary"
                    ):
                        publish("owner/project", report)
                self.assertEqual(
                    b"Reviewed project rules\n",
                    subprocess.check_output(
                        ["git", "-C", str(root), "show", "HEAD:" + name]
                    ),
                )
                self.assertEqual("", git("diff", "--cached", "--name-only"))

    def test_reviewed_hidden_credentials_remain_rejected_before_writes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root, (".env",))
            (root / ".env").write_text("private-fixture")
            value = json.loads(report.read_text())
            value["files"] = [".env"]
            report.write_text(json.dumps(value))
            index = (root / ".git/index").read_bytes()
            with patch("sandbox_publish.Path.cwd", return_value=root):
                with self.assertRaisesRegex(ValueError, "credentials/cache"):
                    publish("owner/project", report)
            self.assertEqual(index, (root / ".git/index").read_bytes())
            self.assertFalse((root / "docs/harness").exists())

    def test_hidden_files_added_after_frozen_baseline_are_rejected_before_writes(self):
        for name, committed in ((".env.local", False), ("config/.netrc", True)):
            with self.subTest(path=name), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                git, report = self.fixture(root)
                target = root / name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text("private-fixture")
                if committed:
                    git("add", "--", name)
                    git("commit", "--quiet", "-m", "post-baseline hidden file")
                value = json.loads(report.read_text())
                value["files"] = [name]
                report.write_text(json.dumps(value))
                index = (root / ".git/index").read_bytes()
                head = git("rev-parse", "HEAD")
                original = subprocess.run

                def stop_before_network(args, **kwargs):
                    if "push" in args:
                        raise AssertionError("hidden input reached network boundary")
                    return original(args, **kwargs)

                with (
                    patch("sandbox_publish.Path.cwd", return_value=root),
                    patch(
                        "sandbox_publish.subprocess.run",
                        side_effect=stop_before_network,
                    ),
                ):
                    with self.assertRaisesRegex(ValueError, "fixed baseline"):
                        publish("owner/project", report)
                self.assertEqual(index, (root / ".git/index").read_bytes())
                self.assertEqual(head, git("rev-parse", "HEAD"))
                self.assertFalse((root / "docs/harness").exists())

    def test_precommitted_unreviewed_change_is_rejected_before_network(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root)
            (root / "unreviewed.txt").write_text("changed")
            git("add", "unreviewed.txt")
            git("commit", "--quiet", "-m", "prior change")
            with patch("sandbox_publish.Path.cwd", return_value=root):
                with self.assertRaisesRegex(ValueError, "unreviewed"):
                    publish("owner/project", report)

    def test_other_push_host_is_rejected_before_credentials_are_requested(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root)
            git("config", "remote.origin.pushurl", "https://other.example/project.git")
            with patch("sandbox_publish.Path.cwd", return_value=root):
                with self.assertRaisesRegex(ValueError, "destination overrides"):
                    publish("owner/project", report)

    def test_secret_committed_then_deleted_is_rejected_before_network(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root)
            (root / ".env").write_text("private-fixture")
            git("add", ".env")
            git("commit", "--quiet", "-m", "temporary file")
            git("rm", "--quiet", ".env")
            git("commit", "--quiet", "-m", "delete temporary file")
            original = subprocess.run

            def safe_run(args, **kwargs):
                if "push" in args:
                    raise AssertionError("network reached before history validation")
                return original(args, **kwargs)

            with (
                patch("sandbox_publish.Path.cwd", return_value=root),
                patch.dict(os.environ, {"GITHUB_TOKEN": "fixture"}),
                patch("sandbox_publish.subprocess.run", side_effect=safe_run),
            ):
                with self.assertRaisesRegex(ValueError, "unreviewed|credential"):
                    publish("owner/project", report)

    def test_secret_blob_in_reviewed_file_then_reverted_is_still_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root)
            (root / "reviewed.txt").write_text("github_pat_" + "a" * 36)
            git("add", "reviewed.txt")
            git("commit", "--quiet", "-m", "prior sensitive version")
            (root / "reviewed.txt").write_text("base")
            git("add", "reviewed.txt")
            git("commit", "--quiet", "-m", "restore")
            with patch("sandbox_publish.Path.cwd", return_value=root):
                with self.assertRaisesRegex(ValueError, "private credential"):
                    publish("owner/project", report)

    def test_staged_secret_deleted_from_worktree_cannot_be_committed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root)
            (root / ".env").write_text("private-fixture")
            git("add", ".env")
            (root / ".env").unlink()
            with patch("sandbox_publish.Path.cwd", return_value=root):
                with self.assertRaisesRegex(ValueError, "unreviewed staged"):
                    publish("owner/project", report)

    def test_ignored_input_is_rejected_without_staging_or_delivery_writes(self):
        self.assert_ignored_publication_is_unchanged("harness-output/", True)

    def test_ignored_generated_report_is_rejected_without_staging(self):
        self.assert_ignored_publication_is_unchanged("docs/harness/", False)

    def assert_ignored_publication_is_unchanged(self, pattern, include_input):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root)
            (root / ".git/info/exclude").write_text(pattern + "\n")
            (root / "reviewed.txt").write_text("reviewed change")
            if include_input:
                value = json.loads(report.read_text())
                value["files"].append("harness-output/qa.md")
                report.write_text(json.dumps(value))
            index_before = (root / ".git/index").read_bytes()
            head_before = git("rev-parse", "HEAD")
            with patch("sandbox_publish.Path.cwd", return_value=root):
                with self.assertRaisesRegex(ValueError, "ignored publication path"):
                    publish("owner/project", report)
            self.assertEqual(index_before, (root / ".git/index").read_bytes())
            self.assertEqual(head_before, git("rev-parse", "HEAD"))
            self.assertFalse((root / "docs/harness").exists())

    def test_existing_fixed_outputs_resume_without_recommit(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root)
            delivery = root / ("docs/harness/run-" + "a" * 12)
            delivery.mkdir(parents=True)
            (delivery / "qa.md").write_bytes(
                (root / "harness-output/qa.md").read_bytes()
            )
            (delivery / "report.md").write_text(json.loads(report.read_text())["body"])
            git("add", "docs/harness")
            (root / "reviewed.txt").write_text("reviewed change")
            original = subprocess.run

            def stop_before_network(args, **kwargs):
                if "push" in args:
                    raise RuntimeError("fixture network boundary")
                return original(args, **kwargs)

            with (
                patch("sandbox_publish.Path.cwd", return_value=root),
                patch(
                    "sandbox_publish.subprocess.run", side_effect=stop_before_network
                ),
            ):
                for attempt in range(2):
                    with self.assertRaisesRegex(
                        RuntimeError, "fixture network boundary"
                    ):
                        publish("owner/project", report)
                    if attempt == 0:
                        head = git("rev-parse", "HEAD")
                    else:
                        self.assertEqual(head, git("rev-parse", "HEAD"))
            self.assertEqual("", git("diff", "--cached", "--name-only"))

    def test_different_fixed_output_is_not_treated_as_reviewed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root)
            delivery = root / ("docs/harness/run-" + "a" * 12)
            delivery.mkdir(parents=True)
            (delivery / "qa.md").write_text("unreviewed different QA")
            git("add", "docs/harness")
            index = (root / ".git/index").read_bytes()
            with patch("sandbox_publish.Path.cwd", return_value=root):
                with self.assertRaisesRegex(ValueError, "delivery artifact differs"):
                    publish("owner/project", report)
            self.assertEqual(index, (root / ".git/index").read_bytes())

    def test_reverted_different_fixed_output_remains_rejected_in_history(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root)
            delivery = root / ("docs/harness/run-" + "a" * 12)
            delivery.mkdir(parents=True)
            output = delivery / "qa.md"
            output.write_text("prior unreviewed QA")
            git("add", "docs/harness")
            git("commit", "--quiet", "-m", "prior different QA")
            output.write_bytes((root / "harness-output/qa.md").read_bytes())
            git("add", "docs/harness")
            git("commit", "--quiet", "-m", "restore reviewed QA")
            index = (root / ".git/index").read_bytes()
            head = git("rev-parse", "HEAD")
            with patch("sandbox_publish.Path.cwd", return_value=root):
                with self.assertRaisesRegex(
                    ValueError, "differs in publication history"
                ):
                    publish("owner/project", report)
            self.assertEqual(index, (root / ".git/index").read_bytes())
            self.assertEqual(head, git("rev-parse", "HEAD"))
