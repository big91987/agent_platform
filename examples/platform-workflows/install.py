#!/usr/bin/env python3
"""Install the example through public APIs; never edit platform persistence."""

import argparse
import http.cookiejar
import json
import os
import re
import shlex
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

HERE = Path(__file__).resolve().parent


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
            if canonical(item) not in (None, "", 0, False, [], {})
        }
    if isinstance(value, list):
        return [canonical(item) for item in value]
    return value


def projection(value, keys):
    return {key: value.get(key) for key in keys}


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

    def apply(self, collection, key, spec):
        route = "/api/" + collection
        saved = self.data["objects"].get(key)
        inventory = self.api.call("GET", route)
        if saved:
            current = next((obj for obj in inventory if obj["id"] == saved["id"]), None)
            if not current:
                raise ValueError(
                    f"Installed {key} was removed; inspect before reinstalling"
                )
            previous = saved["spec"]
            if projection(current, previous) != previous:
                raise ValueError(
                    f"Installed {key} was edited outside this installer; reconcile before upgrading"
                )
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
    if not installation.upgrade or not agents:
        return
    for run in workflow_runs(api):
        if run["status"] in ("running", "waiting", "stopping") and any(
            node.get("agent_id") in agents for node in run["definition"]["nodes"]
        ):
            raise ValueError(
                "Installed Agents are in an active Run; finish or stop it before upgrading: "
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
        if run["status"] != "completed" and any(
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
    parser.add_argument("--base-agent", required=True)
    parser.add_argument("--token-env", default="WORKFLOW_GITHUB_TOKEN")
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
        choices=["software-delivery", "qa-rework", "collaboration-check"],
        default="software-delivery",
    )
    parser.add_argument("--upgrade", action="store_true")
    parser.add_argument("--prepare-browser", action="store_true")
    args = parser.parse_args()
    try:
        test_command = verification_command(args.test_command_json)
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
        "requirements": args.skill_root / "defining-platform-products-cn",
        "design": args.skill_root / "platform-architecture-cn",
        "development": args.skill_root / "managing-engineering-delivery-cn",
        "qa": args.qa_skill,
    }
    for path in skills.values():
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
    prior_graph = installation.data["objects"].get("workflow", {}).get("spec", {})
    if args.authorized_user is None:
        args.authorized_user = prior_graph.get("authorized_users", [])
    prior_entry = installation.data.get("github_entry")
    if prior_entry and not args.github_config:
        args.github_config = Path(prior_entry["path"])
        if not args.github_token_file:
            args.github_token_file = Path(prior_entry["config"]["token_file"])
    if args.git_proxy is None:
        args.git_proxy = (prior_entry or {}).get("config", {}).get("git_proxy", "")
    check_upgrade_runs(api, installation)
    base = next(
        (a for a in api.call("GET", "/api/agents") if a["id"] == args.base_agent), None
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
    agents = {}
    if args.template == "collaboration-check":
        agents["collaboration"] = installation.apply(
            "agents",
            "collaboration",
            {
                "name": args.prefix + " · 协作验收",
                "executor": base["executor"],
                "model": base["model"],
                "enabled": True,
                "authorized_users": args.authorized_user,
                "sandbox": "workspace-write",
                "inherit_env": False,
                "instructions": "专用编排验收：只完成当前节点指令。使用真实文件及平台注册工具，不模拟交接。需要用户回答时提问并结束本轮，不交接。其他时候自主继续。无需读取其他仓库。",
                "skills": [],
                "env": {
                    key: os.environ[key]
                    for key in ("HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY")
                    if key in os.environ
                },
                "network_access": False,
                "allow_elevation": False,
                "trust_hooks": False,
                "native_config": "",
                "seed_dir": "",
            },
        )["id"]
    roles = ["requirements", "design", "development", "qa", "report"]
    if args.template == "software-delivery":
        roles.insert(0, "intake")
    for role in roles:
        spec = {
            "name": args.prefix + " · " + role,
            "executor": base["executor"],
            "model": base["model"],
            "enabled": True,
            "authorized_users": args.authorized_user,
            "sandbox": "workspace-write",
            "inherit_env": False,
            "instructions": (
                common + "\n" + (HERE / f"prompts/{role}.md").read_text()
            ).replace("{{verification_command}}", command_label),
            "skills": [str(skills[role].resolve())] if role in skills else [],
            "env": {
                **installation.data["objects"]
                .get(role, {})
                .get("spec", {})
                .get("env", {}),
                **{
                    key: os.environ[key]
                    for key in ("HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY")
                    if key in os.environ
                },
            },
            "network_access": False,
            "allow_elevation": False,
            "trust_hooks": False,
            "native_config": "",
            "seed_dir": "",
        }
        if role in ("design", "development", "qa"):
            spec["tool_servers"] = [
                {
                    "server_id": browser["id"],
                    "tools": ["check"],
                    "approvals": {"check": "auto"},
                }
            ]
        agents[role] = installation.apply("agents", role, spec)["id"]
    connectors = {}
    for role in ("prepare", "issue", "tests", "publish", "pr", "comment"):
        spec = {
            "name": args.prefix + " · " + role,
            "enabled": True,
            "authorized_users": args.authorized_user,
            "workspace_root": str(root),
            "timeout_seconds": 300,
        }
        if role in ("issue", "pr", "comment"):
            spec.update(
                kind="github.issue"
                if role == "issue"
                else "github.issue_comment"
                if role == "comment"
                else "github.pull_request",
                repository=args.repository,
                token_env=args.token_env,
            )
        else:
            spec.update(
                kind="command",
                executable=sys.executable if role != "tests" else "/usr/bin/env",
                args=[
                    str(HERE / "repository.py"),
                    role,
                    "--repository",
                    args.repository,
                    "--base",
                    args.base,
                ]
                if role != "tests"
                else test_command,
            )
            spec["env_refs"] = {
                key: key for key in ("HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY")
            }
            if role != "tests":
                spec["env_refs"]["GH_TOKEN"] = args.token_env
        connectors[role] = installation.apply("connectors", "connector-" + role, spec)[
            "id"
        ]
        api.call("POST", "/api/connectors/" + connectors[role] + "/check", {})
    graph = json.loads((HERE / (args.template + ".json")).read_text())
    graph["name"] = args.prefix + " · " + graph["name"]
    graph["authorized_users"] = args.authorized_user
    for node in graph["nodes"]:
        if node["kind"] == "agent":
            node["agent_id"] = agents[node["agent_id"]]
        if node["kind"] == "connector":
            role = node["connector_id"]
            node["connector_id"] = connectors[role]
            if role == "pr":
                node["connector_input"]["base"] = args.base
    for hook in graph.get("hooks", []):
        hook["connector_id"] = connectors[hook["connector_id"]]
    workflow = installation.apply(
        "workflows",
        "workflow" if args.template == "software-delivery" else args.template,
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
