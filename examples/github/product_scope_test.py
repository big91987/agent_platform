"""Delivery scope must not accidentally authorize an entire tooling directory."""

import tempfile
import unittest
from pathlib import Path

from product_scope import allowed_product_path, extra_files, validate_files


class ProductScopeTest(unittest.TestCase):
    def test_exact_file_does_not_allow_siblings_children_or_control_instructions(self):
        settings = {"extra_product_files": ["scripts/service.py"]}
        self.assertTrue(allowed_product_path("scripts/service.py", settings))
        for name in ("scripts/deploy.py", "scripts/service.py/child", "app/AGENTS.md"):
            self.assertFalse(allowed_product_path(name, settings))
        for name in (
            "scripts/*",
            "../service.py",
            ".github/workflows/a.yml",
            "app/AGENTS.md",
            ":(glob)*",
        ):
            with self.subTest(name=name), self.assertRaises(ValueError):
                extra_files({"extra_product_files": [name]})

    def test_directory_and_symlink_cannot_expand_the_configured_scope(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / "scripts").mkdir()
            settings = {"extra_product_files": ["scripts"]}
            with self.assertRaises(ValueError):
                validate_files(root, settings)
            (root / "external").symlink_to(root / "scripts", target_is_directory=True)
            with self.assertRaises(ValueError):
                validate_files(root, {"extra_product_files": ["external/service.py"]})
