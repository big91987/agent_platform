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
