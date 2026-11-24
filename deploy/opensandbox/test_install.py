import importlib.util
import os
import tempfile
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location(
    "sandbox_install", Path(__file__).with_name("install.py")
)
installer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installer)


class InstallationTests(unittest.TestCase):
    def test_repeated_prepare_preserves_credentials_and_operator_changes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = installer.prepare(Path(directory) / "runtime", 8095)
            config = root / "sandbox.toml"
            original = config.read_text() + "\n# operator settings\n"
            config.write_text(original)
            installer.prepare(root, 9000)
            self.assertEqual(config.read_text(), original)
            self.assertEqual(config.stat().st_mode & 0o777, 0o600)
            self.assertEqual(root.stat().st_mode & 0o777, 0o700)

    def test_explicit_rootless_endpoint_survives_upgrade(self):
        with tempfile.TemporaryDirectory() as directory:
            root = installer.prepare(Path(directory) / "runtime", 8096)
            installer.docker_environment(root, "unix:///run/user/1009/docker.sock")
            before = (root / "docker.env").read_bytes()
            installer.docker_environment(root, None)
            self.assertEqual((root / "docker.env").read_bytes(), before)
            self.assertEqual((root / "docker.env").stat().st_mode & 0o777, 0o600)
            with self.assertRaises(ValueError):
                installer.docker_environment(root, "unix:///var/run/docker.sock")
            self.assertEqual((root / "docker.env").read_bytes(), before)
            self.assertIn("EnvironmentFile=", installer.service_unit(root))

    def test_docker_endpoint_rejects_injected_environment_and_links(self):
        with tempfile.TemporaryDirectory() as directory:
            root = installer.prepare(Path(directory) / "runtime", 8096)
            for endpoint in [
                "tcp://localhost:2375",
                "unix:///tmp/docker.sock\nOTHER=value",
            ]:
                with self.assertRaises(ValueError):
                    installer.docker_environment(root, endpoint)
            self.assertFalse((root / "docker.env").exists())
            outside = Path(directory) / "outside"
            outside.write_text("unchanged")
            (root / "docker.env").symlink_to(outside)
            with self.assertRaises(ValueError):
                installer.docker_environment(root, "unix:///run/user/1009/docker.sock")
            self.assertEqual(outside.read_text(), "unchanged")

    def test_shared_or_linked_configuration_is_rejected_without_writing(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            outside = root / "outside"
            outside.write_text("preserve")
            (root / "sandbox.toml").symlink_to(outside)
            with self.assertRaises(ValueError):
                installer.prepare(root, 8095)
            self.assertEqual(outside.read_text(), "preserve")
            (root / "sandbox.toml").unlink()
            os.chmod(root, 0o755)
            with self.assertRaises(ValueError):
                installer.prepare(root, 8095)
            self.assertFalse((root / "sandbox.toml").exists())


if __name__ == "__main__":
    unittest.main()
