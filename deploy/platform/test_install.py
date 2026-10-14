import fcntl
import importlib.util
import io
import json
import stat
import subprocess
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock

spec = importlib.util.spec_from_file_location(
    "platform_install", Path(__file__).with_name("install.py")
)
installer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installer)


class InstallationTest(unittest.TestCase):
    def invoke(self, parent, revision="a" * 40, *options):
        binary, config = self.fixture(parent, revision)
        return subprocess.run(
            [
                sys.executable,
                str(Path(__file__).with_name("install.py")),
                "--root",
                str(parent / "instance"),
                "--binary",
                str(binary),
                "--revision",
                revision,
                "--sandbox-config",
                str(config),
                "--sandbox-image",
                "sha256:" + "c" * 64,
                "--auth-home",
                str(parent / "native-auth"),
                *options,
            ],
            capture_output=True,
            text=True,
            timeout=10,
        )

    def test_cli_declares_bounded_environment_memory_and_lifetime(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            result = self.invoke(
                parent,
                "a" * 40,
                "--sandbox-memory-mib",
                "8192",
                "--sandbox-ttl",
                "14400",
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            root = parent / "instance"
            config = json.loads((root / "instance.json").read_text())
            self.assertEqual((config["memory_mib"], config["ttl"]), (8192, 14400))
            unit = (root / "agent-platform-8795.service").read_text()
            self.assertIn('"--sandbox-memory-mib" "8192"', unit)
            self.assertIn('"--sandbox-ttl" "14400"', unit)

    def test_invalid_memory_is_rejected_before_creating_an_instance(self):
        for value in ["0", "255", "65537"]:
            with self.subTest(memory=value), tempfile.TemporaryDirectory() as directory:
                parent = Path(directory)
                result = self.invoke(parent, "a" * 40, "--sandbox-memory-mib", value)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse((parent / "instance").exists())

    def test_environment_update_requires_explicit_choice_and_preserves_existing_install(
        self,
    ):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            self.prepare(parent)
            root = parent / "instance"
            credentials = (root / "credentials.env").read_bytes()
            database = root / "data" / "retained.sqlite"
            database.write_bytes(b"retained-run-data")
            before = (root / "instance.json").read_bytes()
            rejected = self.invoke(
                parent,
                "b" * 40,
                "--sandbox-memory-mib",
                "8192",
                "--sandbox-ttl",
                "14400",
            )
            self.assertNotEqual(rejected.returncode, 0)
            self.assertEqual((root / "instance.json").read_bytes(), before)
            accepted = self.invoke(
                parent,
                "b" * 40,
                "--sandbox-memory-mib",
                "8192",
                "--sandbox-ttl",
                "14400",
                "--update-environment",
            )
            self.assertEqual(accepted.returncode, 0, accepted.stderr)
            changed = (root / "instance.json").read_bytes()
            unchanged = self.invoke(parent, "c" * 40)
            self.assertEqual(unchanged.returncode, 0, unchanged.stderr)
            self.assertEqual((root / "instance.json").read_bytes(), changed)
            self.assertEqual((root / "credentials.env").read_bytes(), credentials)
            self.assertEqual(database.read_bytes(), b"retained-run-data")
            self.assertEqual(json.loads(changed)["memory_mib"], 8192)

    def test_environment_update_rejects_changed_unit_and_keeps_original_settings(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            self.prepare(parent)
            root = parent / "instance"
            unit = root / "agent-platform-8795.service"
            unit.write_text(unit.read_text() + "# externally edited\n")
            before = (root / "instance.json").read_bytes()
            active = (root / "agent-platform").resolve()
            result = self.invoke(
                parent,
                "b" * 40,
                "--sandbox-memory-mib",
                "8192",
                "--sandbox-ttl",
                "14400",
                "--update-environment",
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual((root / "instance.json").read_bytes(), before)
            self.assertEqual((root / "agent-platform").resolve(), active)

    def test_legacy_config_and_unit_upgrade_preserve_existing_values(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            self.prepare(parent)
            root = parent / "instance"
            config = root / "instance.json"
            legacy = json.loads(config.read_text())
            legacy.pop("memory_mib", None)
            config.write_text(json.dumps(legacy))
            before = config.read_bytes()
            unit = root / "agent-platform-8795.service"
            unit.write_text(
                unit.read_text().replace(' "--sandbox-memory-mib" "2048"', "")
            )
            upgraded = self.invoke(parent, "b" * 40)
            self.assertEqual(upgraded.returncode, 0, upgraded.stderr)
            self.assertEqual(config.read_bytes(), before)
            self.assertIn('"--sandbox-memory-mib" "2048"', unit.read_text())

    def test_partial_environment_update_can_finish_with_same_explicit_parameters(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            self.prepare(parent)
            config = parent / "instance" / "instance.json"
            old = config.read_bytes()
            first = self.invoke(
                parent,
                "b" * 40,
                "--sandbox-memory-mib",
                "8192",
                "--sandbox-ttl",
                "14400",
                "--update-environment",
            )
            self.assertEqual(first.returncode, 0, first.stderr)
            # Reconstruct a crash after the new unit was saved, before config rename.
            config.write_bytes(old)
            continued = self.invoke(
                parent,
                "b" * 40,
                "--sandbox-memory-mib",
                "8192",
                "--sandbox-ttl",
                "14400",
                "--update-environment",
            )
            self.assertEqual(continued.returncode, 0, continued.stderr)
            self.assertEqual(json.loads(config.read_text())["memory_mib"], 8192)

    def test_busy_installation_does_not_replace_environment_defaults(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            self.prepare(parent)
            root = parent / "instance"
            before = (root / "instance.json").read_bytes()
            with (root / ".install.lock").open("w") as lock:
                (root / ".install.lock").chmod(0o600)
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
                result = self.invoke(
                    parent,
                    "b" * 40,
                    "--sandbox-memory-mib",
                    "8192",
                    "--update-environment",
                )
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual((root / "instance.json").read_bytes(), before)

    def test_cli_holds_installation_lock_until_activation_finishes(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            self.prepare(parent)
            binary, config = self.fixture(parent, "b" * 40)

            def activation(result):
                competing = self.invoke(
                    parent,
                    "c" * 40,
                    "--sandbox-memory-mib",
                    "4096",
                    "--update-environment",
                )
                self.assertNotEqual(
                    competing.returncode,
                    0,
                    "activation lost exclusive configuration ownership",
                )
                return {"ok": True, "revision": result["revision"]}

            arguments = [
                "install.py",
                "--root",
                str(parent / "instance"),
                "--binary",
                str(binary),
                "--revision",
                "b" * 40,
                "--sandbox-config",
                str(config),
                "--sandbox-image",
                "sha256:" + "c" * 64,
                "--auth-home",
                str(parent / "native-auth"),
                "--sandbox-memory-mib",
                "8192",
                "--update-environment",
                "--activate",
            ]
            with (
                mock.patch.object(sys, "argv", arguments),
                mock.patch.object(installer, "activate", side_effect=activation),
                redirect_stdout(io.StringIO()),
            ):
                installer.main()
            self.assertEqual(
                json.loads((parent / "instance" / "instance.json").read_text())[
                    "memory_mib"
                ],
                8192,
            )

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

    def test_runtime_environment_upgrade_preserves_secrets_and_rejects_public_files(
        self,
    ):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            self.prepare(parent)
            root = parent / "instance"
            unit = root / "agent-platform-8795.service"
            optional = "EnvironmentFile=-" + str(root.resolve() / "runtime.env") + "\n"
            # Recognized original unit is the only supported automatic migration.
            unit.write_text(unit.read_text().replace(optional, ""))
            runtime = root / "runtime.env"
            runtime.write_text("PROJECT_GITHUB_TOKEN=private-fixture-only\n")
            runtime.chmod(0o600)
            self.prepare(parent, "b" * 40)
            self.assertIn(optional, unit.read_text())
            self.assertEqual(
                runtime.read_text(), "PROJECT_GITHUB_TOKEN=private-fixture-only\n"
            )
            before = (root / "agent-platform").resolve()
            runtime.chmod(0o644)
            with self.assertRaisesRegex(ValueError, "private"):
                self.prepare(parent, "c" * 40)
            self.assertEqual((root / "agent-platform").resolve(), before)
            runtime.chmod(0o600)
            runtime.write_text("AGENT_PLATFORM_PASSWORD=override\n")
            with self.assertRaisesRegex(ValueError, "runtime"):
                self.prepare(parent, "c" * 40)

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
                    update_environment=True,
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
