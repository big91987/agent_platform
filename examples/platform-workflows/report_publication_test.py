"""Publication safety, uncertain effects, and recovery, without product calls."""

import hashlib
import io
import json
import tempfile
import unittest
import zipfile
from pathlib import Path

import report_publication as publication

RUN = "a" * 32
HEAD = "b" * 40


class Remote:
    def __init__(self):
        self.release = None
        self.assets = []
        self.writes = []
        self.lose = None
        self.private = True

    def call(self, method, path, body=None, data_file=None):
        if method == "GET":
            if path == "repos/owner/repo":
                return {"private": self.private}
            if "/releases/tags/" in path:
                return self.release
            if path.endswith("/assets?per_page=100"):
                return self.assets
            raise AssertionError(path)
        self.writes.append(path)
        if data_file is None:
            self.release = {
                **body,
                "id": 42,
                "html_url": "https://github.com/owner/repo/releases/tag/"
                + body["tag_name"],
            }
            value = self.release
        else:
            value = {
                "name": Path(data_file).name,
                "state": "uploaded",
                "size": Path(data_file).stat().st_size,
                "digest": "sha256:"
                + hashlib.sha256(Path(data_file).read_bytes()).hexdigest(),
                "browser_download_url": "https://github.com/owner/repo/releases/download/report/"
                + Path(data_file).name,
            }
            self.assets.append(value)
        if self.lose == ("create" if data_file is None else Path(data_file).name):
            self.lose = None
            raise RuntimeError("response lost after remote effect")
        return value


class ReportPublicationTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / "workspace"
        self.root.mkdir()
        self.state = Path(self.tmp.name).resolve() / "owned-publication"
        self.remote = Remote()
        self.prefix = "docs/workflow/runs/" + RUN + "/"
        self.files = {}
        for name, data in [
            ("report.md", b"# Blocked\nNo product execution."),
            ("dashboard.html", b"<html>Blocked</html>"),
        ]:
            self.files[self.prefix + name] = data
        evidence = [
            {
                "path": path,
                "bytes": len(data),
                "sha256": hashlib.sha256(data).hexdigest(),
            }
            for path, data in self.files.items()
        ]
        whitelist = json.dumps({"files": evidence}).encode()
        self.files[self.prefix + "whitelist.json"] = whitelist
        for path, data in self.files.items():
            target = self.root / path
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
        archive = self.root / (self.prefix + "evidence.zip")
        with zipfile.ZipFile(archive, "w") as z:
            for path, data in list(self.files.items())[:2]:
                z.writestr(path, data)
        self.files[self.prefix + "evidence.zip"] = archive.read_bytes()
        self.descriptor = {
            "title": "【Harness验证】隔离报告发布与恢复",
            "product_head": HEAD,
            "body_file": self.prefix + "report.md",
            "dashboard": self.prefix + "dashboard.html",
            "whitelist": self.prefix + "whitelist.json",
            "archive": self.prefix + "evidence.zip",
            "files": [
                {
                    "path": path,
                    "bytes": len(data),
                    "sha256": hashlib.sha256(data).hexdigest(),
                }
                for path, data in self.files.items()
            ],
        }
        self.descriptor_path = self.root / (self.prefix + "report-publication.json")
        self.save_descriptor()

    def save_descriptor(self):
        self.descriptor_path.write_text(json.dumps(self.descriptor))

    def publish(self):
        return publication.publish(
            self.root, RUN, "owner/repo", HEAD, self.state, self.remote
        )

    def test_publish_then_recovery_checks_remote_digests_without_reposting(self):
        result = self.publish()
        writes = list(self.remote.writes)
        self.assertTrue(result["remote_digest_verified"])
        self.assertEqual(result["dashboard_delivery"], "download attachment")
        self.assertEqual(len(result["assets"]), 4)
        self.assertEqual(self.publish(), result)
        self.assertEqual(self.remote.writes, writes)

    def test_lost_create_response_is_reconciled_and_missing_assets_complete_once(self):
        self.remote.lose = "create"
        with self.assertRaises(RuntimeError):
            self.publish()
        self.publish()
        self.assertEqual(
            len([x for x in self.remote.writes if x.endswith("/releases")]), 1
        )

    def test_lost_upload_response_never_reuploads_existing_asset(self):
        self.remote.lose = "evidence.zip"
        with self.assertRaises(RuntimeError):
            self.publish()
        self.publish()
        self.assertEqual(
            len([x for x in self.remote.writes if "name=evidence.zip" in x]), 1
        )

    def test_unknown_missing_remote_effect_is_preserved_without_retry(self):
        self.remote.lose = "create"
        with self.assertRaises(RuntimeError):
            self.publish()
        self.remote.release = None
        count = len(self.remote.writes)
        with self.assertRaisesRegex(ValueError, "unknown"):
            self.publish()
        self.assertEqual(len(self.remote.writes), count)

    def test_changed_artifacts_after_attempt_cannot_replace_release(self):
        self.publish()
        self.descriptor["title"] = "【Harness验证】different binding"
        self.save_descriptor()
        with self.assertRaisesRegex(ValueError, "changed"):
            self.publish()

    def test_wrong_digest_remote_asset_never_overwrites_or_deletes(self):
        self.publish()
        self.remote.assets[0]["digest"] = "sha256:" + "0" * 64
        count = len(self.remote.writes)
        with self.assertRaisesRegex(ValueError, "digest"):
            self.publish()
        self.assertEqual(len(self.remote.writes), count)

    def test_public_repository_refuses_before_external_writes(self):
        self.remote.private = False
        with self.assertRaisesRegex(ValueError, "private"):
            self.publish()
        self.assertEqual(self.remote.writes, [])

    def test_symlinked_parent_and_source_changes_refuse_before_writes(self):
        folder = self.root / (self.prefix + "outside")
        folder.symlink_to(Path(self.tmp.name), target_is_directory=True)
        self.descriptor["body_file"] = self.prefix + "outside/report.md"
        self.descriptor["files"][0]["path"] = self.descriptor["body_file"]
        (Path(self.tmp.name) / "report.md").write_bytes(
            self.files[self.prefix + "report.md"]
        )
        self.save_descriptor()
        with self.assertRaises((ValueError, OSError)):
            self.publish()
        self.assertEqual(self.remote.writes, [])
        self.descriptor["body_file"] = self.prefix + "report.md"
        self.descriptor["files"][0]["path"] = self.descriptor["body_file"]
        self.save_descriptor()
        (self.root / self.descriptor["body_file"]).write_text("changed")
        with self.assertRaisesRegex(ValueError, "digest"):
            self.publish()
        self.assertEqual(self.remote.writes, [])

    def test_prepare_head_binding_rejects_missing_failed_and_drifted_receipts(self):
        receipt = {
            "node_id": "prepare",
            "status": "completed",
            "connector_receipt": {"exit_code": 0, "output": json.dumps({"head": HEAD})},
        }
        self.assertEqual(
            publication.prepared_head({"previous_results": [receipt]}, HEAD), HEAD
        )
        for context, current in [
            ({}, HEAD),
            ({"previous_results": [receipt]}, "c" * 40),
            ({"previous_results": [{**receipt, "status": "failed"}]}, HEAD),
        ]:
            with self.assertRaises(ValueError):
                publication.prepared_head(context, current)

    def test_protocol_state_name_attachment_cannot_collide_with_owned_state(self):
        path = self.prefix + "state.json"
        data = b'{"safe": true}'
        (self.root / path).write_bytes(data)
        self.descriptor["files"].append(
            {"path": path, "bytes": len(data), "sha256": publication.digest(data)}
        )
        self.save_descriptor()
        result = self.publish()
        self.assertEqual(len(result["assets"]), 5)
        self.assertTrue(
            json.loads((self.state / RUN / "state.json").read_text())[
                "create_attempted"
            ]
        )
        self.assertEqual(self.publish(), result)

    def test_title_credential_is_rejected_before_writes(self):
        self.descriptor["title"] = "【Harness验证】sk-" + "x" * 24
        self.save_descriptor()
        with self.assertRaisesRegex(ValueError, "credential"):
            self.publish()
        self.assertEqual(self.remote.writes, [])

    def test_extra_and_nested_compressed_archives_are_rejected(self):
        extra = self.prefix + "other.zip"
        buffer = io.BytesIO()
        with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_DEFLATED) as z:
            z.writestr("secret.txt", "sk-" + "x" * 24)
        data = buffer.getvalue()
        (self.root / extra).write_bytes(data)
        item = {"path": extra, "bytes": len(data), "sha256": publication.digest(data)}
        self.descriptor["files"].append(item)
        self.save_descriptor()
        with self.assertRaises(ValueError):
            self.publish()
        self.assertEqual(self.remote.writes, [])
        self.descriptor["files"].pop()
        whitelist_path = self.descriptor["whitelist"]
        data = json.dumps({"files": [item]}).encode()
        (self.root / whitelist_path).write_bytes(data)
        for entry in self.descriptor["files"]:
            if entry["path"] == whitelist_path:
                entry.update(bytes=len(data), sha256=publication.digest(data))
        self.save_descriptor()
        with self.assertRaises(ValueError):
            self.publish()
        self.assertEqual(self.remote.writes, [])

    def test_crash_temporary_files_do_not_block_state_or_receipt_recovery(self):
        self.state.mkdir()
        (self.state / "state.tmp").write_text("interrupted")
        publication.save_state(self.state / "state.json", {"intent": True})
        self.assertEqual(
            json.loads((self.state / "state.json").read_text()), {"intent": True}
        )
        self.assertEqual((self.state / "state.tmp").read_text(), "interrupted")
        (self.root / (self.prefix + "report-publication-receipt.md.tmp")).write_text(
            "interrupted"
        )
        result = self.publish()
        publication.write_receipt(self.root.resolve(), RUN, result)
        self.assertIn(
            result["release_url"],
            (self.root / (self.prefix + "report-publication-receipt.md")).read_text(),
        )

    def test_extra_zip_member_and_foreign_run_refuse_before_writes(self):
        with zipfile.ZipFile(self.root / self.descriptor["archive"], "a") as z:
            z.writestr("outside.txt", "not allowed")
        data = (self.root / self.descriptor["archive"]).read_bytes()
        for item in self.descriptor["files"]:
            if item["path"] == self.descriptor["archive"]:
                item.update(bytes=len(data), sha256=hashlib.sha256(data).hexdigest())
        self.save_descriptor()
        with self.assertRaisesRegex(ValueError, "whitelist"):
            self.publish()
        self.assertEqual(self.remote.writes, [])
        self.descriptor["files"][0]["path"] = (
            "docs/workflow/runs/" + "c" * 32 + "/report.md"
        )
        self.save_descriptor()
        with self.assertRaises(ValueError):
            self.publish()
        self.assertEqual(self.remote.writes, [])


if __name__ == "__main__":
    unittest.main()
