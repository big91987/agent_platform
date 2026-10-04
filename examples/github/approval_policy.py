"""Issue opt-in and stage approval instructions for the GitHub example."""

import re
from dataclasses import dataclass


def read_autonomous(issue_body: str) -> bool:
    """Accept exactly one explicit checkbox in the dedicated Issue-form section.

    Quotes, code, comments and other headings cannot grant this permission.
    Ambiguous duplicate sections or checkboxes fail closed.
    """
    sections = 0
    in_section = False
    choices = []
    fence = None
    comment = False
    for line in issue_body.splitlines():
        if comment:
            if "-->" in line:
                comment = False
            continue
        if "<!--" in line:
            comment = "-->" not in line.split("<!--", 1)[1]
            continue
        if fence:
            if re.fullmatch(
                r" {0,3}" + re.escape(fence[0]) + r"{" + str(fence[1]) + r",}\s*", line
            ):
                fence = None
            continue
        opening = re.match(r" {0,3}(`{3,}|~{3,})", line)
        if opening:
            marker = opening.group(1)
            fence = (marker[0], len(marker))
            continue
        if re.match(r" {0,3}#{1,6}(?:\s|$)", line):
            in_section = line.rstrip() == "### 自主推进"
            if in_section:
                sections += 1
            continue
        if in_section and re.match(r"- \[[ xX]\] ", line):
            choices.append(line.rstrip())
    return (
        sections == 1
        and len(choices) == 1
        and choices[0]
        in (
            "- [x] 按推荐方案自主推进",
            "- [X] 按推荐方案自主推进",
        )
    )


@dataclass(frozen=True)
class _StagePolicy:
    target_stage: str
    automatic_handoff: bool
    automatic_clarification: bool
    automatic_rework: bool
    integration_only: bool


def _resolve_policy(stage, autonomous=False, integrating=False):
    targets = {
        "requirements": "design",
        "design": "development",
        "development": "qa",
        "qa": "report",
    }
    if stage not in targets:
        raise ValueError(f"unsupported approval stage: {stage}")
    automatic = stage in ("development", "qa") if integrating else bool(autonomous)
    return _StagePolicy(
        targets[stage], automatic, automatic, automatic, bool(integrating)
    )


_POLICY_START = "<!-- pipeline-stage-applicability -->"
_POLICY_END = "<!-- /pipeline-stage-applicability -->"


def stage_applicability(stage):
    """Shared task and registered-Agent guidance; routing remains unchanged."""
    if stage not in ("requirements", "design", "development", "qa"):
        return ""
    common = (
        "阶段适用性：先根据 Issue、已接受决定、上游摘要和现有实现判断本阶段需要做什么，"
        "本段优先于本 Agent/Skill 中按阶段一律产出文档或形式审批的默认要求。"
        "不以任务标题或‘脚本修复’标签直接判断无设计影响；检查目标、范围、验收、接口、数据、权限、部署和恢复影响。"
    )
    if stage in ("requirements", "design"):
        target = "design" if stage == "requirements" else "development"
        common += (
            "已有材料已足够且本阶段无新增决策/产物时，在 summary 写清不适用理由、依据、明确的目标与验收约束、"
            "沿用的决定和下游工作，直接调用 submit_handoff，target_stage="
            + target
            + "。"
            "可引用已有真实文档；没有文档时允许 artifacts=[]，不可为凑附件新建空 PRD、原型、HLD 或不适用报告。"
            "这个无新增工作分支不需要再次询问是否进入下一阶段，严格模式也不为纯转交增加形式确认；"
            "不能伪称用户批准或本阶段新增工作已完成。"
            "若有新需求取舍、架构/接口/数据/权限/部署行为变化，则只完成相关的澄清或设计，"
            "按风险提供必要产物，按 Runner 阶段确认策略处理实际新增决定。"
            "原型只在需要验证交互时制作；没有 UI 设计工作就不要求原型。"
        )
    else:
        common += (
            "上游可通过摘要确认本阶段不适用；依据 Issue 与有效交接约束继续实现/验证，"
            "不得仅因缺少 PRD、原型或 HLD 拒绝已明确任务，也不要求上游补形式文档。"
            "必须执行变更影响范围内的测试与既有门禁；无 UI 变化不新增原型或无关界面专项，"
            "但不能删减已承诺验收、已有回归、独立 QA 或代码审查。QA 仍须提供 qa.md 和真实验证证据。"
        )
    return common + (
        "目标/验收不足或相互冲突不能标为不适用，应按确认策略澄清；P0 风险、权限审批、数据迁移审批不得借此跳过。"
        "流程仍依次交接下一阶段，不跨节点、不伪造产物或通过状态。"
    )


def update_stage_instructions(instructions, stage):
    policy = stage_applicability(stage)
    if not policy:
        return instructions
    if _POLICY_START in instructions:
        before, remaining = instructions.split(_POLICY_START, 1)
        if _POLICY_END not in remaining:
            raise ValueError(
                "Incomplete stage applicability policy; inspect instructions"
            )
        _, after = remaining.split(_POLICY_END, 1)
        instructions = before.rstrip() + after
    return (
        instructions.rstrip()
        + "\n\n"
        + _POLICY_START
        + "\n"
        + policy
        + "\n"
        + _POLICY_END
    )


def approval_policy(stage, autonomous=False, integrating=False) -> str:
    """Append the effective confirmation rules after the stage work requirements."""
    policy = _resolve_policy(stage, autonomous, integrating)
    readiness = {
        "requirements": "目标、范围和可验证验收已明确；有新增需求工作时按规模补充必要需求文档并自查，否则沿用 Issue 和已有基线",
        "design": "有新增设计工作时完成相关方案及验证，按影响提供必要契约或 HLD，交互设计需要时才做原型；否则沿用已有设计",
        "development": "完成已接受设计的实现，运行所需功能测试、格式/lint检查和真实浏览器验证，修复阻断问题并展示可运行成果",
        "qa": "独立验证 Issue、有效需求/设计依据和实际交付，完成所有必需验收、回归和真实浏览器检查，产出 qa.md、报告和必要截图",
    }
    lines = [
        "\n阶段确认策略（以本段决定是否需要普通阶段确认；工作内容和验证要求继续适用）："
    ]
    if policy.integration_only:
        lines.append(
            "当前为已 ready PR 的集成修复。授权仅覆盖同步主线、解决冲突、保持已接受产品行为的修复和独立 QA 重跑。"
            "不得扩大已接受产品范围；上游需求或设计取舍无法由既有决策确定时，提出明确问题并等待用户，不能猜测。"
        )
    elif autonomous:
        lines.append(
            "Issue 已选择自主推进；这是工作方式授权，不代表用户已审查或批准具体产物。"
        )
    else:
        lines.append(
            "严格模式：保留用户的阶段确认，不得把沉默、自查通过或旧阶段批准视作本阶段批准。"
        )

    if policy.automatic_clarification:
        lines.append(
            "普通澄清、可逆实现选择和非阻断不确定项，依据证据采用推荐方案与可逆默认，记录假设、依据及影响后继续；"
            "不要为形式审批暂停，也不要重复询问已接受决定。"
        )
    else:
        lines.append(
            "需要用户决定的澄清应明确推荐方案和影响，取得答复后再推进依赖该决定的工作。"
        )

    lines.append("本阶段就绪条件：" + readiness[stage] + "。")
    lines.append(
        "验收条款注明来源（用户明确要求、项目既有约束、Agent 推荐默认）、验证方法和负责阶段。"
        "新增推荐默认应与变更风险相称，先确认验证方法可执行；不要把工具缺口留到下游才发现。"
        "研发就绪与最终 QA 放行分别判断：已明确归属 QA 的检查可交给 QA 执行，但研发自己负责的必需检查不能跳过。"
        "已接受条款不能因为工具不支持就删除、降低或写成通过。"
        "遇到验证能力缺口，先查看 check 的能力并自行补充任务级验证：可在既有执行权限内编写测试，"
        "浏览器自定义断言写入 docs/05-validation/tasks/<issue>/browser-scripts/*.js，通过 check 的 page_script 执行。"
        "原生缩放用 zoom，可访问性树用 accessibility；CSS 放大不替代原生缩放，树快照不冒充读屏器实测。"
        "补充脚本不替代已有质量门禁，不修改共享 Harness、受保护配置或验收条款来制造通过。"
        "仍缺权限、设备或接口时，记录已尝试的方法、缺口及最小补齐方案，按现有交接策略退回负责该决定的阶段；"
        "不要只反复回复环境不支持，也不要把同一缺口原样交给同样没有能力的 QA。"
    )
    if policy.automatic_handoff:
        lines.append(
            "满足就绪条件且所需检查通过后，简要展示成果、验证证据和剩余非阻断问题，"
            f"自动调用 submit_handoff，target_stage={policy.target_stage}，无需再询问是否进入下一阶段。"
        )
    else:
        lines.append(
            "满足就绪条件后展示成果、验证结果和未解决问题，请用户明确确认本次交接；"
            f"只有确认后才能调用 submit_handoff，target_stage={policy.target_stage}。"
        )
    if stage == "development":
        lines.append("研发交接只启动独立 QA，不直接创建 PR。")
    if stage == "qa":
        lines.append(
            "所有 QA 交接 artifacts 必须包含 qa.md；成功交给 report，由交付流程处理 PR，不能直接合入主线。创建新 PR 前，根据已接受的需求、实际代码差异和本轮验证证据，编写 docs/05-validation/tasks/<issue>/pull-request.md 并加入 artifacts。第一行使用 # 加具体功能标题，不得只写实现 Issue 编号；正文必须包含二级标题：背景、实现内容、验证结果、风险与限制、界面效果。背景说明用户问题，功能说明实际行为和关键规则；验证区分通过、失败和未测，并附真实证据；UI 变化附可访问的截图/预览链接，无 UI 变化写不适用及原因。链接使用当前仓库和任务分支的 GitHub URL，不使用本机绝对路径；不要把计划功能写成已实现，不虚构测试通过，不要求用户再次审批文案。已有 PR 保留原文，不因这个新文档要求阻塞旧任务。"
        )

    if policy.automatic_rework:
        lines.append(
            "普通上游返工无需另行审批：按缺陷和验证证据选择更早的目标阶段，调用 submit_handoff 并记录原因、修改要求、"
            "受影响结论和交接文档；接收方按既有决策和证据继续修复。遇到下述阻断条件则先等待用户。"
        )
    else:
        lines.append(
            "发现上游问题时说明回退目标和影响，先取得用户确认；QA 已证实产品缺陷仍可按既有规则自主回退。"
        )

    lines.append(
        "必须等待用户的阻断条件：目标无法判断、互斥目标无法取舍，或 P0 级重大不可逆数据损失、"
        "安全/隐私风险、重大外部财务承诺。只暂停依赖该决定的动作，并给出具体问题、证据和推荐方案。"
        "自主推进不豁免任何测试或验收：失败、未运行、缺证据均不得写为通过，也不能交付虚假完成状态。"
        "记录自主决策时标明依据，不得伪造‘用户已确认/批准’。最终 PR 合并和高风险部署始终保留人工授权。"
    )
    lines.append(stage_applicability(stage))
    return "\n".join(lines) + "\n"
