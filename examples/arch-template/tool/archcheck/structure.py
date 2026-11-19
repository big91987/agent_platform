"""Validate evidence-bound architecture proposals and render actual/proposed structure."""

import re
from pathlib import PurePosixPath


def _path(value):
    if (
        not isinstance(value, str)
        or not value
        or "\\" in value
        or re.match(r"^[A-Za-z]:", value)
        or any(ord(c) < 32 or ord(c) == 127 for c in value)
    ):
        raise ValueError(
            "architecture plan requires canonical repository-relative paths"
        )
    path = PurePosixPath(value)
    if path.is_absolute() or ".." in path.parts or path.as_posix() != value:
        raise ValueError(f"invalid architecture plan path: {value!r}")
    return value


def _under(file, path):
    return path == "." or file == path or file.startswith(path + "/")


def _fields(item, fields, label):
    if not isinstance(item, dict) or set(item) != set(fields.split()):
        raise ValueError(f"invalid architecture plan {label} fields")


def _text(value, name):
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"architecture plan requires {name}")


def _paths(values):
    if (
        not isinstance(values, list)
        or not values
        or len(set(map(str, values))) != len(values)
    ):
        raise ValueError("architecture plan requires nonempty unique paths")
    return [_path(p) for p in values]


def _refs(values, count):
    if (
        not isinstance(values, list)
        or any(type(v) is not int or not 1 <= v <= count for v in values)
        or len(set(values)) != len(values)
    ):
        raise ValueError("invalid architecture plan finding_refs")
    return values


def validate_plan(request, plan, findings, resolve):
    """Check facts and references, not the optimality or compilability of proposals."""
    _fields(plan, "summary modules changes", "root")
    _text(plan["summary"], "summary")
    if (
        not isinstance(plan["modules"], list)
        or not plan["modules"]
        or not isinstance(plan["changes"], list)
    ):
        raise ValueError("architecture plan requires modules and changes arrays")
    files = {f["path"]: f for f in request["facts"]["files"]}
    owned, names, target_owners = set(), set(), {}
    modules = []
    for module in plan["modules"]:
        _fields(
            module,
            "name current_responsibility responsibility current_paths target_paths finding_refs evidence",
            "module",
        )
        for field in ("name", "current_responsibility", "responsibility"):
            _text(module[field], field)
        if module["name"] in names:
            raise ValueError("duplicate architecture plan module name")
        names.add(module["name"])
        current = _paths(module["current_paths"])
        targets = _paths(module["target_paths"])
        members = set()
        for path in current:
            selected = {f for f in files if _under(f, path)}
            if not selected:
                raise ValueError(
                    f"current path is not scanned production structure: {path}"
                )
            if members & selected:
                raise ValueError("overlapping current paths inside architecture module")
            members |= selected
        if owned & members:
            raise ValueError(
                "production source belongs to multiple architecture plan modules"
            )
        owned |= members
        refs = _refs(module["finding_refs"], len(findings))
        if set(targets) != set(current) and not refs:
            raise ValueError("changed module structure requires finding_refs")
        evidence = resolve(request, module["evidence"])
        if not evidence or not any(e["path"] in members for e in evidence):
            raise ValueError(
                "module responsibility requires evidence in its production source"
            )
        for path in targets:
            if path in target_owners:
                raise ValueError("multiple modules claim the same target path")
            target_owners[path] = module["name"]
        modules.append(dict(module, current_files=sorted(members), evidence=evidence))
    if owned != set(files):
        raise ValueError(
            "architecture plan module overview does not cover all scanned production source"
        )
    changes, covered, seen, whole_moves = [], set(), set(), {}
    known_files = set(request["sources"])
    for key in ("assets", "excluded"):
        known_files.update(request.get("coverage", {}).get(key, []))
    target_files = set()
    existing_dirs = {
        str(parent) for f in known_files for parent in PurePosixPath(f).parents
    }
    for change in plan["changes"]:
        _fields(
            change,
            "action source_path source_symbol target_path responsibility reason dependency_change finding_refs evidence",
            "change",
        )
        action = change["action"]
        if action not in {"move", "extract", "merge", "adjust"}:
            raise ValueError("unknown architecture plan action")
        source, target = _path(change["source_path"]), _path(change["target_path"])
        if any(
            f != target and (_under(target, f) or _under(f, target))
            for f in known_files | target_files
        ):
            raise ValueError("architecture plan target has a file/directory conflict")
        target_files.add(target)
        if target == "." or target in existing_dirs:
            raise ValueError("change target must name a file, not a directory")
        kind = "production" if source in files else "context"
        if kind == "context" and (
            source not in request["sources"] or action != "adjust" or target != source
        ):
            raise ValueError(
                "source must be scanned production; context permits only explicit in-place adjustment"
            )
        if kind == "production" and not any(
            _under(target, p) for m in modules for p in m["target_paths"]
        ):
            raise ValueError("change target is outside recommended module paths")
        if action == "adjust" and target != source:
            raise ValueError("in-place adjustment must keep its original path")
        if action in {"move", "extract", "merge"} and target == source:
            raise ValueError("use adjust for an in-place responsibility change")
        for field in ("responsibility", "reason", "dependency_change"):
            _text(change[field], field)
        refs = _refs(change["finding_refs"], len(findings))
        if not refs:
            raise ValueError("each architecture change must explain a finding")
        covered.update(refs)
        evidence = resolve(request, change["evidence"])
        local = [e for e in evidence if e["path"] == source]
        if not local:
            raise ValueError("architecture change requires evidence in its source_path")
        symbol = change["source_symbol"]
        location = dict(line=local[0]["line"], end=local[0]["end"], kind="evidence")
        if symbol is not None:
            _text(symbol, "source_symbol")
            matches = [
                s
                for s in files.get(source, {}).get("symbols", [])
                if s["name"] == symbol
                and any(s["line"] <= e["line"] and e["end"] <= s["end"] for e in local)
            ]
            if len(matches) != 1:
                raise ValueError(
                    "source_symbol must match one parsed symbol containing its source evidence; use null when not parsed"
                )
            location = dict(
                line=matches[0]["line"], end=matches[0]["end"], kind="symbol"
            )
        if action == "move" and symbol is not None:
            raise ValueError("move means a whole file; use extract for a symbol")
        identity = (action, source, symbol, target)
        if identity in seen:
            raise ValueError("duplicate architecture change")
        seen.add(identity)
        if action == "move":
            if (
                target in known_files
                or target in whole_moves.values()
                or source in whole_moves
            ):
                raise ValueError(
                    "whole-file move conflicts with an existing target or move; use explicit merge"
                )
            whole_moves[source] = target
        changes.append(
            dict(
                change,
                evidence=evidence,
                source_kind=kind,
                source_location=location,
                target_status="existing" if target in known_files else "proposed",
            )
        )
    if covered != set(range(1, len(findings) + 1)):
        raise ValueError(
            "architecture changes must address every finding, using adjust where no relocation is needed"
        )
    for module in modules:
        for target in module["target_paths"]:
            if not any(_under(f, target) for f in files) and not any(
                c["source_kind"] == "production" and _under(c["target_path"], target)
                for c in changes
            ):
                raise ValueError("new module path has no justified object change")
    return dict(
        status="ready",
        summary=plan["summary"],
        modules=modules,
        changes=changes,
        verification="facts_checked_proposal_needs_review",
    )


def _short(text):
    text = " ".join(text.split())
    return text[:100] + ("…" if len(text) > 100 else "")


def _tree(paths, notes):
    tree = {}
    for path in sorted(paths):
        node = tree
        for part in PurePosixPath(path).parts:
            node = node.setdefault(part, {})
    lines = ["." + ("  # " + _short(notes["."]) if notes.get(".") else "")]

    def walk(node, prefix="", parent=""):
        items = sorted(node.items(), key=lambda item: (not bool(item[1]), item[0]))
        for i, (name, children) in enumerate(items):
            last = i == len(items) - 1
            path = f"{parent}/{name}" if parent else name
            note = "  # " + _short(notes[path]) if notes.get(path) else ""
            lines.append(
                prefix
                + ("└── " if last else "├── ")
                + name
                + ("/" if children else "")
                + note
            )
            walk(children, prefix + ("    " if last else "│   "), path)

    walk(tree)
    return lines


def _cell(value):
    return str(value).replace("|", "\\|").replace("\n", " ")


def render_structure(report):
    """Both trees are derived from data; model-provided ASCII is never trusted."""
    files = {f["path"] for f in report.get("facts", {}).get("files", [])}
    model = report.get("model_review", {})
    plan = model.get("architecture_plan", {})
    notes = {}
    if plan.get("status") == "ready":
        for m in plan["modules"]:
            for p in m["current_paths"]:
                notes[p] = m["current_responsibility"]
    problems = {}
    for i, f in enumerate(model.get("findings", []), 1):
        for path in {e["path"] for e in f["evidence"]} & files:
            problems.setdefault(path, []).append((f"发现 {i}", f["title"]))
    for i, f in enumerate(report.get("rules", {}).get("findings", []), 1):
        if f["source"] in files:
            problems.setdefault(f["source"], []).append((f"规则 R{i}", f["message"]))
    for path, issues in problems.items():
        notes[path] = (
            "、".join(i[0] for i in issues)
            + "："
            + "；".join(i[1] for i in issues)
            + ("；" + notes[path] if notes.get(path) else "")
        )
    lines = ["## 代码结构与模块划分", "", "### 现状结构（扫描事实）", ""]
    if not files:
        return lines + ["源码分析未完成，没有可展示的生产结构。", ""]
    lines += [
        "仅展示本次扫描的生产源码；测试、规范及资源见检查范围。路径来自扫描事实；职责与问题注释来自模型评审或确定规则，模型判断仍需复核。",
        "",
        "```text",
        *_tree(files, notes),
        "```",
        "",
        "### 推荐结构（建议，尚未实施）",
        "",
    ]
    if plan.get("status") != "ready":
        reason = (
            "未运行模型，不生成推荐结构。"
            if model.get("status") == "not_run"
            else "结构方案未完成，不以当前结构代替推荐方案。"
        )
        return lines + [reason, *plan.get("errors", []), ""]
    lines += [
        plan["summary"],
        "",
        "路径与引用已核验；职责建议、拆包后的可见性、依赖和事务行为尚需实施复核。",
        "",
    ]
    proposed, target_notes = set(files), {}
    actions = {
        "move": "整文件移动",
        "extract": "提取",
        "merge": "合并",
        "adjust": "原地调整",
    }
    for c in plan["changes"]:
        if c["source_kind"] != "production":
            continue
        if c["action"] == "move" or (
            c["action"] == "merge" and c["source_symbol"] is None
        ):
            proposed.discard(c["source_path"])
        proposed.add(c["target_path"])
        target_notes[c["target_path"]] = (
            f"{actions[c['action']]}（{'建议目标' if c['target_status'] == 'proposed' else '复用已有文件'}）：{c['responsibility']}"
        )
    for m in plan["modules"]:
        for path in m["target_paths"]:
            proposed.add(path)
            target_notes.setdefault(path, m["responsibility"])
    lines += [
        "```text",
        *_tree(proposed, target_notes),
        "```",
        "",
        "### 模块职责对照",
        "",
        "| 模块 | 当前职责与位置 | 推荐职责与位置 | 关联发现 |",
        "| --- | --- | --- | --- |",
    ]
    for m in plan["modules"]:
        lines.append(
            "| "
            + " | ".join(
                map(
                    _cell,
                    [
                        m["name"],
                        m["current_responsibility"]
                        + "；"
                        + "、".join(m["current_paths"]),
                        m["responsibility"] + "；" + "、".join(m["target_paths"]),
                        "、".join(map(str, m["finding_refs"])) or "保留",
                    ],
                )
            )
            + " |"
        )
    lines += [
        "",
        "### 文件／函数调整对照",
        "",
        "| 操作 | 当前对象 | 推荐位置与职责 | 调整理由及依赖变化 | 关联发现 |",
        "| --- | --- | --- | --- | --- | --- |",
    ]
    for c in plan["changes"]:
        source = c["source_path"] + (
            " · " + c["source_symbol"] if c["source_symbol"] else ""
        )
        if c["source_location"]:
            source += f"：{c['source_location']['line']}-{c['source_location']['end']}"
        if c["source_kind"] == "context":
            source += "（规范／上下文文件，不计生产结构）"
        elif c["source_symbol"] is None:
            source += "（文件级建议；未指定解析符号）"
        lines.append(
            "| "
            + " | ".join(
                map(
                    _cell,
                    [
                        actions[c["action"]],
                        source,
                        c["target_path"] + "；" + c["responsibility"],
                        c["reason"] + "；" + c["dependency_change"],
                        "、".join(map(str, c["finding_refs"])),
                    ],
                )
            )
            + " |"
        )
    if not plan["changes"]:
        lines.append("| 保留 | 当前生产结构 | 保留当前结构 | 本轮无必要结构调整 | — |")
    return lines + [""]
