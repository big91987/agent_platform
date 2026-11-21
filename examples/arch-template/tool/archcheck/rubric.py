"""Fixed assessment points: coverage, not finding count, is the scoring unit."""

CRITERIA = {
    "module_responsibilities": {
        "cohesion": ("职责内聚", "模块围绕同一能力组织，不混入不相关职责。"),
        "interfaces": (
            "接口边界",
            "调用方通过明确的小接口使用模块能力，不穿透内部实现。",
        ),
        "transport": (
            "协议与用例分工",
            "协议编解码与业务用例有清晰边界；网关传输可以保持内聚。",
        ),
        "composition": (
            "装配职责",
            "启动和组合根负责依赖装配，不承载应归属业务能力的规则。",
        ),
    },
    "business_rule_ownership": {
        "owner": (
            "规则归属",
            "规则有可定位的能力和对象负责，不要求简单CRUD引入富领域模型。",
        ),
        "duplication": (
            "规则唯一性",
            "同一业务不变量不在多个实现中重复维护；相似协议分支不等于重复规则。",
        ),
        "policy_io": ("规则与I/O分离", "规则可独立于HTTP、SQL等实现细节表达和修改。"),
        "change_locality": (
            "规则变更范围",
            "规则调整不要求同步修改不相关能力或多份派生策略。",
        ),
    },
    "dependency_direction": {
        "allowed_dependencies": (
            "依赖方向",
            "实际导入遵守已声明的允许、禁止及第三方依赖规则。",
        ),
        "encapsulation": (
            "模块封装",
            "模块不绕过其他模块的公开契约直接访问内部对象或存储。",
        ),
        "technology": (
            "技术依赖隔离",
            "策略与用例不依赖可替换的外部技术实现，必要传输代码按项目架构评价。",
        ),
        "cycles": ("循环依赖", "模块依赖没有循环；不能仅凭同包或同构造函数判定循环。"),
    },
    "state_ownership": {
        "owner": ("状态负责人", "可变状态有明确的能力或对象负责。"),
        "lifetime": ("状态生命周期", "会话、请求和业务状态的作用域及清理责任清楚。"),
        "mutation": (
            "状态修改边界",
            "状态通过负责该状态的接口修改，不被不相关能力任意改写。",
        ),
        "authority": ("状态权威来源", "同一事实不由相互独立的可变副本同时充当权威。"),
    },
    "testability": {
        "policy_seams": (
            "规则验证入口",
            "核心规则可通过明确接口或纯函数验证，不必启动整个系统。",
        ),
        "external_seams": (
            "外部依赖替换",
            "数据库、网络等外部依赖具有适当的隔离或集成验证入口。",
        ),
        "failure_signals": (
            "失败可定位",
            "错误与失败结果保留可定位语义，供验证和排查使用。",
        ),
        "state_control": (
            "验证状态控制",
            "验证所需状态可创建、传入或隔离，不依赖不可控的隐式全局状态。",
        ),
    },
    "necessary_abstractions": {
        "responsibility": (
            "抽象有实际职责",
            "层、接口和共享对象各自承载当前明确责任。",
        ),
        "complexity": (
            "复杂度适当",
            "不为简单用例添加空层、过度泛化或无实际收益的间接调用。",
        ),
        "reuse": ("复用边界", "共享实现围绕真正相同的职责，不把无关对象塞进通用模块。"),
        "variation": (
            "变化点明确",
            "已有的多实现或扩展点对应真实差异，不为假设性需求预建框架。",
        ),
    },
    "standards_alignment": {
        "archived_rules": (
            "规范可定位",
            "所需编码和架构约定有明确归档位置与引用，评审材料可核对。",
        ),
        "coding": (
            "编码规范一致",
            "命名、类型、错误及代码组织与所提供的语言和项目编码规范一致。",
        ),
        "architecture": (
            "架构规范一致",
            "模块、职责及分层实现与所提供的项目架构约定一致。",
        ),
        "rule_consistency": (
            "可执行约束一致",
            "已提供的项目规则和实现口径一致；临时观察配置不当作既有规范扣分。",
        ),
    },
}

RUBRIC = {
    "conformant": {
        "points": 100,
        "description": "该检查点有证据支持符合要求，未发现偏差。",
    },
    "low": {"points": 80, "description": "该检查点存在局部命名、轻量重复或封装偏差。"},
    "medium": {
        "points": 50,
        "description": "该检查点存在具体职责或依赖混合，阻碍独立修改或验证。",
    },
    "high": {
        "points": 0,
        "description": "该检查点的关键边界失效；不代表整个维度失效。",
    },
}

RULE_CRITERIA = {
    "dependency": "allowed_dependencies",
    "forbidden_import": "allowed_dependencies",
    "external_dependency": "allowed_dependencies",
    "private_entry": "encapsulation",
    "cycle": "cycles",
}


def aggregate_status(criteria):
    statuses = {c["status"] for c in criteria}
    if "insufficient_evidence" in statuses:
        return "insufficient_evidence"
    if "deviation" in statuses:
        return "deviation"
    if "conformant" in statuses:
        return "conformant"
    return "not_applicable"
