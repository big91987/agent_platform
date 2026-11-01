import argparse
import json
import subprocess
from pathlib import Path

from .engine import analyze
from .review import bundle, execute, validate


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Architecture rules and evidence-bound model review"
    )
    sub = parser.add_subparsers(dest="action", required=True)
    for name in ("check", "bundle", "review", "validate-review"):
        command = sub.add_parser(name)
        command.add_argument("--root", type=Path, default=Path("."))
        if name != "validate-review":
            command.add_argument("--config", default="architecture.json")
        command.add_argument("--out", type=Path)
        if name in ("bundle", "review"):
            command.add_argument("--max-bytes", type=int, default=2_000_000)
        if name == "review":
            command.add_argument(
                "--command",
                required=True,
                help="JSON argv of trusted reviewer; request on stdin, JSON result on stdout",
            )
            command.add_argument("--timeout", type=float, default=600)
            command.add_argument("--pass-env", action="append", default=[])
        if name == "validate-review":
            command.add_argument("--bundle", required=True, type=Path)
            command.add_argument("--result", required=True, type=Path)
    args = parser.parse_args(argv)
    try:
        if args.action == "check":
            report, _ = analyze(args.root, args.config)
        elif args.action == "bundle":
            report = bundle(args.root, args.config, args.max_bytes)
        elif args.action == "review":
            request = bundle(args.root, args.config, args.max_bytes)
            result = execute(
                args.root,
                request,
                json.loads(args.command),
                args.timeout,
                args.pass_env,
            )
            report = validate(args.root, request, result)
        else:
            report = validate(
                args.root,
                json.loads(args.bundle.read_text()),
                json.loads(args.result.read_text()),
            )
        code = (
            2
            if report.get("status") == "incomplete"
            else (1 if report.get("status") in ("violations", "review_required") else 0)
        )
    except (
        ValueError,
        OSError,
        TimeoutError,
        KeyError,
        TypeError,
        subprocess.SubprocessError,
    ) as error:
        report = dict(
            status="incomplete",
            errors=[str(error)],
            model_review=dict(
                status="incomplete"
                if args.action in ("review", "validate-review")
                else "not_run"
            ),
        )
        code = 2
    text = json.dumps(report, ensure_ascii=False, indent=2) + "\n"
    if args.out:
        args.out.parent.mkdir(parents=True, exist_ok=True)
        args.out.write_text(text)
    print(text, end="")
    return code
