import hashlib
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).with_name("tooling")))
from full_harness.common import controls, files
from full_harness.quality import python_files


class CheckpointScopeTest(unittest.TestCase):
    def test_evidence_volume_does_not_block_control_or_python_checks(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".github").mkdir()
            control = root / ".github/check.yml"
            control.write_text("on: push")
            (root / "app.py").write_text("print(1)")
            evidence = root / "screenshots"
            evidence.mkdir()
            for n in range(31):
                (evidence / f"{n}.png").write_bytes(b"x" * 2_000_000)
            expected = hashlib.sha256(b"on: push0").hexdigest()
            self.assertEqual(controls(root), {".github/check.yml": expected})
            self.assertEqual(python_files(root), ["app.py"])
            # Full import limits remain intact; this fix does not raise them.
            with self.assertRaisesRegex(ValueError, "Workspace exceeds import limit"):
                files(root)
            control.chmod(0o755)
            self.assertNotEqual(controls(root)[".github/check.yml"], expected)
            control.write_text("on: pull_request")
            self.assertNotEqual(controls(root)[".github/check.yml"], expected)

    def test_control_symlink_and_oversized_control_still_fail(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".github").mkdir()
            control = root / ".github/check.yml"
            control.symlink_to(root / "missing")
            with self.assertRaisesRegex(ValueError, "Symlinks"):
                controls(root)
            control.unlink()
            control.write_bytes(b"x" * 2_000_001)
            with self.assertRaisesRegex(ValueError, "File exceeds import limit"):
                controls(root)
