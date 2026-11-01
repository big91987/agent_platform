"""Behavior contracts: parser failures and uncovered code must never pass."""

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

TOOL = Path(__file__).resolve().parents[1] / "archcheck.py"


class CheckTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.write("go.mod", "module example.com/demo\n\ngo 1.25.0\n")

    def write(self, name, text):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)

    def check(self, modules, languages=None, **options):
        config = dict(
            source_roots=["internal"],
            languages=languages or ["go"],
            modules=modules,
            **options,
        )
        self.write("architecture.json", json.dumps(config))
        result = subprocess.run(
            [sys.executable, str(TOOL), "check", "--root", str(self.root)],
            capture_output=True,
            text=True,
        )
        self.assertTrue(result.stdout, result.stderr)
        return result.returncode, json.loads(result.stdout)

    def test_allowed_dependency_and_external_forbidden_import(self):
        self.write(
            "internal/domain/rule.go",
            "package domain\nfunc Allow() bool { return true }\n",
        )
        self.write(
            "internal/app/usecase.go",
            'package app\nimport "example.com/demo/internal/domain"\nvar Allowed = domain.Allow\n',
        )
        modules = [
            dict(
                name="domain",
                paths=["internal/domain/**"],
                allow=[],
                forbid_imports=["database/sql"],
            ),
            dict(name="app", paths=["internal/app/**"], allow=["domain"]),
        ]
        code, report = self.check(modules)
        self.assertEqual(code, 0, report)
        self.assertEqual(report["model_review"]["status"], "not_run")
        self.write(
            "internal/domain/rule.go", 'package domain\nimport _ "database/sql"\n'
        )
        code, report = self.check(modules)
        self.assertEqual(code, 1, report)
        self.assertEqual(report["findings"][0]["rule"], "forbidden_import")
        self.assertEqual(report["findings"][0]["line"], 2)

    def test_new_uncovered_code_and_unknown_config_fail(self):
        self.write("internal/domain/a.go", "package domain\n")
        modules = [dict(name="domain", paths=["internal/domain/**"], allow=[])]
        self.write("internal/new/a.go", "package new\n")
        self.assertEqual(self.check(modules)[0], 2)
        self.assertEqual(self.check(modules, unknown_option=True)[0], 2)

    def test_go_package_cannot_be_split_into_file_level_architecture_modules(self):
        self.write("internal/business/model.go", "package business\n")
        self.write("internal/business/usecase.go", "package business\n")
        code, report = self.check(
            [
                dict(name="domain", paths=["internal/business/model.go"], allow=[]),
                dict(
                    name="application",
                    paths=["internal/business/usecase.go"],
                    allow=["domain"],
                ),
            ]
        )
        self.assertEqual(code, 2, report)
        self.assertIn("Go package", " ".join(report["errors"]))

    def test_external_allowlist_permits_standard_library_but_rejects_other_dependencies(
        self,
    ):
        modules = [
            dict(
                name="domain", paths=["internal/domain/**"], allow=[], allow_external=[]
            )
        ]
        self.write("internal/domain/rule.go", 'package domain\nimport _ "time"\n')
        code, report = self.check(modules)
        self.assertEqual(code, 0, report)
        self.write(
            "internal/domain/rule.go", 'package domain\nimport _ "example.org/policy"\n'
        )
        code, report = self.check(modules)
        self.assertEqual(code, 1, report)
        self.assertEqual(report["findings"][0]["rule"], "external_dependency")
        self.assertEqual(report["findings"][0]["line"], 2)
        modules[0]["allow_external"] = ["example.org/policy"]
        code, report = self.check(modules)
        self.assertEqual(code, 0, report)

    def test_go_file_public_entry_cannot_claim_package_encapsulation(self):
        self.write("internal/domain/model.go", "package domain\n")
        self.write("internal/domain/private.go", "package domain\n")
        code, report = self.check(
            [
                dict(
                    name="domain",
                    paths=["internal/domain/**"],
                    allow=[],
                    public=["internal/domain/model.go"],
                )
            ]
        )
        self.assertEqual(code, 2, report)
        self.assertIn("public", " ".join(report["errors"]))

    def test_empty_and_overlapping_module_paths_fail(self):
        self.write("internal/a/a.go", "package a\n")
        self.assertEqual(
            self.check([dict(name="missing", paths=["missing/**"], allow=[])])[0], 2
        )
        self.assertEqual(
            self.check(
                [
                    dict(name="a", paths=["internal/**"], allow=[]),
                    dict(name="b", paths=["internal/**"], allow=[]),
                ]
            )[0],
            2,
        )

    def test_syntax_error_and_package_cycle_fail(self):
        self.write("internal/a/a.go", "package a\nfunc Bad(\n")
        modules = [dict(name="all", paths=["internal/**"], allow=[])]
        self.assertEqual(self.check(modules)[0], 2)
        self.write(
            "internal/a/a.go", 'package a\nimport _ "example.com/demo/internal/b"\n'
        )
        self.write(
            "internal/b/b.go", 'package b\nimport _ "example.com/demo/internal/a"\n'
        )
        code, report = self.check(modules)
        self.assertEqual(code, 1, report)
        self.assertTrue(any(f["rule"] == "cycle" for f in report["findings"]))

    def test_vue_alias_type_import_and_public_entry(self):
        self.write(
            "tsconfig.json",
            json.dumps(
                dict(
                    compilerOptions=dict(
                        baseUrl=".",
                        paths={"@/*": ["src/*"]},
                        moduleResolution="Bundler",
                        module="ESNext",
                    )
                )
            ),
        )
        self.write("package.json", json.dumps(dict(dependencies={"vue": "3.5.22"})))
        self.write(
            "src/features/keys/index.ts", 'export type { Key } from "./types";\n'
        )
        self.write(
            "src/features/keys/types.ts", "export type Key = { name: string };\n"
        )
        self.write(
            "src/app/App.vue",
            '<script setup lang="ts">\nimport type { Key } from "@/features/keys";\nconst key: Key = {name:"demo"};\n</script><template>{{key.name}}</template>',
        )
        config = dict(
            source_roots=["src"],
            languages=["typescript"],
            modules=[
                dict(name="app", paths=["src/app/**"], allow=["keys"]),
                dict(
                    name="keys",
                    paths=["src/features/keys/**"],
                    allow=[],
                    public=["src/features/keys/index.ts"],
                ),
            ],
        )
        self.write("architecture.json", json.dumps(config))
        args = [sys.executable, str(TOOL), "check", "--root", str(self.root)]
        r = subprocess.run(args, capture_output=True, text=True)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.write(
            "src/app/App.vue",
            '<script setup lang="ts">\nimport type { Key } from "@/features/keys/types";\n</script>',
        )
        r = subprocess.run(args, capture_output=True, text=True)
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertEqual(json.loads(r.stdout)["findings"][0]["rule"], "private_entry")

    def test_computed_import_is_incomplete_not_clean(self):
        self.write("internal/a.ts", 'const name = "./other"; import(name);\n')
        code, report = self.check(
            [dict(name="all", paths=["internal/**"], allow=[])], ["typescript"]
        )
        self.assertEqual(code, 2, report)

    def test_missing_relative_import_is_incomplete(self):
        self.write("internal/a.ts", 'export { missing } from "./missing";\n')
        code, report = self.check(
            [dict(name="all", paths=["internal/**"], allow=[])], ["typescript"]
        )
        self.assertEqual(code, 2, report)

    def test_style_asset_import_is_visible_and_not_an_unresolved_code_import(self):
        self.write("internal/style.css", "body { color: black; }")
        self.write("internal/main.ts", 'import "./style.css";')
        code, report = self.check(
            [dict(name="all", paths=["internal/**"], allow=[])], ["typescript"]
        )
        self.assertEqual(code, 0, report)
        self.assertIn("internal/style.css", report["coverage"]["assets"])

    def test_crlf_source_snapshot_is_not_falsely_stale(self):
        (self.root / "internal").mkdir()
        (self.root / "internal/a.go").write_bytes(b"package all\r\n")
        self.assertEqual(
            self.check([dict(name="all", paths=["internal/**"], allow=[])])[0], 0
        )

    def test_mts_and_cts_are_code_and_unresolved_imports_fail(self):
        self.write("internal/main.ts", "export const valid = true;")
        for extension in ("mts", "cts"):
            self.write(
                "internal/extra." + extension, 'export { missing } from "./missing";'
            )
            code, report = self.check(
                [dict(name="all", paths=["internal/**"], allow=[])], ["typescript"]
            )
            self.assertEqual(code, 2, report)
            (self.root / ("internal/extra." + extension)).unlink()

    def test_ts_extension_substitution_precedes_existing_javascript(self):
        self.write("internal/app.ts", 'import { value } from "./dep.js";')
        self.write("internal/dep.ts", 'export const value = "typescript";')
        self.write("internal/dep.js", 'export const value = "javascript";')
        modules = [
            dict(name="app", paths=["internal/app.ts"], allow=["js"]),
            dict(name="js", paths=["internal/dep.js"], allow=[]),
            dict(name="ts", paths=["internal/dep.ts"], allow=[]),
        ]
        code, report = self.check(modules, ["typescript"])
        self.assertEqual(code, 1, report)
        self.assertTrue(
            any(
                f["rule"] == "dependency" and f["target"] == "internal/dep.ts"
                for f in report["findings"]
            )
        )

    def test_local_go_replacement_cannot_hide_module_boundary(self):
        self.write(
            "go.mod",
            "module example.com/demo\n\ngo 1.25.0\nreplace example.com/local => ./internal/lib\n",
        )
        self.write("internal/app/app.go", 'package app\nimport _ "example.com/local"\n')
        self.write("internal/lib/local.go", "package local\n")
        self.assertEqual(
            self.check(
                [
                    dict(name="app", paths=["internal/app/**"], allow=[]),
                    dict(name="lib", paths=["internal/lib/**"], allow=[]),
                ]
            )[0],
            2,
        )

    def test_root_source_scope_accepts_its_own_go_module(self):
        self.write("main.go", "package main\nfunc main() {}\n")
        self.write(
            "architecture.json",
            json.dumps(
                dict(
                    source_roots=["."],
                    languages=["go"],
                    modules=[dict(name="entry", paths=["*.go"], allow=[])],
                )
            ),
        )
        result = subprocess.run(
            [sys.executable, str(TOOL), "check", "--root", str(self.root)],
            capture_output=True,
            text=True,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_go_source_roots_cannot_skip_ancestor_module_or_workspace(self):
        self.write("lib/go.mod", "module example.com/local\n")
        self.write("lib/src/local.go", "package local\n")
        self.write("internal/app.go", 'package app\nimport _ "example.com/local/src"\n')
        self.write(
            "architecture.json",
            json.dumps(
                dict(
                    source_roots=["lib/src", "internal"],
                    languages=["go"],
                    modules=[
                        dict(name="lib", paths=["lib/**"], allow=[]),
                        dict(name="app", paths=["internal/**"], allow=[]),
                    ],
                )
            ),
        )
        command = [sys.executable, str(TOOL), "check", "--root", str(self.root)]
        result = subprocess.run(command, capture_output=True, text=True)
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)

    def test_component_go_config_rejects_repository_workspace(self):
        (self.root / "go.mod").unlink()
        command = [sys.executable, str(TOOL), "check", "--root", str(self.root)]
        self.write("service/go.mod", "module example.com/service\n")
        self.write("service/src/main.go", "package main\n")
        self.write(
            "service/architecture.json",
            json.dumps(
                dict(
                    source_roots=["src"],
                    languages=["go"],
                    modules=[dict(name="all", paths=["src/**"], allow=[])],
                )
            ),
        )
        self.write("go.work", "go 1.25.0\nuse ./service\n")
        result = subprocess.run(
            [*command, "--config", "service/architecture.json"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)

    def test_unreadable_directory_is_incomplete(self):
        self.write("internal/good.ts", "export const good = true;")
        self.write("internal/hidden/bad.ts", 'import "./missing";')
        hidden = self.root / "internal/hidden"
        hidden.chmod(0)
        try:
            self.assertEqual(
                self.check(
                    [dict(name="all", paths=["internal/**"], allow=[])], ["typescript"]
                )[0],
                2,
            )
        finally:
            hidden.chmod(0o700)

    def test_component_config_can_bind_shared_repository_documents(self):
        self.write(
            "web/architecture.json",
            json.dumps(
                dict(
                    source_roots=["src"],
                    languages=["typescript"],
                    modules=[dict(name="all", paths=["src/**"], allow=[])],
                    documents=["../docs/principles.md"],
                )
            ),
        )
        self.write("docs/principles.md", "Business rules have one owner.")
        self.write("web/src/main.ts", "export const value = true;")
        r = subprocess.run(
            [
                sys.executable,
                str(TOOL),
                "check",
                "--root",
                str(self.root),
                "--config",
                "web/architecture.json",
            ],
            capture_output=True,
            text=True,
        )
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("docs/principles.md", json.loads(r.stdout)["hashes"])

    def test_source_symlink_cannot_leave_scope(self):
        self.write("outside.go", "package outside\n")
        (self.root / "internal").mkdir()
        (self.root / "internal/escape.go").symlink_to(self.root / "outside.go")
        self.assertEqual(
            self.check([dict(name="all", paths=["internal/**"], allow=[])])[0], 2
        )


if __name__ == "__main__":
    unittest.main()
