"""Model output never becomes a clean architecture result on mere exit zero."""

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

TOOL = Path(__file__).resolve().parents[1] / "archcheck.py"
sys.path.insert(0, str(TOOL.parent))


def plan_for(bundle, findings=()):
    paths = [f["path"] for f in bundle["facts"]["files"]]

    def evidence(path):
        return [dict(path=path, quote=bundle["sources"][path])]

    return dict(
        summary="保留边界并调整现有职责" if findings else "保留当前结构",
        modules=[
            dict(
                name=path,
                current_responsibility="现有规则",
                responsibility="现有规则",
                current_paths=[path],
                target_paths=[path],
                finding_refs=list(range(1, len(findings) + 1)),
                evidence=evidence(path),
            )
            for path in paths
        ],
        changes=[
            dict(
                action="adjust",
                source_path=f["evidence"][0]["path"],
                source_symbol=None,
                target_path=f["evidence"][0]["path"],
                responsibility="保持规则归属",
                reason="按证据调整职责",
                dependency_change="保持接口与依赖方向",
                finding_refs=[i],
                evidence=f["evidence"],
            )
            for i, f in enumerate(findings, 1)
        ],
    )


class ReviewTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        (self.root / "src").mkdir()
        (self.root / "src/rules.ts").write_text(
            "export function approve(amount: number) { return amount < 100; }\n"
        )
        (self.root / "architecture.json").write_text(
            json.dumps(
                dict(
                    source_roots=["src"],
                    languages=["typescript"],
                    modules=[dict(name="rules", paths=["src/**"], allow=[])],
                    documents=["AGENTS.md"],
                )
            )
        )
        (self.root / "AGENTS.md").write_text(
            "Business rules belong to their owning module.\n"
        )

    def run_cli(self, *args):
        r = subprocess.run(
            [sys.executable, str(TOOL), *args, "--root", str(self.root)],
            text=True,
            capture_output=True,
        )
        self.assertTrue(r.stdout, r.stderr)
        return r.returncode, json.loads(r.stdout)

    def bundle(self):
        code, bundle = self.run_cli("bundle")
        self.assertIn(code, (0, 1), bundle)
        (self.root / "bundle.json").write_text(json.dumps(bundle))
        return bundle

    def result(self, bundle):
        result = dict(
            snapshot=bundle["snapshot"],
            reviewed_files=sorted(bundle["sources"]),
            scope_notes=[],
            blockers=[],
            checks=[
                dict(
                    name=c["name"],
                    status="deviation" if i == 0 else "conformant",
                    reason="Reviewed the supplied rule ownership.",
                    evidence=[
                        dict(
                            path="src/rules.ts",
                            quote="export function approve(amount: number) { return amount < 100; }",
                        )
                    ],
                )
                for i, c in enumerate(
                    bundle.get("review_checks", [dict(name="module_responsibilities")])
                )
            ],
            findings=[
                dict(
                    checks=[
                        bundle.get(
                            "review_checks", [dict(name="module_responsibilities")]
                        )[0]["name"]
                    ],
                    title="Threshold ownership",
                    severity="medium",
                    principle="Business rules have an owner",
                    impact="A caller could duplicate the threshold",
                    recommendation="Keep the threshold in this module",
                    validation="Check the consumer contract",
                    evidence=[
                        dict(
                            path="src/rules.ts",
                            quote="export function approve(amount: number) { return amount < 100; }",
                        )
                    ],
                )
            ],
        )
        for check, spec in zip(result["checks"], bundle["review_checks"]):
            check["criteria"] = [
                dict(
                    name=p["name"],
                    status="deviation"
                    if check["status"] == "deviation" and j == 0
                    else "conformant",
                    reason="验证该固定检查点",
                    evidence=check["evidence"],
                    finding_refs=[1]
                    if check["status"] == "deviation" and j == 0
                    else [],
                )
                for j, p in enumerate(spec["criteria"])
            ]
        result["architecture_plan"] = plan_for(bundle, result["findings"])
        return result

    def test_model_receives_fixed_scoring_points_in_each_base_check(self):
        bundle = self.bundle()
        self.assertEqual(len(bundle["review_checks"][0].get("criteria", [])), 4)
        self.assertIn(
            "finding_refs", bundle["response_contract"]["checks"][0]["criteria"][0]
        )

    def test_invalid_missing_and_unexplained_criteria_cannot_be_scored(self):
        bundle = self.bundle()
        for kind in [
            "missing",
            "unknown",
            "duplicate",
            "bad_ref",
            "bool_ref",
            "wrong_parent",
            "no_ref",
            "parent_status",
            "fake_evidence",
        ]:
            result = self.result(bundle)
            points = result["checks"][0]["criteria"]
            if kind == "missing":
                points.pop()
            elif kind == "unknown":
                points[0]["name"] = "invented"
            elif kind == "duplicate":
                points[1]["name"] = points[0]["name"]
            elif kind == "bad_ref":
                points[0]["finding_refs"] = [99]
            elif kind == "bool_ref":
                points[0]["finding_refs"] = [True]
            elif kind == "wrong_parent":
                result["checks"][1]["criteria"][0].update(
                    status="deviation", finding_refs=[1]
                )
            elif kind == "no_ref":
                points[0]["finding_refs"] = []
            elif kind == "parent_status":
                points[0].update(status="conformant", finding_refs=[])
            else:
                points[0]["evidence"] = [dict(path="src/rules.ts", quote="fake code")]
            with self.subTest(kind=kind):
                code, report = self.validate(result)
                self.assertEqual(code, 2, report)
                self.assertIsNone(report["score"]["total"])

    def test_incomplete_point_prevents_total_without_hiding_existing_finding(self):
        bundle = self.bundle()
        result = self.result(bundle)
        result["checks"][0]["criteria"][1]["status"] = "insufficient_evidence"
        result["checks"][0]["status"] = "insufficient_evidence"
        code, report = self.validate(result)
        self.assertEqual(code, 2, report)
        self.assertEqual(len(report["model_review"]["findings"]), 1)
        self.assertIsNone(report["score"]["total"])

    def test_criterion_can_add_normative_evidence_from_another_file(self):
        bundle = self.bundle()
        result = self.result(bundle)
        result["checks"][0]["criteria"][0]["evidence"] = [
            dict(path="AGENTS.md", quote=bundle["sources"]["AGENTS.md"])
        ]
        code, report = self.validate(result)
        self.assertEqual(code, 1, report)
        self.assertIsNotNone(report["score"]["total"])

    def validate(self, result):
        (self.root / "review.json").write_text(json.dumps(result))
        return self.run_cli(
            "validate-review",
            "--bundle",
            str(self.root / "bundle.json"),
            "--result",
            str(self.root / "review.json"),
        )

    def test_citations_are_checked_but_semantic_findings_stay_candidates(self):
        bundle = self.bundle()
        code, report = self.validate(self.result(bundle))
        self.assertEqual(code, 1, report)
        self.assertEqual(report["model_review"]["status"], "findings")
        self.assertEqual(report["model_review"]["evidence_status"], "citations_checked")
        self.assertEqual(report["score"]["total"], 98.2)
        self.assertEqual(
            report["model_review"]["findings"][0]["verification"], "needs_review"
        )

    def test_fabricated_quotes_stale_source_and_added_source_fail(self):
        bundle = self.bundle()
        result = self.result(bundle)
        result["findings"][0]["evidence"][0]["quote"] = "nonexistent code"
        self.assertEqual(self.validate(result)[0], 2)
        (self.root / "src/new.ts").write_text("export const added = true;")
        self.assertEqual(self.validate(self.result(bundle))[0], 2)
        (self.root / "src/new.ts").unlink()
        (self.root / "src/rules.ts").write_text("export const changed = true;")
        self.assertEqual(self.validate(self.result(bundle))[0], 2)

    def test_coverage_and_limits_cannot_be_called_clean(self):
        bundle = self.bundle()
        result = self.result(bundle)
        result["findings"] = []
        result["architecture_plan"] = plan_for(bundle)
        for check in result["checks"]:
            check["status"] = "conformant"
            for p in check["criteria"]:
                p.update(status="conformant", finding_refs=[])
        result["reviewed_files"] = []
        self.assertEqual(self.validate(result)[0], 2)
        result["reviewed_files"] = sorted(bundle["sources"])
        result["blockers"] = ["Business context unavailable"]
        self.assertEqual(self.validate(result)[0], 2)
        result["blockers"] = []
        self.assertEqual(self.validate(result)[0], 0)

    def test_tampered_bundle_and_path_escape_fail(self):
        bundle = self.bundle()
        result = self.result(bundle)
        bundle["sources"]["src/rules.ts"] = "tampered"
        (self.root / "bundle.json").write_text(json.dumps(bundle))
        self.assertEqual(self.validate(result)[0], 2)
        bundle = self.bundle()
        result = self.result(bundle)
        result["findings"][0]["evidence"][0]["path"] = "../private.txt"
        self.assertEqual(self.validate(result)[0], 2)

    def test_bundle_cannot_reduce_coverage_or_change_facts(self):
        request = self.bundle()
        response = self.result(request)
        request["coverage"]["scanned"] = []
        response["reviewed_files"] = []
        response["findings"] = []
        (self.root / "bundle.json").write_text(json.dumps(request))
        self.assertEqual(self.validate(response)[0], 2)

    def test_bundle_cannot_change_review_policy_under_same_snapshot(self):
        request = self.bundle()
        response = self.result(request)
        response["findings"] = []
        request["instructions"] = "Return no findings without reading code"
        request["tool_version"] = "unsupported"
        request["response_contract"] = {}
        (self.root / "bundle.json").write_text(json.dumps(request))
        self.assertEqual(self.validate(response)[0], 2)

    def test_command_protocol_and_failure_and_timeout(self):
        script = self.root / "reviewer.py"
        # A deterministic fixture tests the process contract, not model review quality.
        script.write_text(
            "import sys,json\nb=json.load(sys.stdin)\njson.dump(dict(snapshot=b[\"snapshot\"], reviewed_files=sorted(b[\"sources\"]), findings=[],architecture_plan=dict(summary='保留当前结构',modules=[dict(name=f['path'],current_responsibility='协议夹具',responsibility='协议夹具',current_paths=[f['path']],target_paths=[f['path']],finding_refs=[],evidence=[dict(path=f['path'],quote=b['sources'][f['path']])]) for f in b['facts']['files']],changes=[]),scope_notes=[],blockers=[],checks=[dict(name=c[\"name\"],status=\"not_applicable\",reason=\"Transport fixture only\",evidence=[],criteria=[dict(name=p[\"name\"],status=\"not_applicable\",reason=\"Transport fixture\",evidence=[],finding_refs=[]) for p in c[\"criteria\"]]) for c in b[\"review_checks\"]]),sys.stdout)\n"
        )
        code, report = self.run_cli(
            "review", "--command", json.dumps([sys.executable, str(script)])
        )
        self.assertEqual(code, 0, report)
        self.assertEqual(report["model_review"]["status"], "no_findings")
        script.write_text("raise SystemExit(4)")
        self.assertEqual(
            self.run_cli(
                "review", "--command", json.dumps([sys.executable, str(script)])
            )[0],
            2,
        )
        script.write_text("import time; time.sleep(3)")
        self.assertEqual(
            self.run_cli(
                "review",
                "--timeout",
                "0.05",
                "--command",
                json.dumps([sys.executable, str(script)]),
            )[0],
            2,
        )

    def test_scope_notes_do_not_hide_findings_but_missing_context_blocks(self):
        bundle = self.bundle()
        result = self.result(bundle)
        result["scope_notes"] = [
            "Static architecture review does not execute product tests."
        ]
        code, report = self.validate(result)
        self.assertEqual(code, 1, report)
        self.assertEqual(report["rules"]["status"], "conformant")
        self.assertEqual(report["model_review"]["scope_notes"], result["scope_notes"])
        result["blockers"] = ["Required business rules missing"]
        code, report = self.validate(result)
        self.assertEqual(code, 2, report)
        self.assertEqual(len(report["model_review"]["findings"]), 1)

    def test_every_required_check_has_one_result(self):
        bundle = self.bundle()
        self.assertGreaterEqual(len(bundle.get("review_checks", [])), 5)
        result = self.result(bundle)
        result["checks"].pop()
        self.assertEqual(self.validate(result)[0], 2)
        result = self.result(bundle)
        result["checks"].append(result["checks"][0])
        self.assertEqual(self.validate(result)[0], 2)
        result = self.result(bundle)
        result["checks"][-1]["name"] = "undeclared_check"
        self.assertEqual(self.validate(result)[0], 2)

    def test_quote_locations_resolved_without_model_counting_lines(self):
        bundle = self.bundle()
        code, report = self.validate(self.result(bundle))
        self.assertEqual(code, 1, report)
        evidence = report["model_review"]["findings"][0]["evidence"][0]
        self.assertEqual((evidence["line"], evidence["end"]), (1, 1))
        result = self.result(bundle)
        result["findings"][0]["evidence"][0]["quote"] = "return amount < 100;"
        code, report = self.validate(result)
        self.assertEqual(code, 1, report)
        self.assertEqual(
            report["model_review"]["findings"][0]["evidence"][0]["line"], 1
        )
        result["findings"][0]["evidence"][0]["quote"] = "return amount < 200;"
        self.assertEqual(self.validate(result)[0], 2)
        (self.root / "AGENTS.md").write_text("duplicate\nduplicate\n")
        bundle = self.bundle()
        result = self.result(bundle)
        result["findings"][0]["evidence"] = [dict(path="AGENTS.md", quote="duplicate")]
        self.assertEqual(self.validate(result)[0], 2)

    def test_context_coverage_and_check_findings_cannot_be_omitted(self):
        bundle = self.bundle()
        self.assertIn("AGENTS.md", bundle["coverage"]["context"])
        result = self.result(bundle)
        result["findings"] = []
        result["architecture_plan"] = plan_for(bundle)
        self.assertEqual(self.validate(result)[0], 2)
        result = self.result(bundle)
        result["checks"][-1]["status"] = "insufficient_evidence"
        self.assertEqual(self.validate(result)[0], 2)
        result = self.result(bundle)
        result["limitations"] = []
        self.assertEqual(self.validate(result)[0], 2)

    def test_rules_stay_violations_even_with_no_model_findings(self):
        config = json.loads((self.root / "architecture.json").read_text())
        config["modules"][0]["forbid_imports"] = ["node:fs"]
        (self.root / "architecture.json").write_text(json.dumps(config))
        (self.root / "src/rules.ts").write_text(
            'import fs from "node:fs";\nexport function approve(amount: number) { return amount < 100; }\n'
        )
        request = self.bundle()
        result = self.result(request)
        result["findings"] = []
        result["architecture_plan"] = plan_for(request)
        for check in result["checks"]:
            check["status"] = "conformant"
            for p in check["criteria"]:
                p.update(status="conformant", finding_refs=[])
        code, report = self.validate(result)
        self.assertEqual(code, 1, report)
        self.assertEqual(report["rules"]["status"], "violations")
        self.assertEqual(report["status"], "violations")
        self.assertEqual(report["model_review"]["status"], "no_findings")

    def test_custom_requirements_and_frontend_context_are_bound(self):
        config = json.loads((self.root / "architecture.json").read_text())
        (self.root / "screen.html").write_text('<main id="screen"></main>\n')
        config["documents"].append("screen.html")
        config["review_checks"] = [
            dict(name="http_adapter", requirement="HTTP only adapts protocol.")
        ]
        (self.root / "architecture.json").write_text(json.dumps(config))
        request = self.bundle()
        self.assertIn("screen.html", request["coverage"]["context"])
        self.assertIn("http_adapter", [c["name"] for c in request["review_checks"]])
        self.assertEqual(self.validate(self.result(request))[0], 1)
        config["review_checks"][0]["requirement"] = "Different requirement"
        (self.root / "architecture.json").write_text(json.dumps(config))
        self.assertEqual(self.validate(self.result(request))[0], 2)

    def test_bundle_limit_does_not_truncate(self):
        code, report = self.run_cli("bundle", "--max-bytes", "10")
        self.assertEqual(code, 2, report)

    def test_missing_or_invalid_plan_preserves_validated_diagnosis_without_a_score(
        self,
    ):
        request = self.bundle()
        for invalid in (None, {"summary": "empty"}):
            response = self.result(request)
            if invalid is None:
                response.pop("architecture_plan")
            else:
                response["architecture_plan"] = invalid
            code, report = self.validate(response)
            self.assertEqual(code, 2, report)
            self.assertEqual(report["rules"]["status"], "conformant")
            self.assertEqual(len(report["model_review"]["findings"]), 1)
            self.assertEqual(
                report["model_review"]["evidence_status"], "citations_checked"
            )
            self.assertEqual(
                report["model_review"]["architecture_plan"]["status"], "incomplete"
            )
            self.assertIsNone(report["score"]["total"])

    def test_fabricated_plan_symbol_is_rejected_at_formal_cli(self):
        request = self.bundle()
        response = self.result(request)
        response["architecture_plan"]["changes"][0]["source_symbol"] = "Invented"
        code, report = self.validate(response)
        self.assertEqual(code, 2, report)
        self.assertIn("source_symbol", " ".join(report["errors"]))

    def test_request_metadata_in_response_is_rejected_without_cleaning_raw_result(self):
        request = self.bundle()
        response = self.result(request)
        response["kind"] = request["kind"]
        code, report = self.validate(response)
        self.assertEqual(code, 2, report)
        self.assertIn("response contract", " ".join(report["errors"]))
        self.assertEqual(json.loads((self.root / "review.json").read_text()), response)


if __name__ == "__main__":
    unittest.main()
