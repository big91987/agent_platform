#!/usr/bin/env python3
"""Compose architecture checks into an existing development graph."""

import copy
import json
import shlex
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent
sys.path.insert(0, str(ROOT))
from scaffold import hashes  # noqa: E402


def check_program():
    expected = hashes(ROOT / "tool")
    return (
        """import hashlib,json,subprocess,sys,tempfile
from pathlib import Path
root=Path.cwd().resolve()
tool=root/'.harness/archcheck'
expected="""
        + repr(expected)
        + r"""
def incomplete(reason):
    try:
        folder=root/'harness-output/architecture'
        if not folder.resolve().is_relative_to(root): raise ValueError('report directory escapes workspace')
        folder.mkdir(parents=True,exist_ok=True)
        report={'status':'incomplete','model_review':{'status':'not_run'},'errors':[reason],'analysis_completed':False}
        for name,text in [('latest-report.json',json.dumps(report,ensure_ascii=False,indent=2)),('latest-report.md','# 架构检查\n\n结果：incomplete\n分析未执行或未产生有效结果：'+reason+'\n')]:
            target=folder/name
            if target.is_symlink(): raise ValueError('report must not be a symlink')
            with tempfile.NamedTemporaryFile(mode='w',dir=folder,delete=False) as output:
                output.write(text)
                temporary=Path(output.name)
            temporary.replace(target)
    except (OSError,ValueError) as error:
        print('Cannot retain a fresh architecture report: '+str(error),file=sys.stderr)
        print(json.dumps({'status':'incomplete','errors':[reason,'Fresh report unavailable; previous reports are not results of this invocation.']}))
        raise SystemExit(3)
    print(json.dumps({'status':'incomplete','errors':[reason]}))
    raise SystemExit(2)

try:
    skip={'node_modules','__pycache__','dist','.DS_Store','SOURCE.json'}
    actual={p.relative_to(tool).as_posix():hashlib.sha256(p.read_bytes()).hexdigest() for p in tool.rglob('*') if p.is_file() and not any(x in skip for x in p.relative_to(tool).parts)}
    if actual!=expected: raise ValueError('tool source drift')
    for name in expected:
        path=tool/name
        if path.is_symlink() or not path.resolve().is_relative_to(root): raise ValueError('tool symlink')
    manifest=json.loads((tool/'SOURCE.json').read_text())
    if not isinstance(manifest,dict) or manifest.get('files')!=expected: raise ValueError('tool manifest drift')
except (OSError,ValueError,TypeError,KeyError):
    incomplete('Install the selected architecture tool version before running this workflow.')
try:
    with tempfile.TemporaryDirectory(prefix='archcheck-import-') as cache:
        result=subprocess.run([sys.executable,'-I','-X','pycache_prefix='+cache,str(tool/'inspect_project.py'),'--root',str(root)],capture_output=True,text=True,timeout=240)
    if result.stderr: print(result.stderr,file=sys.stderr,end='')
    summary=json.loads(result.stdout)
    expected_status={0:'conformant',1:'violations',2:'incomplete'}.get(result.returncode)
    report=json.loads((root/'harness-output/architecture/latest-report.json').read_text())
    if expected_status is None or summary.get('status')!=expected_status or report.get('status')!=expected_status:
        raise ValueError('analysis outcome does not match its exit code')
except (OSError,ValueError,TypeError,AttributeError,subprocess.SubprocessError):
    incomplete('Architecture analysis did not return a valid structured result; inspect the execution log.')
print(result.stdout,end='')
raise SystemExit(result.returncode)
"""
    )


def add_check(original):
    graph = copy.deepcopy(original)
    nodes = {node["id"]: node for node in graph["nodes"]}
    if {"architecture_check", "architecture_blocked"}.intersection(nodes) or not {
        "tests",
        "development",
    }.issubset(nodes):
        raise ValueError(
            "expected a development workflow without an architecture check"
        )
    tests = nodes["tests"]
    if tests.get("execution", {}).get("output", {}).get("source") != "exit_code":
        raise ValueError("fixed tests must route on process exit code")
    routes = [r for r in tests["rule_handoff"]["rules"] if r.get("when") == "$ == 0"]
    if len(routes) != 1 or routes[0]["target"] not in nodes:
        raise ValueError("expected one explicit successful test destination")
    following = routes[0]["target"]
    edges = [
        e for e in graph["edges"] if e["source"] == "tests" and e["target"] == following
    ]
    if len(edges) != 1:
        raise ValueError("expected one matching test-success edge")
    routes[0]["target"] = "architecture_check"
    edges[0]["target"] = "architecture_check"
    rules = [
        {
            "when": "$ == 0",
            "target": following,
            "title": "架构规则通过",
            "content": "声明范围内的依赖检查通过，继续独立验证。",
        },
        {
            "when": "$ == 1",
            "target": "development",
            "title": "架构规则违规",
            "content": "查看源码位置与违规规则，修复后重新验证。",
        },
    ]
    rules.append(
        {
            "when": "$ == 2",
            "target": "architecture_blocked",
            "title": "检查未完成",
            "content": "保留错误报告，修复工具或配置后重试。",
        }
    )
    graph["nodes"].append(
        {
            "id": "architecture_blocked",
            "name": "处理架构检查阻塞",
            "kind": "approval",
            "prompt": "架构检查未完成。查看错误报告，修复工具或配置后确认重试；本结果不能作为质量通过。",
        }
    )
    graph["edges"].append(
        {
            "source": "architecture_blocked",
            "route": "next",
            "target": "architecture_check",
            "title": "修复后重试",
        }
    )
    node = {
        "id": "architecture_check",
        "name": "架构检查",
        "kind": "execution",
        "execution_timeout_seconds": 300,
        "execution": {
            "mode": "command",
            "command": shlex.join(["python3", "-I", "-c", check_program()]),
            "output": {"source": "exit_code"},
            "artifacts": [
                "harness-output/architecture/latest-report.json",
                "harness-output/architecture/latest-report.md",
            ],
        },
        "rule_handoff": {
            "schema_scope": "value",
            "rules": rules,
            **{
                key: {"action": "stop"}
                for key in ("on_read", "on_parse", "on_invalid", "on_expression")
            },
        },
    }
    graph["nodes"].append(node)
    destinations = [
        h
        for h in graph.get("hooks", [])
        if h["event"] == "handoff.after"
        and h.get("node", "") in ("", "architecture_check")
    ]
    if not destinations:
        destination = next(
            (
                h
                for h in graph.get("hooks", [])
                if h["event"] in ("run.failed", "run.completed")
            ),
            None,
        )
        if destination:
            graph.setdefault("hooks", []).append(
                {
                    "event": "handoff.after",
                    "node": "architecture_check",
                    "connector_id": destination["connector_id"],
                    "input": copy.deepcopy(destination.get("input", {})),
                    "body": "**架构检查已产生结果**\n\n{{summary}}\n下一节点：{{target}}\n\n报告：{{artifacts}}\n\n[查看与处理]({{run_url}})",
                }
            )
    for index, route in enumerate(rules, 1):
        graph["edges"].append(
            {
                "source": node["id"],
                "route": f"rule_{index}",
                "target": route["target"],
                "mode": "automatic",
                "title": route["title"],
                "description": route["content"],
            }
        )
    return graph


def inspection_definition(source, name):
    """Independent initial inspection; existing development Runs stay frozen."""
    graph = add_check(source)
    nodes = {node["id"]: node for node in graph["nodes"]}
    prepare = nodes["prepare"]
    prepare["rule_handoff"]["rules"] = [
        {
            "when": "$ == 0",
            "target": "architecture_check",
            "title": "源码准备完成",
            "content": "",
        }
    ]
    prepare["execution_timeout_seconds"] = 600
    prepare["execution"]["command"] += (
        " && npm ci --ignore-scripts --no-audit --no-fund --prefix .harness/archcheck"
    )
    check = nodes["architecture_check"]
    check["rule_handoff"]["rules"][0]["target"] = "done"
    check["rule_handoff"]["rules"][1]["target"] = "architecture_blocked"
    selected = [
        prepare,
        check,
        nodes["architecture_blocked"],
        {"id": "done", "name": "检查完成", "kind": "end"},
    ]
    edges = [{"source": "prepare", "route": "rule_1", "target": "architecture_check"}]
    for index, rule in enumerate(check["rule_handoff"]["rules"], 1):
        edges.append(
            {
                "source": "architecture_check",
                "route": f"rule_{index}",
                "target": rule["target"],
                "title": rule["title"],
                "description": rule["content"],
                "mode": "automatic",
            }
        )
    edges.append(
        {
            "source": "architecture_blocked",
            "route": "next",
            "target": "architecture_check",
            "title": "修复后重试",
        }
    )
    hooks = [
        copy.deepcopy(h)
        for h in graph.get("hooks", [])
        if h["event"] in ("run.completed", "run.failed")
    ]
    destination = next(
        (
            h
            for h in graph.get("hooks", [])
            if h["event"] == "handoff.after"
            and h.get("node", "") in ("", "architecture_check")
        ),
        next(iter(hooks), None),
    )
    if destination:
        hooks.append(
            {
                "event": "handoff.after",
                "node": "architecture_check",
                "connector_id": destination["connector_id"],
                "input": copy.deepcopy(destination.get("input", {})),
                "body": "**架构检查已产生结果**\n\n{{summary}}\n下一节点：{{target}}\n\n报告：{{artifacts}}\n\n[查看与处理]({{run_url}})",
            }
        )
    return {
        "name": name,
        "enabled": True,
        "environment": source["environment"],
        "context_version": source["context_version"],
        "authorized_users": source.get("authorized_users", []),
        "entry": "prepare",
        "start_nodes": [],
        "hooks": hooks,
        "max_steps": 10,
        "nodes": selected,
        "edges": edges,
    }


def main():
    import argparse

    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    result = add_check(json.loads(args.input.read_text()))
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")


if __name__ == "__main__":
    main()
