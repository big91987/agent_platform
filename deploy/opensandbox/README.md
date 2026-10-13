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
