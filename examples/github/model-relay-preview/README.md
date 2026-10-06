# Model Relay 本机持续预览接入

这是 Go/SQLite Model Relay 测试项目的正式部署示例，不是 Agent Platform 内核节点，也不自动合并产品 PR。获授权的维护者在平台运行页取得草稿 PR，核对项目测试与独立 QA 后合并到 `main`；GitHub Actions 的独立部署 Workflow 从精确的 main SHA 构建并运行项目 `make verify`。获授权的维护者通过 GitHub Actions 显式运行该 Workflow、设置 `deploy=true` 后才切换本机服务，再核对 `/healthz`。失败保留或恢复上一个版本。静态网页的旧 `local-preview` 控制器不能用于此 Go/SQLite 项目。

在目标 Mac 上，从固定版本的平台维护源安装可信控制器。要求 Git、Go/C 工具链、Node/npm、Playwright Chromium、Python 3.11+，以及有读取目标私有仓库权限的 Git 配置。若 Git 访问需要代理，安装时传 `--git-proxy <proxy-url>`。持续服务由该用户的 launchd 会话运行，仅绑定 loopback。使用独立私有目录和未占用端口；不要复用 Agent 平台、旧静态预览或临时试看的端口。

```sh
python3 examples/github/model-relay-preview/install.py \
  --repository <owner>/model-relay \
  --root <private-preview-root> \
  --port <unused-loopback-port>
```

安装不部署产品，也不创建管理员密码。控制器复制到私有目录，项目升级该控制器时以相同参数重跑安装器；正在执行部署时文件锁阻止覆盖；若上次部署中断且阶段记录未闭合，先用原控制器从同一显式 Actions 入口恢复，再升级。安装器先检查既有服务配置，拒绝覆盖人工改过的 LaunchAgent，安装后以 `controller-install.json` 记录控制器 SHA-256；新装与升级使用同一来源。目标仓库通过受支持入口安装 Workflow，并把生成的 YAML 与版本摘要提交到产品 PR：

```sh
python3 examples/github/model-relay-preview/install_workflow.py --project <model-relay-checkout>
# 后续从新版维护源升级时加 --upgrade；手工改过目标文件会明确拒绝覆盖。
```

在仓库 Actions variables 配置安装器打印的 `MODEL_RELAY_PREVIEW_ROOT` 和 `MODEL_RELAY_PREVIEW_URL`。创建 `local-preview` Environment 并限制 `main`。私有仓库套餐可能不支持 Required reviewer；因此本 Workflow 不依赖该规则，`push main` 只准备，不执行部署。只有以仓库 Owner 身份通过 Actions 页面或正式 workflow_dispatch API 明确选择 `main` 并设置 `deploy=true`，才会执行切换。操作可以由用户本人或已有明确授权的维护者完成；自动化维护者应沿用已有授权，不额外要求用户重复点击。授权不改变 Owner 校验、main 限制、测试门禁或回执核验要求，也不将测试仓授权扩大到平台源仓。目标仓库还需能访问受信的同机自托管 Runner；已有可访问且满足标签的 Runner 可直接复用。若现有 Runner 只注册在另一个个人仓库，应先确认空闲、原仓库不再依赖它并取得所有者授权，再按 GitHub 官方流程从旧仓库注销、注册到目标仓库；标签相同不等于跨仓库共享。Runner 允许仓库 Workflow 在主机执行代码。Runner 需具有 `self-hosted`、`macOS`、`ARM64`、`he-full` 标签和上述构建工具，并能读取仓库。不能让普通 PR 分支取得部署权限。

合并后 `push main` 自动准备：只接受当前远端 main 的完整 SHA。Workflow 在准备与手动切换作业中，把各自的 `github.token`（`contents: read`）仅交给可信控制器用于私有仓库 `git fetch` 和切换前的 main 复核；控制器随即移除令牌，不把它传入项目 `make verify`、构建或服务命令，也不依赖 Runner 的个人 Git 登录状态。在隔离工作树运行项目固定 `make verify` 并保存私有日志。准备通过后仍服务旧版。获授权的维护者在仓库 Actions → Deploy Model Relay locally → Run workflow 选择 `main`、勾选 `deploy`，或通过正式 workflow_dispatch API 提交同样的输入；新运行重新验证当前 main 后执行切换。部署时再次检查 main 未变化、计划与二进制摘要不变；若有新提交，旧计划拒绝切换。重复运行已部署 SHA 仍核对实际二进制、指针和 healthz 的版本/schema，成立后才跳过，不生成新版本。未勾选 `deploy` 的手动运行也只准备。

首次部署通过项目正式 `init` 创建数据目录和独立主密钥，初始管理员密码仅写入部署根 `private/initial-admin-password.txt`（0600），不进入 GitHub 日志。运维者在主机安全读取并保管，登录 `MODEL_RELAY_PREVIEW_URL`。不配置默认供应商或假上游；真实供应商 URL/凭据由管理员之后从正式网页录入，未经供应商响应不能声称联调通过。

候选必须提供只读 `deployment-contract` 命令，返回不超过 4 KiB 的单一 JSON。控制器协议版本1接受明确的 schema1 或 schema2 能力：`contract_version=1`、`binary_version=<full-main-sha>`，以及storage中的 `init_schema`、`serve_schemas`、`upgrade_from`、`explicit_upgrade`、`backup_schemas`、`restore_schemas`。schema2候选只serve2，声明从1升级、backup/restore兼容1和2。prepare将完整契约、控制器协议版本与二进制摘要绑定到计划，activate停服前重新读取比较；未知版本、类型错误、能力矛盾、乱码、过量输出、失败或漂移均拒绝。JSON不允许传递任意可执行命令。已有旧控制器的 `deployed.json` 未包含契约时，只兼容其记录的原SHA、原二进制摘要及实际current指针均不变的schema1旧发布；任意新无契约候选不能借命令失败进入兼容路径。新安装需要带契约的产品版本，schema2不能自动降级到schema1。

后续部署在私有部署锁内先停止旧服务，由旧二进制正式backup生成新的权威回退快照并记录摘要。schema2候选随后执行固定命令 `upgrade --data-dir <installation> --key-file <existing-key> --output <distinct-new-upgrade-backup>`；产品内部迁移前备份与控制器快照分开。只接受退出0且版本正确的 `migrated`（1→2）或 `already_current`（2→2）结果，再切换current、启动候选并核对实际healthz的version及整数storage_schema。首次init直接创建候选schema；未登记却已存在的数据、密钥或初始密码文件保留并拒绝，不猜测归属或直接启动。

迁移、启动或健康失败时，先确认进程退出/端口关闭，隔离失败数据，用旧二进制正式restore原回退快照到同一私有根内的空暂存目录；restore成功后原子移动到正式data路径，再切回旧版本并核对healthz。部分恢复目录保留，不覆盖或宣称可用。备份包含加密凭据，主密钥单独保存并校验摘要；恢复不混并候选健康窗口内的新增记录，它们保留在failed-data中。恢复失败明确报告“recovery incomplete”，不写成功部署记录。

`activation.json`保存目标/旧版本、二进制与回退备份摘要及必要执行阶段，不包含主密钥、密码或产品数据库内容。进程中断后的prepare只安排恢复，不停止服务或重放迁移；获授权维护者沿原Actions入口显式deploy，activate先恢复原快照并以失败结果报告“interrupted activation recovered”。确认旧版本恢复后，再显式运行一次才开始新发布。即使main期间变化，也不以旧二进制重新备份不确定的新数据；不手改阶段记录或业务库。相同SHA只有已提交且实际健康一致才是无副作用重入。

产品CLI由有限时长的监督子进程执行并继承部署锁。控制器父进程被终止时，监督者继续限时收尾；未确认产品进程退出则保留现场，后续恢复必须重新取得同一部署锁。契约读取15秒、backup/init60秒、upgrade/restore120秒，单次服务等待20秒，并为10分钟部署作业留出恢复预算。产品stdout/stderr各限制4 KiB，错误不向Actions打印原始产品输出；初始密码只写私有文件。Git fetch与launchctl也有限时。该边界支持受信同机产品CLI，不保证机器断电期间的可用性；再次运行同一正式入口处理保留的中断记录。

维护源回归可通过 `python3 -m unittest discover -s examples/github/model-relay-preview/tests -p '*_test.py' -v` 运行。它涵盖真实Git、子进程/超时/父进程死亡、安装器、契约漂移、旧协议和数据恢复夹具；测试里的launchctl及服务健康主要为隔离替身。必须另行验证真实新旧Go二进制、标准安装、原Actions的非空旧安装升级/失败恢复和部署后浏览器，不能据该回归称整条Pipeline通过。

验收时分别核对：平台 Run 的 PR head → 获授权合并后的 main SHA → Actions 自动准备、显式触发部署与部署记录 → 部署根 `deployed.json` 的 SHA 与二进制摘要 → 固定 URL 的真实 `/healthz` 和管理页。临时手工服务、`make verify` 中的短命服务、草稿 PR 本身均不算部署成功。预览仅本机可访问；主机睡眠或退出登录后不可用。产品真实供应商联调独立标记。
