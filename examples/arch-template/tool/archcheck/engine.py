"""One architecture rule vocabulary over located language facts."""

import fnmatch
import hashlib
import json
import os
import re
import subprocess
import tempfile
from pathlib import Path

from . import VERSION

LANGUAGES = {
    ".go": "go",
    ".ts": "typescript",
    ".mts": "typescript",
    ".cts": "typescript",
    ".mjs": "typescript",
    ".cjs": "typescript",
    ".tsx": "typescript",
    ".js": "typescript",
    ".jsx": "typescript",
    ".vue": "typescript",
}
OTHER_CODE = {
    ".py",
    ".rs",
    ".java",
    ".kt",
    ".c",
    ".cpp",
    ".cs",
    ".rb",
    ".php",
}
IGNORED = {
    ".git",
    "node_modules",
    "vendor",
    "dist",
    "__pycache__",
    ".harness",
    "harness-output",
    ".harness-output",
}


def digest(value):
    return hashlib.sha256(value).hexdigest()


def matches(name, patterns):
    return any(fnmatch.fnmatchcase(name, pattern) for pattern in patterns)


def relative_file(root, name):
    if (
        not isinstance(name, str)
        or not name
        or Path(name).is_absolute()
        or ".." in Path(name).parts
    ):
        raise ValueError(f"invalid relative path: {name!r}")
    path = root / name
    current = root
    for part in Path(name).parts:
        current = current / part
        if current.is_symlink():
            raise ValueError(f"symlink is outside supported source analysis: {name}")
    return path


def strings(value, name):
    if not isinstance(value, list) or any(
        not isinstance(x, str) or not x for x in value
    ):
        raise ValueError(f"{name} must be a list of nonempty strings")
    return value


def load_config(root, filename):
    path = relative_file(root, filename)
    config = json.loads(path.read_text())
    allowed = {
        "source_roots",
        "languages",
        "modules",
        "exclude",
        "documents",
        "review_checks",
    }
    if not isinstance(config, dict) or set(config) - allowed:
        raise ValueError("unknown architecture configuration field")
    for key in ("source_roots", "languages"):
        if not strings(config.get(key), key):
            raise ValueError(f"{key} cannot be empty")
    if set(config["languages"]) - {"go", "typescript"}:
        raise ValueError("language has no installed analyzer")
    strings(config.get("exclude", []), "exclude")
    strings(config.get("documents", []), "documents")
    modules = config.get("modules")
    if not isinstance(modules, list) or not modules:
        raise ValueError("modules cannot be empty")
    checks = config.get("review_checks", [])
    if not isinstance(checks, list) or any(
        not isinstance(c, dict)
        or set(c) != {"name", "requirement"}
        or any(not isinstance(v, str) or not v.strip() for v in c.values())
        for c in checks
    ):
        raise ValueError("review_checks requires named requirements")
    if len({c["name"] for c in checks}) != len(checks):
        raise ValueError("duplicate review check names")
    names = set()
    for module in modules:
        if not isinstance(module, dict) or set(module) - {
            "name",
            "paths",
            "allow",
            "public",
            "forbid_imports",
            "allow_external",
        }:
            raise ValueError("unknown module configuration field")
        name = module.get("name")
        if not isinstance(name, str) or not name or name in names:
            raise ValueError("module name must be unique and nonempty")
        names.add(name)
        if not strings(module.get("paths"), "paths"):
            raise ValueError("module paths cannot be empty")
        for key in ("allow", "public", "forbid_imports", "allow_external"):
            strings(module.get(key, []), key)
    for module in modules:
        if set(module.get("allow", [])) - names:
            raise ValueError(f"unknown allowed module in {module['name']}")
    base = Path(filename).parent.as_posix()

    def scoped(name):
        result = os.path.normpath((Path(base) / name).as_posix())
        relative_file(root, result)
        return result

    for key in ("source_roots", "exclude", "documents"):
        if key in config:
            config[key] = [scoped(name) for name in config[key]]
    for module in config["modules"]:
        for key in ("paths", "public"):
            if key in module:
                module[key] = [scoped(name) for name in module[key]]
    config["_base"] = base
    return config


def collect(root, config, filename):
    sources, excluded, assets = {}, [], []
    base = config["_base"]
    if "go" in config["languages"]:
        mod = relative_file(root, (Path(base) / "go.mod").as_posix())
        for directory in {base, *config["source_roots"]}:
            current = relative_file(root, directory)
            while current.is_relative_to(root):
                if (current / "go.work").exists():
                    raise ValueError("Go workspaces are not supported by this analyzer")
                manifest = current / "go.mod"
                if manifest.exists() and manifest != mod:
                    raise ValueError("source scope crosses an unsupported Go module")
                if current == root:
                    break
                current = current.parent
        if mod.exists() and re.search(r"^\s*replace\b", mod.read_text(), re.M):
            raise ValueError(
                "Go replacement directives are not supported by this analyzer"
            )

    def scan_error(error):
        raise error

    for directory in config["source_roots"]:
        base = relative_file(root, directory)
        if not base.is_dir():
            raise ValueError(f"source root does not exist: {directory}")
        for current, dirs, files in os.walk(
            base, followlinks=False, onerror=scan_error
        ):
            for name in dirs:
                if (Path(current) / name).is_symlink():
                    raise ValueError("symlink directory in source roots")
            dirs[:] = sorted(d for d in dirs if d not in IGNORED)
            for name in sorted(files):
                path = Path(current) / name
                rel = path.relative_to(root).as_posix()
                relative_file(root, rel)
                if (
                    name == "go.mod"
                    and "go" in config["languages"]
                    and rel != (Path(config["_base"]) / "go.mod").as_posix()
                ):
                    raise ValueError(f"nested Go module is not supported: {rel}")
                if matches(rel, config.get("exclude", [])):
                    excluded.append(rel)
                    continue
                language = LANGUAGES.get(path.suffix)
                if not language:
                    if path.suffix in OTHER_CODE:
                        raise ValueError(f"unsupported source language: {rel}")
                    assets.append(rel)
                    continue
                if language not in config["languages"]:
                    raise ValueError(f"language not enabled for {rel}")
                sources[rel] = path.read_bytes().decode("utf-8")
    if not sources:
        raise ValueError("no source files selected")
    members = {}
    for path in sorted(sources):
        found = [m["name"] for m in config["modules"] if matches(path, m["paths"])]
        if len(found) != 1:
            raise ValueError(
                f"source must belong to exactly one module: {path} ({found})"
            )
        members[path] = found[0]
    for module in config["modules"]:
        if module["name"] not in members.values():
            raise ValueError(f"module has no source files: {module['name']}")
        if module.get("public") and not any(
            matches(p, module["public"])
            for p in sources
            if members[p] == module["name"]
        ):
            raise ValueError(f"module has no public entry: {module['name']}")
    context = [filename, *config.get("documents", [])]
    for name in (
        "go.mod",
        "go.sum",
        "tsconfig.json",
        "package.json",
        "package-lock.json",
    ):
        name = (Path(config["_base"]) / name).as_posix()
        if (root / name).exists():
            context.append(name)
    texts = dict(sources)
    for name in context:
        texts[name] = relative_file(root, name).read_bytes().decode("utf-8")
    hashes = {p: digest(t.encode()) for p, t in sorted(texts.items())}
    return dict(
        sources=sources,
        texts=texts,
        hashes=hashes,
        members=members,
        coverage=dict(
            scanned=sorted(sources),
            excluded=sorted(set(excluded)),
            assets=sorted(set(assets)),
            context=sorted(set(texts) - set(sources)),
        ),
    )


def parse(root, inventory, language, config):
    adapter = Path(__file__).parent / "analyzers"
    files = [
        dict(path=p, text=t)
        for p, t in inventory["sources"].items()
        if LANGUAGES[Path(p).suffix] == language
    ]
    if not files:
        raise ValueError(f"enabled analyzer has no source: {language}")
    payload = dict(
        root=str(root),
        project_root=config["_base"],
        files=files,
        assets=inventory["coverage"]["assets"],
    )
    command = (
        ["go", "run", str(adapter / "go.go")]
        if language == "go"
        else ["node", str(adapter / "typescript.cjs")]
    )
    env = dict(os.environ)
    if language == "go":
        env.update(GOWORK="off", GO111MODULE="off", GOPROXY="off", GOTOOLCHAIN="local")
        # The helper only imports the standard library, and never executes project code.
    # Node's synchronous fd-0 reader can see EAGAIN on macOS when Python
    # writes a payload larger than the pipe buffer. A private regular file
    # supplies the same complete JSON without a nonblocking pipe or retry.
    with tempfile.TemporaryFile(mode="w+", encoding="utf-8") as request:
        json.dump(payload, request)
        request.seek(0)
        result = subprocess.run(
            command,
            stdin=request,
            text=True,
            capture_output=True,
            timeout=120,
            cwd=root,
            env=env,
        )
    if result.returncode:
        raise ValueError(f"{language} analyzer failed: {result.stderr.strip()[:2000]}")
    parsed = json.loads(result.stdout)
    if parsed.get("errors"):
        raise ValueError("; ".join(parsed["errors"]))
    if sorted(x["path"] for x in parsed["files"]) != sorted(x["path"] for x in files):
        raise ValueError("analyzer source coverage differs")
    return parsed["files"]


def dependencies(root, files, inventory, config_base):
    by_path = {f["path"]: f for f in files}
    go_packages = {
        str(Path(f["path"]).parent): f["unit"] for f in files if f["language"] == "go"
    }
    module = None
    if go_packages:
        found = re.search(
            r"^module\s+(\S+)",
            inventory["texts"].get((Path(config_base) / "go.mod").as_posix(), ""),
            re.M,
        )
        if not found:
            raise ValueError("Go source requires a root go.mod module declaration")
        module = found.group(1).strip('"')
    edges = []
    for file in files:
        for item in file.get("imports", []):
            spec = item["specifier"]
            target = item.get("target")
            external = not target
            if file["language"] == "go" and (
                spec == module or spec.startswith(module + "/")
            ):
                folder = (
                    Path(config_base) / spec[len(module) :].lstrip("/")
                ).as_posix()
                if folder not in go_packages:
                    raise ValueError(
                        f"local import outside analyzed scope: {file['path']} -> {spec}"
                    )
                target = next(
                    f["path"] for f in files if f["unit"] == go_packages[folder]
                )
                external = False
            if target and target not in by_path:
                raise ValueError(f"local dependency outside analyzed scope: {target}")
            edges.append(
                dict(
                    source=file["path"],
                    target=target,
                    import_name=spec,
                    line=item["line"],
                    external=external,
                    standard_library=item.get("standard_library", False),
                )
            )
    return edges


def violations(files, edges, inventory, config):
    modules = {m["name"]: m for m in config["modules"]}
    members = inventory["members"]
    by_path = {f["path"]: f for f in files}
    findings = []
    graph = {f["unit"]: set() for f in files}
    for edge in edges:
        source, target = edge["source"], edge["target"]
        origin = modules[members[source]]

        def add(rule, message):
            findings.append(
                dict(
                    rule=rule,
                    source=source,
                    line=edge["line"],
                    target=target,
                    message=message,
                )
            )

        if matches(edge["import_name"], origin.get("forbid_imports", [])):
            add(
                "forbidden_import",
                f"{origin['name']} forbids import {edge['import_name']}",
            )
        if (
            not target
            and not edge["standard_library"]
            and "allow_external" in origin
            and not matches(edge["import_name"], origin["allow_external"])
        ):
            add(
                "external_dependency",
                f"{origin['name']} does not allow external import {edge['import_name']}",
            )
        if not target:
            continue
        a, b = by_path[source]["unit"], by_path[target]["unit"]
        if a != b:
            graph[a].add(b)
        if members[source] == members[target]:
            continue
        destination = modules[members[target]]
        if destination["name"] not in origin.get("allow", []):
            add(
                "dependency", f"{origin['name']} cannot depend on {destination['name']}"
            )
        if destination.get("public") and not matches(target, destination["public"]):
            add(
                "private_entry", f"use a declared public entry of {destination['name']}"
            )
    # Tarjan SCC emits one finding per strongly connected component, including within a module.
    index, low, stack, active = {}, {}, [], set()

    def visit(node):
        index[node] = low[node] = len(index)
        stack.append(node)
        active.add(node)
        for child in sorted(graph[node]):
            if child not in index:
                visit(child)
                low[node] = min(low[node], low[child])
            elif child in active:
                low[node] = min(low[node], index[child])
        if low[node] == index[node]:
            component = []
            while True:
                child = stack.pop()
                active.remove(child)
                component.append(child)
                if child == node:
                    break
            if len(component) > 1:
                path = next(f["path"] for f in files if f["unit"] in component)
                findings.append(
                    dict(
                        rule="cycle",
                        source=path,
                        line=1,
                        target=None,
                        message="dependency cycle: " + ", ".join(sorted(component)),
                    )
                )

    for node in sorted(graph):
        if node not in index:
            visit(node)
    return findings


def analyze(root, filename="architecture.json"):
    root = Path(root).resolve()
    config = load_config(root, filename)
    inventory = collect(root, config, filename)
    files = []
    for language in config["languages"]:
        files.extend(parse(root, inventory, language, config))
    owners = {}
    modules = {module["name"]: module for module in config["modules"]}
    for file in files:
        if file["language"] != "go":
            continue
        owner = inventory["members"][file["path"]]
        if modules[owner].get("public"):
            raise ValueError(
                f"Go public entry is a package, not a file list: {file['unit']}"
            )
        previous = owners.setdefault(file["unit"], owner)
        if previous != owner:
            raise ValueError(
                f"Go package spans multiple architecture modules: {file['unit']} ({previous}, {owner})"
            )
    edges = dependencies(root, files, inventory, config["_base"])
    findings = violations(files, edges, inventory, config)
    verify_snapshot(root, inventory["hashes"])
    current = collect(root, load_config(root, filename), filename)
    if (
        current["hashes"] != inventory["hashes"]
        or current["coverage"] != inventory["coverage"]
    ):
        raise ValueError("source inventory changed during analysis")
    report = dict(
        tool_version=VERSION,
        status="violations" if findings else "conformant",
        snapshot=digest(json.dumps(inventory["hashes"], sort_keys=True).encode()),
        hashes=inventory["hashes"],
        coverage=inventory["coverage"],
        facts=dict(files=files, dependencies=edges),
        rules=dict(
            status="violations" if findings else "conformant", findings=findings
        ),
        model_review=dict(status="not_run"),
        limits=[
            "Static import/package analysis; no runtime call-graph or cross-service inference.",
            "Go includes all selected build-tag variants; run Go build/test for active build validity.",
            "Vue template auto-imports and runtime component names require explicit imports or separate review.",
        ],
    )
    return report, inventory["texts"]


def verify_snapshot(root, hashes):
    for name, expected in hashes.items():
        if digest(relative_file(root, name).read_bytes()) != expected:
            raise ValueError(f"source snapshot changed: {name}")
