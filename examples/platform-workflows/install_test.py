"""Project verification commands remain argv, never implicit shell snippets."""

import importlib.util
import json
import subprocess
import sys
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location(
    "workflow_install", Path(__file__).with_name("install.py")
)
install = importlib.util.module_from_spec(spec)
spec.loader.exec_module(install)


class VerificationCommandTest(unittest.TestCase):
    def test_go_project_can_select_its_own_verification_entry(self):
        self.assertEqual(
            install.verification_command('["make", "verify"]'), ["make", "verify"]
        )

    def test_argument_is_not_evaluated_as_shell_code(self):
        literal = "$(exit 91); untouched"
        command = install.verification_command(
            json.dumps(
                [sys.executable, "-c", "import sys; print(sys.argv[1])", literal]
            )
        )
        result = subprocess.run(command, text=True, capture_output=True, check=True)
        self.assertEqual(result.stdout.strip(), literal)

    def test_invalid_command_cannot_become_env_options(self):
        for value in (
            "[]",
            '"go test ./..."',
            '["--ignore-environment"]',
            '["go", 1]',
            '["go", "a\\u0000b"]',
            "{}",
        ):
            with self.subTest(value=value), self.assertRaises(ValueError):
                install.verification_command(value)


class SharedBrowserTest(unittest.TestCase):
    def test_two_projects_reference_one_registration(self):
        import tempfile

        class API:
            url = "http://localhost"

            def __init__(self):
                self.items = []

            def call(self, method, path, body=None):
                if method == "GET":
                    return self.items
                if method == "POST" and path.endswith("/discover"):
                    return {}
                if method == "POST":
                    self.items.append(dict(body))
                    return self.items[-1]
                raise AssertionError((method, path))

        api = API()
        with tempfile.TemporaryDirectory() as tmp:
            for project in ("first", "second"):
                installation = install.Installation(
                    api, Path(tmp) / project / "install.json", {"project": project}
                )
                result = install.install_shared_browser(
                    api, installation, Path(tmp) / "shared.json", Path(tmp) / "evidence"
                )
                self.assertEqual(result["id"], "browser-validation")
            self.assertEqual(len(api.items), 1)
            self.assertIn("{{workspace}}", api.items[0]["connection"]["args"])


if __name__ == "__main__":
    unittest.main()
