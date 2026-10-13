# OpenSandbox 隔离实例

用于当前开发分支的服务器部署。安装器固定 `opensandbox-server==1.1.1` 和
虚拟环境内 `pip==25.3`，记录实际依赖及安装报告。需要 Linux、Python 3.10+
（含 venv）和当前用户可访问的 Docker daemon；不改变系统 Python。

```sh
python3 deploy/opensandbox/install.py --root "$HOME/agent-platform-opensandbox" --port 8095
systemd-analyze --user verify "$HOME/agent-platform-opensandbox/agent-platform-opensandbox.service"
systemctl --user link "$HOME/agent-platform-opensandbox/agent-platform-opensandbox.service"
systemctl --user daemon-reload
systemctl --user enable --now agent-platform-opensandbox.service
curl --noproxy '*' --fail http://127.0.0.1:8095/health
```

目录为 0700，配置为 0600，首次生成 API key；再次安装保留配置及密钥。
改端口须显式修改配置，升级不会用命令行默认覆盖已有配置。安装路径暂不支持
空白、systemd 展开符或引号。服务及沙箱映射端口仅监听回环地址；平台应在同机
通过认证连接，不能把此 API 直接开放给 Issue 内容或 Agent。Docker bridge 的
动态端口范围为 45000–46000，保留 Docker 默认隔离，不挂载宿主目录。

升级前应先停止此控制面服务，保留配置、metadata 和现有 Docker 容器，再运行安装器、校验 unit 并启动；不要在控制面处理请求时替换其虚拟环境依赖。当前未验证在途任务的控制面升级恢复。

安装仅生成用户服务，不自动改变系统的 login/linger 策略。长期服务器部署须核验
用户服务在退出登录和重启后的生命周期，按服务器运维策略启用 linger 或配置系统服务。

真实生命周期探针（会拉取镜像并创建最多 600 秒、1 CPU/512Mi 的临时容器）：

```sh
"$HOME/agent-platform-opensandbox/venv/bin/python" deploy/opensandbox/probe.py \
  --config "$HOME/agent-platform-opensandbox/sandbox.toml" \
  --report "$HOME/agent-platform-opensandbox/probe-01.jsonl"
```

报告独占创建、逐事件落盘；同一报告不得重跑。创建响应丢失时通过报告中的
`agent-platform-probe` metadata 核查服务，不能直接重发创建。已知 sandbox ID
在退出时走正式 DELETE 并核验 404；失败报告和未知状态保留。此探针只证明
基础设施创建/观测/删除，不证明原生 Agent、浏览器或产品 Pipeline 验收。

官方依据：
[固定发行版](https://github.com/opensandbox-group/OpenSandbox/releases/tag/release-1.1.1)、
[服务配置](https://github.com/opensandbox-group/OpenSandbox/blob/release-1.1.1/server/configuration.md)。

平台同源命令／文件／取消探针：

```sh
go build -o bin/opensandbox-probe ./cmd/opensandbox-probe
bin/opensandbox-probe \
  --service-config "$HOME/agent-platform-opensandbox/sandbox.toml" \
  --image python:3.12-slim \
  --report "$HOME/agent-platform-opensandbox/sdk-probe-01.jsonl"
```

在运行服务的 Linux 主机执行；跨平台构建可用 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/opensandbox-probe-linux ./cmd/opensandbox-probe`。
探针通过平台固定官方 Go SDK v1.1.1 实际创建环境，执行 Python、上传/下载并
逐字节核验文件及实际0600权限，取消远端命令后独立查询停止状态，最后删除并
核对404。此探针使用2CPU/2Gi资源，环境TTL600秒，总执行有界。
配置须为运行用户所有的私有普通文件；不把控制面密钥传入沙箱。报告独占创建、
逐事件fsync，失败不覆盖、不自动重放创建或命令。已知ID只做归属核验及清理；
无ID或结果未知需保留报告后人工核查。探针成功仍不代表Run、原生Agent、浏览器
或产品验收通过。

固定execd v1.1.0要求shell `command`、`init.text`执行ID以及八进制数字形式的
上传权限。平台适配器在首次请求前引用字面参数和转换权限，保留原始执行事件；
这些版本兼容细节由平台承担，项目无需自行打补丁。服务升级需重新验证协议，
不在请求失败后自动改表示形式再试。

固定 Linux/amd64 任务运行镜像：

```sh
bash deploy/opensandbox/runtime/build.sh "$HOME/agent-platform-opensandbox/runtime-build-01"
cat "$HOME/agent-platform-opensandbox/runtime-build-01/image-id"
```

构建目录必须不存在；每次保留独立源码摘要、完整构建日志、实际镜像ID及运行检查。
基础Node/Go镜像固定到官方平台摘要，Codex/tsx/Playwright使用版本和npm锁文件。
镜像包含Python、TypeScript、Go、Git及Chromium，预建`/workspace`；其他项目工具和
依赖由项目自行准备，不能把此镜像描述成支持全部语言或项目。

构建只显式复用受信运行用户已有的HTTP/HTTPS/NO_PROXY配置（兼容大小写），使用
Docker预定义build args，不把代理写进最终镜像ENV。APT使用官方HTTPS源，索引更新
失败即停止；不绕过签名/证书检查或换host网络。最终无网络临时容器实际验证Python、
TypeScript与Go共享工作区文件、Chromium启动及Codex原生双向传输选项；此检查不复制
登录凭据、不调用模型、不做产品QA。运行容器自身的出网配置仍需由受信部署配置提供。

平台启动时`--sandbox-image`使用该次`image-id`的不可变值，而不是可变标签。
镜像更新只影响新Run；旧Run保持其原镜像/TTL和环境归属。环境释放会删除容器内
工作区与原生运行记录，先由流水线归档必要产物。默认TTL一小时并非无限保留保证，
长任务需在启动平台时显式选择合适TTL（当前上限24小时）。


完整运行镜像的SDK探针：

```sh
bin/opensandbox-probe --service-config "$HOME/agent-platform-opensandbox/sandbox.toml" \
  --image "$(cat "$HOME/agent-platform-opensandbox/runtime-build-01/image-id")" \
  --report "$HOME/agent-platform-opensandbox/runtime-sdk-probe-01.jsonl" --runtime
```

除前置Python/文件/取消门禁外，此模式用正式SDK实际执行TypeScript、启动原生
Codex app-server、以私有capability完成WebSocket initialize，在同一原生进程中发现两份无凭据Skill、禁用非所选项并重新读取确认只有所选项启用，再按所属执行ID
停止进程并删除环境。固定Docker端口映射使用8080；原生进程运行时项目服务选其他端口。
启动等待只在同一进程仍运行且没有HTTP响应的短暂连接错误时有界进行；HTTP认证拒绝
不会反复尝试，不发送模型turn、不复制登录凭据。报告写入或停止失败均使探针失败，
保留原回执与未确认原因。此探针不能代替平台Run、真实角色交接、登录或产品QA。

会话的文件列表、下载和HTML预览通过所属Run的OpenSandbox读取`/workspace`；
服务身份改变、环境未就绪或已经释放时不会回退到宿主同名路径。读取通过固定
Python目录描述符完成，拒绝链接、隐藏路径和越界路径；单文件上限8MiB，目录
扫描上限5000项/64层，读取30秒有界并沿正式执行ID取消。较大产物应由项目
流水线发布到既有交付入口。文件不可用会显示具体错误，会话与原生消息仍可查看。
环境释放或TTL过期会失去容器文件；此接口不是永久归档。交接文档、报告与需人审
产物应先经流水线归档并核验发布回执，再明确释放环境。

基础SDK探针也使用同一文件读取实现验证列表和逐字节结果，报告
`artifact_access_confirmed`；运行时模式另验证无凭据角色材料准备与接续复用。
这些探针没有启动正式平台角色会话，也没有发送模型任务。

### 当前原生命令隔离限制与待授权方案

固定运行镜像在112的SDK创建、Python/TypeScript、文件、取消，以及
Codex认证WebSocket initialize/停止/释放已实际通过。原生模型turn尚未执行。
在Docker默认过滤下，`codex sandbox linux`仍不能创建namespace；降为非root用户
也真实失败。112内核已允许user namespace，不应盲改宿主sysctl或反复重发命令。

固定OpenSandbox1.1.1有以下官方创建扩展：

```json
{"extensions":{"bootstrap.execd.isolation":"enable"}}
```

该扩展会为任务容器增加`SYS_ADMIN`，设置`apparmor=unconfined`和
`seccomp=unconfined`，并调整bwrap所需系统路径及临时存储。它扩大容器权限，
不是通常的工作区准备。**当前没有启用，也没有通过平台任务输入暴露。**

待审查的诊断范围仅为112独立8095实例的一次新无凭据测试容器；不更改宿主
sysctl，不用privileged/hostnetwork，不挂载宿主目录，不提供模型认证或产品凭据。
先验证原生workspace写入、边界拒绝和进程取消，再讨论正式受信配置及更窄的
AppArmor/seccomp策略。未经新增权限授权，保持原策略；不切成无限权限原生Agent
或复用旧Run来绕过限制。未来正式配置必须由运维入口选择、在新Run中冻结并展示，
保持旧Run的原权限，不依赖Agent解释Issue正文自行授权。
