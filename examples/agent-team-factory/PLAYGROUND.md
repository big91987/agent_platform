# Agent Team 开发实验台（单用户原型）

用户在页面提交需求。Leader通过工具创建任务、选择Skill、处理消息并接受成果；固定消费者自动领取可执行任务。没有research→PRD→开发等写死的阶段。新需求、Worker结果和发给Leader的消息使Leader接续原生会话。空闲时不调用模型；Worker是预先启动的消费者，每项任务独立原生会话和worktree，避免跨需求污染。

这是单机受信用户的可丢弃实验原型，不是多租户或无人值守生产平台。原生可见输出、工具调用、结果、输入和stderr留存于私有运行库，在页面按需求展示；没有返回的隐含思维不补造。

## 启动

沿用本目录README的固定原生CLI与Python/Git要求。运行库和产品仓库为独立目录。对用户提供的真正空仓，可以先执行一次标准bootstrap；它只写README、项目规则和忽略规则，不实施产品。

```sh
python3 playground.py --root /private/team-runtime bootstrap --repository /private/product-repo
python3 playground.py --root /private/team-runtime init \
  --repository /private/product-repo --codex /pinned/codex --workers 3 \
  --model gpt-5.6-sol --env-file /private/non-secret-env.json
python3 playground.py --root /private/team-runtime serve --port 8991
```

`non-secret-env.json`仅包含已准备好的PATH、GOTOOLCHAIN、GOWORK等执行环境，不放凭据。默认原生执行上限1800秒，可写Worker显式允许测试网络并保持工作区文件沙箱，Leader为只读仓库。权限配置沿用原生执行内核，只有本例绑定身份的团队工具预授权。

网页与执行API均只绑定loopback。当前112通过已有受信G112会话接入，不增加LAN或公网监听。先在专属G112 tmux窗口运行：

```sh
python3 team_relay.py --root /private/team-runtime --port 8991
```

在本机运行：

```sh
python3 team_proxy.py --tmux-target YOUR_DEDICATED_G112_PANE --port 8990
```

打开`http://127.0.0.1:8990/`。页面实时连接执行状态，不是快照。独立终端通道使用带请求标识的压缩分帧与逐帧确认，避免大输入或轨迹超过终端行/滚动缓冲限制。访问密钥只保留在执行机，代理检查本机Host/Origin，不进入浏览器或URL。未知POST回执不会自动重放。专属relay窗口不要混用Shell命令；连接中断时先核状态，不盲目重复操作。

## 使用

- 需求池启动为空。协调者不预填需求、不模拟用户、不自动导入Issue；由用户点击提交后才唤醒Leader。
- 点击顶部 Leader 或任一 Worker 卡片，进入成员详情：“执行轨迹”按轮次展示真实原生消息、工具输入输出、完整下发输入、实际启动配置与 stderr；“Skill 与工具”列出任务实际选择的 Skill、可选 Skill 正文及该角色团队工具的说明与 JSON Schema。Leader 不固定加载五份 Skill，而是在派发任务时选择 Worker 需要的一份。
- 点击需求可看原始输入、Leader回复、任务的Skill/依赖/结果、团队消息、该需求下的每轮轨迹和代码/产物。
- 页面刷新不并行堆积读取；已结束轮次在当前查看期间不重复拉取。原始事件保留，普通原生消息直接显示，工具参数与结果可展开。
- “暂停新任务”只暂停新派发和新Leader轮次，已经开始的执行继续；它不声称已取消进程。“继续Leader”用于明确处理执行错误后接续，不会重跑已接受任务。
- Worker提交后先机械保存候选；Leader显式`accept_task`后才合入本地实验分支并释放依赖。没有Issue/PR、远端推送或自动产品部署。
- Leader只有队列/通信/接受/完成工具，没有启动Worker的工具。Peer消息不作为用户授权，工具身份由原生启动参数绑定，不接受sender字段。
- 发生Git冲突时预检查拒绝并保持集成目录干净，Leader可以创建带原候选和当前版本的解决任务；未知集成仍按内核保留pending并阻止继续，不自动撤销或重放。
- 服务重启发现未结束轮次会拒绝。先核验原进程、会话和证据，再明确恢复；本原型尚未提供崩溃现场的自动修复工具。

## 维护与验证

唯一维护源在本目录。源码按固定提交归档，使用`install.py`的摘要校验安装到独立release目录，不在运行副本打补丁。旧批次运行库不迁移进本原型，也不重写历史。

`test_team.py`使用独立临时仓库和协议夹具验证任务领取、幂等、依赖、事件唤醒、身份边界、显式接受与HTTP鉴权。这些不是产品验收或真实模型团队跑通的证据。真实Leader派发、Worker工作与协作效果，等待用户从页面提交真实需求后观察，不以模拟轨迹冒称成功。
