#!/usr/bin/env python3
"""Export selected reference scaffolds and one portable checker maintenance source."""

import argparse
import fcntl
import hashlib
import json
import runpy
import shutil
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent
SKIP = {"node_modules", "__pycache__", "dist", ".DS_Store", "SOURCE.json"}


def files(root):
    return {
        p.relative_to(root).as_posix(): p
        for p in sorted(root.rglob("*"))
        if p.is_file() and not any(s in SKIP for s in p.relative_to(root).parts)
    }


def hashes(root):
    return {
        n: hashlib.sha256(p.read_bytes()).hexdigest() for n, p in files(root).items()
    }


def copy(source, dest):
    for name, path in files(source).items():
        target = dest / name
        if target.exists() and target.read_bytes() != path.read_bytes():
            raise ValueError(f"conflicting template path: {name}")
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(path, target)


def ignore_artifacts(project):
    path = project / ".gitignore"
    if path.is_symlink():
        raise ValueError("project .gitignore must not be a symlink")
    text = path.read_text() if path.exists() else ""
    block = "harness-output/\n.harness-output/\n"
    if not text.endswith(block):
        path.write_text(
            text + ("\n" if text and not text.endswith("\n") else "") + block
        )


def install_tool(dest):
    copy(ROOT / "tool", dest)
    version = runpy.run_path(str(ROOT / "tool/archcheck/__init__.py"))["VERSION"]
    manifest = dict(version=version, files=hashes(dest))
    (dest / "SOURCE.json").write_text(json.dumps(manifest, indent=2) + "\n")


def adopt(project):
    if not project.is_dir():
        raise ValueError("existing project directory is required")
    parent = project / ".harness"
    if parent.is_symlink():
        raise ValueError("tool parent must not be a symlink")
    target = parent / "archcheck"
    if target.exists() or target.is_symlink():
        raise ValueError(
            "tool already installed; use --update-tool after reviewing changes"
        )
    ignore_artifacts(project)
    parent.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(dir=parent) as temporary:
        candidate = Path(temporary) / "archcheck"
        install_tool(candidate)
        candidate.rename(target)
    print(str(target))


def upgrade(project):
    tool = project / ".harness/archcheck"
    with (tool / "SOURCE.json").open("rb") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            raise ValueError(
                "tool is running; finish active checks before upgrading"
            ) from None
        manifest = json.load(lock)
        if manifest["files"] != hashes(tool):
            raise ValueError(
                "installed tool has local changes; reconcile with its maintenance source before upgrading"
            )
        ignore_artifacts(project)
        with tempfile.TemporaryDirectory(dir=tool.parent) as temp:
            candidate = Path(temp) / "new"
            candidate.mkdir()
            install_tool(candidate)
            previous = Path(temp) / "previous"
            tool.rename(previous)
            try:
                candidate.rename(tool)
            except BaseException:
                previous.rename(tool)
                raise
    print(
        "Tool upgraded from maintenance source. Reinstall locked parser dependencies, then run the project verification entry."
    )


def merge_config(base, new):
    for key in ("source_roots", "languages", "exclude", "documents"):
        base[key] = list(dict.fromkeys(base.get(key, []) + new.get(key, [])))
    checks = {c["name"]: c for c in base.get("review_checks", [])}
    for check in new.get("review_checks", []):
        if check["name"] in checks and checks[check["name"]] != check:
            raise ValueError("conflicting review requirement: " + check["name"])
        checks[check["name"]] = check
    if checks:
        base["review_checks"] = list(checks.values())
    existing = {m["name"]: m for m in base["modules"]}
    for module in new["modules"]:
        if module["name"] not in existing:
            base["modules"].append(module)
            existing[module["name"]] = module
        else:
            target = existing[module["name"]]
            for key in ("paths", "allow", "forbid_imports"):
                target[key] = list(
                    dict.fromkeys(target.get(key, []) + module.get(key, []))
                )
            # A selected component's explicit dependency restriction must survive
            # composition regardless of selection order. Empty means forbid all.
            if "allow_external" in module:
                target["allow_external"] = list(
                    dict.fromkeys(
                        target.get("allow_external", []) + module["allow_external"]
                    )
                )
    return base


SETUP = '''#!/usr/bin/env python3
"""Install only the locked dependencies selected for this project."""
from pathlib import Path
import subprocess
root=Path(__file__).resolve().parent
if (root/'go.mod').exists(): subprocess.run(['go','mod','download'],cwd=root,check=True)
if (root/'web').exists():
    for folder in (root/'.harness/archcheck',root/'web'):
        subprocess.run(['npm','ci','--ignore-scripts','--no-fund','--no-audit'],cwd=folder,check=True)
'''
VERIFY = '''#!/usr/bin/env python3
"""One project entry: language checks, behavior tests and architecture constraints."""
from pathlib import Path
import subprocess,sys,json
root=Path(__file__).resolve().parent
def run(args,cwd=root): subprocess.run(args,cwd=cwd,check=True)
tool=str(root/'.harness/archcheck/archcheck.py')
def architecture(config, output):
    result=subprocess.run([sys.executable,tool,'check','--root',str(root),'--config',config,'--out',output],cwd=root,capture_output=True,text=True)
    report=json.loads(result.stdout)
    print('Architecture:',config,report['status'],'report:',output)
    if result.returncode:
        print(json.dumps(report.get('errors',report.get('rules',{}).get('findings')),ensure_ascii=False,indent=2))
        raise SystemExit(result.returncode)
if (root/'go.mod').exists():
    source=[str(p.relative_to(root)) for folder in ('cmd','internal') for p in (root/folder).rglob('*.go')]
    unformatted=subprocess.check_output(['gofmt','-l',*source],cwd=root,text=True)
    if unformatted: raise SystemExit('Run gofmt on: '+unformatted)
    for args in (['go','mod','verify'],['go','vet','./...'],['go','test','-race','./...'],['go','build','./...']): run(args)
    architecture('architecture.json','.harness-output/backend-architecture.json')
if (root/'web').exists():
    run(['npm','run','verify'],root/'web')
    architecture('web/architecture.json','.harness-output/frontend-architecture.json')
print('Language, behavior and architecture checks passed. Model review is a separate recorded step.')
'''


def export(destination, backends, frontend):
    if not backends and not frontend:
        raise ValueError("select at least one backend or frontend")
    if destination.exists():
        raise ValueError("destination already exists; export only to a new directory")
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(dir=destination.parent) as temp:
        project = Path(temp) / "project"
        project.mkdir()
        config = dict(
            source_roots=[], languages=[], exclude=[], documents=[], modules=[]
        )
        for backend in dict.fromkeys(backends):
            source = ROOT / "backend/go" / backend / "template"
            selected = json.loads((source / "architecture.json").read_text())
            for name, path in files(source).items():
                if name in ("architecture.json", "AGENTS.md", "go.mod", "go.sum"):
                    continue
                target = project / name
                if target.exists() and target.read_bytes() != path.read_bytes():
                    raise ValueError(f"conflicting backend file: {name}")
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(path, target)
            merge_config(config, selected)
        if backends:
            config["documents"].append("docs/engineering-principles.md")
            base = (
                ROOT
                / "backend/go"
                / ("saas" if "saas" in backends else "model-gateway")
                / "template"
            )
            for name in ("go.mod", "go.sum"):
                if (base / name).exists():
                    shutil.copyfile(base / name, project / name)
            (project / "architecture.json").write_text(
                json.dumps(config, ensure_ascii=False, indent=2) + "\n"
            )
        if frontend:
            copy(ROOT / "frontend/vue/admin-console/template", project / "web")
            frontend_config_path = project / "web/architecture.json"
            frontend_config = json.loads(frontend_config_path.read_text())
            frontend_config["documents"].append("../docs/engineering-principles.md")
            frontend_config_path.write_text(
                json.dumps(frontend_config, ensure_ascii=False, indent=2) + "\n"
            )
        install_tool(project / ".harness/archcheck")
        (project / "docs").mkdir(exist_ok=True)
        shutil.copyfile(
            ROOT / "principles.md", project / "docs/engineering-principles.md"
        )
        docs = [
            "docs/engineering-principles.md",
            *[f"docs/{name}.md" for name in dict.fromkeys(backends)],
        ]
        links = "\n".join(f"- [{name}]({name})" for name in docs)
        (project / "AGENTS.md").write_text(
            "# Project engineering index\n\nRead the applicable architecture and business contracts before changing code:\n"
            + links
            + "\n"
            + ("- [Vue console](web/AGENTS.md)\n" if frontend else "")
            + "\nKeep business knowledge, architecture decisions and testing requirements in docs/, linked from this index. Run python3 verify.py before delivery. Architecture rule success is not model review or product acceptance.\n"
        )
        (project / "setup.py").write_text(SETUP)
        (project / "verify.py").write_text(VERIFY)
        (project / ".gitignore").write_text(
            "**/node_modules/\n**/dist/\n**/__pycache__/\n*.db\n*.db-*\nharness-output/\n.harness-output/\n"
        )
        (project / "README.md").write_text(
            "# Service reference scaffold\n\nSelected backend: "
            + ", ".join(backends or ["none"])
            + "; frontend: "
            + ("Vue" if frontend else "none")
            + ".\n\n```sh\npython3 setup.py\npython3 verify.py\n```\n\nRead AGENTS.md and the scenario documents for responsibility boundaries, startup commands and limitations. The quota API binds 127.0.0.1:8080; the gateway binds 127.0.0.1:8081; Vue development proxies /api to 8080. Set UPSTREAM_URL explicitly for the gateway. No credentials are copied or assumed.\n\n.harness/archcheck is an exported tool, not a link to the platform checkout. SOURCE.json records its exact bytes. Upgrade through the maintenance-source scaffold.py --update-tool PROJECT; local tool edits are refused. Project source remains owned by this project.\n\nModel review: use the tool bundle/review/validate-review commands. Its candidate findings are distinct from executable rule violations; see the installed tool README.\n"
        )
        (project / "template-source.json").write_text(
            json.dumps(
                dict(
                    version="0.4.0",
                    backends=backends,
                    frontend="vue" if frontend else None,
                    files=hashes(project),
                ),
                ensure_ascii=False,
                indent=2,
            )
            + "\n"
        )
        project.rename(destination)
    print(str(destination))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--destination", type=Path)
    parser.add_argument(
        "--backend", choices=["saas", "model-gateway"], action="append", default=[]
    )
    parser.add_argument("--frontend", choices=["vue"])
    parser.add_argument("--update-tool", type=Path)
    parser.add_argument("--install-tool", type=Path)
    args = parser.parse_args()
    try:
        if args.install_tool:
            if args.destination or args.backend or args.frontend or args.update_tool:
                raise ValueError(
                    "tool installation cannot be combined with other options"
                )
            adopt(args.install_tool.resolve())
        elif args.update_tool:
            if args.destination or args.backend or args.frontend:
                raise ValueError(
                    "tool upgrade cannot be combined with scaffold options"
                )
            upgrade(args.update_tool.resolve())
        else:
            if not args.destination:
                raise ValueError("destination is required")
            export(args.destination.resolve(), args.backend, bool(args.frontend))
    except (ValueError, OSError, KeyError) as error:
        print(str(error), file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
