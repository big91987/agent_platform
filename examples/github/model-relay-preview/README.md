# Model Relay 本机持续预览接入

这是 Go/SQLite Model Relay 测试项目的正式部署示例，不是 Agent Platform 内核节点，也不自动合并产品 PR。用户在平台运行页取得草稿 PR，检查后自行合并到 `main`；GitHub Actions 的独立部署 Workflow 从精确的 main SHA 构建并运行项目 `make verify`。用户在 GitHub Actions 页面手动运行该 Workflow、勾选 `deploy` 后才切换本机服务，再核对 `/healthz`。失败保留或恢复上一个版本。静态网页的旧 `local-preview` 控制器不能用于此 Go/SQLite 项目。

在目标 Mac 上，从固定版本的平台维护源安装可信控制器。要求 Git、Go/C 工具链、Node/npm、Playwright Chromium、Python 3.11+，以及有读取目标私有仓库权限的 Git 配置。若 Git 访问需要代理，安装时传 `--git-proxy <proxy-url>`。持续服务由该用户的 launchd 会话运行，仅绑定 loopback。使用独立私有目录和未占用端口；不要复用 Agent 平台、旧静态预览或临时试看的端口。

```sh
python3 examples/github/model-relay-preview/install.py \
  --repository <owner>/model-relay \
  --root <private-preview-root> \
  --port <unused-loopback-port>
```

安装不部署产品，也不创建管理员密码。控制器复制到私有目录，项目升级该控制器时以相同参数重跑安装器；正在执行部署时文件锁阻止覆盖。目标仓库通过受支持入口安装 Workflow，并把生成的 YAML 与版本摘要提交到产品 PR：

```sh
python3 examples/github/model-relay-preview/install_workflow.py --project <model-relay-checkout>
# 后续从新版维护源升级时加 --upgrade；手工改过目标文件会明确拒绝覆盖。
```

在仓库 Actions variables 配置安装器打印的 `MODEL_RELAY_PREVIEW_ROOT` 和 `MODEL_RELAY_PREVIEW_URL`。创建 `local-preview` Environment 并限制 `main`。私有仓库套餐可能不支持 Required reviewer；因此本 Workflow 不依赖该规则，`push main` 只准备，不执行部署。只有仓库 Owner 在 Actions 页面明确手动选择 `main` 并勾选 `deploy`，才会执行切换。目标仓库还需能访问受信的同机自托管 Runner；已有可访问且满足标签的 Runner 可直接复用。若现有 Runner 只注册在另一个个人仓库，应先确认空闲、原仓库不再依赖它并取得所有者授权，再按 GitHub 官方流程从旧仓库注销、注册到目标仓库；标签相同不等于跨仓库共享。Runner 允许仓库 Workflow 在主机执行代码。Runner 需具有 `self-hosted`、`macOS`、`ARM64`、`he-full` 标签和上述构建工具，并能读取仓库。不能让普通 PR 分支取得部署权限。

合并后 `push main` 自动准备：只接受当前远端 main 的完整 SHA，在隔离工作树运行项目固定 `make verify` 并保存私有日志。准备通过后仍服务旧版。用户在仓库 Actions → Deploy Model Relay locally → Run workflow 选择 `main`、勾选 `deploy`，新运行重新验证当前 main 后执行切换。部署时再次检查 main 未变化、计划与二进制摘要不变；若有新提交，旧计划拒绝切换。重复运行已部署 SHA 不生成新版本。未勾选 `deploy` 的手动运行也只准备。

首次部署通过项目正式 `init` 创建数据目录和独立主密钥，初始管理员密码仅写入部署根 `private/initial-admin-password.txt`（0600），不进入 GitHub 日志。运维者在主机安全读取并保管，登录 `MODEL_RELAY_PREVIEW_URL`。不配置默认供应商或假上游；真实供应商 URL/凭据由管理员之后从正式网页录入，未经供应商响应不能声称联调通过。

后续部署先停止旧服务并用旧二进制正式 `backup`。候选健康失败时，控制器隔离失败数据、用旧二进制正式 `restore` 原备份并重新启动旧版本；备份或恢复失败会明确报错并保留现场。备份包含加密凭据，主密钥单独保存，整个部署根需由同一受信用户独占。升级有迁移或数据契约变化时，审批者必须核对项目发布说明中的兼容与恢复证据；本示例不会自行证明任意 schema 迁移安全。

验收时分别核对：平台 Run 的 PR head → 用户合并后的 main SHA → Actions 自动准备、人工手动触发部署与部署记录 → 部署根 `deployed.json` 的 SHA 与二进制摘要 → 固定 URL 的真实 `/healthz` 和管理页。临时手工服务、`make verify` 中的短命服务、草稿 PR 本身均不算部署成功。预览仅本机可访问；主机睡眠或退出登录后不可用。产品真实供应商联调独立标记。
