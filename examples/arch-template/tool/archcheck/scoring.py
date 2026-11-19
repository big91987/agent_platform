"""A versioned rubric over validated judgments; scoring never changes the gate."""

from .review import DEFAULT_CHECKS
from .rubric import CRITERIA, RUBRIC, RULE_CRITERIA

LABELS = {
    "module_responsibilities": "模块职责",
    "business_rule_ownership": "规则归属",
    "dependency_direction": "分层与依赖",
    "state_ownership": "状态归属",
    "testability": "可测试性",
    "necessary_abstractions": "抽象合理性",
    "standards_alignment": "规范一致性",
}


def score_report(report):
    """Derive seven equal-weight dimensions, retaining every deduction's evidence."""
    model = report.get("model_review", {})
    rules = report.get("rules", {})
    findings = model.get("findings", [])
    rule_findings = rules.get("findings", [])
    checks = model.get("checks", [])
    by_name = {c["name"]: c for c in checks}
    reasons = []
    if report.get("status") not in {"conformant", "violations", "review_required"}:
        reasons.append("检查未完成。")
    if rules.get("status") not in {"conformant", "violations"}:
        reasons.append("规则扫描未完成。")
    if model.get("status") not in {"findings", "no_findings"}:
        reasons.append("模型评审未完成或未执行。")
    if model.get("evidence_status") != "citations_checked":
        reasons.append("模型引用尚未通过源码核验。")
    if model.get("blockers"):
        reasons.append("仍有评审阻塞项。")
    if len(by_name) != len(checks) or not set(DEFAULT_CHECKS) <= by_name.keys():
        reasons.append("七个基础维度缺失或检查项重复。")
    if any(
        c["status"] not in {"conformant", "deviation", "not_applicable"} for c in checks
    ):
        reasons.append("存在证据不足或未知状态的检查项。")
    if any(f["rule"] not in RULE_CRITERIA for f in rule_findings):
        reasons.append("存在尚未纳入评分口径的规则。")
    dimensions, raw_points = [], []
    criteria_completed = 0
    for name in DEFAULT_CHECKS:
        check = by_name.get(name, {})
        points = check.get("criteria", [])
        point_names = [p.get("name") for p in points]
        valid_points = len(point_names) == len(CRITERIA[name]) and set(
            point_names
        ) == set(CRITERIA[name])
        if not valid_points:
            reasons.append(f"{LABELS[name]}的固定检查点缺失、重复或未知。")
        point_map = {p.get("name"): p for p in points}
        rows, deductions, values = [], [], []
        for point_name, (label, _) in CRITERIA[name].items():
            point = point_map.get(point_name, {})
            status = point.get("status", "not_reviewed")
            refs = point.get("finding_refs", [])
            valid_refs = (
                isinstance(refs, list)
                and all(
                    type(i) is int
                    and 1 <= i <= len(findings)
                    and name in findings[i - 1]["checks"]
                    for i in refs
                )
                and len(refs) == len(set(refs))
            )
            if not valid_refs:
                reasons.append(f"{LABELS[name]}·{label}的问题引用无效。")
                refs = []
            linked = [
                dict(
                    source="model",
                    finding=i,
                    title=findings[i - 1]["title"],
                    severity=findings[i - 1]["severity"],
                    evidence=findings[i - 1]["evidence"],
                    verification="needs_review",
                    criterion=point_name,
                )
                for i in refs
            ]
            if status == "deviation" and not linked:
                reasons.append(f"{LABELS[name]}·{label}有偏差但缺少扣分依据。")
            if status != "deviation" and linked:
                reasons.append(f"{LABELS[name]}·{label}的状态与问题引用不一致。")
            if name == "dependency_direction":
                linked += [
                    dict(
                        source="rule",
                        finding=i,
                        title=f["message"],
                        severity="high",
                        evidence=[
                            dict(path=f["source"], line=f["line"], end=f["line"])
                        ],
                        criterion=point_name,
                    )
                    for i, f in enumerate(rule_findings, 1)
                    if RULE_CRITERIA.get(f["rule"]) == point_name
                ]
            level = (
                min((d["severity"] for d in linked), key=lambda s: RUBRIC[s]["points"])
                if linked
                else ("conformant" if status == "conformant" else None)
            )
            value = RUBRIC[level]["points"] if level else None
            if status not in {"conformant", "deviation", "not_applicable"}:
                reasons.append(f"{LABELS[name]}·{label}未完成评审。")
            else:
                criteria_completed += 1
            if status != "not_applicable" or linked:
                if value is None:
                    reasons.append(f"{LABELS[name]}·{label}尚无可计分结论。")
                else:
                    values.append(value)
            rows.append(
                dict(
                    name=point_name,
                    label=label,
                    status="deviation" if linked else status,
                    points=value,
                    level=level,
                    reason=point.get("reason", "未评审"),
                    evidence=point.get("evidence", []),
                    deductions=linked,
                )
            )
            deductions.extend(linked)
        mean = sum(values) / len(values) if values else None
        applicable = sum(p["status"] != "not_applicable" for p in rows)
        if valid_points and mean is not None and len(values) == applicable:
            raw_points.append(mean)
        if check.get("status") != "not_applicable" and mean is None:
            reasons.append(f"{LABELS[name]}没有适用检查点的有效结论。")
        dimensions.append(
            dict(
                name=name,
                label=LABELS[name],
                status="deviation"
                if deductions
                else check.get("status", "not_reviewed"),
                points=round(mean, 1)
                if mean is not None and len(values) == applicable
                else None,
                reason=check.get("reason", "未评审"),
                deductions=deductions,
                criteria=rows,
            )
        )
    if not raw_points:
        reasons.append("没有可计分的适用维度。")
    applicable = sum(d["status"] != "not_applicable" for d in dimensions)
    if len(raw_points) != applicable:
        reasons.append("适用维度尚未全部得到可计分结论。")
    completed = sum(
        c["status"] in {"conformant", "deviation", "not_applicable"} for c in checks
    )
    expected = len(DEFAULT_CHECKS) + len(set(by_name) - set(DEFAULT_CHECKS))
    return dict(
        rubric_version="2.0",
        status="unavailable" if reasons else "available",
        total=None if reasons else round(sum(raw_points) / len(raw_points), 1),
        formula="每维四个固定检查点；同点按最严重档计分且不按问题条数累扣，维度内取适用点均值，再对七维等权平均。不适用排除分母，缺证据不出总分。",
        rubric=RUBRIC,
        dimensions=dimensions,
        reasons=list(dict.fromkeys(reasons)),
        rule_violations=len(rule_findings),
        completeness=dict(
            completed=completed,
            expected=expected,
            scored=len(raw_points),
            criteria_completed=criteria_completed,
            criteria_expected=sum(len(c) for c in CRITERIA.values()),
            applicable=applicable,
            basis="reviewer_reported_citations_checked"
            if model.get("evidence_status") == "citations_checked"
            else "not_validated",
        ),
        note="模型辅助架构诊断评分，模型发现及严重度仍待复核；不改变规则门禁和检查退出码。",
    )
