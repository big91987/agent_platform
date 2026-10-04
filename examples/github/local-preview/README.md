# 可选：合并 main 后部署到本机

这是静态 HTML/CSS/JS 产品的 macOS 预览部署示例，与 Agent 交接流程独立。合并到 `main` 后由 GitHub Actions 部署；也可手动运行工作流。不负责构建 Node 服务、容器或数据库迁移。

## 安装

需要同机自托管 Runner、已登录的 macOS 用户、Git、Node、Python 3.12+。先在宿主配置目标仓库的 Git 读取权限。安装器会创建独立产品 clone、复制部署控制器并安装用户级 LaunchAgent：

```sh
python3 examples/github/local-preview/scripts/install_local_preview.py \
  --repository owner/project-a \
  --root '<persistent-deployment-root>' \
  --port 5533
```

可用 `--git-proxy '<proxy-url>'` 配置 Git 网络代理。不同项目必须使用不同部署目录和端口；同一目录不允许改绑仓库或端口。安装不会切换产品版本，首次部署前访问页面返回未部署。服务在该 macOS 用户登录会话中运行。

将 `.github/workflows/deploy-local.yml` 复制到目标仓库同名位置，按宿主环境调整 `runs-on` 和 PATH。在目标仓库设置：

| 配置 | 用途 |
|---|---|
| Actions Variable `LOCAL_DEPLOY_ROOT` | 安装时的部署目录绝对路径 |
| Actions Variable `LOCAL_PREVIEW_URL` | 本机查看地址，例如 `http://127.0.0.1:5533/` |
| Environment `local-data-review` | **必须配置 Required reviewers**，用于需要审批的部署 |

只有创建 Environment 而没有配置审批人，不会形成审批拦截。若 GitHub 账户/仓库不支持此保护功能，不应直接使用该模板的审批路径。

## 每次发布

产品目录需有 `app/index.html`、`app/app.js`、`app/styles.css`。其他 `app/` 静态资源会一并发布。部署前执行 JS 语法检查及 `tests/*.test.cjs`。

产品提交中应包含覆盖当前 app 内容的 `deploy/release.json`。在完成数据兼容验证后生成并一同提交：

```sh
python3 '<platform-root>/examples/github/local-preview/scripts/local_deploy.py' declare \
  --repo '<product-worktree>' \
  --impact none \
  --compatible-from '<verified-current-deployed-commit-sha>' \
  --notes '描述实际验证过的数据兼容行为'
```

`compatible-from` 是确实验证过的已部署版本，不是随意填写的 main SHA。首次尚无已部署版本可不传。需要迁移或破坏性变更时，使用 `--impact migration` / `destructive`，并提供 `--migration-plan deploy/<plan>.md`；计划必须包含备份、操作、验证与恢复步骤。

工作流先准备精确 commit 的只读版本目录，然后：

- 内容指纹、数据声明和兼容基线均通过时，自动切换 `current` 并检查健康状态。
- 需要审批时，在该次 Actions Run 的 Summary 看原因和迁移计划，在 Environment 审批入口确认后继续。
- 健康检查失败会恢复上一个版本；不会删除持久数据。
- 手动运行勾选 `review` 可以强制审批。

**当前策略仍较保守**：除数据变化、声明缺失/过期、兼容基线不符外，变更涉及 `app/`、`docs/`、`tests/`、`deploy/` 及少量顶层文档之外的路径时也会要求审批。这是沿用的现有策略，不等于只拦截数据库变更；本次收集没有修订该策略。不要宣称所有 merge 都会直接部署。

### 连续合并与过期部署

同一仓库的部署使用同一个 concurrency group，`cancel-in-progress: true` 让新 Run 替换旧的准备或等待审批 Run。Actions 中仍会保留各次触发记录，不代表每条都实际部署。
控制器在准备和激活前重新读取 main：过期请求或已部署版本正常跳过，不再要求审批。
激活步骤用 `exec python3` 接收取消信号；SIGINT/SIGTERM 在切换期间触发恢复旧版本及部署记录，再退出。文件锁继续保证同一时刻只有一个版本切换。
进程被 SIGKILL 或机器断电不保证自动回滚；这是信号清理的限制，不得将其描述成跨崩溃事务。

连续合并时仍按实际线上版本检查数据兼容性，不能用最后一次提交的声明隐藏中间未部署的数据迁移。迁移、破坏性变更仍需审批最新计划。
正常合并的 QA/Review 由分支保护的 `pipeline/refresh` 控制；管理员手动绕过属于人工放行，不等于 QA 通过。本部署器仍只检查静态构建和数据声明，并未实现独立的部署前集成 QA 门禁。

升级顺序：先更新部署目录中的控制器，再合入新 YAML（新 YAML 依赖 `deploy` 输出）。否则不能启用取消策略。

## 数据与运行边界

同一协议、主机名、端口下的浏览器 localStorage 不因静态文件切换而清空。部署器不清理 `data/`，但也**不执行数据库迁移脚本**；有服务端存储的项目需要另行适配迁移和健康检查。迁移计划目前用于人工审查，不能把它当成自动迁移实现。

`http://127.0.0.1:5533/__deployment.json` 可查看已部署 SHA（端口随配置）。日志在部署目录 `logs/`，计划在 `plans/`，历史静态版本在 `releases/`。页面仅绑定 loopback。

控制器在安装时复制到运行目录，更新本示例后需以相同参数重跑安装器；更新 YAML 不会替换控制器。安装器重启预览服务，但不主动更新产品版本。运行中的 reading_list 部署没有被本次整理迁移。

## 来源与验证

基于 reading_list 的已使用部署实现，来源 commit 和原始文件 SHA-256 见 `SOURCE.json` 的 `upstream_files`。本目录作了仓库名、目录、端口与工作流变量参数化，文件不再与源快照逐字相同。源快照未附独立 LICENSE，未在此添加或推定新的许可；再分发须确认原仓库许可。

```sh
python3 -m unittest discover -s examples/github/local-preview/tests -p '*_test.py' -v
```
