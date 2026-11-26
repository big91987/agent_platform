import importlib.util
import json
import os
import plistlib
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


def load(name):
    spec = importlib.util.spec_from_file_location(
        name, Path(__file__).with_name(name + ".py")
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class InstallTest(unittest.TestCase):
    def test_private_files_and_existing_symlink_refused(self):
        m = load("install")
        r = load("service")
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / "private.json"
            m.write(p, b'{"x":"ok"}')
            self.assertEqual(r.read_private(p), {"x": "ok"})
            p.chmod(0o644)
            with self.assertRaises(ValueError):
                r.read_private(p)
            link = Path(d) / "link"
            link.symlink_to(p)
            with self.assertRaises(ValueError):
                m.write(link, b"{}")
            self.assertEqual(json.loads(p.read_text()), {"x": "ok"})

    def test_separate_loopback_instance_and_cli_arguments(self):
        m = load("install")
        root = Path("/private/instance")
        a = m.specification(
            root, root / "bin", 8797, Path("/bin/claude"), Path("/private/claude"), 3
        )["argv"]
        self.assertEqual(a[a.index("-listen") + 1], "127.0.0.1:8797")
        self.assertEqual(a[a.index("-data") + 1], "/private/instance/data")
        self.assertEqual(a[a.index("-claude-config-home") + 1], "/private/claude")

    def test_upgrade_preserves_instance_and_refuses_binary_drift(self):
        m = load("install")
        with tempfile.TemporaryDirectory() as d:
            root = Path(d).resolve()
            binary, new = root / "agent-platform", root / "candidate"
            m.write(binary, b"old", 0o700)
            m.write(new, b"new", 0o700)
            config = m.specification(
                root, binary, 8797, Path("/bin/claude"), root / "cc", 5
            )
            config["build"] = {"revision": "old", "sha256": m.digest(b"old")}
            original = json.dumps(config).encode()
            m.write(root / "instance.json", original)
            m.write(root / "credentials.json", b'{"PRIVATE":"fixture"}')
            (root / "data").mkdir()
            (root / "data/db").write_bytes(b"history")
            plist = root / "job.plist"
            plistraw = plistlib.dumps(
                {
                    "Label": "com.agent-platform.local.8797",
                    "WorkingDirectory": str(root),
                    "ProgramArguments": [
                        "/bin/python3",
                        str(root / "service.py"),
                        str(root),
                    ],
                }
            )
            m.write(plist, plistraw)
            attempts = []

            def fail_new_start(argv, **_options):
                attempts.append(argv[1])
                if argv[1] == "kickstart":
                    self.assertEqual(
                        argv[2:],
                        ["-k", f"gui/{os.getuid()}/com.agent-platform.local.8797"],
                    )
                    expected = b"new" if len(attempts) == 2 else b"old"
                    self.assertEqual(binary.read_bytes(), expected)
                    build = json.loads((root / "instance.json").read_bytes())["build"]
                    self.assertEqual(build["sha256"], m.digest(expected))
                if attempts == ["print", "kickstart"]:
                    raise subprocess.CalledProcessError(1, argv)
                return subprocess.CompletedProcess(argv, 0)

            with patch.object(m.subprocess, "run", side_effect=fail_new_start):
                with self.assertRaises(subprocess.CalledProcessError):
                    m.upgrade(root, new, {"revision": "new"}, 8797, True, True, plist)
            self.assertEqual(attempts, ["print", "kickstart", "kickstart"])
            self.assertEqual(binary.read_bytes(), b"old")
            self.assertEqual((root / "instance.json").read_bytes(), original)

            with patch.object(
                m.subprocess,
                "run",
                side_effect=[
                    subprocess.CompletedProcess([], 113),
                    subprocess.CalledProcessError(5, ["launchctl", "bootstrap"]),
                ],
            ) as unloaded:
                with self.assertRaises(subprocess.CalledProcessError):
                    m.upgrade(root, new, {"revision": "new"}, 8797, True, False, plist)
                self.assertEqual(
                    [c.args[0][1] for c in unloaded.call_args_list],
                    ["print", "bootstrap"],
                )
            self.assertEqual(binary.read_bytes(), b"old")
            self.assertEqual((root / "instance.json").read_bytes(), original)

            with patch.object(
                m.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)
            ) as control:
                with self.assertRaisesRegex(ValueError, "no active work"):
                    m.upgrade(root, new, {"revision": "new"}, 8797, True, False, plist)
                self.assertEqual(
                    [call.args[0][1] for call in control.call_args_list], ["print"]
                )
                control.reset_mock()
                self.assertTrue(
                    m.upgrade(root, new, {"revision": "new"}, 8797, True, True, plist)
                )
                self.assertEqual(
                    [call.args[0][1] for call in control.call_args_list],
                    ["print", "kickstart"],
                )
                self.assertFalse(
                    m.upgrade(root, new, {"revision": "new"}, 8797, True, True, plist)
                )
                self.assertEqual(
                    [call.args[0][1] for call in control.call_args_list],
                    ["print", "kickstart"],
                )
                current = json.loads((root / "instance.json").read_text())
                self.assertEqual(current["build"]["revision"], "new")
                current["build"] = config["build"]
                self.assertEqual(current, config)
                saved = root / "previous-builds" / m.digest(b"old")
                self.assertEqual((saved / "agent-platform").read_bytes(), b"old")
                self.assertEqual((saved / "instance.json").read_bytes(), original)
                self.assertEqual(
                    (root / "credentials.json").read_bytes(), b'{"PRIVATE":"fixture"}'
                )
                self.assertEqual((root / "data/db").read_bytes(), b"history")
                self.assertEqual(plist.read_bytes(), plistraw)
                binary.write_bytes(b"drift")
                with self.assertRaisesRegex(ValueError, "recorded digest"):
                    m.upgrade(root, new, {"revision": "new"}, 8797, True, True, plist)
