"""The readable report locates governance findings without changing judgments."""

import copy
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from inspect_project import render


class ReportTests(unittest.TestCase):
    def report(self):
        return {
            "status": "review_required",
            "rules": {"status": "conformant", "findings": []},
            "model_review": {
                "status": "findings",
                "evidence_status": "citations_checked",
                "checks": [],
                "findings": [
                    {
                        "checks": ["module_responsibilities"],
                        "title": "调用入口承担存储实现",
                        "severity": "medium",
                        "principle": "接口适配与存储实现分离",
                        "impact": "存储变化需要修改入口",
                        "recommendation": "将 Handle 中的 SQL 移入存储实现；Handle 调用应用操作。",
                        "validation": "检查入口不再导入 database/sql，并运行原有回归。",
                        "evidence": [
                            {
                                "path": "src/http.go",
                                "line": 5,
                                "end": 5,
                                "quote": 'db.Exec("UPDATE limits")',
                            }
                        ],
                        "verification": "needs_review",
                    }
                ],
                "scope_notes": [],
                "blockers": [],
            },
            "facts": {
                "files": [
                    {
                        "path": "src/http.go",
                        "symbols": [
                            {"name": "Handle", "line": 3, "end": 7},
                            {"name": "Other", "line": 9, "end": 12},
                        ],
                    }
                ]
            },
            "coverage": {"scanned": ["src/http.go"], "context": ["AGENTS.md"]},
        }

    def test_readable_findings_keep_original_advice_and_locate_code_objects(self):
        report = self.report()
        original = copy.deepcopy(report)
        text = render(report)
        self.assertIn("src/http.go:5", text)
        self.assertIn("Handle", text)
        self.assertNotIn("Other", text)
        finding = report["model_review"]["findings"][0]
        for field in ("title", "principle", "impact", "recommendation", "validation"):
            self.assertIn(finding[field], text)
        self.assertIn(finding["evidence"][0]["quote"], text)
        self.assertIn("待复核", text)
        self.assertEqual(report, original)

    def test_no_findings_and_incomplete_model_cannot_hide_rule_violation(self):
        report = self.report()
        report["status"] = "violations"
        report["rules"] = {
            "status": "violations",
            "findings": [
                {
                    "rule": "forbidden_import",
                    "source": "src/http.go",
                    "line": 2,
                    "target": None,
                    "message": "entry forbids import database/sql",
                }
            ],
        }
        for status in ("no_findings", "incomplete"):
            report["model_review"] = {"status": status}
            text = render(report)
            self.assertIn("src/http.go:2", text)
            self.assertIn("entry forbids import database/sql", text)
            self.assertIn("存在规则违规", text)
            if status == "incomplete":
                self.assertIn("模型评审未完成", text)
            else:
                self.assertIn("模型未提出候选问题", text)

    def test_missing_analysis_never_prints_a_passing_rule_result(self):
        text = render(
            {
                "status": "incomplete",
                "errors": ["analyzer failed"],
                "model_review": {"status": "incomplete"},
            }
        )
        self.assertIn("规则检查未完成", text)
        self.assertIn("analyzer failed", text)
        self.assertNotIn("未发现规则违规", text)

    def test_document_evidence_is_not_assigned_a_code_symbol(self):
        report = self.report()
        report["model_review"]["findings"][0]["evidence"] = [
            {
                "path": "AGENTS.md",
                "line": 2,
                "end": 3,
                "quote": "HTTP must not import SQL.\nUse explicit module interfaces.",
            }
        ]
        text = render(report)
        self.assertIn("AGENTS.md:2-3", text)
        self.assertNotIn("`Handle`", text)
        self.assertIn("HTTP must not import SQL.", text)
