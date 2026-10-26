#!/usr/bin/env python3
"""Install both review graphs via the public API with drift-checked reentry."""

import argparse
import json
import sys
from pathlib import Path

from workflows import definition

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "sdk/python"))
from agent_platform_client import Client  # noqa: E402


def comparable(graph):
    # Compare requested fields only; server-owned IDs/timestamps/defaults aren't
    # editable template configuration. Explicit user edits to these fields fail.
    return {
        key: graph[key]
        for key in (
            "name",
            "enabled",
            "environment",
            "context_version",
            "entry",
            "max_steps",
            "authorized_users",
            "nodes",
            "edges",
        )
    }


def install(
    client,
    repository,
    credential_ref,
    skill,
    manifest,
    *,
    model_refs,
    proxy="",
    authorized_users=(),
):
    previous = json.loads(manifest.read_text()) if manifest.exists() else {}
    existing = client._json("/api/workflows")
    results = {}
    for engine in ("cli", "delegate"):
        desired = definition(
            repository, credential_ref, engine, skill, model_refs, proxy
        )
        desired["authorized_users"] = list(authorized_users)
        old = previous.get(engine)
        if old:
            actual = client._json("/api/workflows/" + old["id"])
            # Remote node normalization is checked using the saved API response.
            if comparable(actual) != comparable(old):
                raise ValueError("Installed review workflow was externally edited")
            if comparable(desired) == comparable(old["requested"]):
                results[engine] = old
                continue
            runs = client.workflow_runs(workflow_id=old["id"])
            if any(r["status"] in ("running", "waiting", "stopping") for r in runs):
                raise ValueError("Finish or formally stop review Runs before upgrading")
            result = client._json(
                "/api/workflows/" + old["id"],
                {**desired, "revision": actual["revision"]},
                method="PUT",
            )
        else:
            if any(w["name"] == desired["name"] for w in existing):
                raise ValueError("Workflow name exists without installation manifest")
            result = client._json("/api/workflows", desired)
        result["requested"] = desired
        results[engine] = result
        manifest.parent.mkdir(parents=True, exist_ok=True)
        # Save every confirmed side effect before attempting another graph.
        manifest.write_text(
            json.dumps({**previous, **results}, ensure_ascii=False, indent=2) + "\n"
        )
        manifest.chmod(0o600)
    return {engine: value["id"] for engine, value in results.items()}


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--platform-url", required=True)
    parser.add_argument("--token-file", required=True, type=Path)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--credential-ref", required=True)
    parser.add_argument("--skill", required=True)
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--proxy", default="")
    parser.add_argument("--authorized-user", action="append", default=[])
    args = parser.parse_args()
    refs = {
        key: "CODE_REVIEW_" + key.removeprefix("OCR_LLM_")
        for key in ("OCR_LLM_URL", "OCR_LLM_TOKEN", "OCR_LLM_MODEL", "OCR_LLM_PROTOCOL")
    }
    client = Client(args.platform_url, args.token_file.read_text().strip())
    print(
        json.dumps(
            install(
                client,
                args.repository,
                args.credential_ref,
                args.skill,
                args.manifest,
                model_refs=refs,
                proxy=args.proxy,
                authorized_users=args.authorized_user,
            )
        )
    )
