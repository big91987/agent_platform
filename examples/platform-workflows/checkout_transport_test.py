"""Execute the actual pre-checkout step without network or repository code."""

import json
import os
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path


class CheckoutTransportTest(unittest.TestCase):
    def run_step(self, template, config):
        source = Path(__file__).with_name(template).read_text()
        before_checkout = source.split("      - uses: actions/checkout@", 1)[0]
        self.assertIn("      - name: Configure GitHub transport", before_checkout)
        script = before_checkout.split("        run: |\n", 1)[1]
        script = textwrap.dedent(script)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            private = root / "private.json"
            private.write_text(json.dumps(config))
            environment = root / "environment"
            environment.touch()
            result = subprocess.run(
                ["bash", "-e", "-c", script],
                env={
                    **os.environ,
                    "AGENT_PLATFORM_CONFIG": str(private),
                    "GITHUB_OUTPUT": str(environment),
                    "http_proxy": "http://runner-http.example:1",
                    "https_proxy": "http://runner-https.example:2",
                    "HTTP_PROXY": "http://RUNNER-http.example:3",
                    "HTTPS_PROXY": "http://RUNNER-https.example:4",
                },
                cwd=root,
                capture_output=True,
                text=True,
                timeout=5,
            )
            return result, environment.read_text()

    def test_actions_environment_keys_are_unique_ignoring_case(self):
        # GitHub rejects case-only duplicate env keys before a job can start.
        for template in ("sandbox-entry.yml", "github-entry.yml"):
            with self.subTest(template=template):
                keys = None
                for line in Path(__file__).with_name(template).read_text().splitlines():
                    if line == "        env:":
                        keys = set()
                    elif keys is not None and line.startswith("          "):
                        key = line.strip().split(":", 1)[0].casefold()
                        self.assertNotIn(key, keys)
                        keys.add(key)
                    else:
                        keys = None

    def test_proxy_reaches_checkout_without_exporting_private_config(self):
        for template in ("sandbox-entry.yml", "github-entry.yml"):
            with self.subTest(template=template):
                proxy = "http://proxy.example:7890"
                result, environment = self.run_step(
                    template, {"git_proxy": proxy, "token": "private-token"}
                )
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(
                    dict(line.split("=", 1) for line in environment.splitlines()),
                    dict.fromkeys(
                        ("http_proxy", "https_proxy"),
                        proxy,
                    ),
                )
                self.assertEqual(result.stdout, ("::add-mask::" + proxy + "\n") * 2)
                self.assertEqual(result.stderr, "")
                self.assertNotIn(
                    "private-token", environment + result.stdout + result.stderr
                )

    def test_mask_command_escapes_workflow_command_encoding(self):
        proxy = "http://proxy.example:7890?value=%0A"
        result, environment = self.run_step("sandbox-entry.yml", {"git_proxy": proxy})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            result.stdout, ("::add-mask::" + proxy.replace("%", "%25") + "\n") * 2
        )
        self.assertEqual(
            dict(line.split("=", 1) for line in environment.splitlines())[
                "https_proxy"
            ],
            proxy,
        )

    def test_optional_proxy_does_not_clear_runner_environment(self):
        for config in ({}, {"git_proxy": ""}):
            result, environment = self.run_step("sandbox-entry.yml", config)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(
                dict(line.split("=", 1) for line in environment.splitlines()),
                {
                    "http_proxy": "http://runner-http.example:1",
                    "https_proxy": "http://runner-https.example:2",
                },
            )

    def test_invalid_proxy_cannot_inject_environment_or_leak_credentials(self):
        for proxy in (
            "http://private:secret@proxy.example:7890",
            "http://proxy.example:7890\nGH_TOKEN=injected",
            "http://proxy.example:7890\x00",
            "file:///private/config",
            "http://",
            123,
        ):
            with self.subTest(proxy=repr(proxy)):
                result, environment = self.run_step(
                    "sandbox-entry.yml", {"git_proxy": proxy}
                )
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(environment, "")
                self.assertNotIn("secret", result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
