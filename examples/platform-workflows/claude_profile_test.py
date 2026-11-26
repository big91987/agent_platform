import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from asset_snapshot import (
    PARSERS,
    prepare_arch_parsers,
    snapshot,
    snapshot_arch,
    tree_digest,
)
from claude_profile import configure, role_skills
from repository import ReviewedSourceChanged, git, task_docs, verify_reviewed_tree
from review_pair import decision, expose_artifacts, load_ocr_runtime, prepared_head


class ClaudeProfileTest(unittest.TestCase):
    def test_dual_review_cannot_pass_incomplete_or_changed_source(self):
        self.assertEqual(decision(0, 0, "no_findings", True), 0)
        self.assertEqual(decision(10, 0, "no_findings", True), 10)
        self.assertEqual(decision(0, 1, "findings", True), 10)
        for args in (
            (20, 0, "no_findings", True),
            (0, 2, "no_findings", True),
            (0, 0, "not_run", True),
            (0, 0, "no_findings", False),
        ):
            with self.subTest(args=args):
                self.assertEqual(decision(*args), 20)

    def test_artifact_export_refuses_linked_parent_and_replaces_linked_destination(
        self,
    ):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            outside = root / "outside"
            outside.mkdir()
            sentinel = outside / "review-pair.json"
            sentinel.write_text("unchanged")
            private = root / "private"
            private.mkdir()
            (private / "review-pair.json").write_text("new report")
            workspace = root / "work"
            workspace.mkdir()
            fd = os.open(workspace, os.O_DIRECTORY)
            self.addCleanup(os.close, fd)
            output = workspace / "harness-output"
            output.symlink_to(outside)
            with self.assertRaises(OSError):
                expose_artifacts(fd, private)
            self.assertEqual(sentinel.read_text(), "unchanged")
            output.unlink()
            output.mkdir()
            (output / sentinel.name).symlink_to(sentinel)
            expose_artifacts(fd, private)
            self.assertEqual(sentinel.read_text(), "unchanged")
            self.assertEqual((output / sentinel.name).read_text(), "new report")

    def test_connector_reviews_private_checkout_and_returns_incomplete_receipt(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            work = root / "checkout"
            work.mkdir()
            git(work, "init", "-q")
            git(work, "config", "user.name", "Test")
            git(work, "config", "user.email", "test@localhost")
            source = work / "app.py"
            source.write_text("initial")
            git(work, "add", "-A")
            git(work, "commit", "-qm", "base")
            base = git(work, "rev-parse", "HEAD")
            source.write_text("candidate")
            old_state = root / "old-state"
            old_state.mkdir()
            (work / ".git/agent-platform-review").symlink_to(old_state)
            cache = root / "cache"
            cache.mkdir()
            # Fail the pinned binary digest locally, before any provider call.
            (cache / "ocr").write_text("unavailable")
            arch = root / "arch"
            arch.mkdir()
            (arch / "inspect_project.py").write_text(
                "import json,subprocess\nfrom pathlib import Path\n"
                "head = subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()\n"
                "Path('harness-output/architecture/latest-report.json').write_text(json.dumps({"
                "'git_head':head,'model_review':{'status':'not_run'},'root':str(Path.cwd()),"
                "'input':Path('app.py').read_text()}))\n"
            )
            context = dict(
                workspace_path=str(work),
                previous_results=[
                    dict(
                        node_id="prepare",
                        status="completed",
                        connector_receipt=dict(
                            exit_code=0, output=json.dumps({"head": base})
                        ),
                    )
                ],
            )
            result = subprocess.run(
                [
                    os.sys.executable,
                    str(Path(__file__).with_name("review_pair.py").resolve()),
                    "--arch-tool-root",
                    str(arch),
                    "--arch-digest",
                    tree_digest(arch),
                    "--ocr-cache",
                    str(cache),
                ],
                cwd=work,
                input=json.dumps(context),
                capture_output=True,
                text=True,
            )
            self.assertEqual(result.returncode, 20, result.stderr)
            receipt = json.loads(result.stdout)
            report = json.loads(
                (work / "harness-output/architecture/latest-report.json").read_text()
            )
            self.assertEqual(report["git_head"], receipt["candidate"])
            self.assertEqual(report["input"], "candidate")
            self.assertFalse(Path(report["root"]).is_relative_to(work))
            self.assertTrue(receipt["source_unchanged"])
            self.assertEqual(list(old_state.iterdir()), [])

    def test_publish_entry_pushes_exact_reviewed_files_with_ignored_output(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            work = root / "checkout"
            remote = root / "remote.git"
            work.mkdir()
            git(root, "init", "--bare", str(remote))
            git(work, "init", "-q")
            git(work, "config", "user.name", "Test")
            git(work, "config", "user.email", "test@localhost")
            run = {
                "run_id": "a" * 32,
                "workspace_path": str(work.resolve()),
                "previous_results": [],
            }
            branch = "workflow/" + run["run_id"]
            git(work, "switch", "-c", branch)
            origin = "https://github.com/owner/example.git"
            git(work, "remote", "add", "origin", origin)
            git(work, "config", "remote.origin.pushurl", str(remote))
            (work / ".gitignore").write_text("harness-output/\n")
            (work / "remove").write_text("original")
            git(work, "add", "-A")
            git(work, "commit", "-qm", "base")
            (work / "remove").unlink()
            special = ":(glob)*\nexample.py"
            (work / special).write_text("pass")
            (work / "harness-output").mkdir()
            (work / "harness-output/excluded").write_text("private output")
            candidate = load_ocr_runtime().candidate_commit(work)
            run["previous_results"].append(
                dict(
                    node_id="code_review",
                    status="completed",
                    connector_receipt={
                        "exit_code": 0,
                        "output": json.dumps(
                            dict(
                                candidate=candidate, exit_code=0, source_unchanged=True
                            )
                        ),
                    },
                )
            )
            directory = task_docs(run)
            (work / directory).mkdir(parents=True)
            for node in ("qa", "report"):
                name = directory + "/" + ("pr.md" if node == "report" else node + ".md")
                (work / name).write_text("Reviewed evidence")
                run["previous_results"].append(
                    dict(
                        node_id=node,
                        status="completed",
                        result=dict(route="next", artifacts=[name]),
                    )
                )
            repository = str(Path(__file__).with_name("repository.py").resolve())
            precheck = [os.sys.executable, repository, "check-reviewed"]
            (work / "unexpected.py").write_text("pass")
            rejected = subprocess.run(
                precheck,
                cwd=work,
                input=json.dumps(run),
                text=True,
                capture_output=True,
            )
            self.assertEqual(rejected.returncode, 10, rejected.stderr)
            self.assertIn("unexpected.py", rejected.stdout)
            (work / "unexpected.py").unlink()
            subprocess.run(
                precheck, cwd=work, input=json.dumps(run), text=True, check=True
            )
            receipt = json.loads(
                subprocess.check_output(
                    [
                        os.sys.executable,
                        repository,
                        "publish",
                        "--repository",
                        "owner/example",
                        "--reviewed-source",
                        "--task-docs",
                    ],
                    cwd=work,
                    input=json.dumps(run),
                    text=True,
                )
            )
            self.assertEqual(receipt["head"], git(remote, "rev-parse", branch))
            names = subprocess.check_output(
                ["git", "-C", str(remote), "ls-tree", "-rz", "--name-only", branch]
            ).split(b"\0")
            self.assertIn(os.fsencode(special), names)
            self.assertNotIn(b"remove", names)
            self.assertNotIn(b"harness-output/excluded", names)
            self.assertIn(os.fsencode(directory + "/qa.md"), names)
            self.assertIn(os.fsencode(directory + "/pr.md"), names)

    def test_role_flow_requires_tests_reviews_qa_and_delivery_gate(self):
        graph = json.loads(
            Path(__file__).with_name("software-delivery.json").read_text()
        )
        modified = configure(graph)
        self.assertIn("intake", [n["id"] for n in graph["nodes"]])
        self.assertEqual(
            [n["id"] for n in modified["nodes"]],
            [
                "prepare",
                "issue",
                "requirements",
                "design",
                "development",
                "tests",
                "code_review",
                "qa",
                "approval",
                "report",
                "publish_check",
                "publish",
                "pr",
                "done",
            ],
        )
        self.assertEqual(
            {
                (e["source"], e["target"])
                for e in modified["edges"]
                if e["route"] != "next"
            },
            {
                ("design", "requirements"),
                ("development", "design"),
                ("tests", "development"),
                ("qa", "development"),
                ("report", "qa"),
            },
        )
        for blocked in (
            "requirements",
            "design",
            "development",
            "tests",
            "code_review",
            "qa",
            "approval",
            "publish_check",
        ):
            seen = set()
            pending = [modified["entry"]]
            while pending:
                n = pending.pop()
                if n in seen or n == blocked:
                    continue
                seen.add(n)
                pending.extend(
                    e["target"] for e in modified["edges"] if e["source"] == n
                )
            self.assertNotIn("done", seen, blocked)
        skills = role_skills(Path("/roles"), Path("/qa"))
        self.assertEqual(skills["development"], [])
        self.assertEqual(skills["report"], [])
        self.assertEqual(
            skills["requirements"],
            [
                Path("/roles/defining-platform-products-cn"),
                Path("/roles/batch-grill-me"),
            ],
        )

    def test_reviewed_tree_accepts_evidence_but_rejects_source_changes(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            git(root, "init", "-q")
            source = root / "app.py"
            source.write_text("pass\n")
            git(root, "add", "--all")
            candidate = git(root, "write-tree")
            run = {"run_id": "a" * 32, "previous_results": []}
            directory = task_docs(run)
            evidence = directory + "/evidence.html"
            pr = directory + "/pr.md"
            review = {
                "node_id": "code_review",
                "status": "completed",
                "connector_receipt": {
                    "exit_code": 0,
                    "output": json.dumps(
                        {
                            "candidate": candidate,
                            "exit_code": 0,
                            "source_unchanged": True,
                        }
                    ),
                },
            }
            run["previous_results"].append(review)
            (root / directory).mkdir(parents=True)
            for name in (evidence, pr):
                (root / name).write_text("Evidence")
            qa = {
                "node_id": "qa",
                "status": "completed",
                "result": {"route": "next", "artifacts": [evidence]},
            }
            run["previous_results"].append(qa)
            git(root, "add", "--all")
            verify_reviewed_tree(root, run, git(root, "write-tree"))

            # A report can ask QA to supplement the evidence without a source
            # change or a fresh code review. Both declared additions still belong.
            supplement = directory + "/evidence-supplement.html"
            (root / supplement).write_text("Supplemented evidence")
            run["previous_results"].extend(
                [
                    {
                        "node_id": "report",
                        "status": "completed",
                        "result": {"route": "qa"},
                    },
                    {**qa, "result": {"route": "next", "artifacts": [supplement]}},
                ]
            )
            git(root, "add", "--all")
            verify_reviewed_tree(root, run, git(root, "write-tree"))

            # A source fix needs a new approval from QA after the new review.
            specification = directory + "/requirements.md"
            (root / specification).write_text("Approved requirements")
            source.write_text("print('fixed')\n")
            git(root, "add", "--all")
            next_candidate = git(root, "write-tree")
            run["previous_results"].append(
                {
                    **review,
                    "connector_receipt": {
                        "exit_code": 0,
                        "output": json.dumps(
                            {
                                "candidate": next_candidate,
                                "exit_code": 0,
                                "source_unchanged": True,
                            }
                        ),
                    },
                }
            )
            with self.assertRaisesRegex(ValueError, "QA handoff"):
                verify_reviewed_tree(root, run, next_candidate)

            artifact = directory + "/evidence-2.html"
            (root / artifact).write_text("Second round evidence")
            verdict = {"route": "development", "artifacts": [artifact]}
            run["previous_results"].append({**qa, "result": verdict})
            git(root, "add", "--all")
            tree = git(root, "write-tree")
            with self.assertRaisesRegex(ValueError, "QA handoff"):
                verify_reviewed_tree(root, run, tree)
            verdict["route"] = "next"
            verdict["artifacts"] = [evidence]
            with self.assertRaisesRegex(ValueError, "no new evidence"):
                verify_reviewed_tree(root, run, next_candidate)
            verdict["artifacts"] = [artifact]
            verify_reviewed_tree(root, run, tree)
            verdict["artifacts"].append(specification)
            (root / specification).write_text("Modified after code review")
            git(root, "add", "--all")
            with self.assertRaisesRegex(ReviewedSourceChanged, "requirements.md"):
                verify_reviewed_tree(root, run, git(root, "write-tree"))
            (root / specification).write_text("Approved requirements")
            source.chmod(0o755)
            git(root, "add", "--all")
            with self.assertRaisesRegex(ReviewedSourceChanged, "app.py"):
                verify_reviewed_tree(root, run, git(root, "write-tree"))
            source.chmod(0o644)
            (root / directory / "unlisted.json").write_text("{}")
            git(root, "add", "--all")
            with self.assertRaisesRegex(ReviewedSourceChanged, "unlisted.json"):
                verify_reviewed_tree(root, run, git(root, "write-tree"))

    def test_install_retains_asset_versions_and_detects_local_drift(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "skill"
            source.mkdir()
            (source / "SKILL.md").write_text("first")
            old, previous = snapshot(source, root / "installed", False)
            (source / "SKILL.md").write_text("second")
            with self.assertRaisesRegex(ValueError, "--upgrade"):
                snapshot(source, root / "installed", False, previous)
            new, _ = snapshot(source, root / "installed", True, previous)
            self.assertEqual((old / "SKILL.md").read_text(), "first")
            self.assertEqual((new / "SKILL.md").read_text(), "second")
            (new / "SKILL.md").write_text("drift")
            with self.assertRaisesRegex(ValueError, "recorded digest"):
                snapshot(source, root / "installed", True, previous)

    def test_parser_runtime_upgrade_preserves_frozen_copy_and_checks_drift(self):
        versions = dict.fromkeys(PARSERS, "1.0.0")
        packages = {
            "node_modules/" + name: {"version": v} for name, v in versions.items()
        }

        def locked_install(argv, *, cwd, **kwargs):
            self.assertEqual(argv[:2], ["npm", "ci"])
            modules = cwd / "node_modules"
            modules.mkdir()
            (modules / "parser").write_text("locked dependency")

        with (
            tempfile.TemporaryDirectory() as temporary,
            patch(
                "asset_snapshot.parser_runtime", return_value={"node": "a", "npm": "x"}
            ) as runtime,
            patch(
                "asset_snapshot.subprocess.run", side_effect=locked_install
            ) as installs,
            patch(
                "asset_snapshot.subprocess.check_output",
                return_value=json.dumps(versions),
            ),
        ):
            root = Path(temporary)
            source = root / "tool"
            source.mkdir()
            (source / "package.json").write_text(json.dumps({"dependencies": versions}))
            (source / "package-lock.json").write_text(
                json.dumps({"packages": packages})
            )
            assets = root / "assets"
            # Exercise an installation made before runtime-specific copies existed.
            old, previous = snapshot(source, assets, False)
            previous["parser_installation"] = prepare_arch_parsers(old)
            old_receipt = old.with_name("tool-parsers.json").read_bytes()
            self.assertEqual(
                snapshot_arch(source, assets, False, previous), (old, previous)
            )
            runtime.return_value = {"node": "b", "npm": "x"}
            with self.assertRaisesRegex(ValueError, "--upgrade"):
                snapshot_arch(source, assets, False, previous)
            dependency = old / "node_modules/parser"
            dependency.write_text("tamper")
            with self.assertRaisesRegex(ValueError, "parser files"):
                snapshot_arch(source, assets, True, previous)
            dependency.write_text("locked dependency")
            new, current = snapshot_arch(source, assets, True, previous)
            self.assertNotEqual(old, new)
            self.assertEqual(previous["sha256"], current["sha256"])
            self.assertEqual(
                current["parser_installation"]["runtime"], runtime.return_value
            )
            self.assertEqual(
                snapshot_arch(source, assets, False, current), (new, current)
            )
            self.assertEqual(snapshot_arch(source, assets, False), (new, current))
            self.assertEqual(
                old.with_name("tool-parsers.json").read_bytes(), old_receipt
            )
            self.assertEqual(installs.call_count, 2)
            (new / "node_modules/parser").write_text("tamper")
            with self.assertRaisesRegex(ValueError, "parser files"):
                snapshot_arch(source, assets, True, current)

    def test_source_snapshot_ignores_generated_dependencies_and_private_environment(
        self,
    ):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "tool"
            source.mkdir()
            (source / "package-lock.json").write_text("{}")
            (source / ".env.local").write_text("private fixture")
            dependencies = source / "node_modules"
            (dependencies / ".bin").mkdir(parents=True)
            (dependencies / "parser").write_text("generated fixture")
            link = dependencies / ".bin" / "parser"
            link.symlink_to("../parser")
            installed, receipt = snapshot(source, root / "installed", False)
            self.assertTrue(link.is_symlink())
            self.assertFalse((installed / "node_modules").exists())
            self.assertFalse((installed / ".env.local").exists())
            (dependencies / "parser").write_text("updated generated fixture")
            self.assertEqual(tree_digest(source), receipt["sha256"])
            self.assertEqual(
                snapshot(source, root / "installed", False, receipt)[0], installed
            )

    def test_review_uses_completed_platform_receipt_not_agent_result(self):
        row = {
            "node_id": "prepare",
            "status": "completed",
            "result": {"head": "b" * 40},
            "connector_receipt": {
                "exit_code": 0,
                "output": json.dumps({"head": "a" * 40}),
            },
        }
        self.assertEqual(prepared_head({"previous_results": [row]}), "a" * 40)
        row["connector_receipt"]["log"] = {"truncated": True}
        with self.assertRaises(ValueError):
            prepared_head({"previous_results": [row]})
