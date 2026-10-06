import importlib.util
import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

SOURCE = Path(__file__).resolve().parents[1] / "controller.py"
SPEC = importlib.util.spec_from_file_location("preview_isolation", SOURCE)
controller = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(controller)


class DeploymentIsolationTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name)
        self.preview = self.install("preview", 5545, "example.preview")
        self.validation = self.install("validation", 5546, "example.validation")

    def install(self, name, port, label):
        root = self.base / name
        root.mkdir(mode=0o700, parents=True)
        (root / "preview.json").write_text(
            json.dumps(
                {
                    "repository": "example/model-relay",
                    "port": port,
                    "label": label,
                }
            )
        )
        return root

    def dispatch(self, root, action="prepare", protected=None):
        args = [
            str(SOURCE),
            action,
            "--root",
            str(root),
            "--plan",
            str(root / "plans/check.json"),
            "--sha",
            "a" * 40,
            "--protected-root",
            str(protected or self.preview),
        ]
        with (
            patch("sys.argv", args),
            patch.object(controller, "prepare") as prepare,
            patch.object(controller, "activate") as activate,
        ):
            controller.main()
            self.assertEqual(prepare.call_count, int(action == "prepare"))
            self.assertEqual(activate.call_count, int(action == "activate"))

    def test_distinct_installations_dispatch_both_actions(self):
        for action in ("prepare", "activate"):
            self.dispatch(self.validation, action)

    def test_refuses_same_nested_and_symlinked_roots_before_dispatch(self):
        nested = self.install("preview/nested", 5546, "example.validation")
        alias = self.base / "alias"
        alias.symlink_to(self.preview, target_is_directory=True)
        for root in (self.preview, nested, alias):
            for action in ("prepare", "activate"):
                with (
                    self.subTest(root=root, action=action),
                    self.assertRaisesRegex(ValueError, "overlap"),
                ):
                    self.dispatch(root, action)
        with self.assertRaisesRegex(ValueError, "overlap"):
            self.dispatch(self.preview, protected=nested)

    def test_refuses_shared_service_port_or_label(self):
        path = self.validation / "preview.json"
        original = json.loads(path.read_text())
        for field, value in (("port", 5545), ("label", "example.preview")):
            path.write_text(json.dumps({**original, field: value}))
            with (
                self.subTest(field=field),
                self.assertRaisesRegex(ValueError, "service"),
            ):
                self.dispatch(self.validation)

    def test_refuses_nested_database_links_and_service_repository_aliases(self):
        (self.preview / "data").mkdir()
        (self.validation / "data").mkdir()
        original = self.preview / "data/database.sqlite"
        original.write_text("test-only")
        alias = self.validation / "data/database.sqlite"
        for make_link in (
            lambda: alias.symlink_to(original),
            lambda: os.link(original, alias),
        ):
            make_link()
            with self.assertRaisesRegex(ValueError, "overlap|shared"):
                self.dispatch(self.validation)
            alias.unlink()
        for name in ("current", "repository"):
            (self.preview / name).mkdir()
            (self.validation / name).symlink_to(
                self.preview / name, target_is_directory=True
            )
            with self.assertRaisesRegex(ValueError, "overlap"):
                self.dispatch(self.validation)
            (self.validation / name).unlink()

    def test_refuses_nested_release_source_alias(self):
        old = self.preview / "releases" / ("a" * 40) / "source"
        old.mkdir(parents=True)
        release = self.validation / "releases" / ("a" * 40)
        release.mkdir(parents=True)
        (release / "source").symlink_to(old, target_is_directory=True)
        with self.assertRaisesRegex(ValueError, "overlap"):
            self.dispatch(self.validation)

    def test_refuses_shared_data_key_and_missing_protected_installation(self):
        data = self.preview / "data"
        data.mkdir()
        (self.validation / "data").symlink_to(data, target_is_directory=True)
        with self.assertRaisesRegex(ValueError, "overlap"):
            self.dispatch(self.validation)
        (self.validation / "data").unlink()
        key = self.preview / "master.key"
        key.write_text("test-only")
        os.link(key, self.validation / "master.key")
        with self.assertRaisesRegex(ValueError, "shared"):
            self.dispatch(self.validation)
        (self.validation / "master.key").unlink()
        with self.assertRaises(FileNotFoundError):
            self.dispatch(self.validation, protected=self.base / "missing")


if __name__ == "__main__":
    unittest.main()
