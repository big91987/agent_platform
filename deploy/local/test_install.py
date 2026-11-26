import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


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
