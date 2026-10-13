#!/usr/bin/env python3
"""Bounded infrastructure probe, not product or Pipeline acceptance."""

import argparse
import json
import os
import time
import urllib.error
import urllib.request
import uuid
from pathlib import Path

try:
    import tomllib
except ImportError:
    import tomli as tomllib


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--report", type=Path, required=True)
    parser.add_argument("--image", default="python:3.12-slim")
    args = parser.parse_args()
    os.umask(0o077)
    with args.config.open("rb") as source:
        settings = tomllib.load(source)
    server = settings["server"]
    if server["host"] != "127.0.0.1" or not server.get("api_key"):
        raise ValueError("probe requires an authenticated local deployment")
    endpoint = f"http://127.0.0.1:{server['port']}"
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    probe_id = uuid.uuid4().hex
    # Never overwrite an uncertain previous attempt or automatically replay it.
    with args.report.open("x", encoding="utf-8") as report:

        def record(event, **values):
            report.write(
                json.dumps({"event": event, "time": time.time(), **values}) + "\n"
            )
            report.flush()
            os.fsync(report.fileno())

        def request(method, path, body=None):
            raw = None if body is None else json.dumps(body).encode()
            req = urllib.request.Request(
                endpoint + path,
                method=method,
                data=raw,
                headers={
                    "OPEN-SANDBOX-API-KEY": server["api_key"],
                    "Content-Type": "application/json",
                },
            )
            with client.open(req, timeout=180) as response:
                value = response.read()
                return json.loads(value) if value else None

        sandbox_id = None
        record("create_intent", probe_id=probe_id, image=args.image)
        try:
            created = request(
                "POST",
                "/sandboxes",
                {
                    "image": {"uri": args.image},
                    "entrypoint": ["tail", "-f", "/dev/null"],
                    "timeout": 600,
                    "resourceLimits": {"cpu": "1", "memory": "512Mi"},
                    "metadata": {"agent-platform-probe": probe_id},
                },
            )
            sandbox_id = created["id"]
            record("created", sandbox_id=sandbox_id, response=created)
            deadline = time.monotonic() + 120
            while time.monotonic() < deadline:
                current = request("GET", f"/sandboxes/{sandbox_id}")
                record("observed", response=current)
                state = current["status"]["state"]
                if state == "Running":
                    break
                if state in {"Failed", "Terminated"}:
                    raise RuntimeError(f"sandbox reached {state}")
                time.sleep(1)
            else:
                raise TimeoutError("sandbox readiness deadline exceeded")
            print(f"Sandbox {sandbox_id}: Running; lifecycle probe only.")
        except urllib.error.HTTPError as error:
            record(
                "http_error",
                status=error.code,
                body=error.read().decode(errors="replace"),
            )
            raise RuntimeError(
                f"OpenSandbox HTTP {error.code}; inspect private report"
            ) from None
        except Exception as error:
            record("error", kind=type(error).__name__)
            raise
        finally:
            if sandbox_id:
                record("delete_intent", sandbox_id=sandbox_id)
                request("DELETE", f"/sandboxes/{sandbox_id}")
                record("delete_accepted", sandbox_id=sandbox_id)
                try:
                    request("GET", f"/sandboxes/{sandbox_id}")
                except urllib.error.HTTPError as error:
                    if error.code != 404:
                        raise
                    record("absence_confirmed", sandbox_id=sandbox_id)
                    print("Sandbox deletion confirmed by API 404.")
                else:
                    raise RuntimeError(
                        "sandbox still present after delete; inspect before retry"
                    )


if __name__ == "__main__":
    main()
