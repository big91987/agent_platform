"""Plans must refer to real objects and render proposals without erasing facts."""

import copy
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from archcheck.review import resolve_evidence
from archcheck.structure import render_structure, validate_plan


class StructureTests(unittest.TestCase):
    def setUp(self):
        self.request = {
            "sources": {
                "internal/api/http.go": "package api\nfunc Handle() {\n runFlow()\n}\n",
                "internal/api/types.go": "package api\ntype Request struct {}\n",
                "README.md": "Keep HTTP and workflow responsibilities separate.\n",
            },
            "facts": {
                "files": [
                    dict(
                        path="internal/api/http.go",
                        language="go",
                        unit="go:internal/api",
                        symbols=[dict(name="Handle", line=2, end=4)],
                    ),
                    dict(
                        path="internal/api/types.go",
                        language="go",
                        unit="go:internal/api",
                        symbols=[dict(name="Request", line=2, end=2)],
                    ),
                ]
            },
        }
        self.evidence = dict(path="internal/api/http.go", quote=" runFlow()")
        self.findings = [dict(title="HTTP 承担流程", evidence=[self.evidence])]
        self.plan = dict(
            summary="保留 HTTP 入口，提取流程；入口通过显式接口调用流程。",
            modules=[
                dict(
                    name="api",
                    current_responsibility="请求适配和流程混合",
                    responsibility="HTTP 适配和可独立调用的流程分工",
                    current_paths=["internal/api"],
                    target_paths=["internal/api", "internal/flow"],
                    finding_refs=[1],
                    evidence=[self.evidence],
                )
            ],
            changes=[
                dict(
                    action="extract",
                    source_path="internal/api/http.go",
                    source_symbol="Handle",
                    target_path="internal/flow/run.go",
                    responsibility="流程编排",
                    reason="请求入口只保留输入输出转换",
                    dependency_change="HTTP 通过注入的流程接口调用，不让流程依赖 HTTP。",
                    finding_refs=[1],
                    evidence=[self.evidence],
                )
            ],
        )

    def validate(self, plan=None, findings=None):
        return validate_plan(
            self.request,
            self.plan if plan is None else plan,
            self.findings if findings is None else findings,
            resolve_evidence,
        )

    def report(self, plan):
        return dict(
            facts=self.request["facts"],
            rules=dict(findings=[]),
            model_review=dict(
                status="findings", findings=self.findings, architecture_plan=plan
            ),
        )

    def test_extraction_keeps_original_file_and_explains_responsibilities(self):
        raw = copy.deepcopy(self.plan)
        plan = self.validate()
        self.assertEqual(plan["status"], "ready")
        self.assertEqual(self.plan, raw)
        self.assertEqual(plan["changes"][0]["source_location"]["line"], 2)
        text = "\n".join(render_structure(self.report(plan)))
        self.assertIn("现状结构", text)
        self.assertIn("推荐结构", text)
        self.assertEqual(
            text.count("http.go"), 3
        )  # current tree, proposed tree, change table
        self.assertIn("run.go", text)
        self.assertIn("发现 1", text)
        self.assertIn("流程编排", text)
        self.assertIn("注入的流程接口", text)

    def test_invalid_paths_symbols_references_and_uncovered_findings_fail(self):
        mutations = [
            lambda p: p["changes"][0].update(source_path="missing.go"),
            lambda p: p["changes"][0].update(source_symbol="Invented"),
            lambda p: p["changes"][0].update(target_path="../escape.go"),
            lambda p: p["changes"][0].update(target_path="/tmp/escape.go"),
            lambda p: p["changes"][0].update(target_path="internal/\nfile.go"),
            lambda p: p["changes"][0].update(
                target_path="internal/api/http.go/child.go"
            ),
            lambda p: p["changes"][0].update(finding_refs=[True]),
            lambda p: p["changes"][0].update(finding_refs=[2]),
            lambda p: p.update(changes=[]),
            lambda p: p["modules"][0].update(current_paths=["README.md"]),
            lambda p: p["modules"][0].update(current_paths=["internal/api/http.go"]),
        ]
        for mutate in mutations:
            plan = copy.deepcopy(self.plan)
            mutate(plan)
            with self.subTest(plan=plan), self.assertRaises(ValueError):
                self.validate(plan)

    def test_no_findings_retains_structure_and_cannot_create_unjustified_layers(self):
        self.plan["changes"] = []
        self.plan["modules"][0].update(target_paths=["internal/api"], finding_refs=[])
        self.assertEqual(self.validate(findings=[])["status"], "ready")
        self.plan["modules"][0]["target_paths"].append("internal/domain")
        with self.assertRaises(ValueError):
            self.validate(findings=[])

    def test_a_proposed_file_cannot_use_an_existing_file_as_a_directory(self):
        self.plan["modules"][0]["target_paths"] = ["internal/api"]
        self.plan["changes"][0]["target_path"] = "internal/api/http.go/child.go"
        with self.assertRaisesRegex(ValueError, "file/directory"):
            self.validate()

    def test_unparsed_object_uses_quote_location_without_inventing_a_symbol(self):
        self.plan["changes"][0]["source_symbol"] = None
        plan = self.validate()
        change = plan["changes"][0]
        self.assertEqual(change["source_location"]["line"], 3)
        self.assertEqual(change["source_location"]["kind"], "evidence")
        self.assertIsNone(change["source_symbol"])

    def test_cross_module_extraction_can_address_a_target_modules_finding(self):
        source = copy.deepcopy(self.plan["modules"][0])
        target = copy.deepcopy(source)
        source.update(
            name="http",
            current_paths=["internal/api/http.go"],
            target_paths=["internal/api/http.go"],
            finding_refs=[],
        )
        target.update(
            name="flow",
            current_paths=["internal/api/types.go"],
            target_paths=["internal/api/types.go", "internal/flow"],
            evidence=[
                dict(path="internal/api/types.go", quote="type Request struct {}")
            ],
        )
        self.plan["modules"] = [source, target]
        self.assertEqual(self.validate()["status"], "ready")

    def test_move_removes_only_whole_file_and_merge_can_reuse_existing_target(self):
        self.plan["changes"][0].update(action="move", source_symbol=None)
        plan = self.validate()
        text = "\n".join(render_structure(self.report(plan)))
        self.assertEqual(
            text.count("http.go"), 2
        )  # current tree and change table, not proposal
        self.plan["changes"][0].update(target_path="internal/api/types.go")
        with self.assertRaises(ValueError):
            self.validate()
        self.plan["changes"][0].update(action="merge", source_symbol="Handle")
        self.plan["modules"][0]["target_paths"] = ["internal/api"]
        plan = self.validate()
        self.assertEqual(plan["changes"][0]["target_status"], "existing")

    def test_rules_only_current_tree_does_not_invent_a_recommendation(self):
        report = dict(
            facts=self.request["facts"],
            rules=dict(findings=[]),
            model_review=dict(status="not_run"),
        )
        text = "\n".join(render_structure(report))
        self.assertIn("http.go", text)
        self.assertIn("未运行模型", text)
        self.assertNotIn("run.go", text)

    def test_normative_adjustment_is_not_misrepresented_as_production_structure(self):
        evidence = dict(path="README.md", quote=self.request["sources"]["README.md"])
        self.plan["modules"][0].update(target_paths=["internal/api"], finding_refs=[])
        self.plan["changes"][0].update(
            action="adjust",
            source_path="README.md",
            source_symbol=None,
            target_path="README.md",
            evidence=[evidence],
        )
        plan = self.validate()
        self.assertEqual(plan["changes"][0]["source_kind"], "context")
        text = "\n".join(render_structure(self.report(plan)))
        self.assertIn("上下文文件", text)
        self.assertNotIn("── README.md", text)


if __name__ == "__main__":
    unittest.main()
