# 独立 Linux 平台实例

使用经标准验证的源码快照，在Linux上构建CGO版本。SQLite需要CGO；
不能把无CGO的探针构建方式直接用于平台服务。构建固定Go1.25.6，使用
项目go.sum校验依赖，并保留源码清单、模块校验、工具链、构建日志和二进制摘要。

```sh
bash deploy/platform/build.sh SOURCE_COMMIT NEW_BUILD_DIRECTORY
python3 deploy/platform/install.py \
  --root "$HOME/agent-platform-8795" \
  --binary NEW_BUILD_DIRECTORY/agent-platform \
  --revision SOURCE_COMMIT \
  --listen 127.0.0.1:8795 --base-url http://127.0.0.1:8795 \
  --sandbox-config "$HOME/agent-platform-opensandbox/sandbox.toml" \
  --sandbox-image IMMUTABLE_IMAGE_ID --activate
```

`SOURCE_COMMIT`使用实际40位提交摘要；源码必须来自该提交的受信快照。
从SSH或受信CI传输后核对原始归档摘要，再解压和构建。`NEW_BUILD_DIRECTORY`
必须不存在；失败日志不覆盖。需要本机C编译器、Python3及可用的用户systemd。

默认监听回环。团队内网访问时显式指定服务器的私有IP和相同base URL；
当前安装入口不配置公网监听、TLS或反向代理，不将内部测试服务描述成公网生产部署。
OpenSandbox控制面及动态执行端口仍只监听回环。

安装器先核验二进制`--version`的精确revision、私有服务配置和不可变沙箱镜像，
再创建独立0700目录和数据。首次生成私有0600的`credentials.env`，只供运行用户
取得初始admin密码；安装输出不会包含密码。已有账号密码存于本实例数据中，
升级保留credentials、instance.json和data，不复制其他平台的数据或账号。

不同版本二进制保留在`releases/COMMIT/`，每个有实际文件摘要和版本manifest；
活动入口为受管理symlink。同提交不同字节、外部链接、服务配置改变均拒绝，
先检查现场再重新交付，不能覆盖旧证据。当前入口用于同配置二进制升级，
不自动迁移配置、不自动回滚数据库。

`--activate`验证服务归属和端口，生成/链接`agent-platform-PORT.service`并启动；
仅重启该实例。随后通过`/api/health`核对实际revision。核对失败保留原始服务日志，
不连续重启或冒称部署成功。未指定该参数只准备安装文件。

```sh
systemctl --user status agent-platform-8795.service
journalctl --user -u agent-platform-8795.service
curl --noproxy '*' --fail http://127.0.0.1:8795/api/health
```

用户服务在退出登录/主机重启后的持续运行取决于服务器的login/linger策略；
安装器不更改该策略。没有真实登出/重启验证时应单独列为未验证。

OpenSandbox的服务/执行可用性、原生角色会话和实际Pipeline是不同门槛；
健康页面证明平台与数据库工作，不证明模型登录或任务通过。当前沙箱Agent入口
仍在维护源门禁下；未完成原生执行能力验证前不会回退宿主或自动放宽容器权限。


已部署实例可通过正式API做基础链路检查：

```sh
python3 deploy/platform/check.py \
  --root "$HOME/agent-platform-8795" \
  --template examples/platform-workflows/opensandbox-runtime-check.json \
  --report NEW_PRIVATE_REPORT.jsonl
```

检查读取本实例私有初始凭据，仅在进程内登录；完整意图、API回执和原执行日志
写入独立0600报告。模板验证Python、TypeScript、任意项目命令、同Run共享文件、
重复请求去重、人工确认与环境释放；人工确认由基础回归操作者完成。它不执行模型
或产品验收。失败保持原Run、环境与报告，先核对现场，不能用另一报告名盲重试。
报告存在时拒绝覆盖；写请求发送结果未知也不重放。默认等待各阶段有界120秒。

`browser-check.cjs`只读检查实际画布和Run呈现，供已安装固定Playwright运行时使用；
通过stdin传入url/workflow/run和临时password，禁止把该输入写入日志、命令参数或
截图。输出含原始截图base64，须由受信调用者存入私有文件，只公开检查范围及摘要。
浏览器检查与命令执行、原生Agent及产品QA分别报告。


### 原生 Agent 的出网代理

任务环境需要代理访问模型服务时，在对应 Agent 节点的“高级：原生配置、Hook 与环境”中配置环境 JSON。例如：

```json
{
  "http_proxy": "http://<proxy-host>:<port>",
  "https_proxy": "http://<proxy-host>:<port>",
  "HTTP_PROXY": "http://<proxy-host>:<port>",
  "HTTPS_PROXY": "http://<proxy-host>:<port>",
  "no_proxy": "<platform-host>,127.0.0.1,localhost",
  "NO_PROXY": "<platform-host>,127.0.0.1,localhost"
}
```

使用已有受信代理，内部平台地址应直连，以便原生工作流工具返回平台。该配置只传入所选 Agent 的原生进程及其子命令，不配置所有执行节点或改宿主全局网络。保存后的配置作用于新 Run；已经开始的 Run 保持原定义，不通过改快照补环境。代理带私有认证时使用受信凭据配置，不将密码放在 Issue、任务参数或公开报告。

无凭据 HTTPS 收到响应只能证明连接可达；仍须通过实际模型轮次、工具调用、接续和停止回执核验完整链路。112 的同角色澄清接续验证结果见现有 opensandbox-workflow-plan.md，不能当作任意模型、供应商或产品验收通过。

### 执行步骤的私有环境配置

需要私有仓库或外部服务的执行节点，可在实例根目录配置可选的 `runtime.env`（运行用户所有、0600普通文件）。例如 `PROJECT_GITHUB_TOKEN=...`；不要将该文件提交到仓库。标准安装/升级验证其权限和条目，并添加受管理的可选 EnvironmentFile，省略文件不改变原执行行为。只自动升级精确匹配的原标准unit，外部修改需先处理冲突。

在画布执行节点的环境变量引用JSON填写 `{"GITHUB_TOKEN":"PROJECT_GITHUB_TOKEN"}`，只有该步骤得到指定值；Agent和其他节点不继承。任务参数不能修改该引用。缺失/空值会在派发前停止，执行日志对所选原值脱敏；项目脚本仍应避免把秘密放入命令参数、文件、Git remote或业务报告。管理员须按仓库范围提供最小凭据。HOME、PATH和平台控制变量不能由该入口覆盖。
