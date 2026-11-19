"""Scores explain validated judgments without rewarding missing evidence."""

import copy
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from archcheck.scoring import score_report

NAMES = (
    "module_responsibilities",
    "business_rule_ownership",
    "dependency_direction",
    "state_ownership",
    "testability",
    "necessary_abstractions",
    "standards_alignment",
)

POINTS = {
    "module_responsibilities": ["cohesion", "interfaces", "transport", "composition"],
    "business_rule_ownership": ["owner", "duplication", "policy_io", "change_locality"],
    "dependency_direction": [
        "allowed_dependencies",
        "encapsulation",
        "technology",
        "cycles",
    ],
    "state_ownership": ["owner", "lifetime", "mutation", "authority"],
    "testability": [
        "policy_seams",
        "external_seams",
        "failure_signals",
        "state_control",
    ],
    "necessary_abstractions": ["responsibility", "complexity", "reuse", "variation"],
    "standards_alignment": [
        "archived_rules",
        "coding",
        "architecture",
        "rule_consistency",
    ],
}


def criteria(name):
    return [
        dict(
            name=p,
            status="conformant",
            reason="有对应边界",
            finding_refs=[],
            evidence=[
                dict(path="src/app.go", line=2, end=2, quote="type App struct {}")
            ],
        )
        for p in POINTS[name]
    ]


def reviewed():
    return {
        "status": "conformant",
        "rules": {"status": "conformant", "findings": []},
        "model_review": {
            "status": "no_findings",
            "evidence_status": "citations_checked",
            "checks": [
                dict(
                    name=n,
                    status="conformant",
                    reason="边界明确",
                    criteria=criteria(n),
                    evidence=[
                        dict(
                            path="src/app.go", line=2, end=2, quote="type App struct {}"
                        )
                    ],
                )
                for n in NAMES
            ],
            "findings": [],
            "blockers": [],
        },
    }


def add_finding(report, names, severity="medium", title="请求入口维护共享状态"):
    report["status"] = "review_required"
    model = report["model_review"]
    model["status"] = "findings"
    for c in model["checks"]:
        if c["name"] in names:
            c.update(status="deviation", reason=title)
            c["criteria"][0]["status"] = "deviation"
            c["criteria"][0]["finding_refs"].append(len(model["findings"]) + 1)
    model["findings"].append(
        dict(
            checks=names,
            severity=severity,
            title=title,
            verification="needs_review",
            principle="状态归属明确",
            impact="不相关职责相互依赖",
            recommendation="将状态交给对应能力维护",
            validation="复查模块边界",
            evidence=[
                dict(path="src/app.go", line=2, end=2, quote="type App struct {}")
            ],
        )
    )


class ScoringTests(unittest.TestCase):
    def test_score_is_per_dimension_and_repeated_findings_do_not_multiply_penalties(
        self,
    ):
        report = reviewed()
        add_finding(report, ["module_responsibilities", "state_ownership"])
        before = copy.deepcopy(report)
        score = score_report(report)
        # One medium point in each of two dimensions: 87.5 each.
        self.assertEqual(score["total"], 96.4)
        self.assertEqual(report, before)
        report["model_review"]["findings"] *= 3
        for check in report["model_review"]["checks"]:
            if check["status"] == "deviation":
                check["criteria"][0]["finding_refs"] = [1, 2, 3]
        self.assertEqual(score_report(report)["total"], 96.4)
        add_finding(report, ["state_ownership"], "high")
        self.assertEqual(score_report(report)["total"], 94.6)
        row = score["dimensions"][0]
        self.assertEqual(row["points"], 87.5)
        self.assertEqual(row["deductions"][0]["evidence"][0]["path"], "src/app.go")

    def test_rule_violation_penalizes_its_point_and_keeps_gate_separate(self):
        report = reviewed()
        report["status"] = "violations"
        report["rules"] = dict(
            status="violations",
            findings=[
                dict(
                    rule="forbidden_import",
                    source="src/app.go",
                    line=1,
                    target=None,
                    message="domain forbids net/http",
                )
            ],
        )
        score = score_report(report)
        self.assertEqual(score["total"], 96.4)
        self.assertEqual(score["rule_violations"], 1)
        row = next(
            r for r in score["dimensions"] if r["name"] == "dependency_direction"
        )
        self.assertEqual(row["points"], 75)
        self.assertEqual(row["criteria"][0]["points"], 0)
        self.assertEqual(row["deductions"][0]["source"], "rule")
        self.assertEqual(report["status"], "violations")

    def test_incomplete_unvalidated_and_unreviewed_reports_never_get_a_total(self):
        cases = []
        r = reviewed()
        r["status"] = "incomplete"
        cases.append(r)
        r = reviewed()
        r["model_review"]["blockers"] = ["缺失入口源码"]
        cases.append(r)
        r = reviewed()
        r["model_review"].pop("evidence_status")
        cases.append(r)
        r = reviewed()
        r["model_review"]["checks"].pop()
        cases.append(r)
        r = reviewed()
        r["model_review"]["checks"][0]["status"] = "insufficient_evidence"
        cases.append(r)
        r = reviewed()
        r["model_review"] = dict(status="not_run")
        cases.append(r)
        for report in cases:
            with self.subTest(report=report):
                score = score_report(report)
                self.assertIsNone(score["total"])
                self.assertEqual(score["status"], "unavailable")
                self.assertTrue(score["reasons"])

    def test_not_applicable_is_not_a_free_hundred_and_extra_checks_do_not_dilute_score(
        self,
    ):
        report = reviewed()
        add_finding(report, ["module_responsibilities"])
        report["model_review"]["checks"][-1]["status"] = "not_applicable"
        for p in report["model_review"]["checks"][-1]["criteria"]:
            p["status"] = "not_applicable"
        self.assertEqual(score_report(report)["total"], 97.9)
        report["model_review"]["checks"].append(
            dict(
                name="custom_rule",
                status="conformant",
                reason="已核对",
                evidence=[],
                criteria=[],
            )
        )
        self.assertEqual(score_report(report)["total"], 97.9)
        report["model_review"]["checks"][-1]["status"] = "insufficient_evidence"
        self.assertIsNone(score_report(report)["total"])
        report = reviewed()
        for c in report["model_review"]["checks"]:
            c["status"] = "not_applicable"
            for p in c["criteria"]:
                p["status"] = "not_applicable"
        self.assertIsNone(score_report(report)["total"])

    def test_one_severe_point_does_not_zero_an_entire_dimension(self):
        report = reviewed()
        add_finding(report, ["module_responsibilities"], "high")
        score = score_report(report)
        self.assertEqual(score["dimensions"][0]["points"], 75)
        self.assertEqual(score["total"], 96.4)
        self.assertEqual(score["rubric_version"], "2.0")
        self.assertEqual(score["completeness"]["criteria_completed"], 28)

    def test_zero_requires_every_applicable_point_to_be_severely_deviating(self):
        report = reviewed()
        add_finding(report, ["module_responsibilities"], "high")
        for p in report["model_review"]["checks"][0]["criteria"]:
            p.update(status="deviation", finding_refs=[1])
        self.assertEqual(score_report(report)["dimensions"][0]["points"], 0)

    def test_missing_or_unresolved_points_never_get_a_total(self):
        for kind in [
            "missing",
            "unknown",
            "duplicate",
            "insufficient_evidence",
            "bad_ref",
        ]:
            report = reviewed()
            add_finding(report, ["module_responsibilities"], "high")
            points = report["model_review"]["checks"][0]["criteria"]
            if kind == "missing":
                points.pop()
            elif kind == "unknown":
                points[0]["name"] = "invented"
            elif kind == "duplicate":
                points[1]["name"] = points[0]["name"]
            elif kind == "bad_ref":
                points[0]["finding_refs"] = [99]
            else:
                points[1]["status"] = kind
            with self.subTest(kind=kind):
                self.assertIsNone(score_report(report)["total"])

    def test_point_not_applicable_uses_explicit_denominator_and_unrounded_means(self):
        report = reviewed()
        add_finding(report, ["module_responsibilities"])
        report["model_review"]["checks"][0]["criteria"][-1]["status"] = "not_applicable"
        score = score_report(report)
        self.assertEqual(score["dimensions"][0]["points"], 83.3)
        self.assertEqual(score["total"], 97.6)

    def test_rule_cannot_be_waived_by_na_and_duplicates_do_not_multiply(self):
        report = reviewed()
        report["status"] = "violations"
        check = report["model_review"]["checks"][2]
        check["criteria"][1]["status"] = "not_applicable"
        rule = dict(
            rule="private_entry",
            source="src/app.go",
            line=1,
            target=None,
            message="private module access",
        )
        report["rules"] = dict(status="violations", findings=[rule, rule])
        score = score_report(report)
        self.assertEqual(score["dimensions"][2]["points"], 75)
        self.assertEqual(score["dimensions"][2]["criteria"][1]["points"], 0)
        self.assertEqual(score["rule_violations"], 2)
        self.assertEqual(report["status"], "violations")


if __name__ == "__main__":
    unittest.main()
