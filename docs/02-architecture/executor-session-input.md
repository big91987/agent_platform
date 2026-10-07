# Codex 执行器会话输入映射

依据：[工作流角色指令与会话输入](workflow-session-input.md)。本说明覆盖当前 Codex 执行器，不声明其他执行器已验证。

## 角色与项目规则

`Agent.instructions` 是角色职责。`nativeConfig` 将其追加到 Agent 原生配置中的 `developer_instructions`；已有 developer 指令在前，角色职责在后。默认托管工作区与外部工作区使用同一映射，不因工作区来源改变指令通道。

托管工作区通过现有 seed 复制流程保留 `AGENTS.md` 原始内容。外部工作区直接使用已授权工作目录。平台不创建角色用的 `AGENTS.md`，不改写已有项目规则，也不手工读取项目规则后再次注入。Codex 按原生工作目录规则发现项目指令。

已有 app-server 通道继续从会话 native 配置读取 developer 指令，经 `thread/start` 或 `thread/resume` 的 `developerInstructions` 传递，并保留原有平台回复格式和只读访问提示。当前任务、节点 Session Prompt、交接与用户修正由消息输入传递；不写回角色或项目规则。注册工具沿用已有配置和授权通道。

## 准备、恢复与兼容

新会话准备仍通过暂存目录生成 native 配置、复制 seed、核验 Skill 范围后发布。`prepared` 标记存在时保留已准备状态；本次修复不改写旧会话的配置、原生历史或工作区，不迁移已冻结 Run。此前已准备的托管工作区可能仍含旧版本追加的角色内容；它们按旧快照继续，新映射在新准备的会话生效。

继续会话仍要求原 native session record 存在；缺失记录不会偷偷变成新会话。外部工作目录必须保持已授权的路径和可访问状态。没有新增执行器适配器，也没有增加全局项目规则副本或本机隐含配置。

## 验证边界

`TestPreparationKeepsProjectRulesSeparateFromRole` 调用真实准备入口，覆盖托管与外部工作区、项目规则存在与不存在、重复准备。它断言项目规则字节保持原样（不存在时不创建），并解码会话 TOML 验证既有 developer 指令和角色共同保留。认证使用临时空 fixture；仅原生 Skill 发现子进程用协议 fixture 隔离。

现有外部工作区执行测试继续覆盖 native CWD、项目规则保留、恢复和目录失效处理。上述测试证明平台准备和传递边界；Codex 实际发现项目规则及角色同时生效，须由真实原生执行路径验收并记录在交付验证中。
