import importlib.util
import json
import stat
import tempfile
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location(
    "platform_install", Path(__file__).with_name("install.py")
)
installer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installer)


class InstallationTest(unittest.TestCase):
    def fixture(self, parent, revision="a" * 40):
        binary = parent / ("binary-" + revision[:12])
        if not binary.exists():
            binary.write_text(
                "#!/bin/sh\nprintf '%s\\n' '"
                + json.dumps({"version": "0.1.0", "revision": revision})
                + "'\n"
            )
        binary.chmod(0o700)
        config = parent / "sandbox.toml"
        if not config.exists():
            config.write_text(
                "[server]\nhost='127.0.0.1'\nport=8095\napi_key='private-test-only'\n"
            )
            config.chmod(0o600)
        return binary, config

    def prepare(self, parent, revision="a" * 40):
        binary, config = self.fixture(parent, revision)
        return installer.prepare(
            parent / "instance",
            binary,
            revision,
            "127.0.0.1:8795",
            "http://127.0.0.1:8795",
            config,
            "sha256:" + "c" * 64,
            3600,
            parent / "native-auth",
        )

    def test_new_install_and_upgrade_preserve_private_config_credentials_and_data(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            initial = self.prepare(parent)
            root = parent / "instance"
            credential = (root / "credentials.env").read_bytes()
            self.assertNotIn(b"admin\n", credential)
            self.assertEqual(
                stat.S_IMODE((root / "credentials.env").stat().st_mode), 0o600
            )
            self.assertEqual(stat.S_IMODE(root.stat().st_mode), 0o700)
            database = root / "data" / "retained.sqlite"
            database.write_bytes(b"old-instance-data")
            config = (root / "instance.json").read_bytes()
            upgraded = self.prepare(parent, "b" * 40)
            self.assertEqual((root / "credentials.env").read_bytes(), credential)
            self.assertEqual((root / "instance.json").read_bytes(), config)
            self.assertEqual(database.read_bytes(), b"old-instance-data")
            self.assertEqual(upgraded["revision"], "b" * 40)
            self.assertEqual(initial["revision"], "a" * 40)
            self.assertTrue(
                (root / "releases" / ("a" * 40) / "agent-platform").exists()
            )
            self.assertEqual(
                (root / "agent-platform").resolve(),
                root.resolve() / "releases" / ("b" * 40) / "agent-platform",
            )

    def test_mismatched_binary_revision_and_public_sandbox_config_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            binary, config = self.fixture(parent)
            with self.assertRaisesRegex(ValueError, "revision"):
                installer.prepare(
                    parent / "instance",
                    binary,
                    "b" * 40,
                    "127.0.0.1:8795",
                    "http://127.0.0.1:8795",
                    config,
                    "sha256:" + "c" * 64,
                    3600,
                    parent / "native-auth",
                )
            config.chmod(0o644)
            with self.assertRaisesRegex(ValueError, "private"):
                self.prepare(parent)
            self.assertFalse((parent / "instance").exists())

    def test_upgrade_cannot_silently_change_listener_or_sandbox_binding(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            self.prepare(parent)
            root = parent / "instance"
            before = (root / "agent-platform").resolve()
            binary, config = self.fixture(parent, "b" * 40)
            with self.assertRaisesRegex(ValueError, "configuration"):
                installer.prepare(
                    root,
                    binary,
                    "b" * 40,
                    "127.0.0.1:8796",
                    "http://127.0.0.1:8796",
                    config,
                    "sha256:" + "c" * 64,
                    3600,
                    parent / "native-auth",
                )
            self.assertEqual((root / "agent-platform").resolve(), before)

    def test_symlinked_private_files_and_changed_same_revision_binary_fail_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            self.prepare(parent)
            root = parent / "instance"
            credential = root / "credentials.env"
            outside = parent / "outside"
            credential.rename(outside)
            credential.symlink_to(outside)
            with self.assertRaisesRegex(ValueError, "private"):
                self.prepare(parent, "b" * 40)
            credential.unlink()
            outside.rename(credential)
            binary, _ = self.fixture(parent)
            binary.write_text(binary.read_text() + "# changed bytes\n")
            with self.assertRaisesRegex(ValueError, "same revision"):
                self.prepare(parent)


if __name__ == "__main__":
    unittest.main()
