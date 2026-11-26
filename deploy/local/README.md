# 本机独立平台服务（macOS）

将平台二进制及启动器安装到新的私有目录，由当前用户的 launchd 运行；仅监听 loopback。已有服务、数据库和原生会话不复用。GitHub 自托管 Runner 必须在同一台机器运行，Issue 入口使用本机 URL；托管 Runner 不能直接访问此地址。

先构建指定开发分支版本，再执行：

```sh
python3 deploy/local/install.py --root /absolute/private/instance --binary /absolute/agent-platform --port 8797 --claude /opt/homebrew/bin/claude --claude-config-home /absolute/.claude --activate
```

`credentials.json` 为0600服务端环境变量映射，默认空；如需 GitHub 发布或审查模型凭据，由操作者写入该文件，不能进入图、Issue、仓库或日志。CC 的供应商连接从 `--claude-config-home` 中按执行器白名单读取；项目和角色权限由平台配置。

安装器拒绝占用端口、现存实例和同名 LaunchAgent，避免覆盖旧服务。本次提供新实例安装入口，已有实例升级尚未提供自动化命令；不要删除数据来强行重装。安装后核对 `/api/health` 与二进制版本、实际原生会话及角色交接后才投递产品任务。

服务标识 `com.agent-platform.local.<port>`。停止使用 `launchctl bootout gui/<uid>/com.agent-platform.local.<port>`；保留实例目录即可保留历史。启动用 `launchctl bootstrap gui/<uid> ~/Library/LaunchAgents/com.agent-platform.local.<port>.plist`。不要把服务目录或凭据提交到 Git。
