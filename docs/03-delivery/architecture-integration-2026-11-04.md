# 架构检查接入与初检

维护源是 examples/arch-template，现有项目通过 scaffold.py --install-tool 接入，不复制产品样板覆盖现有代码。工具版本 0.3.0；架构规则、模型评审和产品验收分别记录。本轮仅启用确定规则，没有调用模型服务。

## 当前规则

| 仓库 | 声明范围 | 当前边界 | 初检 |
| --- | --- | --- | --- |
| Agent Platform | cmd、internal、web 的 Go 与 JavaScript 生产代码 | 启动组装 → platform/web；platform → web；web 不反向依赖后端 | 74个源文件，0条依赖违规 |
| Model Relay | cmd、internal 的 Go 与 JavaScript 生产代码 | 启动 → relay；relay 与控制台源码分别归属 | 29个源文件，0条依赖违规 |

排除的测试及资源文件逐项保留在 JSON；两仓检查都不包含私有运行目录或 vendored Harness 源码。平台排除77件测试/浏览器设施、资源4件；Model Relay排除35件测试、资源4件。该范围不是全仓所有语言覆盖。

Model Relay 的26个Go源文件仍集中于启动和 relay；控制台3个JavaScript文件没有 feature 分包。平台63个Go文件与11个JavaScript文件有顶层边界，但 internal/platform 仍是集中业务包。本轮声明真实现状，不以文件名伪造 Application/Domain 分层。后续领域拆分必须在规范、代码、测试和 architecture.json 中同步收紧；当前通过只能说明这些已声明的依赖规则成立。

## 使用与证据

平台运行 `python3 -I examples/arch-template/tool/inspect_project.py`；Model Relay 运行已安装的 `.harness/archcheck/inspect_project.py`。CI、固定验证和平台节点使用同一入口；JSON包含完整源码事实与摘要，Markdown供人阅读，每次独立保留。

固定测试成功后可插入架构节点，再进入现有审查与独立QA；质量违规与工具/配置失败走不同出口。报告作为固定普通文件进入步骤归档，不能将目录名当成平台文件附件。工具/配置失败停在明确的阻塞处理节点，保留报告后人工重试。

初次平台扫描发现 macOS 下 Node 对大管道的同步读取触发 EAGAIN；百万字节回归已复现并在维护源修复为私有普通文件 stdin。工具清单损坏、额外源码和字节码替换应返回 incomplete；不能返回 violations 让研发盲修产品。

实际安装、运行、GitHub回执和最终验证将在本文件补充；本段初检不代表已部署、已合并或完整产品QA。
