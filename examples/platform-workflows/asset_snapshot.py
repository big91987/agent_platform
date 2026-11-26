"""Keep installed role assets at immutable, content-addressed paths."""

import hashlib
import json
import os
import shutil
import stat
import subprocess
import tempfile
from pathlib import Path


def excluded(name):
    return (
        name
        in (
            ".git",
            "__pycache__",
            ".DS_Store",
            ".ruff_cache",
            ".pytest_cache",
            "node_modules",
            ".env",
            ".npmrc",
            "credentials.json",
        )
        or name.endswith(".pyc")
        or name.startswith(".env.")
    )


def tree_digest(root, dependencies=False):
    root = root.resolve(strict=True)
    rows = []
    for directory, subdirectories, files in os.walk(root):
        if not dependencies:
            subdirectories[:] = [name for name in subdirectories if not excluded(name)]
            files = [name for name in files if not excluded(name)]
        for name in subdirectories + files:
            path = Path(directory) / name
            relative = path.relative_to(root).as_posix()
            mode = path.lstat().st_mode
            if stat.S_ISDIR(mode):
                rows.append([relative, "directory"])
            elif stat.S_ISREG(mode):
                rows.append(
                    [
                        relative,
                        mode & 0o111,
                        hashlib.sha256(path.read_bytes()).hexdigest(),
                    ]
                )
            elif (
                dependencies
                and stat.S_ISLNK(mode)
                and path.resolve(strict=True).is_relative_to(root)
            ):
                rows.append([relative, "symlink", os.readlink(path)])
            else:
                raise ValueError(
                    "Asset tree contains an unsupported file: " + str(path)
                )
    return hashlib.sha256(json.dumps(sorted(rows)).encode()).hexdigest()


def snapshot(source, root, upgrade, previous=None):
    source = Path(source).resolve(strict=True)
    digest = tree_digest(source)
    if previous and previous["sha256"] != digest and not upgrade:
        raise ValueError("Use --upgrade to install a new asset version: " + str(source))
    target = root / digest / source.name
    if not target.exists():
        target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        with tempfile.TemporaryDirectory(dir=target.parent) as temporary:
            copy = Path(temporary) / target.name
            shutil.copytree(
                source,
                copy,
                ignore=lambda _, names: [name for name in names if excluded(name)],
            )
            if tree_digest(copy) != digest:
                raise ValueError("Asset changed while being installed: " + str(source))
            copy.rename(target)
    if target.is_symlink() or tree_digest(target) != digest:
        raise ValueError(
            "Installed asset differs from its recorded digest: " + str(target)
        )
    return target, {"source": str(source), "path": str(target), "sha256": digest}


PARSERS = ("typescript", "@vue/compiler-sfc")
PARSER_PROBE = """
const path = require('node:path');
const fs = require('node:fs');
const root = fs.realpathSync('node_modules') + path.sep;
const versions = {};
for (const name of ['typescript', '@vue/compiler-sfc']) {
  const entry = fs.realpathSync(require.resolve(name));
  if (!entry.startsWith(root)) throw new Error('parser resolves outside the installation');
  versions[name] = require(name + '/package.json').version;
}
if (require('typescript').createSourceFile('probe.ts', 'const x: number = 1', 99).parseDiagnostics.length)
  throw new Error('TypeScript probe failed');
if (require('@vue/compiler-sfc').parse('<template><div/></template>').errors.length)
  throw new Error('Vue parser probe failed');
console.log(JSON.stringify(versions));
"""


def parser_environment():
    return {
        key: os.environ[key]
        for key in (
            "PATH",
            "HOME",
            "TMPDIR",
            "LANG",
            "HTTP_PROXY",
            "HTTPS_PROXY",
            "NO_PROXY",
            "http_proxy",
            "https_proxy",
            "no_proxy",
            "npm_config_cache",
            "npm_config_offline",
        )
        if key in os.environ
    }


def parser_runtime(root):
    return {
        key: subprocess.check_output(
            [key, "--version"],
            cwd=root,
            text=True,
            timeout=10,
            env=parser_environment(),
        ).strip()
        for key in ("node", "npm")
    }


def parser_lock(root):
    package = json.loads((root / "package.json").read_text())
    lock = (root / "package-lock.json").read_bytes()
    installed = json.loads(lock)["packages"]
    versions = {name: installed["node_modules/" + name]["version"] for name in PARSERS}
    if any(
        package["dependencies"].get(name) != version
        for name, version in versions.items()
    ):
        raise ValueError("Architecture parsers require exact package-lock versions")
    return {"lock_sha256": hashlib.sha256(lock).hexdigest(), "parsers": versions}


def check_parser_files(root, receipt):
    modules = root / "node_modules"
    if (
        modules.is_symlink()
        or not modules.is_dir()
        or tree_digest(modules, dependencies=True) != receipt["dependencies_sha256"]
    ):
        raise ValueError(
            "Installed architecture parser files differ from the preparation receipt"
        )


def check_parsers(root, receipt):
    check_parser_files(root, receipt)
    if parser_runtime(root) != receipt["runtime"]:
        raise ValueError("Node/npm version differs from the preparation receipt")
    versions = json.loads(
        subprocess.check_output(
            ["node", "-e", PARSER_PROBE],
            cwd=root,
            text=True,
            timeout=15,
            env=parser_environment(),
        )
    )
    if versions != receipt["parsers"]:
        raise ValueError("Architecture parser readiness check failed")


def verify_arch_parsers(root):
    if not (root / "package.json").exists():
        return None
    receipt = json.loads(root.with_name(root.name + "-parsers.json").read_text())
    if any(receipt.get(key) != value for key, value in parser_lock(root).items()):
        raise ValueError(
            "Architecture parser lock differs from the preparation receipt"
        )
    check_parsers(root, receipt)
    return receipt


def prepare_arch_parsers(root):
    if not (root / "package.json").exists():
        return None
    receipt_path = root.with_name(root.name + "-parsers.json")
    if receipt_path.exists():
        return verify_arch_parsers(root)
    receipt = {**parser_lock(root), "runtime": parser_runtime(root)}
    modules = root / "node_modules"
    if modules.exists() or modules.is_symlink():
        raise ValueError(
            "Unrecorded parser installation; inspect the private asset copy"
        )
    log_path = root.with_name(root.name + "-npm-install.log")
    with tempfile.TemporaryDirectory(dir=root.parent) as temporary:
        staging = Path(temporary)
        for name in ("package.json", "package-lock.json"):
            shutil.copyfile(root / name, staging / name)
        fd = os.open(
            log_path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC | os.O_NOFOLLOW, 0o600
        )
        try:
            with os.fdopen(fd, "wb") as log:
                subprocess.run(
                    [
                        "npm",
                        "ci",
                        "--ignore-scripts",
                        "--engine-strict",
                        "--logs-max=0",
                        "--no-audit",
                        "--no-fund",
                        "--userconfig",
                        "/dev/null",
                    ],
                    cwd=staging,
                    env=parser_environment(),
                    stdout=log,
                    stderr=subprocess.STDOUT,
                    check=True,
                    timeout=180,
                )
        except (OSError, subprocess.SubprocessError) as error:
            raise ValueError(
                "Locked architecture parser installation failed; inspect the private npm log"
            ) from error
        dependencies = staging / "node_modules"
        receipt["dependencies_sha256"] = tree_digest(dependencies, dependencies=True)
        check_parsers(staging, receipt)
        # Same filesystem and no changes to the source directory.
        dependencies.rename(modules)
        try:
            with tempfile.NamedTemporaryFile(
                dir=root.parent, mode="w", delete=False
            ) as output:
                json.dump(receipt, output, indent=2)
                temporary_receipt = Path(output.name)
            temporary_receipt.replace(receipt_path)
        except OSError:
            modules.rename(dependencies)
            raise
    return verify_arch_parsers(root)


def snapshot_arch(source, root, upgrade, previous=None):
    source = Path(source).resolve(strict=True)
    if not (source / "package.json").exists():
        return snapshot(source, root, upgrade, previous)
    runtime = parser_runtime(source)
    runtime_key = hashlib.sha256(
        json.dumps(runtime, sort_keys=True).encode()
    ).hexdigest()
    destination = root / ("node-npm-" + runtime_key)
    prior = (previous or {}).get("parser_installation")
    if prior:
        old = Path(previous["path"])
        if old.is_symlink() or not old.resolve(strict=True).is_relative_to(
            root.resolve()
        ):
            raise ValueError("Previous architecture tool is outside installed assets")
        receipt = json.loads(old.with_name(old.name + "-parsers.json").read_text())
        if receipt != prior or tree_digest(old) != previous["sha256"]:
            raise ValueError("Installed architecture asset differs from the manifest")
        if prior["runtime"] != runtime:
            check_parser_files(old, prior)
            if not upgrade:
                raise ValueError(
                    "Use --upgrade to prepare parsers for the new Node/npm version"
                )
        elif old.parent.parent == root and tree_digest(source) == previous["sha256"]:
            # Keep the original source-only location for its recorded runtime.
            destination = root
    path, record = snapshot(source, destination, upgrade, previous)
    parsers = prepare_arch_parsers(path)
    if parsers["runtime"] != runtime:
        raise ValueError("Node/npm version changed during parser preparation")
    record["parser_installation"] = parsers
    return path, record
