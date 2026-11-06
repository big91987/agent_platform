# 架构检查接入与初检

2026-11-05更新：[DeepSeek真实扫描、规则/模型职责和0.4.0修订记录](architecture-reports/model-relay/2026-11-05-deepseek/README.md)。最终工具结果与历史0.3.1初检分别保留；当前模型报告11项齐全、7项待复核发现，规则仍为0违规。

维护源是 examples/arch-template，现有项目通过 scaffold.py --install-tool 接入，不复制产品样板覆盖现有代码。工具版本 0.3.1；架构规则、模型评审和产品验收分别记录。本轮仅启用确定规则，没有调用模型服务。

**Model Relay 的 Template 符合度结论为不符合。** 下文的0条依赖违规仅针对现状声明，不能作为架构 Template 验收结果。逐项要求、代码偏离和检查覆盖缺口见[Model Relay 符合度核查](model-relay-template-conformance-2026-11-04.md)。

## 当前规则

| 仓库 | 声明范围 | 当前边界 | 初检 |
| --- | --- | --- | --- |
| Agent Platform | cmd、internal、web 的 Go 与 JavaScript 生产代码 | 启动组装 → platform/web；platform → web；web 不反向依赖后端 | 74个源文件，0条依赖违规 |
| Model Relay | cmd、internal 的 Go 与 JavaScript 生产代码 | 启动 → relay；relay 与控制台源码分别归属 | 29个源文件，0条依赖违规 |

排除的测试及资源文件逐项保留在 JSON；两仓检查都不包含私有运行目录或 vendored Harness 源码。平台排除77件测试/浏览器设施、资源4件；Model Relay排除35件测试、资源4件。该范围不是全仓所有语言覆盖。

Model Relay 的27个Go源文件包含1个启动文件和26个relay文件；控制台2个JavaScript文件没有 feature 分包。平台63个Go文件与11个JavaScript文件有顶层边界，但 internal/platform 仍是集中业务包。本轮声明真实现状，不以文件名伪造 Application/Domain 分层。后续领域拆分必须在规范、代码、测试和 architecture.json 中同步收紧；当前通过只能说明这些已声明的依赖规则成立。

## 使用与证据

平台运行 `python3 -I examples/arch-template/tool/inspect_project.py`；Model Relay 运行已安装的 `.harness/archcheck/inspect_project.py`。CI、固定验证和平台节点使用同一入口；JSON包含完整源码事实与摘要，Markdown供人阅读，每次独立保留。

固定测试成功后可插入架构节点，再进入现有审查与独立QA；质量违规与工具/配置失败走不同出口。报告作为固定普通文件进入步骤归档，不能将目录名当成平台文件附件。工具/配置失败停在明确的阻塞处理节点，保留报告后人工重试。

初次平台扫描发现 macOS 下 Node 对大管道的同步读取触发 EAGAIN；百万字节回归已复现并在维护源修复为私有普通文件 stdin。工具清单损坏、额外源码和字节码替换应返回 incomplete；不能返回 violations 让研发盲修产品。

实际安装、运行、GitHub回执和最终验证将在本文件补充；本段初检不代表已部署、已合并或完整产品QA。

## 已验证交付

维护源[PR8](https://github.com/big91987/agent_platform/pull/8)以既有开发分支为基线，测试仓[PR78](https://github.com/big91987/model-relay/pull/78)关联[Issue77](https://github.com/big91987/model-relay/issues/77)，两者均为草稿，未合并或部署。

0.3.1版本33项工具测试、11项安装/工作流回归通过；Go vet与平台归档/恢复定向回归通过，独立评审无遗留P1/P2。Model Relay首次安装、明确升级及24件维护源文件逐字节匹配均已验证，make architecture-check实际通过。

维护源精确head0308f6e的[GitHub CI](https://github.com/big91987/agent_platform/actions/runs/41812248327)实际成功，报告附件已下载复核：版本0.3.1、74个生产文件、源码摘要7d8c67b9a3148d526f9a10810bde1ae3a145b37b141fcd09eadf9fcd9b3c7726。报告记录的Git HEAD为PR合并检查点22cf8c289e9029661a09fc24650b3ddeb39be4ed，不能混称源分支提交。

112原研发图revision15及旧Run保持原样。用户明确授权传输源码包后，已通过正常管理员登录及标准安装器创建独立“架构检查 · Model Relay”图，ID为bf86dccc5c0931763a96364b6c545031、revision1；安装重入保持相同ID和版本。新图尚未启动：OpenSandbox控制面访问Docker被宿主机权限拒绝，实际沙箱接口返回500。工作流执行、步骤归档下载及Issue自动状态回写尚未验证。

112宿主机另以Go1.25.6、Node22.14.0执行同版工具和仓库，报告的29个生产文件及源码摘要与本地一致。这属于命令执行验证，同样不能替代Template符合度审查或沙箱工作流验证。固定源码53a2f21的安装包为134947字节，SHA256为bff95f5612b43a56ee8794e1dd9f371a3943aad5639b0665f701b6e0d53aff9c。


后续实测：Model Relay [架构CI](https://github.com/big91987/model-relay/actions/runs/41812694027)在候选d03fe5b上成功。附件已下载核对，报告工具0.3.1、29个文件、源码摘要8868a1cded1f8c735efb31d646883b565393b78787ce199a52b8fec41f7e3ba4；Git HEAD为PR合并检查点8e710c5c2c23b69f5e414ef2020eb08cafb53096，区别于源分支head。

原有PR自动CLI/Delegate审查分别启动Run8ead513642043bba38850c1862c4f646、7f5d01746e85839f4b0c94e7dbd181fc，均在prepare阶段因OpenSandbox创建HTTP500且创建结果unconfirmed而失败，模型审查未执行。保留原Run和环境请求事实，须先核对真实创建回执再通过正式入口恢复；不能盲目重跑或算产品缺陷。已在Issue77写明影响及下一步。
