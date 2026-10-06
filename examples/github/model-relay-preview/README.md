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

### 隔离发布验收

Actions 的 `target` 默认 `preview`；push main 仍只准备预览，显式 `deploy=true` 才激活。需要验证升级和失败恢复时，用相同固定版本安装器另装一个私有根，选择不同端口；配置 `MODEL_RELAY_VALIDATION_ROOT`、`MODEL_RELAY_VALIDATION_URL`，创建限制 main 的 `local-validation` Environment。管理员通过同一 Workflow 选择 `target=validation`。验证目标使用同一完整测试、精确 main、契约和激活控制器，不接受用户输入的目录、SHA 或任意命令；两目标共用仓库部署并发锁，仍要求 Owner 显式部署。

标准安装器输出的是默认预览变量名，安装隔离根时将其根和 URL 填入上述 VALIDATION 变量，保留原 PREVIEW 变量。先按原参数升级隔离根控制器，再通过 `install_workflow.py --upgrade` 将模板和摘要纳入产品 PR。已有只传 deploy 的调用兼容，未配置隔离根时选择 validation 明确失败，不回退到预览。此配置不会初始化旧版本、生成测试数据或自动执行故障；真实版本和数据准备必须另按被验产品的正式安装/页面路径完成并记录。

隔离选择向两阶段控制器传 `--protected-root <preview-root>`。执行前要求两个已安装根不相同、不嵌套、解析软链接后不重叠，仓库相同而端口和服务名不同；current/repository/releases不能指向根外；数据、密钥、备份、日志、计划和恢复记录的状态树拒绝软链接，并检查两根文件 inode 不共享，防止复制目录后残留数据库硬链接。它防止配置误指向预览，不作为同机恶意管理员的隔离沙箱。各自独立的密钥、备份、部署记录与服务生命周期不能人工串接。缺少配置/安装或检查失败时保持失败，不降级成默认目标。

GitHub Deployment 按 local-preview/local-validation 分别记录；验收报告必须明确环境，隔离成功不替代正式预览发布，隔离故障不代表预览失败。故障演练必须在已明确授权且核对独立的根上执行，记录注入边界、旧/新版本和数据守恒；不增加产品故障开关、修改产品权限或编辑数据库/activation 记录。本模板只提供受支持的隔离入口，真实恢复仍需执行后才可写通过。

#### 真实失败恢复演练

仅在上面的隔离目标配置、非空旧安装与已通过产品QA的候选就绪后执行。先保留旧版本、实际二进制、页面创建的对象/身份/状态/历史以及独立密钥的私有对账基线；候选必须是正式合并后的完整main SHA。运行维护源中的有界端口冲突夹具，再通过原Actions显式选择main、target=validation、deploy=true：

```sh
python3 examples/github/model-relay-preview/tests/port_conflict.py \
  --root <private-validation-root> \
  --protected-root <private-preview-root> \
  --candidate-sha <full-main-sha>
```

该命令先核对已安装控制器与本维护源摘要一致、旧服务健康、两根隔离且没有在途activation；只读观察新目标轮次的backup_complete。随后仅绑定已释放的loopback端口，health请求返回明确标记的503夹具响应，使实际候选服务启动/健康失败。观察upgrade_confirmed后，从首个GET /healthz请求起保持25秒，再释放端口，为控制器当前20秒健康等待及随后20秒停服等待留出自动恢复窗口。它不写activation、数据、备份、计划或部署回执，不修改产品二进制、权限或服务配置。未观察到目标窗口、端口已占用、身份变化、升级未确认或没有健康请求，都返回不完整；不会结束其他进程或重试到一个看似成功的场景。等待启动默认最多1800秒，可在1～5400秒内显式设置arm-timeout；升级未确认最多等待130秒，确认后等待首个健康请求最多15秒。每连接只读一次、最多4096字节，读写各限0.1秒，不等待完整请求头、不记录请求内容；计时结束或异常退出关闭自身socket。普通端口探针和升级确认前的请求不启动25秒计时。

夹具JSON的evidence_kind固定为fault-injection，recovery固定not_checked；exit0只说明故障窗口发生过，不证明产品恢复。必须另核对该次正式Actions部署失败、controller的rolled_back/原deployed记录、旧binary和实际健康/schema、原对象/密钥/历史守恒及failed-data隔离。任何一层缺失都不写恢复Pass。若控制器报告恢复未完成，保留数据与备份，待夹具释放后仍沿原显式Actions入口恢复，不编辑记录或手动覆盖数据。确认旧版本恢复后，再次显式Actions无夹具发布候选，验证正常升级与可用性；失败和成功两次证据分别保存。该夹具模拟基础设施端口冲突，不代替backup/upgrade/restore自身错误和重启恢复的其他用例。

首次部署通过项目正式 `init` 创建数据目录和独立主密钥，初始管理员密码仅写入部署根 `private/initial-admin-password.txt`（0600），不进入 GitHub 日志。运维者在主机安全读取并保管，登录 `MODEL_RELAY_PREVIEW_URL`。不配置默认供应商或假上游；真实供应商 URL/凭据由管理员之后从正式网页录入，未经供应商响应不能声称联调通过。

候选必须提供只读 `deployment-contract` 命令，返回不超过 4 KiB 的单一 JSON。控制器协议版本1接受明确的 schema1、schema2 或 schema3 能力：`contract_version=1`、`binary_version=<full-main-sha>`，以及storage中的 `init_schema`、`serve_schemas`、`upgrade_from`、`explicit_upgrade`、`backup_schemas`、`restore_schemas`。schema2候选只serve2，声明从1升级、backup/restore兼容1和2；schema3候选只serve3，声明从1/2升级、backup/restore兼容1/2/3，保持同一显式CLI协议。未知schema仍拒绝，不能靠放宽任意版本绕过发布验证。prepare将完整契约、控制器协议版本与二进制摘要绑定到计划，activate停服前重新读取比较；未知版本、类型错误、能力矛盾、乱码、过量输出、失败或漂移均拒绝。JSON不允许传递任意可执行命令。已有旧控制器的 `deployed.json` 未包含契约时，只兼容其记录的原SHA、原二进制摘要及实际current指针均不变的schema1旧发布；任意新无契约候选不能借命令失败进入兼容路径。新安装需要带契约的产品版本，较新schema不能自动降级到旧schema。

后续部署在私有部署锁内先停止旧服务，由旧二进制正式backup生成新的权威回退快照并记录摘要。schema2/3候选随后执行固定命令 `upgrade --data-dir <installation> --key-file <existing-key> --output <distinct-new-upgrade-backup>`；产品内部迁移前备份与控制器快照分开。只接受退出0且版本正确的 `migrated`（1→2、1/2→3）或 `already_current`（2→2、3→3）结果，再切换current、启动候选并核对实际healthz的version及整数storage_schema。首次init直接创建候选schema；未登记却已存在的数据、密钥或初始密码文件保留并拒绝，不猜测归属或直接启动。

迁移、启动或健康失败时，先确认进程退出/端口关闭，隔离失败数据，用旧二进制正式restore原回退快照到同一私有根内的空暂存目录；restore成功后原子移动到正式data路径，再切回旧版本并核对healthz。部分恢复目录保留，不覆盖或宣称可用。备份包含加密凭据，主密钥单独保存并校验摘要；恢复不混并候选健康窗口内的新增记录，它们保留在failed-data中。恢复失败明确报告“recovery incomplete”，不写成功部署记录。

`activation.json`保存目标/旧版本、二进制与回退备份摘要及必要执行阶段，不包含主密钥、密码或产品数据库内容。进程中断后的prepare只安排恢复，不停止服务或重放迁移；获授权维护者沿原Actions入口显式deploy，activate先恢复原快照并以失败结果报告“interrupted activation recovered”。确认旧版本恢复后，再显式运行一次才开始新发布。即使main期间变化，也不以旧二进制重新备份不确定的新数据；不手改阶段记录或业务库。若新deployed记录已落盘但最后committed标记中断，prepare/activate仅在目标记录、二进制、current指针与实际version/schema健康完全一致时补齐提交；不停止健康候选或丢弃已产生的新记录。不健康候选仍沿原快照恢复。相同SHA只有已提交且实际健康一致才是无副作用重入。

产品CLI由有限时长的监督子进程执行并继承部署锁。控制器父进程被终止时，监督者继续限时收尾；未确认产品进程退出则保留现场，后续恢复必须重新取得同一部署锁。契约读取15秒、backup/init60秒、upgrade/restore120秒，单次服务等待20秒，并为10分钟部署作业留出恢复预算。产品stdout/stderr各限制4 KiB，错误不向Actions打印原始产品输出；初始密码只写私有文件。Git fetch与launchctl也有限时。该边界支持受信同机产品CLI，不保证机器断电期间的可用性；再次运行同一正式入口处理保留的中断记录。

维护源回归可通过 `python3 -m unittest discover -s examples/github/model-relay-preview/tests -p '*_test.py' -v` 运行。它涵盖真实Git、子进程/超时/父进程死亡、安装器、契约漂移、旧协议和数据恢复夹具；测试里的launchctl及服务健康主要为隔离替身。必须另行验证真实新旧Go二进制、标准安装、原Actions的非空旧安装升级/失败恢复和部署后浏览器，不能据该回归称整条Pipeline通过。

验收时分别核对：平台 Run 的 PR head → 获授权合并后的 main SHA → Actions 自动准备、显式触发部署与部署记录 → 部署根 `deployed.json` 的 SHA 与二进制摘要 → 固定 URL 的真实 `/healthz` 和管理页。临时手工服务、`make verify` 中的短命服务、草稿 PR 本身均不算部署成功。预览仅本机可访问；主机睡眠或退出登录后不可用。产品真实供应商联调独立标记。


### 真实 Go 与控制器联合回归

`tests/real_product_lifecycle.py` 是显式运行的跨版本回归入口，不加入快速单元测试的自动发现。提供受信产品 Git 工作副本、完整旧/新 SHA、历史平台控制器和一个不存在的私有工作目录；该工具只读克隆 Git 对象，使用各版本未修改的 `make verify` 后构建精确版本制品。旧版安装由原历史控制器的 prepare/activate 生成，配置/密钥及一条拒绝请求历史经真实管理/调用 API 产生；不读取或复制常驻实例数据，不手写数据库、计划或成功部署记录。

```sh
git show <legacy-platform-ref>:examples/github/model-relay-preview/controller.py \
  > <private-temp>/legacy-controller.py
python3 examples/github/model-relay-preview/tests/real_product_lifecycle.py \
  --project <trusted-product-repository> \
  --old-sha <full-schema1-product-sha> \
  --candidate-sha <full-schema2-product-sha> \
  --legacy-controller <private-temp>/legacy-controller.py \
  --work <new-private-evidence-directory>
```

运行会执行两份产品的完整门禁，所需依赖同正式部署准备；不会跳过测试或消费已有安装的制品记录。仅将 launchd 换为受控子进程，产品 serve、CLI、HTTP健康、SQLite、备份/恢复及控制器状态机都实际执行。API准备的临时数据不能算UI旅程；本层不覆盖真实launchd、GitHub权限/Actions、用户页面或供应商。正式环境仍按前述独立Actions步骤验收。

场景按同一安装连续执行：已知旧无契约版本及重复准备；真实backup/upgrade对已存在输出拒绝后的旧数据恢复；真实upgrade读取阻塞后的限时终止与旧数据恢复；候选真实端口占用与真实restore对非空目标拒绝、明确恢复未完成、再次显式activate恢复原快照；在控制器实际持久化upgrade_confirmed后注入中断，下一次prepare仅安排恢复、activate不能再用旧binary备份新schema；正常升级保留身份/模型启停/历史；相同SHA无停服或产品命令副作用；schema2拒绝无契约旧候选。输出碰撞和中断均为标注故障夹具，不改产品二进制/数据库或构造回执。该集合不声称覆盖所有可能的超时、断电、契约畸形或安装漂移边缘。

工作目录保留脱敏证据JSON、完整验证日志及临时安装以便诊断，目录/文件受私有umask保护，含测试密钥和管理员凭据，不提交到仓库。工具退出时停止自己创建的服务；存在的工作目录始终拒绝复用，防止覆盖失败证据。失败或未完成不能计Pass。


schema3 发布前置：控制器接受该已知profile仅证明协议支持，不证明产品数据库迁移正确。先在维护源完成控制器回归，经原 `install.py` 升级可信安装；有未闭合activation时保留原控制器先恢复。现有schema1/2服务保持，Workflow未变时不生成新副本。schema3产品必须另经自身固定门禁/独立QA、真实1/2→3 CLI及控制器联合验证、原Actions隔离非空升级和失败恢复，最后正式preview；保留旧密钥/授权/计量起点的真实证据。源控制器版本/摘要与产品SHA分别记录，未得到真实产品制品前该联调为Not Run。schema1→2专项回归入口仍只测试其显式声明的旧范围，不冒充schema3验证。


读取超时场景仅将该次隔离upgrade的key-file参数指向新建空FIFO，持有写端使真实Go读取阻塞，原密钥文件与数据库不变；监督器测试截止缩短至1秒。先确认真实读端出现、达到截止，并在仍持有写端时观察EPIPE证明读端已退出，再由原控制器执行旧restore及API对账。监督器或夹具线程退出不确定始终保留ProductStillRunning，禁止转成普通失败后冒险恢复。它只证明受控读取阻塞的超时/恢复边界，不是磁盘设备故障、迁移写入中途超时、OS父进程强杀或断电验收。
