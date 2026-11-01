import hashlib
import io
import tarfile
import tempfile
import unittest
from pathlib import Path

from install import install


class InstallTests(unittest.TestCase):
    def archive(self, root, name):
        path = root / "source.tar"
        with tarfile.open(path, "w") as archive:
            member = tarfile.TarInfo(name)
            member.size = 5
            archive.addfile(member, io.BytesIO(b"entry"))
        return path, hashlib.sha256(path.read_bytes()).hexdigest()

    def test_hash_checked_release_is_reentrant_and_unsafe_archive_is_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            archive, sha = self.archive(root, "examples/agent-team-factory/factory.py")
            target = install(archive, root / "runtime", "1" * 40, sha)
            self.assertEqual(
                (target / "examples/agent-team-factory/factory.py").read_bytes(),
                b"entry",
            )
            self.assertEqual(install(archive, root / "runtime", "1" * 40, sha), target)
            with self.assertRaises(ValueError):
                install(archive, root / "runtime", "2" * 40, "0" * 64)
            unsafe, digest = self.archive(root, "../escape")
            with self.assertRaises(ValueError):
                install(unsafe, root / "runtime", "3" * 40, digest)
            self.assertFalse((root / "escape").exists())


if __name__ == "__main__":
    unittest.main()
