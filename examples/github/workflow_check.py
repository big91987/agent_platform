"""Parse GitHub workflows and syntax-check bash/sh run blocks without executing them.

Requires PyYAML from the example's installed Runner environment. This is a syntax
check, not a substitute for exercising Actions, approvals and deployment.
"""

import argparse
import re
import subprocess
from pathlib import Path

import yaml


def check(path):
    workflow = yaml.load(path.read_text(), Loader=yaml.BaseLoader)
    if (
        not isinstance(workflow, dict)
        or not workflow.get("on")
        or not workflow.get("jobs")
    ):
        raise ValueError(f"{path}: missing workflow triggers/jobs")
    for name, job in workflow["jobs"].items():
        for step in job.get("steps", []):
            if "run" not in step:
                continue
            shell = step.get(
                "shell",
                job.get("defaults", {})
                .get("run", {})
                .get(
                    "shell",
                    workflow.get("defaults", {}).get("run", {}).get("shell", "bash"),
                ),
            )
            if shell not in {"bash", "sh"}:
                raise ValueError(
                    f"{path}/{name}: add a syntax checker for shell {shell}"
                )
            script = re.sub(
                r"\$\{\{.*?\}\}", "expression_value", step["run"], flags=re.DOTALL
            )
            result = subprocess.run(
                [shell, "-n"], input=script, text=True, capture_output=True
            )
            if result.returncode:
                raise ValueError(f"{path}/{name}: {result.stderr}")
    print(f"Workflow syntax passed: {path}", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("paths", type=Path, nargs="+")
    for path in parser.parse_args().paths:
        check(path)


if __name__ == "__main__":
    main()
