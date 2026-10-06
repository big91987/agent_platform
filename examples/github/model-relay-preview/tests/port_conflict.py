#!/usr/bin/env python3
"""Bounded, read-only observer plus TCP fault for an isolated Actions deployment.

Exit zero means the fault window was exercised, never that recovery passed.
"""

import argparse
import importlib.util
import json
import socket
import sys
import time
from pathlib import Path


def read_journal(root):
    try:
        return json.loads((root / "activation.json").read_text())
    except FileNotFoundError:
        return {}


def respond_unavailable(listener):
    """One bounded read/write; incomplete headers cannot hold the listener open."""
    try:
        connection, _ = listener.accept()
    except TimeoutError:
        return False
    with connection:
        connection.settimeout(0.1)
        try:
            request = connection.recv(4096)
        except (TimeoutError, OSError):
            return False
        health_request = request.startswith(b"GET /healthz ")
        body = b'{"status":"unavailable","fixture":"isolated-port-conflict"}'
        response = (
            b"HTTP/1.1 503 Service Unavailable\r\n"
            b"Content-Type: application/json\r\n"
            b"Connection: close\r\n"
            + f"Content-Length: {len(body)}\r\n\r\n".encode()
            + body
        )
        try:
            connection.sendall(response)
        except (TimeoutError, OSError):
            pass
        return health_request


def exercise(
    root,
    port,
    old_sha,
    candidate_sha,
    previous_attempt,
    *,
    arm_timeout=1800,
    hold_seconds=25,
    upgrade_timeout=130,
    first_health_timeout=15,
    poll_seconds=0.05,
):
    deadline = time.monotonic() + arm_timeout
    while time.monotonic() < deadline:
        journal = read_journal(root)
        if (
            journal.get("stage") == "backup_complete"
            and journal.get("attempt") != previous_attempt
            and journal.get("plan", {}).get("sha") == candidate_sha
            and (journal.get("previous") or {}).get("sha") == old_sha
        ):
            break
        time.sleep(poll_seconds)
    else:
        raise RuntimeError(
            "new candidate backup window not observed; no fault injected"
        )
    attempt = journal["attempt"]
    # Binding never displaces an existing process. A missed window is not a pass.
    listener = socket.socket()
    try:
        listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        listener.bind(("127.0.0.1", port))
        listener.listen(8)
    except OSError as error:
        listener.close()
        raise RuntimeError(
            "isolated port already occupied; fault window missed"
        ) from error
    listener.settimeout(poll_seconds)
    health_requests = 0
    first_health_deadline = None
    upgrade_deadline = time.monotonic() + upgrade_timeout
    release_at = None
    try:
        while True:
            journal = read_journal(root)
            if (
                journal.get("attempt") != attempt
                or journal.get("plan", {}).get("sha") != candidate_sha
            ):
                raise RuntimeError(
                    "activation identity changed; release fault immediately"
                )
            stage = journal.get("stage")
            if stage == "upgrade_confirmed" and first_health_deadline is None:
                first_health_deadline = time.monotonic() + first_health_timeout
            if stage not in ("backup_complete", "upgrade_confirmed"):
                raise RuntimeError(
                    "activation left expected window; release fault immediately"
                )
            if release_at is not None and time.monotonic() >= release_at:
                break
            if first_health_deadline is None:
                if time.monotonic() >= upgrade_deadline:
                    raise RuntimeError("upgrade not confirmed; bounded fault released")
            elif release_at is None and time.monotonic() >= first_health_deadline:
                raise RuntimeError("no health request observed; fault outcome unproven")
            if respond_unavailable(listener) and stage == "upgrade_confirmed":
                health_requests += 1
                if release_at is None:
                    release_at = time.monotonic() + hold_seconds
        return {
            "evidence_kind": "fault-injection",
            "attempt": attempt,
            "previous_sha": old_sha,
            "candidate_sha": candidate_sha,
            "upgrade_observed": True,
            "health_requests": health_requests,
            "hold_seconds": hold_seconds,
            "recovery": "not_checked",
        }
    finally:
        listener.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--protected-root", type=Path, required=True)
    parser.add_argument("--candidate-sha", required=True)
    parser.add_argument("--arm-timeout", type=int, default=1800)
    args = parser.parse_args()
    source = Path(__file__).resolve().parents[1] / "controller.py"
    spec = importlib.util.spec_from_file_location("deployment_controller", source)
    controller = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(controller)
    if (
        not controller.valid_sha(args.candidate_sha)
        or not 1 <= args.arm_timeout <= 5400
    ):
        parser.error("full candidate SHA and arm timeout 1..5400 required")
    root = args.root.expanduser().resolve()
    controller.require_isolation(root, args.protected_root)
    installed = controller.read_json(root / "controller-install.json")
    if installed.get("controller_sha256") != controller.digest(
        source
    ) or controller.digest(root / "controller.py") != controller.digest(source):
        raise ValueError("installed controller differs from the tested fault protocol")
    if controller.pending_activation(root):
        raise ValueError("activation already pending; preserve it instead of injecting")
    config = controller.settings(root)
    old = controller.verify_deployed(root, config)
    if old["sha"] == args.candidate_sha:
        raise ValueError("candidate is already deployed; fault requires an upgrade")
    previous_attempt = read_journal(root).get("attempt")
    print(
        json.dumps(
            {
                "phase": "armed",
                "evidence_kind": "fault-injection",
                "candidate_sha": args.candidate_sha,
                "recovery": "not_checked",
            }
        ),
        flush=True,
    )
    result = exercise(
        root,
        config["port"],
        old["sha"],
        args.candidate_sha,
        previous_attempt,
        arm_timeout=args.arm_timeout,
    )
    print(json.dumps(result), flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print("Fault exercise incomplete:", error, file=sys.stderr)
        sys.exit(2)
