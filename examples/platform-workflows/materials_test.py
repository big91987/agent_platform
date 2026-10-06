"""Exercise real ZIP inputs and durable files; isolate external download only."""

import hashlib
import io
import json
import stat
import tempfile
import unittest
import zipfile
from pathlib import Path
from unittest.mock import patch

import materials

REPOSITORY = "example/product"
URL = "https://api.github.com/repos/example/product/releases/assets/123"


def bundle(files=None, version="v1", extra=None, records=None):
    files = files or {
        "docs/prd.md": b"Actual requirements",
        "prototype/index.html": b"<h1>Console</h1>",
    }
    manifest = {
        "version": version,
        "files": records
        if records is not None
        else [
            {"path": n, "sha256": hashlib.sha256(v).hexdigest(), "bytes": len(v)}
            for n, v in files.items()
        ],
    }
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED) as z:
        z.writestr("manifest.json", json.dumps(manifest))
        for name, value in files.items():
            z.writestr(name, value)
        if extra:
            extra(z)
    return output.getvalue()


class MaterialTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.workspace = Path(self.tmp.name)
        self.run = {"run_id": "a" * 32, "parameters": {}}

    def configure(self, raw):
        descriptor = {
            "url": URL,
            "sha256": hashlib.sha256(raw).hexdigest(),
            "version": "v1",
        }
        self.run["parameters"]["material"] = json.dumps(descriptor)
        return descriptor

    def install(self, raw):
        self.configure(raw)
        with patch.object(
            materials, "download", side_effect=lambda d, path: path.write_bytes(raw)
        ):
            return materials.install_material(self.workspace, self.run, REPOSITORY)

    def test_roundtrip_and_retry_do_not_download_or_change_input(self):
        raw = bundle()
        receipt = self.install(raw)
        root = self.workspace / receipt["root"]
        self.assertEqual((root / "docs/prd.md").read_bytes(), b"Actual requirements")
        self.assertEqual(receipt["sha256"], hashlib.sha256(raw).hexdigest())
        self.assertEqual(receipt["version"], "v1")
        materials.verify_material(self.workspace, self.run)
        with patch.object(
            materials, "download", side_effect=AssertionError("must reuse frozen input")
        ):
            self.assertEqual(
                materials.install_material(self.workspace, self.run, REPOSITORY),
                receipt,
            )

    def test_absent_material_preserves_old_run(self):
        self.assertIsNone(
            materials.install_material(self.workspace, self.run, REPOSITORY)
        )
        materials.verify_material(self.workspace, self.run)
        self.assertEqual(list(self.workspace.iterdir()), [])

    def test_descriptor_rejects_wrong_repository_or_url_or_unpinned_input(self):
        good = self.configure(bundle())
        for changes in [
            {"url": "https://api.github.com/repos/other/product/releases/assets/123"},
            {"url": "http://localhost/private.zip"},
            {
                "url": "https://github.com/example/product/releases/latest/download/input.zip"
            },
            {"sha256": "wrong"},
            {"version": ""},
            {"command": "run.sh"},
            {"url": URL + "?token=secret"},
        ]:
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                materials.validate_description(
                    json.dumps({**good, **changes}), REPOSITORY
                )
        github = {
            **good,
            "url": "https://github.com/user-attachments/files/123/input.zip",
        }
        self.assertEqual(
            materials.validate_description(json.dumps(github), REPOSITORY), github
        )

    def test_checksum_and_version_mismatch_never_publish_partial_input(self):
        raw = bundle()
        descriptor = self.configure(raw)
        self.run["parameters"]["material"] = json.dumps(
            {**descriptor, "sha256": "b" * 64}
        )
        with (
            patch.object(
                materials, "download", side_effect=lambda d, p: p.write_bytes(raw)
            ),
            self.assertRaises(ValueError),
        ):
            materials.install_material(self.workspace, self.run, REPOSITORY)
        with self.assertRaises(ValueError):
            self.install(bundle(version="v2"))
        self.assertFalse(
            [p for p in self.workspace.glob(".workflow-input/*/*") if p.is_dir()]
        )

    def test_manifest_file_digest_size_missing_and_undeclared_rejected(self):
        for records in [
            [{"path": "docs/prd.md", "sha256": "b" * 64, "bytes": 19}],
            [{"path": "missing.md", "sha256": "a" * 64, "bytes": 3}],
            [
                {
                    "path": "docs/prd.md",
                    "sha256": hashlib.sha256(b"Actual requirements").hexdigest(),
                    "bytes": 0,
                }
            ],
            [],
        ]:
            with self.subTest(records=records), self.assertRaises(ValueError):
                self.install(bundle(records=records))

    def test_unsafe_archive_names_links_and_duplicate_entries_rejected(self):
        for name in [
            "../escape",
            "/tmp/escape",
            "C:/escape",
            "back\\slash",
            "docs/../escape",
            "docs//x",
            "docs/./x",
        ]:
            with self.subTest(name=name), self.assertRaises(ValueError):
                self.install(bundle(extra=lambda z: z.writestr(name, b"escape")))

        def symlink(z):
            item = zipfile.ZipInfo("link")
            item.external_attr = (stat.S_IFLNK | 0o777) << 16
            z.writestr(item, "/etc/passwd")

        for extra in [
            symlink,
            lambda z: z.writestr("docs/prd.md", b"duplicate"),
            lambda z: z.writestr("DOCS/PRD.md", b"collision"),
        ]:
            with self.subTest(extra=extra), self.assertRaises(ValueError):
                self.install(bundle(extra=extra))
        self.assertFalse((self.workspace.parent / "escape").exists())

    def test_file_count_limit_and_bounded_sizes(self):
        self.install(bundle(files={str(i): b"x" for i in range(256)}))
        with self.assertRaises(ValueError):
            self.install(bundle(files={str(i): b"x" for i in range(257)}))
        for limit in ["MAX_ARCHIVE", "MAX_FILE", "MAX_EXPANDED"]:
            with (
                self.subTest(limit=limit),
                patch.object(materials, limit, 8),
                self.assertRaises(ValueError),
            ):
                self.install(bundle())

    def test_download_failure_can_retry_same_run_without_reset(self):
        raw = bundle()
        self.configure(raw)

        def interrupted(_, path):
            path.write_bytes(raw[:32])
            raise TimeoutError("interrupted")

        with (
            patch.object(materials, "download", side_effect=interrupted),
            self.assertRaises(TimeoutError),
        ):
            materials.install_material(self.workspace, self.run, REPOSITORY)
        receipt = self.install(raw)
        self.assertEqual(
            (self.workspace / receipt["root"] / "docs/prd.md").read_bytes(),
            b"Actual requirements",
        )

    def test_tampered_files_or_manifest_or_archive_cannot_self_attest(self):
        for target in ["docs/prd.md", "manifest.json", "archive.zip"]:
            with self.subTest(target=target):
                receipt = self.install(bundle())
                path = self.workspace / (
                    receipt["archive_path"]
                    if target == "archive.zip"
                    else receipt["root"] + "/" + target
                )
                original = path.read_bytes()
                path.write_bytes(b"changed")
                with self.assertRaises(ValueError):
                    materials.verify_material(self.workspace, self.run)
                with (
                    patch.object(
                        materials,
                        "download",
                        side_effect=AssertionError("no replacement"),
                    ),
                    self.assertRaises(ValueError),
                ):
                    materials.install_material(self.workspace, self.run, REPOSITORY)
                path.write_bytes(original)

    def test_symlinked_storage_or_input_rejected(self):
        external = self.workspace / "external"
        external.mkdir()
        (self.workspace / ".workflow-input").symlink_to(
            external, target_is_directory=True
        )
        with self.assertRaises(ValueError):
            self.install(bundle())
        (self.workspace / ".workflow-input").unlink()
        receipt = self.install(bundle())
        path = self.workspace / receipt["root"] / "docs/prd.md"
        path.unlink()
        path.symlink_to(external / "missing")
        with self.assertRaises(ValueError):
            materials.verify_material(self.workspace, self.run)


class MaterialRecoveryTest(unittest.TestCase):
    setUp = MaterialTest.setUp
    configure = MaterialTest.configure
    install = MaterialTest.install

    def test_case_inconsistent_parent_paths_never_publish_poisoned_input(self):
        for files in [
            {"A/x.txt": b"x", "a/y.txt": b"y"},
            {"é.txt": b"x", "e\u0301.txt": b"y"},
        ]:
            raw = bundle(files=files)
            self.configure(raw)
            with tempfile.TemporaryDirectory() as tmp:
                archive = Path(tmp) / "input.zip"
                archive.write_bytes(raw)
                with self.assertRaises(ValueError):
                    materials._inspect(
                        archive,
                        materials.validate_description(
                            self.run["parameters"]["material"]
                        ),
                    )
            with self.assertRaises(ValueError):
                self.install(raw)
            sha = hashlib.sha256(raw).hexdigest()
            self.assertFalse(
                (self.workspace / ".workflow-input" / ("a" * 32) / sha).exists()
            )

    def test_sigkill_partial_download_is_cleaned_by_same_run_retry(self):
        import os
        import subprocess
        import sys
        import time

        raw = bundle()
        self.configure(raw)
        source = self.workspace / "fixture.zip"
        source.write_bytes(raw)
        marker = self.workspace / "partial-ready"
        script = """import json,sys,time
from pathlib import Path
import materials
workspace=Path(sys.argv[1]); run=json.loads(sys.argv[2])
def partial(description,path):
 path.write_bytes((workspace/'fixture.zip').read_bytes()[:32])
 (workspace/'partial-ready').write_text('ready')
 time.sleep(60)
materials.download=partial
materials.install_material(workspace,run,'example/product')
"""
        process = subprocess.Popen(
            [sys.executable, "-c", script, str(self.workspace), json.dumps(self.run)],
            env={**os.environ, "PYTHONPATH": str(Path(materials.__file__).parent)},
        )
        try:
            deadline = time.monotonic() + 5
            while (
                not marker.exists()
                and time.monotonic() < deadline
                and process.poll() is None
            ):
                time.sleep(0.02)
            self.assertTrue(marker.exists(), "fixture did not enter download")
        finally:
            process.kill()
            process.wait()
        self.assertTrue(list(self.workspace.glob(".workflow-input/*/.pending-*")))
        self.install(raw)
        self.assertEqual(list(self.workspace.glob(".workflow-input/*/.pending-*")), [])

    def test_concurrent_retry_does_not_remove_active_attempt(self):
        import os
        import subprocess
        import sys
        import time

        raw = bundle()
        self.configure(raw)
        (self.workspace / "fixture.zip").write_bytes(raw)
        script = """import json,os,sys,time
from pathlib import Path
import materials
root=Path(sys.argv[1]);run=json.loads(sys.argv[2]);label=sys.argv[3]
def fetch(description,path):
 (root/('download-'+label)).write_text('begun')
 path.write_bytes((root/'fixture.zip').read_bytes())
 if label=='first':
  while not (root/'release-first').exists():time.sleep(.02)
materials.download=fetch
(root/('entered-'+label)).write_text('ready')
materials.install_material(root,run,'example/product')
(root/('done-'+label)).write_text('done')
"""
        children = []

        def launch(label):
            child = subprocess.Popen(
                [
                    sys.executable,
                    "-c",
                    script,
                    str(self.workspace),
                    json.dumps(self.run),
                    label,
                ],
                env={**os.environ, "PYTHONPATH": str(Path(materials.__file__).parent)},
            )
            children.append(child)
            return child

        def wait_file(name):
            deadline = time.monotonic() + 5
            while not (self.workspace / name).exists() and time.monotonic() < deadline:
                time.sleep(0.02)
            self.assertTrue((self.workspace / name).exists(), name)

        try:
            first = launch("first")
            wait_file("download-first")
            active = list(self.workspace.glob(".workflow-input/*/.pending-*"))
            self.assertEqual(len(active), 1)
            second = launch("second")
            wait_file("entered-second")
            self.assertIsNone(second.poll())
            self.assertTrue((active[0] / "archive.zip").exists())
            (self.workspace / "release-first").touch()
            self.assertEqual(first.wait(timeout=5), 0)
            self.assertEqual(second.wait(timeout=5), 0)
            self.assertEqual(len(list(self.workspace.glob("download-*"))), 1)
            self.assertEqual(
                list(self.workspace.glob(".workflow-input/*/.pending-*")), []
            )
        finally:
            for child in children:
                if child.poll() is None:
                    child.kill()
                child.wait()


class MaterialTransportTest(unittest.TestCase):
    def test_gh_destination_is_pinned_even_when_environment_names_another_host(self):
        import os
        import sys

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            gh = root / "gh"
            record = root / "argv.json"
            gh.write_text(
                "#!"
                + sys.executable
                + '\nimport json,os,sys\nfrom pathlib import Path\nPath(os.environ["MATERIAL_TEST_RECORD"]).write_text(json.dumps(sys.argv[1:]))\nos.write(1,b"zip bytes")\n'
            )
            gh.chmod(0o700)
            with patch.dict(
                os.environ,
                {
                    "PATH": str(root) + os.pathsep + os.environ["PATH"],
                    "GH_HOST": "unapproved.example",
                    "MATERIAL_TEST_RECORD": str(record),
                },
            ):
                materials.download({"url": URL}, root / "download.zip")
            argv = json.loads(record.read_text())
            self.assertIn("--hostname", argv)
            self.assertEqual(argv[argv.index("--hostname") + 1], "github.com")
            self.assertEqual((root / "download.zip").read_bytes(), b"zip bytes")

    def test_private_download_stream_is_bounded_and_timeout_kills_process(self):
        import os
        import sys

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            gh = root / "gh"
            for body, error in [
                ('import os; os.write(1,b"x"*10000)', ValueError),
                ("import time; time.sleep(10)", TimeoutError),
            ]:
                gh.write_text("#!" + sys.executable + "\n" + body + "\n")
                gh.chmod(0o700)
                target = root / "download.zip"
                with (
                    patch.dict(
                        os.environ,
                        {"PATH": str(root) + os.pathsep + os.environ["PATH"]},
                    ),
                    patch.object(materials, "MAX_ARCHIVE", 64),
                    patch.object(
                        materials, "TIMEOUT", 5 if error is ValueError else 0.5
                    ),
                    self.assertRaises(error),
                ):
                    materials.download({"url": URL}, target)
                target.unlink(missing_ok=True)

    def test_public_redirect_does_not_add_credentials_or_leave_github(self):
        import urllib.request

        redirect = materials._GitHubRedirect()
        request = urllib.request.Request(
            "https://github.com/user-attachments/files/123/input.zip"
        )
        approved = redirect.redirect_request(
            request,
            None,
            302,
            "Found",
            {},
            "https://release-assets.githubusercontent.com/asset?signature=example",
        )
        self.assertNotIn("Authorization", approved.headers)
        for url in [
            "http://github.com/a",
            "https://localhost/a",
            "https://github.com.evil/a",
            "https://user:secret@github.com/a",
            "https://github.com:444/a",
        ]:
            with self.subTest(url=url), self.assertRaises(ValueError):
                redirect.redirect_request(request, None, 302, "Found", {}, url)
        target = io.BytesIO()
        with self.assertRaises(ValueError):
            materials._copy_bounded(io.BytesIO(b"x" * 65), target, 64)
        self.assertLessEqual(len(target.getvalue()), 64)


if __name__ == "__main__":
    unittest.main()
