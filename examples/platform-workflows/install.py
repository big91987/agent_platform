#!/usr/bin/env python3
"""Install the example through public APIs; never edit platform persistence."""

import argparse
import http.cookiejar
import json
import os
import re
import shlex
import shutil
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

HERE = Path(__file__).resolve().parent
AGENT_EXECUTION_FIELDS = frozenset(
    (
        "executor",
        "model",
        "instructions",
        "skills",
        "tool_servers",
        "sandbox",
        "network_access",
        "allow_elevation",
        "approvals_reviewer",
        "native_config",
        "trust_hooks",
        "inherit_env",
        "env",
        "seed_dir",
    )
)


def proxy_env_refs(previous, environment):
    previous = previous or {}
    names = ("HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY")
    return {
        **{key: previous[key] for key in names if key in previous},
        **{key: key for key in names if key in environment},
    }


def verification_command(value):
    command = json.loads(value)
    if (
        not isinstance(command, list)
        or not command
        or len(command) > 64
        or any(not isinstance(arg, str) or not arg or "\0" in arg for arg in command)
        or command[0].startswith("-")
        or "=" in command[0]
    ):
        raise ValueError(
            "test command must be a non-empty executable/arguments JSON array"
        )
    return command


def repository_command(role, repository, base, test_command):
    command = [
        str(HERE / "repository.py"),
        "verify" if role == "tests" else role,
        "--repository",
        repository,
        "--base",
        base,
        "--task-docs",
        "--materials",
    ]
    if role == "tests":
        command += ["--test-command", json.dumps(test_command)]
    return command


def stage_instructions(role, common, command_label):
    roles = json.loads((HERE / "prompts/roles.json").read_text())
    responsibilities = (HERE / f"prompts/{role}.md").read_text()
    if role.startswith("e2e_"):
        common += "\n\n" + (HERE / "prompts/product-e2e-common.md").read_text()
    guidance = (
        (roles[role], responsibilities)
        if role == "collaboration"
        else (roles[role], common, responsibilities)
    )
    return "\n\n".join(guidance).replace("{{verification_command}}", command_label)


class API:
    def __init__(self, url, username, password):
        self.url = url.rstrip("/")
        parsed = urllib.parse.urlparse(self.url)
        if parsed.scheme != "https" and parsed.hostname not in (
            "localhost",
            "127.0.0.1",
            "::1",
        ):
            raise ValueError("Use HTTPS except for a local platform")
        if parsed.username or parsed.password:
            raise ValueError("Do not embed credentials in the platform URL")
        handlers = [urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())]
        if parsed.hostname in ("localhost", "127.0.0.1", "::1"):
            handlers.append(urllib.request.ProxyHandler({}))
        self.http = urllib.request.build_opener(*handlers)
        self.call("POST", "/api/login", {"username": username, "password": password})

    def call(self, method, path, body=None):
        request = urllib.request.Request(
            self.url + path,
            method=method,
            data=None if body is None else json.dumps(body).encode(),
            headers={"Content-Type": "application/json", "X-Platform-Request": "1"},
        )
        try:
            with self.http.open(request, timeout=180) as response:
                return json.load(response)
        except urllib.error.HTTPError as error:
            raise RuntimeError(
                f"{method} {path}: HTTP {error.code}: {error.read().decode()}"
            ) from None


def canonical(value):
    # Go responses include zero-valued optional fields omitted in template JSON.
    if isinstance(value, dict):
        return {
            key: canonical(item)
            for key, item in value.items()
            if key == "allow_user_input"
            or canonical(item) not in (None, "", 0, False, [], {})
        }
    if isinstance(value, list):
        return [canonical(item) for item in value]
    return value


def projection(value, keys):
    return {key: value.get(key) for key in keys}


def execution_config(value):
    # Omitted optional fields and Go zero values represent the same defaults.
    # Normalize only whole fields: an env entry set to "" or null is meaningful.
    return {
        key: value[key]
        for key in AGENT_EXECUTION_FIELDS
        if value.get(key) not in (None, "", False, [], {})
    }


class Installation:
    def __init__(self, api, path, identity, upgrade=False):
        self.api, self.path, self.upgrade = api, path, upgrade
        self.data = (
            json.loads(path.read_text())
            if path.exists()
            else {"identity": identity, "objects": {}}
        )
        if self.data["identity"] != identity:
            raise ValueError(
                "Manifest belongs to another platform/repository/prefix/workspace; use its original options"
            )

    def save(self):
        self.path.parent.mkdir(parents=True, exist_ok=True)
        tmp = self.path.with_suffix(".tmp")
        tmp.write_text(json.dumps(self.data, ensure_ascii=False, indent=2) + "\n")
        tmp.chmod(0o600)
        tmp.replace(self.path)

    def check(self, collection, key, inventory=None):
        saved = self.data["objects"].get(key)
        if not saved:
            return None
        if inventory is None:
            inventory = self.api.call("GET", "/api/" + collection)
        current = next((obj for obj in inventory if obj["id"] == saved["id"]), None)
        if not current:
            raise ValueError(
                f"Installed {key} was removed; inspect before reinstalling"
            )
        previous = saved["spec"]
        fields = previous.keys()
        execution_drift = False
        if collection == "agents":
            fields = fields - AGENT_EXECUTION_FIELDS
            execution_drift = execution_config(current) != execution_config(previous)
        if execution_drift or projection(current, fields) != projection(
            previous, fields
        ):
            raise ValueError(
                f"Installed {key} was edited outside this installer; reconcile before upgrading"
            )
        return current

    def apply(self, collection, key, spec):
        route = "/api/" + collection
        saved = self.data["objects"].get(key)
        inventory = self.api.call("GET", route)
        if saved:
            current = self.check(collection, key, inventory)
            if canonical(projection(current, spec)) == canonical(spec):
                return current
            if not self.upgrade:
                raise ValueError(
                    f"{key} needs an upgrade; review source changes and use --upgrade"
                )
            body = {**spec, "id": current["id"]}
            if "revision" in current:
                body["revision"] = current["revision"]
            result = self.api.call("PUT", route + "/" + current["id"], body)
        else:
            if any(
                obj["name"] == spec["name"]
                or (spec.get("id") and obj["id"] == spec["id"])
                for obj in inventory
            ):
                raise ValueError(
                    f"{key} already exists without this manifest; reconcile it, do not create duplicates"
                )
            result = self.api.call("POST", route, spec)
        # Store the server-normalized projection, not secrets or session cookies.
        self.data["objects"][key] = {
            "id": result["id"],
            "spec": projection(result, spec),
        }
        self.save()
        print(f"Installed {key}: {result['id']}", flush=True)
        return result


def install_shared_browser(api, installation, manifest, evidence):
    shared = Installation(
        api,
        manifest,
        {"platform": api.url, "capability": "browser-validation"},
        installation.upgrade,
    )
    spec = {
        "id": "browser-validation",
        "name": "浏览器验证",
        "enabled": True,
        "connection": {
            "command": sys.executable,
            "args": [
                str(HERE / "browser_tool.py"),
                "--workspace-root",
                "{{workspace}}",
                "--evidence",
                str(evidence.resolve()),
            ],
            "env_vars": ["PATH"],
        },
    }
    browser = shared.apply("tool-servers", "browser", spec)
    api.call("POST", "/api/tool-servers/" + browser["id"] + "/discover", {})
    installation.data["shared_browser"] = {
        "id": browser["id"],
        "manifest": str(manifest.resolve()),
    }
    installation.save()
    return browser


def installed_agent_ids(installation):
    # Object keys are workflow roles, not collection-prefixed names.
    return {
        value["id"]
        for value in installation.data["objects"].values()
        if value["spec"].get("executor")
    }


def workflow_runs(api):
    before = ""
    seen = set()
    while True:
        path = "/api/workflow-runs"
        if before:
            path += "?" + urllib.parse.urlencode({"before": before})
        runs = api.call("GET", path)
        for run in runs:
            if run["id"] in seen:
                raise ValueError(
                    "Run pagination did not advance; upgrade the platform before installing"
                )
            seen.add(run["id"])
            yield run
        if len(runs) < 200:
            return
        before = runs[-1]["id"]


def check_upgrade_runs(api, installation):
    agents = installed_agent_ids(installation)
    workflows = {
        obj["id"]
        for obj in installation.data["objects"].values()
        if "nodes" in obj["spec"]
    }
    if not installation.upgrade or not (agents or workflows):
        return
    for run in workflow_runs(api):
        if run["status"] in ("running", "waiting", "stopping") and (
            run.get("workflow_id") in workflows
            or any(
                node.get("agent_id") in agents for node in run["definition"]["nodes"]
            )
        ):
            raise ValueError(
                "Installed workflow or legacy Agents are in an active Run; finish or stop it before upgrading: "
                + run["id"]
            )


def retire_project_browser(api, installation):
    saved = installation.data["objects"].get("browser")
    if not saved or saved["id"] == "browser-validation":
        return
    # Preserve a registration still referenced by another Agent or resumable Run.
    for agent in api.call("GET", "/api/agents"):
        if any(
            binding["server_id"] == saved["id"]
            for binding in agent.get("tool_servers", []) or []
        ):
            return
    agents = installed_agent_ids(installation)
    for run in workflow_runs(api):
        if run["status"] not in ("completed", "cancelled") and any(
            node.get("agent_id") in agents for node in run["definition"]["nodes"]
        ):
            return
    spec = {**saved["spec"], "enabled": False}
    installation.apply("tool-servers", "browser", spec)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--platform-url", required=True)
    parser.add_argument("--username", default="admin")
    parser.add_argument("--password-env", default="PLATFORM_ADMIN_PASSWORD")
    parser.add_argument("--repository", required=True)
    parser.add_argument(
        "--base", default="main", help="Repository base branch for preparation and PRs"
    )
    parser.add_argument(
        "--test-command-json",
        default='["npm", "test"]',
        help='Fixed project verification argv; e.g. ["make", "verify"]',
    )
    parser.add_argument(
        "--test-timeout-seconds",
        type=int,
        default=900,
        help="Project verification budget, 1–1800 seconds (default: 900)",
    )
    parser.add_argument("--workspace-root", required=True, type=Path)
    parser.add_argument("--evidence", required=True, type=Path)
    parser.add_argument(
        "--browser-manifest", type=Path, help="Shared platform browser manifest"
    )
    parser.add_argument(
        "--browser-evidence",
        type=Path,
        default=HERE.parents[1] / ".data/browser-evidence",
    )
    parser.add_argument("--skill-root", required=True, type=Path)
    parser.add_argument("--qa-skill", required=True, type=Path)
    parser.add_argument(
        "--base-agent", help="Legacy compatibility: initial executor/model defaults"
    )
    parser.add_argument(
        "--executor", help="Native executor; defaults to prior config or codex"
    )
    parser.add_argument(
        "--model", help="Native model; defaults to prior config or executor default"
    )
    parser.add_argument(
        "--agent-network-access",
        action=argparse.BooleanOptionalAction,
        default=None,
        help="Explicit native Agent network grant; upgrades preserve prior grant when omitted",
    )
    parser.add_argument(
        "--allow-agent-elevation",
        action=argparse.BooleanOptionalAction,
        default=None,
        help="Allow native permission requests for the selected reviewer; does not grant blanket access",
    )
    parser.add_argument(
        "--agent-approvals-reviewer",
        choices=["user", "auto_review"],
        default=None,
        help="Native permission reviewer; explicit auto_review uses Codex review, not blanket approval; preserve on upgrade",
    )
    browser_skill = parser.add_mutually_exclusive_group()
    browser_skill.add_argument(
        "--browser-skill",
        type=Path,
        help="Optional real-browser Skill for product-e2e execution and independent review",
    )
    browser_skill.add_argument(
        "--clear-browser-skill",
        action="store_true",
        help="Explicitly remove product-e2e browser Skill mounts on upgrade",
    )
    parser.add_argument("--token-env", default="WORKFLOW_GITHUB_TOKEN")
    parser.add_argument(
        "--notification-token-env",
        default=None,
        help="Separate GitHub credential name for automatic comments; preserve on upgrade",
    )
    parser.add_argument(
        "--authorized-user",
        action="append",
        default=None,
        help="Platform user ID allowed to call this installation",
    )
    parser.add_argument(
        "--github-config", type=Path, help="Export private CI entry configuration"
    )
    parser.add_argument(
        "--github-token-file",
        type=Path,
        help="Existing authorized platform user's API Token file",
    )
    parser.add_argument(
        "--git-proxy",
        default=None,
        help="Optional Git clone proxy URL; empty disables the recorded proxy",
    )
    parser.add_argument("--prefix", default="software-delivery")
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument(
        "--template",
        choices=[
            "software-delivery",
            "qa-rework",
            "collaboration-check",
            "product-e2e",
        ],
        default="software-delivery",
    )
    parser.add_argument("--upgrade", action="store_true")
    parser.add_argument("--prepare-browser", action="store_true")
    parser.add_argument("--trellis-executable", default=shutil.which("trellis"))
    args = parser.parse_args()
    if args.notification_token_env is not None and not re.fullmatch(
        r"[A-Za-z_][A-Za-z0-9_]*", args.notification_token_env
    ):
        parser.error("notification token must be an environment credential name")
    if not args.trellis_executable:
        parser.error("Trellis CLI is required; install @mindfoldhq/trellis@0.6.15")
    trellis_path = str(Path(args.trellis_executable).resolve(strict=True))
    trellis_version = subprocess.check_output(
        [trellis_path, "--version"], text=True, timeout=15
    ).strip()
    if trellis_version != "0.6.15":
        parser.error(
            "This installer is verified with Trellis 0.6.15; use the documented version"
        )
    try:
        test_command = verification_command(args.test_command_json)
        if not 1 <= args.test_timeout_seconds <= 1800:
            raise ValueError("test timeout must be 1–1800 seconds")
    except ValueError as error:
        parser.error(str(error))
    if not re.fullmatch(r"[a-z][a-z0-9-]{0,39}", args.prefix):
        parser.error("prefix must be lowercase ASCII, up to 40 characters")
    if not args.base or args.base.startswith("-"):
        parser.error("base must be a Git branch name")
    if not re.fullmatch(r"[\w.-]+/[\w.-]+", args.repository):
        parser.error("repository must be owner/name")
    root = args.workspace_root.resolve(strict=True)
    evidence = args.evidence.resolve()
    if evidence.is_relative_to(root):
        parser.error("evidence must be outside task workspaces")
    skills = {
        "requirements": [args.skill_root / "defining-platform-products-cn"],
        "design": [args.skill_root / "platform-architecture-cn"],
        "development": [
            args.skill_root / name
            for name in (
                "managing-engineering-delivery-cn",
                "trellis-before-dev",
                "trellis-check",
                "trellis-spec-bootstrap",
                "trellis-update-spec",
            )
        ],
        "qa": [args.qa_skill],
        "report": [args.skill_root / "managing-engineering-delivery-cn"],
    }
    if args.template == "product-e2e":
        skills.update(
            e2e_plan=[args.qa_skill],
            e2e_plan_review=[args.qa_skill],
            e2e_execute=[args.qa_skill],
            e2e_review=[args.qa_skill],
            e2e_report=[args.qa_skill],
        )
        if args.browser_skill:
            for role in ("e2e_execute", "e2e_review"):
                skills[role].append(args.browser_skill)
    for paths in skills.values():
        for path in paths:
            if not (path / "SKILL.md").is_file():
                parser.error("missing Skill asset: " + str(path))
    if args.prepare_browser:
        sys.path.insert(0, str(HERE.parent / "github/tooling"))
        from full_harness.browser import prepare

        prepare(
            HERE.parent / "github/tooling",
            {
                "inherit": [
                    "PATH",
                    "HOME",
                    "HTTPS_PROXY",
                    "HTTP_PROXY",
                    "NO_PROXY",
                    "PLAYWRIGHT_BROWSERS_PATH",
                ],
                "set": {},
            },
        )
    api = API(args.platform_url, args.username, os.environ[args.password_env])
    installation = Installation(
        api,
        args.manifest,
        {
            "platform": api.url,
            "repository": args.repository,
            "prefix": args.prefix,
            "workspace_root": str(root),
        },
        args.upgrade,
    )
    if args.notification_token_env is None:
        args.notification_token_env = (
            installation.data["objects"]
            .get("connector-comment", {})
            .get("spec", {})
            .get("token_env", args.token_env)
        )
    workflow_key = "workflow" if args.template == "software-delivery" else args.template
    prior_graph = installation.data["objects"].get(workflow_key, {}).get("spec", {})
    if args.authorized_user is None:
        authorization_graph = prior_graph or installation.data["objects"].get(
            "workflow", {}
        ).get("spec", {})
        args.authorized_user = authorization_graph.get("authorized_users", [])
    prior_entry = installation.data.get("github_entry")
    if prior_entry and not args.github_config:
        args.github_config = Path(prior_entry["path"])
        if not args.github_token_file:
            args.github_token_file = Path(prior_entry["config"]["token_file"])
    if args.git_proxy is None:
        args.git_proxy = (prior_entry or {}).get("config", {}).get("git_proxy", "")
    check_upgrade_runs(api, installation)
    installation.check("workflows", workflow_key)
    graph = json.loads((HERE / (args.template + ".json")).read_text())
    previous_nodes = {node["id"]: node for node in prior_graph.get("nodes", [])}
    prior_configs = {}
    for node in graph["nodes"]:
        if node["kind"] != "agent":
            continue
        role = "collaboration" if args.template == "collaboration-check" else node["id"]
        previous = previous_nodes.get(node["id"], {})
        prior = previous.get("agent")
        if prior is None:
            saved = installation.data["objects"].get(role)
            if saved:
                installation.check("agents", role)
                prior = saved["spec"]
        prior_configs[node["id"]] = prior or {}
    if (
        args.template == "product-e2e"
        and not args.browser_skill
        and not args.clear_browser_skill
    ):
        for role in ("e2e_execute", "e2e_review"):
            prior_skills = prior_configs.get(role, {}).get("skills")
            if prior_skills:
                skills[role] = [Path(path) for path in prior_skills]
                for path in skills[role]:
                    if not (path / "SKILL.md").is_file():
                        parser.error("missing installed Skill asset: " + str(path))
    base = {}
    if args.base_agent and any(not prior for prior in prior_configs.values()):
        base = next(
            (a for a in api.call("GET", "/api/agents") if a["id"] == args.base_agent),
            None,
        )
        if not base:
            parser.error("base Agent not found")
    if args.browser_evidence.resolve().is_relative_to(root):
        parser.error("browser evidence must be outside task workspaces")
    browser = install_shared_browser(
        api,
        installation,
        args.browser_manifest or HERE.parents[1] / ".data/shared-browser-install.json",
        args.browser_evidence,
    )
    common = (HERE / "prompts/common.md").read_text()
    command_label = shlex.join(test_command)
    common += (
        "\n本项目由管理员配置的固定验证入口为 `"
        + command_label
        + "`。阶段实现和 QA 以此命令的真实结果为依据。\n"
    )
    for node in graph["nodes"]:
        if node["kind"] != "agent":
            continue
        role = "collaboration" if args.template == "collaboration-check" else node["id"]
        prior = prior_configs[node["id"]]
        node["agent"] = {
            "executor": args.executor
            or prior.get("executor")
            or base.get("executor", "codex"),
            "model": args.model
            if args.model is not None
            else prior.get("model", base.get("model", "")),
            "sandbox": "workspace-write",
            "inherit_env": False,
            "instructions": "\n\n".join(
                filter(
                    None,
                    (
                        node.get("agent", {}).get("instructions", ""),
                        stage_instructions(role, common, command_label),
                    ),
                )
            ),
            "skills": [str(path.resolve()) for path in skills.get(role, [])],
            "env": {
                **(prior.get("env") or {}),
                **{
                    key: os.environ[key]
                    for key in ("HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY")
                    if key in os.environ
                },
            },
            "network_access": args.agent_network_access
            if args.agent_network_access is not None
            else prior.get("network_access", False),
            "allow_elevation": args.allow_agent_elevation
            if args.allow_agent_elevation is not None
            else prior.get("allow_elevation", False),
            **(
                {"approvals_reviewer": args.agent_approvals_reviewer}
                if args.agent_approvals_reviewer is not None
                else {"approvals_reviewer": prior["approvals_reviewer"]}
                if "approvals_reviewer" in prior
                else {}
            ),
            "trust_hooks": False,
            "native_config": "",
            "seed_dir": "",
        }
        if role in ("design", "development", "qa"):
            node["agent"]["tool_servers"] = [
                {
                    "server_id": browser["id"],
                    "tools": ["check"],
                    "approvals": {"check": "auto"},
                }
            ]
        node["allow_user_input"] = True
        node["continuation_limit"] = 3
        node["execution_timeout_seconds"] = 14400
    connectors = {}
    for role in ("prepare", "issue", "tests", "publish", "pr", "comment"):
        spec = {
            "name": args.prefix + " · " + role,
            "enabled": True,
            "authorized_users": args.authorized_user,
            "workspace_root": str(root),
            "timeout_seconds": args.test_timeout_seconds if role == "tests" else 300,
        }
        if role in ("issue", "pr", "comment"):
            spec.update(
                kind="github.issue"
                if role == "issue"
                else "github.issue_comment"
                if role == "comment"
                else "github.pull_request",
                repository=args.repository,
                token_env=args.notification_token_env
                if role == "comment"
                else args.token_env,
            )
        else:
            spec.update(
                kind="command",
                executable=sys.executable,
                args=repository_command(role, args.repository, args.base, test_command),
            )
            if role == "prepare":
                spec["args"] += [
                    "--trellis-executable",
                    trellis_path,
                    "--trellis-version",
                    trellis_version,
                ]
            spec["env_refs"] = proxy_env_refs(
                installation.data["objects"]
                .get("connector-" + role, {})
                .get("spec", {})
                .get("env_refs", {}),
                os.environ,
            )
            if role != "tests":
                spec["env_refs"]["GH_TOKEN"] = args.token_env
        connectors[role] = installation.apply("connectors", "connector-" + role, spec)[
            "id"
        ]
        api.call("POST", "/api/connectors/" + connectors[role] + "/check", {})
    graph["context_version"] = 2
    graph["name"] = args.prefix + " · " + graph["name"]
    graph["authorized_users"] = args.authorized_user
    for node in graph["nodes"]:
        if node["kind"] == "connector":
            role = node["connector_id"]
            node["connector_id"] = connectors[role]
            if role == "pr":
                node["connector_input"]["base"] = args.base
    for hook in graph.get("hooks", []):
        hook["connector_id"] = connectors[hook["connector_id"]]
    workflow = installation.apply(
        "workflows",
        workflow_key,
        graph,
    )
    if args.github_config:
        if (
            args.template != "software-delivery"
            or not args.github_token_file
            or not args.github_token_file.is_file()
        ):
            raise ValueError(
                "GitHub entry requires software-delivery and an existing --github-token-file"
            )
        config_path = args.github_config.resolve()
        token_path = args.github_token_file.resolve()
        if config_path.is_relative_to(root) or token_path.is_relative_to(root):
            raise ValueError(
                "CI configuration and token must be outside Agent workspaces"
            )
        config = {
            "base_url": api.url,
            "repository": args.repository,
            "workflow_id": workflow["id"],
            "workspace_root": str(root),
            "base": args.base,
            "token_file": str(token_path),
            "state_root": str(config_path.parent / (config_path.stem + "-state")),
        }
        config["git_proxy"] = args.git_proxy
        previous = installation.data.get("github_entry")
        if config_path.exists():
            existing = json.loads(config_path.read_text())
            if previous != {"path": str(config_path), "config": existing}:
                raise ValueError(
                    "CI configuration drift; reconcile with original manifest"
                )
            if existing != config and not args.upgrade:
                raise ValueError("CI configuration change requires --upgrade")
        elif previous:
            raise ValueError("Installed CI configuration is missing")
        config_path.parent.mkdir(parents=True, exist_ok=True)
        temporary = config_path.with_suffix(".tmp")
        temporary.write_text(json.dumps(config, ensure_ascii=False, indent=2) + "\n")
        temporary.chmod(0o600)
        temporary.replace(config_path)
        installation.data["github_entry"] = {"path": str(config_path), "config": config}
        installation.save()
    retire_project_browser(api, installation)
    print(api.url + "/workflows/" + workflow["id"])


if __name__ == "__main__":
    try:
        main()
    except (ValueError, RuntimeError, KeyError) as error:
        sys.exit(str(error))
