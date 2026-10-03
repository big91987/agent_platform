import os
import subprocess
import sys
import tempfile
import unittest


class PortableToolingTest(unittest.TestCase):
    def test_bundled_runtime_imports_without_an_external_checkout(self):
        import tooling

        root = tooling.tooling_source({"checkout": "/missing/product"})
        with tempfile.TemporaryDirectory() as cwd:
            result = subprocess.run(
                [
                    sys.executable,
                    "-I",
                    "-c",
                    "import sys;sys.path.insert(0,sys.argv[1]);"
                    "from full_harness.browser import check;"
                    "from full_harness.common import controls;"
                    "from full_harness.quality import python_files;"
                    "assert controls(__import__('pathlib').Path('.')) == {}",
                    str(root),
                ],
                cwd=cwd,
                capture_output=True,
                text=True,
                env={"PATH": os.environ["PATH"]},
            )
        self.assertEqual(result.returncode, 0, result.stderr)
        for name in ("package.json", "package-lock.json", "check.cjs"):
            self.assertTrue((root / "full_harness/browser" / name).is_file())

    def test_explicit_source_is_kept_but_missing_override_fails(self):
        import tooling

        self.assertEqual(
            tooling.tooling_source({"browser_source": str(tooling.BUNDLED)}),
            tooling.BUNDLED,
        )
        with self.assertRaises(ValueError):
            tooling.tooling_source({"browser_source": "/missing/harness"})
