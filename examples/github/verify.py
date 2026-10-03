#!/usr/bin/env python3
"""Owner-configured development gate, usable by native Stop Hook and delivery job."""

import argparse
import hashlib
import json
import subprocess
import sys
from pathlib import Path

from tooling import tooling_source


def quality_commands(fix=False):
    quality = Path(__file__).with_name("quality")
    return [
        [
            str(quality / "node_modules/.bin/prettier"),
            "--write" if fix else "--check",
            "--config",
            str(quality / "prettier.json"),
            "app/**/*.{js,html,css}",
        ],
        [
            str(quality / "node_modules/.bin/eslint"),
            "--no-config-lookup",
            "--config",
            str(quality / "eslint.config.mjs"),
            *(["--fix"] if fix else []),
            "app/**/*.js",
        ],
    ]


def regression_plan(root):
    # Product locators evolve with the UI; executable Harness controls do not.
    plan = root / "tests/browser/core.json"
    if not plan.exists():
        plan = root / ".harness/reading-core.json"
    if not plan.resolve().is_relative_to(root.resolve()):
        raise ValueError("Regression plan must remain inside the workspace")
    actions = json.loads(plan.read_text())
    if not isinstance(actions, list) or not actions:
        raise ValueError("Regression plan must contain browser actions")
    return plan


def verify(config, workspace, number, evidence_name="delivery-checks"):
    root = workspace.resolve()
    plan = root / f"docs/05-validation/tasks/{number}/browser-plan.json"
    if not plan.is_file():
        raise ValueError(
            "Missing feature browser-plan.json; implement and validate the agreed feature before delivery"
        )
    evidence = root / f"docs/05-validation/tasks/{number}/{evidence_name}"
    evidence.mkdir(parents=True, exist_ok=True)
    script = str(tooling_source(config) / "full_harness/browser/check.cjs")
    commands = [["git", "diff", "--check"], *quality_commands()]
    commands += [
        ["node", "--check", str(p)] for p in sorted((root / "app").rglob("*.js"))
    ]
    commands += [
        ["node", script, str(root / "app"), str(evidence / label), str(p)]
        for label, p in (
            ("existing", regression_plan(root)),
            ("feature", plan),
        )
    ]
    tests = sorted((root / "tests").glob("*.test.cjs"))
    if tests:
        commands.append(["node", "--test", *map(str, tests)])
    if (root / "tests/local_deploy_test.py").is_file():
        commands.append(
            [
                config["python"],
                "-m",
                "unittest",
                "discover",
                "-s",
                "tests",
                "-p",
                "local_deploy_test.py",
            ]
        )
    # Product Python, when present, uses the existing repository quality contract.
    if list((root / "app").rglob("*.py")) or (root / "cli").exists():
        commands.append(
            [
                config["python"],
                str(tooling_source(config) / "full_harness/quality.py"),
                "check",
            ]
        )
    records = []
    (evidence / "checks.json").write_text("[]")
    for i, command in enumerate(commands):
        p = subprocess.run(
            command, cwd=root, text=True, capture_output=True, timeout=180
        )
        (evidence / f"check-{i}.log").write_text(p.stdout + p.stderr)
        records.append(
            {
                "command": [
                    v.replace(str(root), "<workspace>")
                    .replace(
                        str(tooling_source(config)),
                        "<browser-tooling>",
                    )
                    .replace(config["checkout"], "<tooling>")
                    .replace(str(Path(__file__).resolve().parents[2]), "<platform>")
                    for v in command
                ],
                "exit_code": p.returncode,
            }
        )
        (evidence / "checks.json").write_text(json.dumps(records, indent=2))
        if p.returncode:
            raise RuntimeError(
                "Check failed: "
                + " ".join(command)
                + "\n"
                + (p.stdout + p.stderr)[-3000:]
            )
    # No host-specific paths in committed evidence.
    for record in records:
        record["command"] = [
            v.replace(str(root), "<workspace>")
            .replace(config["checkout"], "<tooling>")
            .replace(str(Path(__file__).resolve().parents[2]), "<platform>")
            for v in record["command"]
        ]
    (evidence / "checks.json").write_text(json.dumps(records, indent=2))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    parser.add_argument("--workspace")
    parser.add_argument("--issue", type=int)
    parser.add_argument("--hook", action="store_true")
    parser.add_argument(
        "--evidence-name",
        choices=("delivery-checks", "runner-checks"),
        default="delivery-checks",
    )
    parser.add_argument(
        "--fix",
        action="store_true",
        help="Format and auto-fix product JavaScript before checking",
    )
    args = parser.parse_args()
    config = json.loads(Path(args.config).read_text())["pipeline"]
    workspace = Path(args.workspace or Path.cwd()).resolve()
    if args.fix:
        for command in quality_commands(fix=True):
            subprocess.run(command, cwd=workspace, check=True, timeout=180)
        return
    if args.hook:
        payload = json.load(sys.stdin)
        directory = (
            Path(config["registry"])
            / hashlib.sha256(str(workspace).encode()).hexdigest()
        )
        registration = json.loads((directory / "development.json").read_text())
        number = int(registration["inputs"]["issue"])
        # A accepted handoff is rechecked by the delivery Job after this turn ends.
        if Path(registration["result_file"]).exists():
            print("{}")
            return
        try:
            verify(config, workspace, number)
        except Exception as error:
            # Native loop protection: a second Stop failure is visible to the user.
            if payload.get("stop_hook_active"):
                print(
                    json.dumps(
                        {
                            "systemMessage": "Development checks still fail: "
                            + str(error)
                        }
                    )
                )
            else:
                print(
                    json.dumps(
                        {"decision": "block", "reason": str(error)}, ensure_ascii=False
                    )
                )
            return
        print("{}")
    else:
        verify(config, workspace, args.issue, args.evidence_name)
        print("Development checks passed", flush=True)


if __name__ == "__main__":
    main()
